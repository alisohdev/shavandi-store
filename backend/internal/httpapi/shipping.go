package httpapi

import (
	"context"
	"github.com/go-chi/chi/v5"
	"github.com/shavandi/store/backend/internal/shipping"
	"net/http"
)

type Shipping interface {
	List(context.Context, bool) ([]shipping.Method, error)
	Save(context.Context, shipping.Method) (shipping.Method, error)
}

func WithShipping(store Shipping) HandlerOption {
	return func(options *handlerOptions) { options.shipping = store }
}
func registerShippingRoutes(router chi.Router, authentication Authentication, options AuthOptions, store Shipping) {
	list := func(all bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			result, err := store.List(r.Context(), all)
			if err != nil {
				writeCatalogError(w, r, err)
				return
			}
			writeJSON(w, 200, map[string]any{"methods": result})
		}
	}
	router.Get("/api/v1/shipping-methods", list(false))
	security := &authAPI{service: authentication, options: options}
	router.Route("/api/v1/admin/shipping-methods", func(r chi.Router) {
		r.Use(security.browserSecurity)
		r.Use(security.boundedContext)
		r.Use(security.RequireAuthentication)
		r.Use(RequireAdmin)
		r.Get("/", list(true))
		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			var input shipping.Method
			if !decodeAuthJSON(w, r, &input) {
				return
			}
			method, err := store.Save(r.Context(), input)
			if err != nil {
				writeCatalogError(w, r, err)
				return
			}
			writeJSON(w, 200, map[string]any{"method": method})
		})
	})
}
