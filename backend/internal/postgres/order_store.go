package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shavandi/store/backend/internal/orders"
)

type OrderStore struct{ pool *pgxpool.Pool }

func NewOrderStore(pool *pgxpool.Pool) *OrderStore { return &OrderStore{pool: pool} }

var quotePattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func validOrderID(value string) bool { var id pgtype.UUID; return id.Scan(value) == nil && id.Valid }

func (s *OrderStore) Create(ctx context.Context, userID, key, quote string) (orders.Order, error) {
	if !validOrderID(key) || !quotePattern.MatchString(quote) {
		return orders.Order{}, orders.ErrInvalid
	}
	if err := s.Expire(ctx); err != nil {
		return orders.Order{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return orders.Order{}, err
	}
	defer tx.Rollback(ctx)
	if err = lockCatalogMutations(ctx, tx); err != nil {
		return orders.Order{}, err
	}
	var owner string
	if err = tx.QueryRow(ctx, `SELECT id::text FROM shavandi.users WHERE id=$1::uuid AND active FOR UPDATE`, userID).Scan(&owner); err != nil {
		return orders.Order{}, err
	}
	var existing, previousQuote string
	err = tx.QueryRow(ctx, `SELECT id::text,quote_token FROM shavandi.orders WHERE user_id=$1::uuid AND request_key=$2::uuid`, userID, key).Scan(&existing, &previousQuote)
	if err == nil {
		if quote != previousQuote {
			return orders.Order{}, orders.ErrConflict
		}
		return readOrder(ctx, tx, userID, existing)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return orders.Order{}, err
	}
	var pending bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM shavandi.orders WHERE user_id=$1::uuid AND status='pending_confirmation')`, userID).Scan(&pending); err != nil {
		return orders.Order{}, err
	}
	if pending {
		return orders.Order{}, orders.ErrConflict
	}
	cart, err := readCart(ctx, tx, userID)
	if err != nil {
		return orders.Order{}, err
	}
	if len(cart.Lines) == 0 || cart.HasUnavailable || cart.QuoteToken != quote {
		return orders.Order{}, orders.ErrConflict
	}
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO shavandi.orders(user_id,request_key,quote_token,status,subtotal_toman) VALUES($1::uuid,$2::uuid,$3,'pending_confirmation',$4) RETURNING id::text`, userID, key, quote, cart.SubtotalToman).Scan(&id)
	if err != nil {
		return orders.Order{}, err
	}
	for _, line := range cart.Lines {
		result, e := tx.Exec(ctx, `UPDATE shavandi.variants SET reserved_quantity=reserved_quantity+$2,stock_version=stock_version+1 WHERE id=$1::uuid AND stock_on_hand-reserved_quantity >= $2`, line.VariantID, line.Quantity)
		if e != nil {
			return orders.Order{}, e
		}
		if result.RowsAffected() != 1 {
			return orders.Order{}, orders.ErrConflict
		}
		_, e = tx.Exec(ctx, `INSERT INTO shavandi.order_lines(order_id,variant_id,name,size,color,quantity,unit_price_toman) VALUES($1::uuid,$2::uuid,$3,$4,$5,$6,$7)`, id, line.VariantID, line.Name, line.Size, line.Color, line.Quantity, *line.UnitPriceToman)
		if e != nil {
			return orders.Order{}, e
		}
	}
	if _, err = tx.Exec(ctx, `DELETE FROM shavandi.cart_lines WHERE user_id=$1::uuid`, userID); err != nil {
		return orders.Order{}, err
	}
	order, err := readOrder(ctx, tx, userID, id)
	if err != nil {
		return order, err
	}
	err = tx.Commit(ctx)
	return order, err
}

func readOrder(ctx context.Context, query interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, userID, id string) (orders.Order, error) {
	var order orders.Order
	var lines, delivery []byte
	err := query.QueryRow(ctx, `SELECT o.id::text,o.status,o.subtotal_toman,o.created_at,o.expires_at,
 coalesce((SELECT jsonb_agg(jsonb_build_object('variant_id',l.variant_id::text,'name',l.name,'size',l.size,'color',l.color,'quantity',l.quantity,'unit_price_toman',l.unit_price_toman) ORDER BY l.variant_id) FROM shavandi.order_lines l WHERE l.order_id=o.id),'[]'::jsonb)
 , coalesce((SELECT jsonb_build_object('recipient',d.recipient,'phone',d.phone,'province',d.province,'city',d.city,'address',d.address,'postal_code',d.postal_code,'version',d.version,'shipping_method_id',d.shipping_method_id,'shipping_version',d.shipping_version,'shipping_name',d.shipping_name,'shipping_fee_toman',d.shipping_fee_toman) FROM shavandi.order_deliveries d WHERE d.order_id=o.id),'null'::jsonb)
 , coalesce((SELECT status FROM shavandi.order_payments WHERE order_id=o.id),'none'),(SELECT ref_id::text FROM shavandi.order_payments WHERE order_id=o.id),coalesce((SELECT environment FROM shavandi.order_payments WHERE order_id=o.id),'')
 FROM shavandi.orders o WHERE o.id=$1::uuid AND o.user_id=$2::uuid`, id, userID).Scan(&order.ID, &order.Status, &order.SubtotalToman, &order.CreatedAt, &order.ExpiresAt, &lines, &delivery, &order.PaymentStatus, &order.PaymentReference, &order.PaymentEnvironment)
	if errors.Is(err, pgx.ErrNoRows) {
		return order, orders.ErrNotFound
	}
	if err != nil {
		return order, err
	}
	order.Delivery, err = deliveryJSON(delivery)
	if err != nil {
		return order, err
	}
	err = json.Unmarshal(lines, &order.Lines)
	return order, err
}

// Release and status transition share one transaction. Only pending orders release stock.
func releaseOrder(ctx context.Context, tx pgx.Tx, id, status string) error {
	var pending bool
	if err := tx.QueryRow(ctx, `SELECT status='pending_confirmation' FROM shavandi.orders WHERE id=$1::uuid FOR UPDATE`, id).Scan(&pending); err != nil {
		return err
	}
	if !pending {
		return nil
	}
	_, err := tx.Exec(ctx, `UPDATE shavandi.variants v SET reserved_quantity=v.reserved_quantity-l.quantity,stock_version=stock_version+1 FROM shavandi.order_lines l WHERE l.order_id=$1::uuid AND l.variant_id=v.id`, id)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE shavandi.orders SET status=$2 WHERE id=$1::uuid`, id, status)
	return err
}
func (s *OrderStore) Expire(ctx context.Context) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = lockCatalogMutations(ctx, tx); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT id::text FROM shavandi.orders WHERE status='pending_confirmation' AND expires_at<=now() ORDER BY expires_at LIMIT 100 FOR UPDATE`)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err = releaseOrder(ctx, tx, id, "expired"); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (s *OrderStore) List(ctx context.Context, userID string) ([]orders.Order, error) {
	if err := s.Expire(ctx); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text FROM shavandi.orders WHERE user_id=$1::uuid ORDER BY created_at DESC,id LIMIT 20`, userID)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	result := []orders.Order{}
	for _, id := range ids {
		order, e := readOrder(ctx, s.pool, userID, id)
		if e != nil {
			return nil, e
		}
		result = append(result, order)
	}
	return result, nil
}
func (s *OrderStore) Cancel(ctx context.Context, userID, id string) (orders.Order, error) {
	if !validOrderID(id) {
		return orders.Order{}, orders.ErrNotFound
	}
	if err := s.Expire(ctx); err != nil {
		return orders.Order{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return orders.Order{}, err
	}
	defer tx.Rollback(ctx)
	if err = lockCatalogMutations(ctx, tx); err != nil {
		return orders.Order{}, err
	}
	if _, err = readOrder(ctx, tx, userID, id); err != nil {
		return orders.Order{}, err
	}
	var inProgress bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM shavandi.order_payments WHERE order_id=$1::uuid AND status IN ('requesting','ready','unknown'))`, id).Scan(&inProgress); err != nil {
		return orders.Order{}, err
	}
	if inProgress {
		return orders.Order{}, orders.ErrConflict
	}
	if err = releaseOrder(ctx, tx, id, "cancelled"); err != nil {
		return orders.Order{}, err
	}
	order, err := readOrder(ctx, tx, userID, id)
	if err != nil {
		return order, err
	}
	err = tx.Commit(ctx)
	return order, err
}
