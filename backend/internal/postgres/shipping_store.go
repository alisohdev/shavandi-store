package postgres

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shavandi/store/backend/internal/catalog"
	"github.com/shavandi/store/backend/internal/shipping"
	"strings"
	"unicode/utf8"
)

type ShippingStore struct{ pool *pgxpool.Pool }

func NewShippingStore(pool *pgxpool.Pool) *ShippingStore { return &ShippingStore{pool: pool} }
func (s *ShippingStore) List(ctx context.Context, all bool) ([]shipping.Method, error) {
	rows, err := s.pool.Query(ctx, `SELECT id::text,name,fee_toman,active,version FROM shavandi.shipping_methods WHERE $1 OR active ORDER BY name,id`, all)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []shipping.Method{}
	for rows.Next() {
		var method shipping.Method
		if err = rows.Scan(&method.ID, &method.Name, &method.FeeToman, &method.Active, &method.Version); err != nil {
			return nil, err
		}
		result = append(result, method)
	}
	return result, rows.Err()
}
func (s *ShippingStore) Save(ctx context.Context, method shipping.Method) (shipping.Method, error) {
	method.Name = strings.TrimSpace(method.Name)
	if utf8.RuneCountInString(method.Name) < 2 || utf8.RuneCountInString(method.Name) > 100 || method.FeeToman < 0 || method.FeeToman > 1000000000 || (method.ID != "" && (!validOrderID(method.ID) || method.Version < 1)) || (method.ID == "" && method.Version != 0) {
		return method, catalog.ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return method, err
	}
	defer tx.Rollback(ctx)
	if err = lockCatalogMutations(ctx, tx); err != nil {
		return method, err
	}
	if method.ID == "" {
		var count int
		if err = tx.QueryRow(ctx, `SELECT count(*) FROM shavandi.shipping_methods`).Scan(&count); err != nil {
			return method, err
		}
		if count >= 50 {
			return method, catalog.ErrInvalid
		}
		err = tx.QueryRow(ctx, `INSERT INTO shavandi.shipping_methods(name,fee_toman,active) VALUES($1,$2,$3) RETURNING id::text,version`, method.Name, method.FeeToman, method.Active).Scan(&method.ID, &method.Version)
	} else {
		result, e := tx.Exec(ctx, `UPDATE shavandi.shipping_methods SET name=$2,fee_toman=$3,active=$4,version=version+1 WHERE id=$1::uuid AND version=$5`, method.ID, method.Name, method.FeeToman, method.Active, method.Version)
		if e != nil {
			return method, e
		}
		if result.RowsAffected() != 1 {
			return method, catalog.ErrVersionConflict
		}
		method.Version++
	}
	if err != nil {
		return method, err
	}
	err = tx.Commit(ctx)
	return method, err
}
