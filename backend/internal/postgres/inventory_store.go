package postgres

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shavandi/store/backend/internal/catalog"
)

type InventoryStore struct{ pool *pgxpool.Pool }

func NewInventoryStore(pool *pgxpool.Pool) *InventoryStore { return &InventoryStore{pool: pool} }
func (s *InventoryStore) SetStock(ctx context.Context, productID, variantID, actorID string, version int64, quantity int, reason string) error {
	var id pgtype.UUID
	if id.Scan(productID) != nil || id.Scan(variantID) != nil || quantity < 0 || quantity > 1000000 || version < 1 || strings.TrimSpace(reason) == "" || utf8.RuneCountInString(reason) > 300 {
		return catalog.ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = lockCatalogMutations(ctx, tx); err != nil {
		return err
	}
	var actor string
	if err = tx.QueryRow(ctx, `SELECT id::text FROM shavandi.users WHERE id=$1::uuid AND active AND role='admin' FOR SHARE`, actorID).Scan(&actor); err != nil {
		return err
	}
	var reserved int
	var previous int
	var current int64
	err = tx.QueryRow(ctx, `SELECT stock_on_hand,stock_version,reserved_quantity FROM shavandi.variants WHERE id=$1::uuid AND product_id=$2::uuid FOR UPDATE`, variantID, productID).Scan(&previous, &current, &reserved)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.ErrNotFound
	}
	if err != nil {
		return err
	}
	if quantity < reserved {
		return catalog.ErrInvalid
	}
	if current != version {
		return catalog.ErrVersionConflict
	}
	_, err = tx.Exec(ctx, `UPDATE shavandi.variants SET stock_on_hand=$2,stock_version=stock_version+1,updated_at=now() WHERE id=$1::uuid`, variantID, quantity)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO shavandi.stock_changes(variant_id,actor_id,previous_quantity,new_quantity,version,reason) VALUES($1::uuid,$2::uuid,$3,$4,$5,$6)`, variantID, actorID, previous, quantity, version+1, strings.TrimSpace(reason))
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
