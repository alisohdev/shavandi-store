package postgres

import (
	"context"
	"errors"
	"math"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shavandi/store/backend/internal/cart"
)

type CartStore struct{ pool *pgxpool.Pool }

func NewCartStore(pool *pgxpool.Pool) *CartStore { return &CartStore{pool: pool} }

// Set uses an absolute quantity: retrying the same request cannot add twice.
// Locking the customer row serializes per-account line-limit checks.
func (s *CartStore) Set(ctx context.Context, userID, variantID string, quantity int) error {
	var id pgtype.UUID
	if id.Scan(variantID) != nil || !id.Valid || quantity < 0 || quantity > 9 {
		return cart.ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var user string
	if err = tx.QueryRow(ctx, `SELECT id::text FROM shavandi.users WHERE id=$1::uuid AND active FOR UPDATE`, userID).Scan(&user); err != nil {
		return err
	}
	if quantity == 0 {
		_, err = tx.Exec(ctx, `DELETE FROM shavandi.cart_lines WHERE user_id=$1::uuid AND variant_id=$2::uuid`, userID, variantID)
	} else {
		var available bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM shavandi.variants v JOIN shavandi.products p ON p.id=v.product_id JOIN shavandi.effective_active_categories c ON c.id=p.primary_category_id WHERE v.id=$1::uuid AND v.active AND p.publication_state='published')`, variantID).Scan(&available)
		if err != nil {
			return err
		}
		if !available {
			return cart.ErrUnavailable
		}
		var count int
		var exists bool
		err = tx.QueryRow(ctx, `SELECT count(*),coalesce(bool_or(variant_id=$2::uuid),false) FROM shavandi.cart_lines WHERE user_id=$1::uuid`, userID, variantID).Scan(&count, &exists)
		if err != nil {
			return err
		}
		if count >= 50 && !exists {
			return cart.ErrLimit
		}
		_, err = tx.Exec(ctx, `INSERT INTO shavandi.cart_lines(user_id,variant_id,quantity) VALUES($1::uuid,$2::uuid,$3) ON CONFLICT(user_id,variant_id) DO UPDATE SET quantity=excluded.quantity,updated_at=now()`, userID, variantID, quantity)
	}
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *CartStore) Get(ctx context.Context, userID string) (cart.Snapshot, error) {
	result := cart.Snapshot{Lines: []cart.Line{}}
	rows, err := s.pool.Query(ctx, `SELECT v.id::text,sl.slug,p.name,v.size,v.color,l.quantity,
 (v.active AND p.publication_state='published' AND c.id IS NOT NULL),
 coalesce(v.sale_price_override_toman, CASE WHEN v.regular_price_override_toman IS NULL THEN p.sale_price_toman END, v.regular_price_override_toman,p.regular_price_toman)
 FROM shavandi.cart_lines l JOIN shavandi.variants v ON v.id=l.variant_id JOIN shavandi.products p ON p.id=v.product_id
 JOIN shavandi.product_slugs sl ON sl.product_id=p.id AND sl.is_current
 LEFT JOIN shavandi.effective_active_categories c ON c.id=p.primary_category_id
 WHERE l.user_id=$1::uuid ORDER BY l.updated_at,v.id`, userID)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var line cart.Line
		var unit int64
		if err = rows.Scan(&line.VariantID, &line.ProductSlug, &line.Name, &line.Size, &line.Color, &line.Quantity, &line.Available, &unit); err != nil {
			return result, err
		}
		if line.Available {
			if unit < 0 || unit > math.MaxInt64/int64(line.Quantity) {
				return result, errors.New("invalid cart price")
			}
			total := unit * int64(line.Quantity)
			// JSON prices must remain exact in browser integer arithmetic.
			if total > 9007199254740991-result.SubtotalToman {
				return result, errors.New("cart total exceeds safe integer")
			}
			line.UnitPriceToman = &unit
			line.LineTotalToman = &total
			result.SubtotalToman += total
		} else {
			result.HasUnavailable = true
		}
		result.Lines = append(result.Lines, line)
	}
	return result, rows.Err()
}
