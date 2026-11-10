package httpapi

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shavandi/store/backend/internal/auth"
	"github.com/shavandi/store/backend/internal/catalog"
	"github.com/shavandi/store/backend/internal/imagestore"
)

func TestImageUploadAndPublishedRetrieval(t *testing.T) {
	var pngData bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&pngData, img); err != nil {
		t.Fatal(err)
	}
	product := catalog.Product{ID: "0192bbd1-9e2d-4a45-9baf-37f4d6280201", Slug: "image-test", Name: "Image test", Description: "Details", PrimaryCategory: "0192bbd1-9e2d-4a45-9baf-37f4d6280202", RegularPriceToman: 1000, Version: 1, PublicationState: catalog.Published, Variants: []catalog.Variant{{ID: "0192bbd1-9e2d-4a45-9baf-37f4d6280203", SKU: "sku", Active: true}}}
	reader := &stubCatalogReader{adminProduct: product, adminProducts: []catalog.Product{product}}
	authn := &fakeAuthentication{session: auth.Session{User: auth.User{Role: "admin", Active: true}, Method: "admin_password_sms"}}
	store, err := imagestore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(nil, reader, testLogger(), WithAuthentication(authn, AuthOptions{AllowedOrigins: []string{"https://store.example"}, CookieSecure: true}), WithAdminCatalog(reader), WithImageStore(store))
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("version", "1")
	part, err := writer.CreateFormFile("images", "../../unsafe.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(pngData.Bytes())
	_ = writer.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products/"+string(product.ID)+"/images", &body)
	req.Header.Set("Origin", "https://store.example")
	req.Header.Set("X-CSRF-Token", "signed-csrf")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "__Host-shavandi_csrf", Value: "signed-csrf"})
	req.AddCookie(&http.Cookie{Name: "__Host-shavandi_session", Value: "private-session-token"})
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusCreated || len(reader.adminProduct.Images) != 1 {
		t.Fatalf("upload=%d %s", res.Code, res.Body.String())
	}
	ref := reader.adminProduct.Images[0].AssetRef
	if !strings.HasPrefix(ref, "/api/v1/images/") || strings.Contains(ref, "unsafe") {
		t.Fatalf("asset ref=%q", ref)
	}
	imageResponse := httptest.NewRecorder()
	handler.ServeHTTP(imageResponse, httptest.NewRequest(http.MethodGet, ref, nil))
	if imageResponse.Code != http.StatusOK || imageResponse.Header().Get("Content-Type") != "image/png" || imageResponse.Header().Get("X-Content-Type-Options") != "nosniff" || !bytes.Equal(imageResponse.Body.Bytes(), pngData.Bytes()) {
		t.Fatalf("image response=%d headers=%v", imageResponse.Code, imageResponse.Header())
	}
}

func TestImageReorderRemovalAndPrivateAccess(t *testing.T) {
	store, err := imagestore.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	key, err := store.Save(strings.NewReader("image-bytes"), "png")
	if err != nil {
		t.Fatal(err)
	}
	asset := "/api/v1/images/" + key
	p := catalog.Product{ID: "0192bbd1-9e2d-4a45-9baf-37f4d6280201", Version: 1, PublicationState: catalog.Draft, Images: []catalog.Image{{ID: "first", AssetRef: asset}, {ID: "second", AssetRef: "/images/other.png", SortOrder: 1}}}
	reader := &stubCatalogReader{adminProduct: p, adminProducts: []catalog.Product{p}}
	a := &fakeAuthentication{session: auth.Session{User: auth.User{Role: "admin", Active: true}, Method: "admin_password_sms"}}
	h := NewHandler(nil, reader, testLogger(), WithAuthentication(a, AuthOptions{AllowedOrigins: []string{"https://store.example"}, CookieSecure: true}), WithAdminCatalog(reader), WithImageStore(store))
	call := func(method, path, body string, authenticated bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Origin", "https://store.example")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", "signed-csrf")
		req.AddCookie(&http.Cookie{Name: "__Host-shavandi_csrf", Value: "signed-csrf"})
		if authenticated {
			req.AddCookie(&http.Cookie{Name: "__Host-shavandi_session", Value: "private-session-token"})
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w
	}
	if w := call("GET", asset, "", false); w.Code != 401 {
		t.Fatalf("private image=%d", w.Code)
	}
	if w := call("GET", asset, "", true); w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("admin image=%d", w.Code)
	}
	a.session.User.Role = "customer"
	if w := call("GET", asset, "", true); w.Code != 403 {
		t.Fatalf("customer image=%d", w.Code)
	}
	a.session.User.Role = "admin"
	base := "/api/v1/admin/products/" + string(p.ID) + "/images"
	if w := call("PATCH", base, `{"version":1,"image_ids":["first","first"]}`, true); w.Code != 400 {
		t.Fatalf("duplicate order=%d", w.Code)
	}
	if w := call("PATCH", base, `{"version":1,"image_ids":["second","first"]}`, true); w.Code != 200 || reader.adminProduct.Images[0].ID != "second" {
		t.Fatalf("reorder=%d %s", w.Code, w.Body.String())
	}
	if w := call("DELETE", base+"/first", `{"version":1}`, true); w.Code != 409 {
		t.Fatalf("stale removal=%d", w.Code)
	}
	if f, e := store.Read(key); e != nil {
		t.Fatal("stale removal deleted file")
	} else {
		f.Close()
	}
	if w := call("DELETE", base+"/first", `{"version":2}`, true); w.Code != 200 || len(reader.adminProduct.Images) != 1 {
		t.Fatalf("removal=%d %s", w.Code, w.Body.String())
	}
	if f, e := store.Read(key); e == nil {
		f.Close()
		t.Fatal("removed image file remains")
	}
}
