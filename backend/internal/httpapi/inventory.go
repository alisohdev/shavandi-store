package httpapi

import (
	"context"
	"github.com/go-chi/chi/v5"
	"github.com/shavandi/store/backend/internal/catalog"
	"net/http"
)

type Inventory interface {
	SetStock(context.Context, string, string, string, int64, int, string) error
}

func WithInventory(store Inventory) HandlerOption {
	return func(options *handlerOptions) { options.inventory = store }
}
func registerInventoryRoutes(router chi.Router, authentication Authentication, options AuthOptions, products AdminCatalog, store Inventory) {
	security := &authAPI{service: authentication, options: options}
	router.With(security.browserSecurity, security.boundedContext, security.RequireAuthentication, RequireAdmin).Patch("/api/v1/admin/products/{id}/variants/{variantID}/stock", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Version  *int64 `json:"version"`
			Quantity *int   `json:"quantity"`
			Reason   string `json:"reason"`
		}
		if !decodeAuthJSON(w, r, &input) {
			return
		}
		if input.Version == nil || input.Quantity == nil {
			writeError(w, r, 400, "invalid_request", "Stock version and quantity are required")
			return
		}
		session, _ := SessionFromContext(r.Context())
		if err := store.SetStock(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "variantID"), session.User.ID, *input.Version, *input.Quantity, input.Reason); err != nil {
			writeCatalogError(w, r, err)
			return
		}
		product, err := products.GetProduct(r.Context(), catalog.ID(chi.URLParam(r, "id")))
		if err != nil {
			writeCatalogError(w, r, err)
			return
		}
		writeJSON(w, 200, map[string]any{"product": product})
	})
}
