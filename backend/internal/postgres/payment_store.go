package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shavandi/store/backend/internal/orders"
	"github.com/shavandi/store/backend/internal/payments"
)

type PaymentStore struct{ pool *pgxpool.Pool }

func NewPaymentStore(pool *pgxpool.Pool) *PaymentStore { return &PaymentStore{pool: pool} }

const paymentColumns = `id::text,order_id::text,coalesce(authority,''),status,environment,merchant_fingerprint,amount_rial,ref_id`

func scanPayment(row pgx.Row) (payments.Attempt, error) {
	var p payments.Attempt
	err := row.Scan(&p.ID, &p.OrderID, &p.Authority, &p.Status, &p.Environment, &p.MerchantFingerprint, &p.AmountRial, &p.RefID)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, payments.ErrNotFound
	}
	return p, err
}
func (s *PaymentStore) Begin(ctx context.Context, user, id string, version, total int64, environment, merchant string) (payments.Attempt, bool, error) {
	if !validOrderID(id) || version < 1 || total <= 0 || total > 9007199254740991 {
		return payments.Attempt{}, false, payments.ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return payments.Attempt{}, false, err
	}
	defer tx.Rollback(ctx)
	if err = lockCatalogMutations(ctx, tx); err != nil {
		return payments.Attempt{}, false, err
	}
	order, err := readOrder(ctx, tx, user, id)
	if errors.Is(err, orders.ErrNotFound) {
		return payments.Attempt{}, false, payments.ErrNotFound
	}
	if err != nil {
		return payments.Attempt{}, false, err
	}
	var unexpired bool
	if err = tx.QueryRow(ctx, `SELECT expires_at>clock_timestamp() FROM shavandi.orders WHERE id=$1::uuid FOR UPDATE`, id).Scan(&unexpired); err != nil {
		return payments.Attempt{}, false, err
	}
	if order.Status != "pending_confirmation" || !unexpired || order.Delivery == nil || order.Delivery.ShippingMethodID == nil || order.Delivery.ShippingFeeToman == nil || order.Delivery.Version != version || order.SubtotalToman+*order.Delivery.ShippingFeeToman != total {
		return payments.Attempt{}, false, payments.ErrConflict
	}
	existing, e := scanPayment(tx.QueryRow(ctx, `SELECT `+paymentColumns+` FROM shavandi.order_payments WHERE order_id=$1::uuid`, id))
	if e == nil {
		if existing.Environment != environment || existing.MerchantFingerprint != merchant {
			return existing, false, payments.ErrConflict
		}
		return existing, false, nil
	}
	if !errors.Is(e, payments.ErrNotFound) {
		return existing, false, e
	}
	attempt, err := scanPayment(tx.QueryRow(ctx, `INSERT INTO shavandi.order_payments(order_id,environment,merchant_fingerprint,amount_rial,delivery_version,status) VALUES($1::uuid,$2,$3,$4,$5,'requesting') RETURNING `+paymentColumns, id, environment, merchant, total*10, version))
	if err != nil {
		return attempt, false, err
	}
	err = tx.Commit(ctx)
	return attempt, true, err
}
func (s *PaymentStore) Bind(ctx context.Context, id, authority string) error {
	result, err := s.pool.Exec(ctx, `UPDATE shavandi.order_payments SET authority=$2,status='ready' WHERE id=$1::uuid AND status='requesting'`, id, authority)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return payments.ErrConflict
	}
	return nil
}
func (s *PaymentStore) RequestFailed(ctx context.Context, id string, unknown bool) error {
	status := "failed"
	if unknown {
		status = "unknown"
	}
	_, err := s.pool.Exec(ctx, `UPDATE shavandi.order_payments SET status=$2 WHERE id=$1::uuid AND status='requesting'`, id, status)
	return err
}
func (s *PaymentStore) ByAuthority(ctx context.Context, authority string) (payments.Attempt, error) {
	if len(authority) != 36 {
		return payments.Attempt{}, payments.ErrNotFound
	}
	return scanPayment(s.pool.QueryRow(ctx, `SELECT `+paymentColumns+` FROM shavandi.order_payments WHERE authority=$1`, authority))
}
func (s *PaymentStore) ByOrder(ctx context.Context, user, id string) (payments.Attempt, error) {
	if !validOrderID(id) {
		return payments.Attempt{}, payments.ErrNotFound
	}
	return scanPayment(s.pool.QueryRow(ctx, `SELECT `+paymentColumns+` FROM shavandi.order_payments WHERE order_id IN (SELECT id FROM shavandi.orders WHERE id=$1::uuid AND user_id=$2::uuid)`, id, user))
}
func (s *PaymentStore) Claim(ctx context.Context, id string) (bool, error) {
	result, err := s.pool.Exec(ctx, `UPDATE shavandi.order_payments SET busy_until=now()+interval '30 seconds' WHERE id=$1::uuid AND status='ready' AND (busy_until IS NULL OR busy_until<=now())`, id)
	return err == nil && result.RowsAffected() == 1, err
}
func (s *PaymentStore) ReleaseClaim(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `UPDATE shavandi.order_payments SET busy_until=now()+interval '5 seconds' WHERE id=$1::uuid AND status='ready'`, id)
	return err
}
func (s *PaymentStore) Settle(ctx context.Context, id string, ref int64) error {
	if ref <= 0 {
		return payments.ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = lockCatalogMutations(ctx, tx); err != nil {
		return err
	}
	attempt, err := scanPayment(tx.QueryRow(ctx, `SELECT `+paymentColumns+` FROM shavandi.order_payments WHERE id=$1::uuid FOR UPDATE`, id))
	if err != nil {
		return err
	}
	if attempt.Status == "succeeded" {
		if attempt.RefID == nil || *attempt.RefID != ref {
			return payments.ErrConflict
		}
		return nil
	}
	if attempt.Status != "ready" {
		return payments.ErrConflict
	}
	var status string
	var unexpired bool
	if err = tx.QueryRow(ctx, `SELECT status,expires_at>clock_timestamp() FROM shavandi.orders WHERE id=$1::uuid FOR UPDATE`, attempt.OrderID).Scan(&status, &unexpired); err != nil {
		return err
	}
	next := "payment_review"
	if status == "pending_confirmation" && unexpired {
		_, err = tx.Exec(ctx, `UPDATE shavandi.variants v SET stock_on_hand=v.stock_on_hand-l.quantity,reserved_quantity=v.reserved_quantity-l.quantity,stock_version=stock_version+1 FROM shavandi.order_lines l WHERE l.order_id=$1::uuid AND l.variant_id=v.id`, attempt.OrderID)
		if err != nil {
			return err
		}
		next = "paid"
	} else if status == "pending_confirmation" {
		if err = releaseOrder(ctx, tx, attempt.OrderID, "expired"); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `UPDATE shavandi.order_payments SET status='succeeded',ref_id=$2,busy_until=NULL WHERE id=$1::uuid`, id, ref)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE shavandi.orders SET status=$2 WHERE id=$1::uuid`, attempt.OrderID, next)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
