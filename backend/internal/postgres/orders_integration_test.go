//go:build integration

package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/shavandi/store/backend/internal/catalog"
	"github.com/shavandi/store/backend/internal/orders"
)

func TestOrderReservationsAreAtomicIdempotentAndOwned(t *testing.T) {
	pool, _ := openFreshIntegrationDatabase(t)
	ctx := context.Background()
	store := NewOrderStore(pool)
	carts := NewCartStore(pool)
	users := make([]string, 2)
	for i, phone := range []string{"+989555555555", "+989666666666"} {
		if err := pool.QueryRow(ctx, `INSERT INTO shavandi.users(phone,role)VALUES($1,'customer') RETURNING id::text`, phone).Scan(&users[i]); err != nil {
			t.Fatal(err)
		}
	}
	svc := catalog.NewService(NewCatalogRepository(pool))
	category, err := svc.CreateCategory(ctx, catalog.Category{Name: "Orders", Slug: "orders", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	p, err := svc.CreateProductDraft(ctx, catalog.Product{Name: "Last item", Slug: "last-item", Description: "Details", PrimaryCategory: category.ID, RegularPriceToman: 1000, Images: []catalog.Image{{AssetRef: "/images/test.png"}}})
	if err != nil {
		t.Fatal(err)
	}
	p, err = svc.ChangePublication(ctx, p.ID, p.Version, catalog.Published)
	if err != nil {
		t.Fatal(err)
	}
	variant := string(p.Variants[0].ID)
	if _, err = pool.Exec(ctx, `UPDATE shavandi.variants SET stock_on_hand=1 WHERE id=$1::uuid`, variant); err != nil {
		t.Fatal(err)
	}
	quotes := make([]string, 2)
	keys := []string{"0192bbd1-9e2d-4a45-9baf-37f4d6280601", "0192bbd1-9e2d-4a45-9baf-37f4d6280602"}
	for i, user := range users {
		if err = carts.Set(ctx, user, variant, 1); err != nil {
			t.Fatal(err)
		}
		quote, e := carts.Get(ctx, user)
		if e != nil {
			t.Fatal(e)
		}
		quotes[i] = quote.QuoteToken
	}
	var wg sync.WaitGroup
	results := make([]orders.Order, 2)
	failures := make([]error, 2)
	for i := range users {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], failures[i] = store.Create(ctx, users[i], keys[i], quotes[i])
		}(i)
	}
	wg.Wait()
	winner, loser := 0, 1
	if failures[0] != nil {
		winner, loser = 1, 0
	}
	if failures[winner] != nil || !errors.Is(failures[loser], orders.ErrConflict) {
		t.Fatalf("scarce stock race: %v", failures)
	}
	assertReserved := func(want int) {
		t.Helper()
		var reserved, onhand int
		if e := pool.QueryRow(ctx, `SELECT reserved_quantity,stock_on_hand FROM shavandi.variants WHERE id=$1::uuid`, variant).Scan(&reserved, &onhand); e != nil || reserved != want || onhand != 1 {
			t.Fatalf("stock %d/%d: %v", reserved, onhand, e)
		}
	}
	assertReserved(1)

	var admin string
	if err = pool.QueryRow(ctx, `INSERT INTO shavandi.users(phone,role,password_hash) VALUES('+989777777777','admin','$argon2id$v=19$test') RETURNING id::text`).Scan(&admin); err != nil {
		t.Fatal(err)
	}
	var stockVersion int64
	if err = pool.QueryRow(ctx, `SELECT stock_version FROM shavandi.variants WHERE id=$1::uuid`, variant).Scan(&stockVersion); err != nil {
		t.Fatal(err)
	}
	if err = NewInventoryStore(pool).SetStock(ctx, string(p.ID), variant, admin, stockVersion, 0, "Count below reservation"); !errors.Is(err, catalog.ErrInvalid) {
		t.Fatalf("reserved stock overwritten: %v", err)
	}
	replay, err := store.Create(ctx, users[winner], keys[winner], quotes[winner])
	if err != nil || replay.ID != results[winner].ID {
		t.Fatalf("duplicate create: %+v %v", replay, err)
	}
	if _, err = store.Create(ctx, users[winner], keys[loser], quotes[winner]); !errors.Is(err, orders.ErrConflict) {
		t.Fatal("second pending order accepted")
	}
	if _, err = store.Cancel(ctx, users[loser], replay.ID); !errors.Is(err, orders.ErrNotFound) {
		t.Fatal("cross-account cancellation accepted")
	}
	other, err := store.List(ctx, users[loser])
	if err != nil || len(other) != 0 {
		t.Fatal("cross-account list leak")
	}
	if _, err = pool.Exec(ctx, `UPDATE shavandi.products SET regular_price_toman=1500 WHERE id=$1::uuid`, string(p.ID)); err != nil {
		t.Fatal(err)
	}
	again, err := store.Create(ctx, users[winner], keys[winner], quotes[winner])
	if err != nil || again.SubtotalToman != 1000 || again.Lines[0].UnitPriceToman != 1000 {
		t.Fatal("snapshot price changed")
	}
	for range 2 {
		cancelled, e := store.Cancel(ctx, users[winner], replay.ID)
		if e != nil || cancelled.Status != "cancelled" {
			t.Fatal(e)
		}
	}
	assertReserved(0)
	// A stale quote must not reserve stock or clear the customer's cart.
	if _, err = store.Create(ctx, users[loser], keys[loser], quotes[loser]); !errors.Is(err, orders.ErrConflict) {
		t.Fatal("stale price accepted")
	}
	assertReserved(0)
	quote, err := carts.Get(ctx, users[loser])
	if err != nil || len(quote.Lines) != 1 {
		t.Fatal("failed order cleared cart")
	}
	// Force a failure after the stock update: the whole order transaction must roll back.
	if _, err = pool.Exec(ctx, `CREATE FUNCTION shavandi.reject_order_line() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected order-line failure'; END $$; CREATE TRIGGER reject_order_line BEFORE INSERT ON shavandi.order_lines FOR EACH ROW EXECUTE FUNCTION shavandi.reject_order_line()`); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Create(ctx, users[loser], keys[loser], quote.QuoteToken); err == nil {
		t.Fatal("injected persistence failure ignored")
	}
	assertReserved(0)
	if _, err = pool.Exec(ctx, `DROP TRIGGER reject_order_line ON shavandi.order_lines; DROP FUNCTION shavandi.reject_order_line()`); err != nil {
		t.Fatal(err)
	}
	pending, err := store.Create(ctx, users[loser], keys[loser], quote.QuoteToken)
	if err != nil {
		t.Fatal(err)
	}
	assertReserved(1)
	if _, err = pool.Exec(ctx, `UPDATE shavandi.orders SET created_at=now()-interval '20 minutes',expires_at=now()-interval '1 minute' WHERE id=$1::uuid`, pending.ID); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err = store.Expire(ctx); err != nil {
			t.Fatal(err)
		}
	}
	assertReserved(0)
	expired, err := store.Create(ctx, users[loser], keys[loser], quote.QuoteToken)
	if err != nil || expired.Status != "expired" || expired.ID != pending.ID {
		t.Fatalf("expired replay: %+v %v", expired, err)
	}
	assertReserved(0)
}
