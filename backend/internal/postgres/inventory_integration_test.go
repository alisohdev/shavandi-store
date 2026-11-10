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

func TestInventoryVersionAuditAndCartStockChecks(t *testing.T) {
	pool, _ := openFreshIntegrationDatabase(t)
	ctx := context.Background()
	inventory := NewInventoryStore(pool)
	carts := NewCartStore(pool)
	var admin, customer string
	if err := pool.QueryRow(ctx, `INSERT INTO shavandi.users(phone,role,password_hash) VALUES('+989333333333','admin','$argon2id$v=19$test') RETURNING id::text`).Scan(&admin); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO shavandi.users(phone,role) VALUES('+989444444444','customer') RETURNING id::text`).Scan(&customer); err != nil {
		t.Fatal(err)
	}
	svc := catalog.NewService(NewCatalogRepository(pool))
	category, err := svc.CreateCategory(ctx, catalog.Category{Name: "Inventory", Slug: "inventory", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	p, err := svc.CreateProductDraft(ctx, catalog.Product{Name: "Stock product", Slug: "stock-product", Description: "Details", PrimaryCategory: category.ID, RegularPriceToman: 1000, Images: []catalog.Image{{AssetRef: "/images/test.png"}}})
	if err != nil {
		t.Fatal(err)
	}
	v := p.Variants[0]
	if v.StockOnHand != 0 || v.StockVersion != 1 {
		t.Fatalf("initial stock: %+v", v)
	}
	p, err = svc.ChangePublication(ctx, p.ID, p.Version, catalog.Published)
	if err != nil {
		t.Fatal(err)
	}
	if err = carts.Set(ctx, customer, string(v.ID), 1); !errors.Is(err, cart.ErrUnavailable) {
		t.Fatalf("zero stock accepted: %v", err)
	}
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, quantity := range []int{2, 3} {
		wg.Add(1)
		go func(q int) {
			defer wg.Done()
			results <- inventory.SetStock(ctx, string(p.ID), string(v.ID), admin, 1, q, "Initial count")
		}(quantity)
	}
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else if errors.Is(e, catalog.ErrVersionConflict) {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("concurrent stock updates: %d %d", success, conflicts)
	}
	var audits int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM shavandi.stock_changes WHERE variant_id=$1::uuid AND actor_id=$2::uuid AND previous_quantity=0 AND version=2`, string(v.ID), admin).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("audit: %d %v", audits, err)
	}
	public, err := svc.GetPublishedProductBySlug(ctx, p.Slug)
	if err != nil || public.Variants[0].AvailableQuantity < 2 {
		t.Fatalf("public stock: %+v %v", public, err)
	}
	if err = carts.Set(ctx, customer, string(v.ID), 4); !errors.Is(err, cart.ErrUnavailable) {
		t.Fatal("oversized cart accepted")
	}
	if err = carts.Set(ctx, customer, string(v.ID), 2); err != nil {
		t.Fatal(err)
	}
	if err = inventory.SetStock(ctx, string(p.ID), string(v.ID), admin, 2, 1, "Count corrected"); err != nil {
		t.Fatal(err)
	}
	quote, err := carts.Get(ctx, customer)
	if err != nil || !quote.HasUnavailable || quote.SubtotalToman != 0 || quote.Lines[0].AvailableQuantity != 1 {
		t.Fatalf("reduced stock: %+v %v", quote, err)
	}
	if err = carts.Set(ctx, customer, string(v.ID), 1); err != nil {
		t.Fatal(err)
	}
	if err = inventory.SetStock(ctx, string(p.ID), string(v.ID), admin, 3, -1, "Bad count"); !errors.Is(err, catalog.ErrInvalid) {
		t.Fatal("negative stock accepted")
	}
	if err = inventory.SetStock(ctx, string(p.ID), string(v.ID), admin, 3, 1, ""); !errors.Is(err, catalog.ErrInvalid) {
		t.Fatal("missing reason accepted")
	}
	if err = inventory.SetStock(ctx, string(p.ID), string(v.ID), customer, 3, 1, "Customer mutation"); err == nil {
		t.Fatal("customer changed stock")
	}
	if err = inventory.SetStock(ctx, string(category.ID), string(v.ID), admin, 3, 1, "Wrong product"); !errors.Is(err, catalog.ErrNotFound) {
		t.Fatal("cross-product update accepted")
	}
	// An ordinary product edit must preserve inventory and its separate version.
	p, err = svc.GetProduct(ctx, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	p.Description = "Edited copy"
	p, err = svc.EditProduct(ctx, p.ID, p.Version, p)
	if err != nil || p.Variants[0].StockOnHand != 1 || p.Variants[0].StockVersion != 3 {
		t.Fatalf("catalog edit overwrote stock: %+v %v", p.Variants, err)
	}
}
