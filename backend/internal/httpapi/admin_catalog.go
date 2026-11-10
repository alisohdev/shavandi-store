package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/shavandi/store/backend/internal/catalog"
)

func registerAdminCatalogRoutes(router chi.Router, authentication Authentication, options AuthOptions, service AdminCatalog) {
	security := &authAPI{service: authentication, options: options}
	router.Route("/api/v1/admin", func(r chi.Router) {
		r.Use(security.browserSecurity)
		r.Use(security.boundedContext)
		r.Use(security.RequireAuthentication)
		r.Use(RequireAdmin)
		r.Get("/products", func(w http.ResponseWriter, req *http.Request) {
			products, err := service.ListAllProducts(req.Context())
			if err != nil {
				writeCatalogError(w, req, err)
				return
			}
			if products == nil {
				products = []catalog.Product{}
			}
			writeJSON(w, http.StatusOK, map[string]any{"products": products})
		})
		r.Get("/products/{id}", func(w http.ResponseWriter, req *http.Request) {
			product, err := service.GetProduct(req.Context(), catalog.ID(chi.URLParam(req, "id")))
			if err != nil {
				writeCatalogError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"product": product})
		})
		r.Post("/products", func(w http.ResponseWriter, req *http.Request) {
			var body productCreateRequest
			if !decodeAuthJSON(w, req, &body) {
				return
			}
			product := catalog.Product{Slug: body.Slug, Name: body.Name, Description: body.Description, PrimaryCategory: catalog.ID(body.CategoryID), RegularPriceToman: body.RegularPriceToman, SalePriceToman: body.SalePriceToman}
			created, err := service.CreateProductDraft(req.Context(), product)
			if err != nil {
				writeCatalogError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{"product": created})
		})
		r.Patch("/products/{id}", func(w http.ResponseWriter, req *http.Request) {
			var body productPatchRequest
			if !decodeAuthJSON(w, req, &body) {
				return
			}
			if body.Version == nil {
				writeError(w, req, http.StatusBadRequest, "invalid_request", "Product version is required")
				return
			}
			id := catalog.ID(chi.URLParam(req, "id"))
			current, err := service.GetProduct(req.Context(), id)
			if err != nil {
				writeCatalogError(w, req, err)
				return
			}
			changes := catalog.Product{}
			if body.Name != nil {
				changes.Name = *body.Name
			} else {
				changes.Name = current.Name
			}
			if body.Slug != nil {
				changes.Slug = *body.Slug
			} else {
				changes.Slug = current.Slug
			}
			if body.Description != nil {
				changes.Description = *body.Description
			} else {
				changes.Description = current.Description
			}
			if body.CategoryID != nil {
				changes.PrimaryCategory = catalog.ID(*body.CategoryID)
			} else {
				changes.PrimaryCategory = current.PrimaryCategory
			}
			if body.RegularPriceToman != nil {
				changes.RegularPriceToman = *body.RegularPriceToman
			} else {
				changes.RegularPriceToman = current.RegularPriceToman
			}
			if body.SalePriceToman != nil {
				changes.SalePriceToman = body.SalePriceToman
			} else {
				changes.SalePriceToman = current.SalePriceToman
			}
			saved, err := service.EditProduct(req.Context(), id, *body.Version, changes)
			if err != nil {
				writeCatalogError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"product": saved})
		})
		r.Post("/products/{id}/variants", func(w http.ResponseWriter, req *http.Request) {
			var body variantAddRequest
			if !decodeAuthJSON(w, req, &body) {
				return
			}
			if body.Version == nil {
				writeError(w, req, http.StatusBadRequest, "invalid_request", "Product version is required")
				return
			}
			id := catalog.ID(chi.URLParam(req, "id"))
			current, err := service.GetProduct(req.Context(), id)
			if err != nil {
				writeCatalogError(w, req, err)
				return
			}
			variant := catalog.Variant{Size: body.Size, Color: body.Color, RegularPriceOverrideToman: body.RegularPriceOverrideToman, SalePriceOverrideToman: body.SalePriceOverrideToman, Active: body.Active == nil || *body.Active}
			changes := catalog.Product{Variants: []catalog.Variant{variant}}
			// Replacing a default optionless product with sized/color variants retires
			// its default combination in the same transaction, retaining its identity.
			if variant.Active && (variant.Size != nil || variant.Color != nil) {
				for _, existing := range current.Variants {
					if existing.Active && existing.Size == nil && existing.Color == nil {
						existing.Active = false
						changes.Variants = append(changes.Variants, existing)
					}
				}
			}
			saved, err := service.EditProduct(req.Context(), id, *body.Version, changes)
			if err != nil {
				writeCatalogError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{"product": saved})
		})
		r.Patch("/products/{id}/variants/{variantID}", func(w http.ResponseWriter, req *http.Request) {
			var body variantPatchRequest
			if !decodeAuthJSON(w, req, &body) {
				return
			}
			if body.Version == nil {
				writeError(w, req, http.StatusBadRequest, "invalid_request", "Product version is required")
				return
			}
			id := catalog.ID(chi.URLParam(req, "id"))
			variantID := catalog.ID(chi.URLParam(req, "variantID"))
			current, err := service.GetProduct(req.Context(), id)
			if err != nil {
				writeCatalogError(w, req, err)
				return
			}
			var target *catalog.Variant
			for i := range current.Variants {
				if current.Variants[i].ID == variantID {
					target = &current.Variants[i]
					break
				}
			}
			if target == nil {
				writeError(w, req, http.StatusNotFound, "not_found", "The requested catalog resource was not found")
				return
			}
			if body.Size != nil {
				target.Size = body.Size
			}
			if body.Color != nil {
				target.Color = body.Color
			}
			if body.RegularPriceOverrideToman != nil {
				target.RegularPriceOverrideToman = body.RegularPriceOverrideToman
			}
			if body.SalePriceOverrideToman != nil {
				target.SalePriceOverrideToman = body.SalePriceOverrideToman
			}
			if body.Active != nil {
				target.Active = *body.Active
			}
			saved, err := service.EditProduct(req.Context(), id, *body.Version, catalog.Product{Variants: []catalog.Variant{*target}})
			if err != nil {
				writeCatalogError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"product": saved})
		})
		r.Delete("/products/{id}/variants/{variantID}", func(w http.ResponseWriter, req *http.Request) {
			var body variantVersionRequest
			if !decodeAuthJSON(w, req, &body) {
				return
			}
			if body.Version == nil {
				writeError(w, req, http.StatusBadRequest, "invalid_request", "Product version is required")
				return
			}
			id := catalog.ID(chi.URLParam(req, "id"))
			variantID := catalog.ID(chi.URLParam(req, "variantID"))
			current, err := service.GetProduct(req.Context(), id)
			if err != nil {
				writeCatalogError(w, req, err)
				return
			}
			var target *catalog.Variant
			for i := range current.Variants {
				if current.Variants[i].ID == variantID {
					target = &current.Variants[i]
					break
				}
			}
			if target == nil {
				writeError(w, req, http.StatusNotFound, "not_found", "The requested catalog resource was not found")
				return
			}
			target.Active = false
			saved, err := service.EditProduct(req.Context(), id, *body.Version, catalog.Product{Variants: []catalog.Variant{*target}})
			if err != nil {
				writeCatalogError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"product": saved})
		})
		r.Get("/categories", func(w http.ResponseWriter, req *http.Request) {
			categories, err := service.ListAllCategories(req.Context())
			if err != nil {
				writeCatalogError(w, req, err)
				return
			}
			if categories == nil {
				categories = []catalog.Category{}
			}
			writeJSON(w, http.StatusOK, map[string]any{"categories": categories})
		})
		r.Post("/categories", func(w http.ResponseWriter, req *http.Request) {
			var body categoryCreateRequest
			if !decodeAuthJSON(w, req, &body) {
				return
			}
			category := catalog.Category{Name: body.Name, Slug: body.Slug, DisplayOrder: body.DisplayOrder, Active: true}
			if len(body.ParentID) > 0 && string(body.ParentID) != "null" {
				var parent string
				if err := json.Unmarshal(body.ParentID, &parent); err != nil {
					writeError(w, req, http.StatusBadRequest, "invalid_request", "The category request is invalid")
					return
				}
				id := catalog.ID(parent)
				category.ParentID = &id
			}
			created, err := service.CreateCategory(req.Context(), category)
			if err != nil {
				writeCatalogError(w, req, err)
				return
			}
			writeJSON(w, http.StatusCreated, map[string]any{"category": created})
		})
		r.Patch("/categories/{id}", func(w http.ResponseWriter, req *http.Request) {
			var body categoryPatchRequest
			if !decodeAuthJSON(w, req, &body) {
				return
			}
			if body.Version == nil {
				writeError(w, req, http.StatusBadRequest, "invalid_request", "Category version is required")
				return
			}
			id := catalog.ID(chi.URLParam(req, "id"))
			current, err := service.GetCategory(req.Context(), id)
			if err != nil {
				writeCatalogError(w, req, err)
				return
			}
			updated := current
			if body.Name != nil {
				updated.Name = *body.Name
			}
			if body.DisplayOrder != nil {
				updated.DisplayOrder = *body.DisplayOrder
			}
			if body.Active != nil {
				updated.Active = *body.Active
			}
			if body.ParentID != nil {
				if string(*body.ParentID) == "null" {
					updated.ParentID = nil
				} else {
					var parent string
					if err := json.Unmarshal(*body.ParentID, &parent); err != nil {
						writeError(w, req, http.StatusBadRequest, "invalid_request", "The category request is invalid")
						return
					}
					parentID := catalog.ID(parent)
					updated.ParentID = &parentID
				}
			}
			saved, err := service.EditCategory(req.Context(), updated, *body.Version)
			if err != nil {
				writeCatalogError(w, req, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"category": saved})
		})
		r.Post("/products/{id}/publish", publicationHandler(service, catalog.Published))
		r.Post("/products/{id}/hide", publicationHandler(service, catalog.Hidden))
	})
}

func publicationHandler(service interface {
	ChangePublication(context.Context, catalog.ID, int64, catalog.PublicationState) (catalog.Product, error)
}, target catalog.PublicationState) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var body variantVersionRequest
		if !decodeAuthJSON(w, req, &body) {
			return
		}
		if body.Version == nil {
			writeError(w, req, http.StatusBadRequest, "invalid_request", "Product version is required")
			return
		}
		product, err := service.ChangePublication(req.Context(), catalog.ID(chi.URLParam(req, "id")), *body.Version, target)
		if err != nil {
			writeCatalogError(w, req, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"product": product})
	}
}

type categoryCreateRequest struct {
	Name         string          `json:"name"`
	Slug         string          `json:"slug"`
	ParentID     json.RawMessage `json:"parent_id"`
	DisplayOrder int32           `json:"display_order"`
}
type categoryPatchRequest struct {
	Version      *int64           `json:"version"`
	Name         *string          `json:"name"`
	ParentID     *json.RawMessage `json:"parent_id"`
	DisplayOrder *int32           `json:"display_order"`
	Active       *bool            `json:"active"`
}

type variantVersionRequest struct {
	Version *int64 `json:"version"`
}
type variantAddRequest struct {
	Version                   *int64  `json:"version"`
	Size                      *string `json:"size"`
	Color                     *string `json:"color"`
	RegularPriceOverrideToman *int64  `json:"regular_price_override_toman"`
	SalePriceOverrideToman    *int64  `json:"sale_price_override_toman"`
	Active                    *bool   `json:"active"`
}
type variantPatchRequest struct {
	Version                   *int64  `json:"version"`
	Size                      *string `json:"size"`
	Color                     *string `json:"color"`
	RegularPriceOverrideToman *int64  `json:"regular_price_override_toman"`
	SalePriceOverrideToman    *int64  `json:"sale_price_override_toman"`
	Active                    *bool   `json:"active"`
}

type productCreateRequest struct {
	Name              string `json:"name"`
	Slug              string `json:"slug"`
	Description       string `json:"description"`
	CategoryID        string `json:"category_id"`
	RegularPriceToman int64  `json:"regular_price_toman"`
	SalePriceToman    *int64 `json:"sale_price_toman"`
}

type productPatchRequest struct {
	Version           *int64  `json:"version"`
	Name              *string `json:"name"`
	Slug              *string `json:"slug"`
	Description       *string `json:"description"`
	CategoryID        *string `json:"category_id"`
	RegularPriceToman *int64  `json:"regular_price_toman"`
	SalePriceToman    *int64  `json:"sale_price_toman"`
}

var _ interface {
	ListAllProducts(context.Context) ([]catalog.Product, error)
	GetProduct(context.Context, catalog.ID) (catalog.Product, error)
	GetCategory(context.Context, catalog.ID) (catalog.Category, error)
	CreateProductDraft(context.Context, catalog.Product) (catalog.Product, error)
	EditProduct(context.Context, catalog.ID, int64, catalog.Product) (catalog.Product, error)
	ChangePublication(context.Context, catalog.ID, int64, catalog.PublicationState) (catalog.Product, error)
} = (*catalog.Service)(nil)
