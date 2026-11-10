package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/shavandi/store/backend/internal/cart"
)

type Cart interface {
	Get(context.Context, string) (cart.Snapshot, error)
	Set(context.Context, string, string, int) error
}

func WithCart(service Cart) HandlerOption {
	return func(settings *handlerOptions) { settings.cart = service }
}
func registerCartRoutes(router chi.Router, authentication Authentication, options AuthOptions, service Cart) {
	security := &authAPI{service: authentication, options: options}
	router.Route("/api/v1/cart", func(r chi.Router) {
		r.Use(security.browserSecurity)
		r.Use(security.boundedContext)
		r.Use(security.RequireAuthentication)
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			session, _ := SessionFromContext(req.Context())
			snapshot, err := service.Get(req.Context(), session.User.ID)
			if err != nil {
				writeError(w, req, 503, "cart_unavailable", "Cart is unavailable")
				return
			}
			writeJSON(w, 200, snapshot)
		})
		r.Post("/lines/{variantID}", func(w http.ResponseWriter, req *http.Request) {
			var body struct {
				Quantity *int `json:"quantity"`
			}
			if !decodeAuthJSON(w, req, &body) {
				return
			}
			if body.Quantity == nil || *body.Quantity < 0 || *body.Quantity > 9 {
				writeError(w, req, 400, "invalid_request", "Quantity must be between zero and nine")
				return
			}
			session, _ := SessionFromContext(req.Context())
			err := service.Set(req.Context(), session.User.ID, chi.URLParam(req, "variantID"), *body.Quantity)
			if err != nil {
				switch {
				case errors.Is(err, cart.ErrInvalid):
					writeError(w, req, 400, "invalid_request", "Invalid cart line")
				case errors.Is(err, cart.ErrUnavailable):
					writeError(w, req, 409, "variant_unavailable", "This variant is no longer available")
				case errors.Is(err, cart.ErrLimit):
					writeError(w, req, 409, "cart_limit", "Cart has too many lines")
				default:
					writeError(w, req, 503, "cart_unavailable", "Cart could not be saved")
				}
				return
			}
			snapshot, err := service.Get(req.Context(), session.User.ID)
			if err != nil {
				writeError(w, req, 503, "cart_unavailable", "Cart could not be loaded")
				return
			}
			writeJSON(w, 200, snapshot)
		})
	})
}
