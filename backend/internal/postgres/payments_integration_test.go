//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/shavandi/store/backend/internal/catalog"
	"github.com/shavandi/store/backend/internal/orders"
	"github.com/shavandi/store/backend/internal/payments"
	"github.com/shavandi/store/backend/internal/shipping"
)

type paymentGatewayFake struct {
	mu                      sync.Mutex
	requests, verifications int
	requestError            error
	amount                  int64
}

func (g *paymentGatewayFake) Request(_ context.Context, amount int64, _ string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.requests++
	g.amount = amount
	if g.requestError != nil {
		return "", g.requestError
	}
	return fmt.Sprintf("A%035d", g.requests), nil
}
func (g *paymentGatewayFake) Verify(_ context.Context, amount int64, authority string) (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.verifications++
	if amount != 15000 {
		return 0, payments.ErrNotPaid
	}
	if strings.HasSuffix(authority, "1") {
		return 111, nil
	}
	return 222, nil
}
func (g *paymentGatewayFake) Redirect(authority string) string {
	return "https://payment.zarinpal.com/pg/StartPay/" + authority
}
func TestPaymentsOwnershipIdempotencyStockAndLateSettlement(t *testing.T) {
	pool, _ := openFreshIntegrationDatabase(t)
	ctx := context.Background()
	t.Cleanup(func() {
		// These rows belong exclusively to this isolated test; keep production downgrade protection intact.
		cleanupCtx := context.Background()
		if _, e := pool.Exec(cleanupCtx, `DELETE FROM shavandi.order_payments;DELETE FROM shavandi.order_lines;DELETE FROM shavandi.order_deliveries;DELETE FROM shavandi.orders`); e != nil {
			t.Error(e)
		}
	})

	catalogService := catalog.NewService(NewCatalogRepository(pool))
	orderStore := NewOrderStore(pool)
	paymentStore := NewPaymentStore(pool)
	cartStore := NewCartStore(pool)
	category, err := catalogService.CreateCategory(ctx, catalog.Category{Name: "Payments", Slug: "payments", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	product, err := catalogService.CreateProductDraft(ctx, catalog.Product{Name: "Paid item", Slug: "paid-item", Description: "Details", PrimaryCategory: category.ID, RegularPriceToman: 1000, Images: []catalog.Image{{AssetRef: "/images/test.png"}}})
	if err != nil {
		t.Fatal(err)
	}
	product, err = catalogService.ChangePublication(ctx, product.ID, product.Version, catalog.Published)
	if err != nil {
		t.Fatal(err)
	}
	variant := string(product.Variants[0].ID)
	if _, err = pool.Exec(ctx, `UPDATE shavandi.variants SET stock_on_hand=3 WHERE id=$1::uuid`, variant); err != nil {
		t.Fatal(err)
	}
	method, err := NewShippingStore(pool).Save(ctx, shipping.Method{Name: "Postal", FeeToman: 500, Active: true})
	if err != nil {
		t.Fatal(err)
	}
	makeOrder := func(phone string) (string, orders.Order) {
		t.Helper()
		var user, key string
		if e := pool.QueryRow(ctx, `INSERT INTO shavandi.users(phone,role)VALUES($1,'customer')RETURNING id::text`, phone).Scan(&user); e != nil {
			t.Fatal(e)
		}
		if e := pool.QueryRow(ctx, `SELECT gen_random_uuid()::text`).Scan(&key); e != nil {
			t.Fatal(e)
		}
		if e := cartStore.Set(ctx, user, variant, 1); e != nil {
			t.Fatal(e)
		}
		quote, e := cartStore.Get(ctx, user)
		if e != nil {
			t.Fatal(e)
		}
		order, e := orderStore.Create(ctx, user, key, quote.QuoteToken)
		if e != nil {
			t.Fatal(e)
		}
		order, e = orderStore.SaveDelivery(ctx, user, order.ID, orders.Delivery{Recipient: "Payment recipient", Phone: "09123456789", Province: "Tehran", City: "Tehran", Address: "Test address number 12", PostalCode: "1234567890", ShippingMethodID: &method.ID, ShippingVersion: &method.Version})
		if e != nil {
			t.Fatal(e)
		}
		return user, order
	}
	user, order := makeOrder("+989800000001")
	gateway := &paymentGatewayFake{}
	service := &payments.Service{Store: paymentStore, Gateway: gateway, Environment: "production", MerchantFingerprint: strings.Repeat("a", 64)}
	if _, err = service.Start(ctx, user, order.ID, order.Delivery.Version, 1501); !errors.Is(err, payments.ErrConflict) {
		t.Fatal("client amount trusted")
	}
	if _, err = service.Start(ctx, user, order.ID, order.Delivery.Version+1, 1500); !errors.Is(err, payments.ErrConflict) {
		t.Fatal("unreviewed delivery trusted")
	}
	if _, err = service.Start(ctx, "0192bbd1-9e2d-4a45-9baf-37f4d6280999", order.ID, 1, 1500); !errors.Is(err, payments.ErrNotFound) {
		t.Fatal("other account initiated payment")
	}
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := service.Start(ctx, user, order.ID, 1, 1500)
			if e != nil && !errors.Is(e, payments.ErrConflict) {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	if gateway.requests != 1 || gateway.amount != 15000 {
		t.Fatalf("duplicate or wrong-amount request: %d %d", gateway.requests, gateway.amount)
	}
	attempt, err := paymentStore.ByOrder(ctx, user, order.ID)
	if err != nil || attempt.Status != "ready" {
		t.Fatal(err)
	}
	if _, err = orderStore.SaveDelivery(ctx, user, order.ID, *order.Delivery); !errors.Is(err, orders.ErrConflict) {
		t.Fatal("delivery changed during payment")
	}
	if _, err = orderStore.Cancel(ctx, user, order.ID); !errors.Is(err, orders.ErrConflict) {
		t.Fatal("reservation cancelled while gateway active")
	}
	if err = service.CheckOwner(ctx, "0192bbd1-9e2d-4a45-9baf-37f4d6280999", order.ID); !errors.Is(err, payments.ErrNotFound) {
		t.Fatal("owner verification bypass")
	}
	// A database failure after gateway verification must not partially deduct stock.
	if _, err = pool.Exec(ctx, `CREATE FUNCTION shavandi.reject_paid() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.status='paid' THEN RAISE EXCEPTION 'injected payment commit failure'; END IF; RETURN NEW; END $$; CREATE TRIGGER reject_paid BEFORE UPDATE ON shavandi.orders FOR EACH ROW EXECUTE FUNCTION shavandi.reject_paid()`); err != nil {
		t.Fatal(err)
	}
	if err = service.Callback(ctx, attempt.Authority); err == nil {
		t.Fatal("commit failure ignored")
	}
	assertStock := func(onhand, reserved int) {
		t.Helper()
		var physical, held int
		if e := pool.QueryRow(ctx, `SELECT stock_on_hand,reserved_quantity FROM shavandi.variants WHERE id=$1::uuid`, variant).Scan(&physical, &held); e != nil || physical != onhand || held != reserved {
			t.Fatalf("stock %d/%d want %d/%d: %v", physical, held, onhand, reserved, e)
		}
	}
	assertStock(3, 1)
	if _, err = pool.Exec(ctx, `DROP TRIGGER reject_paid ON shavandi.orders;DROP FUNCTION shavandi.reject_paid();UPDATE shavandi.order_payments SET busy_until=NULL`); err != nil {
		t.Fatal(err)
	}
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e := service.Callback(ctx, attempt.Authority)
			if e != nil && !errors.Is(e, payments.ErrConflict) {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	assertStock(2, 0)
	if err = service.Callback(ctx, attempt.Authority); err != nil {
		t.Fatal(err)
	}
	assertStock(2, 0)
	if gateway.verifications != 2 {
		t.Fatalf("duplicate callbacks contacted gateway again: %d", gateway.verifications)
	}
	paid, err := orderStore.List(ctx, user)
	if err != nil || paid[0].Status != "paid" || paid[0].PaymentReference == nil || *paid[0].PaymentReference != "111" {
		t.Fatal("paid state not persisted")
	}
	_, late := makeOrder("+989800000002")
	var lateOwner string
	if err = pool.QueryRow(ctx, `SELECT user_id::text FROM shavandi.orders WHERE id=$1::uuid`, late.ID).Scan(&lateOwner); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Start(ctx, lateOwner, late.ID, 1, 1500); err != nil {
		t.Fatal(err)
	}
	lateAttempt, err := paymentStore.ByOrder(ctx, lateOwner, late.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE shavandi.orders SET created_at=now()-interval '20 minutes',expires_at=now()-interval '1 minute' WHERE id=$1::uuid`, late.ID); err != nil {
		t.Fatal(err)
	}
	if err = orderStore.Expire(ctx); err != nil {
		t.Fatal(err)
	}
	assertStock(2, 0)
	if err = service.Callback(ctx, lateAttempt.Authority); err != nil {
		t.Fatal(err)
	}
	assertStock(2, 0)
	review, err := orderStore.List(ctx, lateOwner)
	if err != nil || review[0].Status != "payment_review" {
		t.Fatal("late payment auto-fulfilled or lost")
	}
	unknownOwner, unknownOrder := makeOrder("+989800000003")
	gateway.requestError = payments.ErrUnavailable
	if _, err = service.Start(ctx, unknownOwner, unknownOrder.ID, 1, 1500); !errors.Is(err, payments.ErrUnavailable) {
		t.Fatal("uncertain request not reported")
	}
	before := gateway.requests
	if _, err = service.Start(ctx, unknownOwner, unknownOrder.ID, 1, 1500); !errors.Is(err, payments.ErrConflict) {
		t.Fatal("uncertain request retried")
	}
	if gateway.requests != before {
		t.Fatal("second charge attempt issued")
	}
}
