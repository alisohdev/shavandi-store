package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/shavandi/store/backend/internal/catalog"
)

const readinessTimeout = 2 * time.Second
const catalogReadTimeout = 3 * time.Second

type DatabaseCheck func(context.Context) error

type errorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type categoryResponse struct {
	ID           catalog.ID  `json:"id"`
	Name         string      `json:"name"`
	Slug         string      `json:"slug"`
	ParentID     *catalog.ID `json:"parent_id"`
	DisplayOrder int32       `json:"display_order"`
}

type categoriesResponse struct {
	Categories []categoryResponse `json:"categories"`
}

type productsResponse struct {
	Products   []catalog.PublicProduct `json:"products"`
	NextCursor string                  `json:"next_cursor,omitempty"`
}

type productResponse struct {
	Product catalog.PublicProduct `json:"product"`
}

func NewHandler(checkDatabase DatabaseCheck, catalogReader catalog.Reader, logger *slog.Logger, options ...HandlerOption) http.Handler {
	router := chi.NewRouter()
	router.Use(requestIDMiddleware(logger))
	router.Use(recoverPanics(logger))
	settings := handlerOptions{}
	for _, option := range options {
		option(&settings)
	}
	if settings.authentication != nil {
		registerAuthRoutes(router, settings.authentication, settings.authOptions)
	}

	router.Get("/health/live", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	router.Get("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
		defer cancel()

		if checkDatabase == nil {
			logger.Warn("database readiness check is not configured", "request_id", requestIDFrom(r.Context()))
			writeError(w, r, http.StatusServiceUnavailable, "database_unavailable", "Service is not ready")
			return
		}
		if err := checkDatabase(ctx); err != nil {
			logger.Warn("database readiness check failed",
				"request_id", requestIDFrom(r.Context()),
				"error", err,
			)
			writeError(w, r, http.StatusServiceUnavailable, "database_unavailable", "Service is not ready")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	router.Get("/api/v1/categories", func(w http.ResponseWriter, r *http.Request) {
		if catalogReader == nil {
			writeError(w, r, http.StatusServiceUnavailable, "catalog_unavailable", "Catalog is not ready")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), catalogReadTimeout)
		defer cancel()
		categories, err := catalogReader.ListActiveCategories(ctx)
		if err != nil {
			writeCatalogError(w, r, err)
			return
		}
		response := categoriesResponse{Categories: make([]categoryResponse, 0, len(categories))}
		for _, category := range categories {
			response.Categories = append(response.Categories, categoryResponse{
				ID: category.ID, Name: category.Name, Slug: category.Slug,
				ParentID: category.ParentID, DisplayOrder: category.DisplayOrder,
			})
		}
		writeJSON(w, http.StatusOK, response)
	})
	router.Get("/api/v1/products", func(w http.ResponseWriter, r *http.Request) {
		if catalogReader == nil {
			writeError(w, r, http.StatusServiceUnavailable, "catalog_unavailable", "Catalog is not ready")
			return
		}
		query := r.URL.Query()
		filter := catalog.ProductFilter{CategorySlug: query.Get("category"), Limit: 20}
		if value := query.Get("limit"); value != "" {
			limit, err := strconv.Atoi(value)
			if err != nil || limit < 1 || limit > 50 {
				writeError(w, r, http.StatusBadRequest, "invalid_request", "Query parameter limit must be an integer from 1 through 50")
				return
			}
			filter.Limit = limit
		}
		if value := query.Get("cursor"); value != "" {
			cursor, err := catalog.DecodeCursor(value)
			if err != nil {
				writeError(w, r, http.StatusBadRequest, "invalid_request", "Query parameter cursor is malformed")
				return
			}
			filter.After = &cursor
		}
		ctx, cancel := context.WithTimeout(r.Context(), catalogReadTimeout)
		defer cancel()
		products, nextCursor, err := catalogReader.ListPublishedProducts(ctx, filter)
		if err != nil {
			writeCatalogError(w, r, err)
			return
		}
		if products == nil {
			products = make([]catalog.PublicProduct, 0)
		}
		writeJSON(w, http.StatusOK, productsResponse{Products: products, NextCursor: nextCursor})
	})
	router.Get("/api/v1/products/{slug}", func(w http.ResponseWriter, r *http.Request) {
		if catalogReader == nil {
			writeError(w, r, http.StatusServiceUnavailable, "catalog_unavailable", "Catalog is not ready")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), catalogReadTimeout)
		defer cancel()
		product, err := catalogReader.GetPublishedProductBySlug(ctx, chi.URLParam(r, "slug"))
		if err != nil {
			writeCatalogError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, productResponse{Product: product})
	})
	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, http.StatusNotFound, "not_found", "The requested resource was not found")
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "The method is not allowed")
	})

	return router
}

func writeCatalogError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, catalog.ErrNotFound), errors.Is(err, catalog.ErrNotPublished):
		writeError(w, r, http.StatusNotFound, "not_found", "The requested catalog resource was not found")
	case errors.Is(err, catalog.ErrInvalid):
		writeError(w, r, http.StatusBadRequest, "invalid_request", "The catalog request is invalid")
	case errors.Is(err, catalog.ErrConflict), errors.Is(err, catalog.ErrVersionConflict):
		writeError(w, r, http.StatusConflict, "conflict", "The catalog resource conflicts with existing data")
	default:
		writeError(w, r, http.StatusInternalServerError, "internal_error", "An internal error occurred")
	}
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, status, errorEnvelope{
		Error: errorBody{
			Code:      code,
			Message:   message,
			RequestID: requestIDFrom(r.Context()),
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
