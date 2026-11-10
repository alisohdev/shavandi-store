package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/shavandi/store/backend/internal/orders"
)

type Orders interface {
	SaveDelivery(context.Context, string, string, orders.Delivery) (orders.Order, error)
	Create(context.Context, string, string, string) (orders.Order, error)
	List(context.Context, string) ([]orders.Order, error)
	Cancel(context.Context, string, string) (orders.Order, error)
}

func WithOrders(service Orders) HandlerOption {
	return func(options *handlerOptions) { options.orders = service }
}
func writeOrderError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, orders.ErrInvalid):
		writeError(w, r, 400, "invalid_request", "Invalid order request")
	case errors.Is(err, orders.ErrNotFound):
		writeError(w, r, 404, "not_found", "Order not found")
	case errors.Is(err, orders.ErrConflict):
		writeError(w, r, 409, "order_conflict", "Refresh the cart or review the pending order")
	default:
		writeError(w, r, 503, "orders_unavailable", "Orders are unavailable")
	}
}
func registerOrderRoutes(router chi.Router, authentication Authentication, options AuthOptions, service Orders) {
	security := &authAPI{service: authentication, options: options}
	router.Route("/api/v1/orders", func(r chi.Router) {
		r.Use(security.browserSecurity)
		r.Use(security.boundedContext)
		r.Use(security.RequireAuthentication)
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			session, _ := SessionFromContext(r.Context())
			result, err := service.List(r.Context(), session.User.ID)
			if err != nil {
				writeOrderError(w, r, err)
				return
			}
			writeJSON(w, 200, map[string]any{"orders": result})
		})
		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			var input struct {
				Key   string `json:"key"`
				Quote string `json:"quote_token"`
			}
			if !decodeAuthJSON(w, r, &input) {
				return
			}
			session, _ := SessionFromContext(r.Context())
			order, err := service.Create(r.Context(), session.User.ID, input.Key, input.Quote)
			if err != nil {
				writeOrderError(w, r, err)
				return
			}
			writeJSON(w, 200, map[string]any{"order": order})
		})

		r.Post("/{id}/delivery", func(w http.ResponseWriter, r *http.Request) {
			var input struct {
				Recipient        string  `json:"recipient"`
				Phone            string  `json:"phone"`
				Province         string  `json:"province"`
				City             string  `json:"city"`
				Address          string  `json:"address"`
				PostalCode       string  `json:"postal_code"`
				Version          int64   `json:"version"`
				ShippingMethodID *string `json:"shipping_method_id"`
				ShippingVersion  *int64  `json:"shipping_version"`
			}
			if !decodeAuthJSON(w, r, &input) {
				return
			}
			session, _ := SessionFromContext(r.Context())
			order, err := service.SaveDelivery(r.Context(), session.User.ID, chi.URLParam(r, "id"), orders.Delivery{Recipient: input.Recipient, Phone: input.Phone, Province: input.Province, City: input.City, Address: input.Address, PostalCode: input.PostalCode, Version: input.Version, ShippingMethodID: input.ShippingMethodID, ShippingVersion: input.ShippingVersion})
			if err != nil {
				writeOrderError(w, r, err)
				return
			}
			writeJSON(w, 200, map[string]any{"order": order})
		})
		r.Post("/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
			var input struct{}
			if !decodeAuthJSON(w, r, &input) {
				return
			}
			session, _ := SessionFromContext(r.Context())
			order, err := service.Cancel(r.Context(), session.User.ID, chi.URLParam(r, "id"))
			if err != nil {
				writeOrderError(w, r, err)
				return
			}
			writeJSON(w, 200, map[string]any{"order": order})
		})
	})
}
