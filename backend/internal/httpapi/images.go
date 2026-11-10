package httpapi

import (
	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/shavandi/store/backend/internal/auth"
	"github.com/shavandi/store/backend/internal/catalog"
	"github.com/shavandi/store/backend/internal/imagestore"
	_ "golang.org/x/image/webp"
)

const imageLimit int64 = 10 << 20
const uploadLimit int64 = 100 << 20
const maxPixels int64 = 25_000_000

func registerImageRoutes(router chi.Router, authentication Authentication, options AuthOptions, catalogService AdminCatalog, store imagestore.Store) {
	security := &authAPI{service: authentication, options: options}
	router.Route("/api/v1/admin/products/{id}/images", func(r chi.Router) {
		r.Use(security.browserSecurity)
		r.Use(security.boundedContext)
		r.Use(security.RequireAuthentication)
		r.Use(RequireAdmin)
		r.Post("/", func(w http.ResponseWriter, req *http.Request) { uploadProductImages(w, req, catalogService, store) })
		r.Patch("/", func(w http.ResponseWriter, req *http.Request) { reorderProductImages(w, req, catalogService) })
		r.Delete("/{imageID}", func(w http.ResponseWriter, req *http.Request) { removeProductImage(w, req, catalogService, store) })
	})
	router.Get("/api/v1/images/{key}", func(w http.ResponseWriter, req *http.Request) {
		serveProductImage(w, req, authentication, options, catalogService, store)
	})
}

func uploadProductImages(w http.ResponseWriter, r *http.Request, svc AdminCatalog, store imagestore.Store) {
	r.Body = http.MaxBytesReader(w, r.Body, uploadLimit)
	if err := r.ParseMultipartForm(12 << 20); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "The image upload is invalid or too large")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	version, err := strconv.ParseInt(r.FormValue("version"), 10, 64)
	if err != nil || version < 1 {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "Product version is required")
		return
	}
	id := catalog.ID(chi.URLParam(r, "id"))
	product, err := svc.GetProduct(r.Context(), id)
	if err != nil {
		writeCatalogError(w, r, err)
		return
	}
	files := r.MultipartForm.File["images"]
	if len(files) == 0 || len(files)+len(product.Images) > 10 {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "Upload between one and ten product images")
		return
	}
	keys := []string{}
	images := append([]catalog.Image(nil), product.Images...)
	cleanup := func() {
		for _, key := range keys {
			_ = store.Delete(key)
		}
	}
	for _, header := range files {
		if header.Size > imageLimit {
			cleanup()
			writeError(w, r, http.StatusBadRequest, "invalid_request", "Each image must be 10 MB or smaller")
			return
		}
		file, e := header.Open()
		if e != nil {
			cleanup()
			writeError(w, r, http.StatusBadRequest, "invalid_request", "An image could not be read")
			return
		}
		data, e := io.ReadAll(io.LimitReader(file, imageLimit+1))
		_ = file.Close()
		if e != nil || int64(len(data)) > imageLimit {
			cleanup()
			writeError(w, r, http.StatusBadRequest, "invalid_request", "Each image must be 10 MB or smaller")
			return
		}
		config, format, e := image.DecodeConfig(bytes.NewReader(data))
		if e != nil || int64(config.Width)*int64(config.Height) > maxPixels || config.Width < 1 || config.Height < 1 {
			cleanup()
			writeError(w, r, http.StatusBadRequest, "invalid_request", "Image data or dimensions are invalid")
			return
		}
		if _, _, e = image.Decode(bytes.NewReader(data)); e != nil {
			cleanup()
			writeError(w, r, http.StatusBadRequest, "invalid_request", "Image data is invalid")
			return
		}
		ext := ""
		switch format {
		case "jpeg":
			ext = "jpg"
		case "png":
			ext = "png"
		case "webp":
			ext = "webp"
		default:
			cleanup()
			writeError(w, r, http.StatusBadRequest, "invalid_request", "Only JPEG, PNG and WebP images are supported")
			return
		}
		key, e := store.Save(bytes.NewReader(data), ext)
		if e != nil {
			cleanup()
			writeError(w, r, http.StatusInternalServerError, "internal_error", "Image storage failed")
			return
		}
		keys = append(keys, key)
		alt := r.FormValue("alt_text")
		if alt == "" {
			alt = path.Base(header.Filename)
		}
		images = append(images, catalog.Image{AssetRef: "/api/v1/images/" + key, SortOrder: int32(len(images)), AltText: &alt})
	}
	product.Images = images
	saved, err := svc.EditProduct(r.Context(), id, version, product)
	if err != nil {
		cleanup()
		writeCatalogError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"product": saved})
}

type reorderImagesRequest struct {
	Version  int64        `json:"version"`
	ImageIDs []catalog.ID `json:"image_ids"`
}

func reorderProductImages(w http.ResponseWriter, r *http.Request, svc AdminCatalog) {
	var body reorderImagesRequest
	if !decodeAuthJSON(w, r, &body) {
		return
	}
	id := catalog.ID(chi.URLParam(r, "id"))
	p, err := svc.GetProduct(r.Context(), id)
	if err != nil {
		writeCatalogError(w, r, err)
		return
	}
	if body.Version < 1 || len(body.ImageIDs) != len(p.Images) {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "Image order must include every product image")
		return
	}
	byID := map[catalog.ID]catalog.Image{}
	for _, img := range p.Images {
		byID[img.ID] = img
	}
	ordered := make([]catalog.Image, 0, len(p.Images))
	for i, imageID := range body.ImageIDs {
		img, ok := byID[imageID]
		if !ok {
			writeError(w, r, http.StatusBadRequest, "invalid_request", "Image order contains an unknown image")
			return
		}
		img.SortOrder = int32(i)
		ordered = append(ordered, img)
		delete(byID, imageID)
	}
	if len(byID) != 0 {
		writeError(w, r, http.StatusBadRequest, "invalid_request", "Image order contains duplicates")
		return
	}
	p.Images = ordered
	saved, err := svc.EditProduct(r.Context(), id, body.Version, p)
	if err != nil {
		writeCatalogError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"product": saved})
}

type removeImageRequest struct {
	Version int64 `json:"version"`
}

func removeProductImage(w http.ResponseWriter, r *http.Request, svc AdminCatalog, store imagestore.Store) {
	var body removeImageRequest
	if !decodeAuthJSON(w, r, &body) {
		return
	}
	id := catalog.ID(chi.URLParam(r, "id"))
	p, err := svc.GetProduct(r.Context(), id)
	if err != nil {
		writeCatalogError(w, r, err)
		return
	}
	imageID := catalog.ID(chi.URLParam(r, "imageID"))
	filtered := make([]catalog.Image, 0, len(p.Images))
	asset := ""
	found := false
	for _, img := range p.Images {
		if img.ID == imageID {
			found = true
			asset = img.AssetRef
			continue
		}
		img.SortOrder = int32(len(filtered))
		filtered = append(filtered, img)
	}
	if !found {
		writeError(w, r, http.StatusNotFound, "not_found", "The requested image was not found")
		return
	}
	p.Images = filtered
	saved, err := svc.EditProduct(r.Context(), id, body.Version, p)
	if err != nil {
		writeCatalogError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"product": saved})
	if !strings.HasPrefix(asset, "/api/v1/images/") {
		return
	}
	key := strings.TrimPrefix(asset, "/api/v1/images/")
	products, e := svc.ListAllProducts(r.Context())
	if e != nil {
		return
	}
	for _, product := range products {
		for _, img := range product.Images {
			if img.AssetRef == asset {
				return
			}
		}
	}
	_ = store.Delete(key)
}

func serveProductImage(w http.ResponseWriter, r *http.Request, authentication Authentication, options AuthOptions, svc AdminCatalog, store imagestore.Store) {
	key := chi.URLParam(r, "key")
	file, err := store.Read(key)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	asset := "/api/v1/images/" + key
	products, err := svc.ListAllProducts(r.Context())
	if err != nil {
		http.NotFound(w, r)
		return
	}
	visible := false
	adminOnly := true
	for _, p := range products {
		for _, img := range p.Images {
			if img.AssetRef == asset {
				visible = true
				if p.PublicationState == catalog.Published {
					adminOnly = false
				}
			}
		}
	}
	if !visible {
		http.NotFound(w, r)
		return
	}
	if adminOnly {
		api := &authAPI{service: authentication, options: options}
		cookieName, _ := api.cookieNames()
		cookie, cookieErr := r.Cookie(cookieName)
		if cookieErr != nil {
			writeAuthError(w, r, auth.ErrUnauthenticated)
			return
		}
		session, authErr := authentication.Authenticate(r.Context(), cookie.Value)
		if authErr != nil {
			writeAuthError(w, r, authErr)
			return
		}
		if !session.IsAdmin() {
			writeAuthError(w, r, auth.ErrForbidden)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	}
	typ := map[string]string{"jpg": "image/jpeg", "png": "image/png", "webp": "image/webp"}[strings.TrimPrefix(strings.ToLower(key[strings.LastIndex(key, "."):]), ".")]
	if typ == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", typ)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", "inline")
	_, _ = io.Copy(w, file)
}
