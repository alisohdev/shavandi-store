//go:build integration

package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/shavandi/store/backend/internal/cart"
	"github.com/shavandi/store/backend/internal/catalog"
)

func TestCartPersistenceIsolationRepricingAndVisibility(t *testing.T) {
	pool, _ := openFreshIntegrationDatabase(t)
	ctx := context.Background()
	store := NewCartStore(pool)
	var first, second string
	if err := pool.QueryRow(ctx, `INSERT INTO shavandi.users(phone,role) VALUES('+989111111111','customer') RETURNING id::text`).Scan(&first); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO shavandi.users(phone,role) VALUES('+989222222222','customer') RETURNING id::text`).Scan(&second); err != nil {
		t.Fatal(err)
	}
	svc := catalog.NewService(NewCatalogRepository(pool))
	category, err := svc.CreateCategory(ctx, catalog.Category{Name: "Cart test", Slug: "cart-test", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	product, err := svc.CreateProductDraft(ctx, catalog.Product{Name: "Cart product", Slug: "cart-product", Description: "Details", PrimaryCategory: category.ID, RegularPriceToman: 1000, Images: []catalog.Image{{AssetRef: "/images/test.png"}}})
	if err != nil {
		t.Fatal(err)
	}
	variant := string(product.Variants[0].ID)
	if _, err = pool.Exec(ctx, `UPDATE shavandi.variants SET stock_on_hand=9 WHERE id=$1::uuid`, variant); err != nil {
		t.Fatal(err)
	}
	if err = store.Set(ctx, first, variant, 2); !errors.Is(err, cart.ErrUnavailable) {
		t.Fatalf("draft accepted: %v", err)
	}
	product, err = svc.ChangePublication(ctx, product.ID, product.Version, catalog.Published)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if e := store.Set(ctx, first, variant, 2); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	snapshot, err := NewCartStore(pool).Get(ctx, first)
	if err != nil || len(snapshot.Lines) != 1 || snapshot.SubtotalToman != 2000 {
		t.Fatalf("persisted cart: %+v %v", snapshot, err)
	}
	other, err := store.Get(ctx, second)
	if err != nil || len(other.Lines) != 0 {
		t.Fatalf("account isolation: %+v %v", other, err)
	}
	if err = store.Set(ctx, second, variant, 1); err != nil {
		t.Fatal(err)
	}
	if err = store.Set(ctx, second, variant, 0); err != nil {
		t.Fatal(err)
	}
	snapshot, err = store.Get(ctx, first)
	if err != nil || len(snapshot.Lines) != 1 {
		t.Fatal("other account removed customer line")
	}
	if _, err = pool.Exec(ctx, `UPDATE shavandi.products SET regular_price_toman=1500 WHERE id=$1::uuid`, string(product.ID)); err != nil {
		t.Fatal(err)
	}
	snapshot, err = store.Get(ctx, first)
	if err != nil || snapshot.SubtotalToman != 3000 {
		t.Fatalf("not repriced: %+v %v", snapshot, err)
	}
	if _, err = svc.ChangePublication(ctx, product.ID, product.Version, catalog.Hidden); err != nil {
		t.Fatal(err)
	}
	snapshot, err = store.Get(ctx, first)
	if err != nil || !snapshot.HasUnavailable || snapshot.SubtotalToman != 0 || snapshot.Lines[0].UnitPriceToman != nil {
		t.Fatalf("hidden price included: %+v %v", snapshot, err)
	}
	if err = store.Set(ctx, first, variant, 3); !errors.Is(err, cart.ErrUnavailable) {
		t.Fatalf("hidden update accepted: %v", err)
	}
	for _, quantity := range []int{-1, 10} {
		if err = store.Set(ctx, first, variant, quantity); !errors.Is(err, cart.ErrInvalid) {
			t.Fatalf("quantity accepted: %d %v", quantity, err)
		}
	}
	if err = store.Set(ctx, first, "not-a-uuid", 1); !errors.Is(err, cart.ErrInvalid) {
		t.Fatal("malformed variant accepted")
	}
	if err = store.Set(ctx, first, variant, 0); err != nil {
		t.Fatal(err)
	}
	snapshot, err = store.Get(ctx, first)
	if err != nil || len(snapshot.Lines) != 0 {
		t.Fatalf("remove unavailable: %+v %v", snapshot, err)
	}
	// Fill fifty distinct lines, then prove existing lines remain editable at the cap.
	if _, err = pool.Exec(ctx, `UPDATE shavandi.products SET publication_state='published' WHERE id=$1::uuid`, string(product.ID)); err != nil {
		t.Fatal(err)
	}
	rows, err := pool.Query(ctx, `INSERT INTO shavandi.variants(id,product_id,sku,size,active,stock_on_hand) SELECT gen_random_uuid(),$1::uuid,'cart-limit-'||n,n::text,true,9 FROM generate_series(1,51) n RETURNING id::text`, string(product.ID))
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	for _, id := range ids[:50] {
		if err = store.Set(ctx, first, id, 1); err != nil {
			t.Fatal(err)
		}
	}
	if err = store.Set(ctx, first, ids[50], 1); !errors.Is(err, cart.ErrLimit) {
		t.Fatalf("line limit bypass: %v", err)
	}
	if err = store.Set(ctx, first, ids[0], 9); err != nil {
		t.Fatal(err)
	}

}
