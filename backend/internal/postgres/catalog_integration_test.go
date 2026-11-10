//go:build integration

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shavandi/store/backend/internal/catalog"
	"github.com/shavandi/store/backend/internal/httpapi"
)

func TestRealPostgresCatalogLifecycleAndPublicReads(t *testing.T) {
	pool, _ := openFreshIntegrationDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	repository := NewCatalogRepository(pool)
	service := catalog.NewService(repository)
	handler := httpapi.NewHandler(nil, service, slog.New(slog.NewTextHandler(io.Discard, nil)))

	root, err := service.CreateCategory(ctx, catalog.Category{Name: "Sportswear", Slug: "sportswear", Active: true})
	if err != nil {
		t.Fatalf("create root category: %v", err)
	}
	jerseys, err := service.CreateCategory(ctx, catalog.Category{
		Name: "Football Jerseys", Slug: "football-jerseys", ParentID: &root.ID, DisplayOrder: 1, Active: true,
	})
	if err != nil {
		t.Fatalf("create child category: %v", err)
	}

	basePrice, productSale := int64(100000), int64(85000)
	variantRegular, variantSale := int64(120000), int64(105000)
	custom := catalog.Product{
		Slug: "custom-football-shirt", Name: "Custom Football Shirt", Description: "A made-to-order jersey.",
		PrimaryCategory: jerseys.ID, RegularPriceToman: basePrice, SalePriceToman: &productSale,
		Variants: []catalog.Variant{
			{SKU: "SHV-TEST-A", Size: stringPointer("XL Tall"), Color: stringPointer("Persian Blue"), Active: true},
			{SKU: "SHV-TEST-B", Size: stringPointer("L"), Color: stringPointer("White"), RegularPriceOverrideToman: &variantRegular, Active: true},
			{SKU: "SHV-TEST-C", Size: stringPointer("M"), Color: stringPointer("Red"), RegularPriceOverrideToman: &variantRegular, SalePriceOverrideToman: &variantSale, Active: true},
		},
		Images: []catalog.Image{{AssetRef: "/images/products/custom-shirt.webp", SortOrder: 0, AltText: stringPointer("Blue football shirt")}},
	}
	draft, err := service.CreateProductDraft(ctx, custom)
	if err != nil {
		t.Fatalf("create custom-variant draft: %v", err)
	}
	if draft.PublicationState != catalog.Draft || len(draft.Variants) != 3 || len(draft.Images) != 1 {
		t.Fatalf("created draft = %#v", draft)
	}
	emptyVariants := draft
	emptyVariants.Variants = []catalog.Variant{}
	if _, err := service.EditProduct(ctx, draft.ID, draft.Version, emptyVariants); !errors.Is(err, catalog.ErrInvalid) {
		t.Fatalf("empty variant replacement error = %v, want invalid", err)
	}
	if _, err := repository.GetPublishedProductBySlug(ctx, draft.Slug); !errors.Is(err, catalog.ErrNotFound) {
		t.Fatalf("draft leaked through public lookup: %v", err)
	}
	draftResponse := httptest.NewRecorder()
	handler.ServeHTTP(draftResponse, httptest.NewRequest(http.MethodGet, "/api/v1/products/"+draft.Slug, nil))
	if draftResponse.Code != http.StatusNotFound {
		t.Fatalf("draft leaked through public HTTP route: %d %s", draftResponse.Code, draftResponse.Body.String())
	}

	edit := draft
	edit.Name = "Custom Football Shirt Edited"
	edit.PublicationState = catalog.Published
	variantToChange := draft.Variants[0]
	for _, variant := range draft.Variants {
		if variant.Size != nil && *variant.Size == "XL Tall" {
			variantToChange = variant
			break
		}
	}
	variantChanges := []catalog.Variant{variantToChange}
	variantChanges[0].SKU = ""
	variantChanges[0].Color = stringPointer("Modified Persian Blue")
	edit.Variants = variantChanges
	edited, err := service.EditProduct(ctx, draft.ID, draft.Version, edit)
	if err != nil {
		t.Fatalf("edit draft: %v", err)
	}
	if edited.ID != draft.ID || edited.PublicationState != catalog.Draft || edited.Version != draft.Version+1 {
		t.Fatalf("draft edit changed identity/state/version incorrectly: %#v", edited)
	}
	if len(edited.Variants) != 3 {
		t.Fatalf("partial variant edit removed omitted variants: %#v", edited.Variants)
	}
	var editedVariant catalog.Variant
	for _, variant := range edited.Variants {
		if variant.ID == variantToChange.ID {
			editedVariant = variant
		}
	}
	if editedVariant.ID != variantToChange.ID || editedVariant.SKU != variantToChange.SKU ||
		editedVariant.Color == nil || *editedVariant.Color != "Modified Persian Blue" {
		t.Fatalf("partial variant edit changed its stable identity or lost its explicit change: %#v", editedVariant)
	}
	published, err := service.ChangePublication(ctx, edited.ID, edited.Version, catalog.Published)
	if err != nil {
		t.Fatalf("publish product: %v", err)
	}
	public, err := repository.GetPublishedProductBySlug(ctx, published.Slug)
	if err != nil {
		t.Fatalf("read published product: %v", err)
	}
	if len(public.Variants) != 3 || len(public.Images) != 1 {
		t.Fatalf("public product options = %d variants and %d images", len(public.Variants), len(public.Images))
	}
	inherited := publicVariantForSize(t, public, "XL Tall")
	if inherited.Price.RegularToman != basePrice || inherited.Price.SaleToman == nil || *inherited.Price.SaleToman != productSale {
		t.Fatalf("inherited product sale price = %#v", inherited.Price)
	}
	inheritedWithoutSale := publicVariantForSize(t, public, "L")
	if inheritedWithoutSale.Price.RegularToman != variantRegular || inheritedWithoutSale.Price.SaleToman != nil {
		t.Fatalf("product sale unexpectedly applied to regular-price override: %#v", inheritedWithoutSale.Price)
	}
	variantSaleOverride := publicVariantForSize(t, public, "M")
	if variantSaleOverride.Price.RegularToman != variantRegular || variantSaleOverride.Price.SaleToman == nil || *variantSaleOverride.Price.SaleToman != variantSale {
		t.Fatalf("variant sale-price override = %#v", variantSaleOverride.Price)
	}
	if inherited.Color == nil || *inherited.Color != "Modified Persian Blue" {
		t.Fatalf("custom size/color were not retained: %#v", inherited)
	}
	publicResponse := httptest.NewRecorder()
	handler.ServeHTTP(publicResponse, httptest.NewRequest(http.MethodGet, "/api/v1/products/"+published.Slug, nil))
	if publicResponse.Code != http.StatusOK {
		t.Fatalf("published product HTTP status = %d: %s", publicResponse.Code, publicResponse.Body.String())
	}
	var publicEnvelope struct {
		Product catalog.PublicProduct `json:"product"`
	}
	if err := json.Unmarshal(publicResponse.Body.Bytes(), &publicEnvelope); err != nil {
		t.Fatalf("decode public product response: %v", err)
	}
	if publicEnvelope.Product.ID != published.ID || len(publicEnvelope.Product.Variants) != 3 || strings.Contains(publicResponse.Body.String(), "sku") {
		t.Fatalf("public product HTTP contract = %s", publicResponse.Body.String())
	}
	variantToRetire := published.Variants[0]
	for _, variant := range published.Variants {
		if variant.Size != nil && *variant.Size == "M" {
			variantToRetire = variant
			break
		}
	}
	variantChanges = []catalog.Variant{variantToRetire}
	variantChanges[0].SKU = ""
	variantChanges[0].Active = false
	retiredProduct, err := service.EditProduct(ctx, published.ID, published.Version, catalog.Product{
		Slug: published.Slug, Name: published.Name, Description: published.Description,
		PrimaryCategory: published.PrimaryCategory, RegularPriceToman: published.RegularPriceToman,
		SalePriceToman: published.SalePriceToman, Variants: variantChanges,
	})
	if err != nil {
		t.Fatalf("explicitly retire variant: %v", err)
	}
	if len(retiredProduct.Variants) != 3 {
		t.Fatalf("retiring one variant removed other variants: %#v", retiredProduct.Variants)
	}
	var retiredVariantFound bool
	for _, variant := range retiredProduct.Variants {
		if variant.ID == variantToRetire.ID && !variant.Active {
			retiredVariantFound = true
		}
	}
	if !retiredVariantFound {
		t.Fatalf("explicitly retired variant was not persisted: %#v", retiredProduct.Variants)
	}
	publicAfterRetirement, err := repository.GetPublishedProductBySlug(ctx, retiredProduct.Slug)
	if err != nil {
		t.Fatalf("read product after variant retirement: %v", err)
	}
	if len(publicAfterRetirement.Variants) != 2 {
		t.Fatalf("public reader returned %d variants after one was retired, want 2", len(publicAfterRetirement.Variants))
	}
	published = retiredProduct
	categoriesResponse := httptest.NewRecorder()
	handler.ServeHTTP(categoriesResponse, httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil))
	if categoriesResponse.Code != http.StatusOK || !strings.Contains(categoriesResponse.Body.String(), `"slug":"football-jerseys"`) {
		t.Fatalf("active categories HTTP response = %d %s", categoriesResponse.Code, categoriesResponse.Body.String())
	}

	conflictingSKU := published.Variants[0].SKU
	failedID := catalog.ID("0192bbd1-9e2d-4a45-9baf-37f4d6280012")
	_, err = service.CreateProductDraft(ctx, catalog.Product{
		ID: failedID, Slug: "rollback-check", Name: "Rollback Check", PrimaryCategory: jerseys.ID,
		Variants: []catalog.Variant{{SKU: conflictingSKU, Active: true}},
	})
	if err == nil || !errors.Is(err, catalog.ErrConflict) {
		t.Fatalf("duplicate SKU error = %v, want catalog conflict", err)
	}
	if _, err := repository.GetProduct(ctx, failedID); !errors.Is(err, catalog.ErrNotFound) {
		t.Fatalf("failed product transaction left a product behind: %v", err)
	}
	retry, err := service.CreateProductDraft(ctx, catalog.Product{
		Slug: "rollback-check", Name: "Rollback Check", PrimaryCategory: jerseys.ID,
	})
	if err != nil {
		t.Fatalf("slug was not rolled back after failed transaction: %v", err)
	}
	if len(retry.Variants) != 1 || retry.Variants[0].Size != nil || retry.Variants[0].Color != nil {
		t.Fatalf("variant-free product did not receive one default variant: %#v", retry.Variants)
	}

	oldSlug := published.Slug
	changedSlug := published
	changedSlug.Slug = "custom-football-shirt-v2"
	changedSlug.Name = "Custom Football Shirt v2"
	changedSlug, err = service.EditProduct(ctx, published.ID, published.Version, changedSlug)
	if err != nil {
		t.Fatalf("change current product slug: %v", err)
	}
	if _, err := repository.GetPublishedProductBySlug(ctx, oldSlug); !errors.Is(err, catalog.ErrNotFound) {
		t.Fatalf("historical slug remained publicly current: %v", err)
	}
	claimOldSlug, err := service.CreateProductDraft(ctx, catalog.Product{
		Slug: oldSlug, Name: "Another Product", PrimaryCategory: jerseys.ID,
	})
	if err == nil || !errors.Is(err, catalog.ErrConflict) {
		t.Fatalf("reusing historical slug returned product %#v and error %v", claimOldSlug, err)
	}
	changedSlug.Slug = oldSlug
	changedSlug, err = service.EditProduct(ctx, changedSlug.ID, changedSlug.Version, changedSlug)
	if err != nil {
		t.Fatalf("restore same product's historical slug: %v", err)
	}
	immutableSKU := changedSlug
	immutableSKU.Variants = append([]catalog.Variant(nil), changedSlug.Variants...)
	immutableSKU.Variants[0].SKU = "SHV-CHANGED"
	if _, err := repository.SaveProduct(ctx, immutableSKU, changedSlug.Version); !errors.Is(err, catalog.ErrInvalid) {
		t.Fatalf("variant SKU edit error = %v, want invalid", err)
	}

	beforeDuplicate := changedSlug
	duplicate := changedSlug
	duplicate.Variants = append(append([]catalog.Variant(nil), changedSlug.Variants...), catalog.Variant{
		ID: "0192bbd1-9e2d-4a45-9baf-37f4d6280011", SKU: "SHV-TEST-DUP", Size: stringPointer("XL Tall"),
		Color: stringPointer("Modified Persian Blue"), Active: true,
	})
	if _, err := repository.SaveProduct(ctx, duplicate, changedSlug.Version); !errors.Is(err, catalog.ErrConflict) {
		t.Fatalf("duplicate active combination error = %v, want conflict", err)
	}
	afterDuplicate, err := repository.GetProduct(ctx, changedSlug.ID)
	if err != nil {
		t.Fatalf("read product after rejected duplicate variant: %v", err)
	}
	if afterDuplicate.Version != beforeDuplicate.Version || len(afterDuplicate.Variants) != len(beforeDuplicate.Variants) {
		t.Fatalf("failed variant update was not rolled back: before %#v after %#v", beforeDuplicate, afterDuplicate)
	}

	concurrentBase := afterDuplicate
	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for _, name := range []string{"Concurrent A", "Concurrent B"} {
		name := name
		wait.Add(1)
		go func() {
			defer wait.Done()
			candidate := concurrentBase
			candidate.Name = name
			<-start
			_, err := repository.SaveProduct(ctx, candidate, concurrentBase.Version)
			results <- err
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	successes, conflicts := 0, 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, catalog.ErrVersionConflict):
			conflicts++
		default:
			t.Fatalf("concurrent update error = %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent updates had %d successes and %d version conflicts", successes, conflicts)
	}

	current, err := repository.GetProduct(ctx, published.ID)
	if err != nil {
		t.Fatalf("read current product before hiding: %v", err)
	}
	hidden, err := service.ChangePublication(ctx, current.ID, current.Version, catalog.Hidden)
	if err != nil {
		t.Fatalf("hide product: %v", err)
	}
	if _, err := repository.GetPublishedProductBySlug(ctx, hidden.Slug); !errors.Is(err, catalog.ErrNotFound) {
		t.Fatalf("hidden product leaked through public lookup: %v", err)
	}
	hiddenResponse := httptest.NewRecorder()
	handler.ServeHTTP(hiddenResponse, httptest.NewRequest(http.MethodGet, "/api/v1/products/"+hidden.Slug, nil))
	if hiddenResponse.Code != http.StatusNotFound {
		t.Fatalf("hidden product leaked through public HTTP route: %d %s", hiddenResponse.Code, hiddenResponse.Body.String())
	}
	republished, err := service.ChangePublication(ctx, hidden.ID, hidden.Version, catalog.Published)
	if err != nil {
		t.Fatalf("republish product: %v", err)
	}
	if _, err := repository.GetPublishedProductBySlug(ctx, republished.Slug); err != nil {
		t.Fatalf("republished product is unavailable: %v", err)
	}

	for i := 0; i < 2; i++ {
		_, err := createPublishedProduct(ctx, service, catalog.Product{
			Slug: fmt.Sprintf("pagination-shirt-%d", i), Name: fmt.Sprintf("Pagination Shirt %d", i),
			Description: "A jersey for pagination coverage.", PrimaryCategory: jerseys.ID,
			RegularPriceToman: 50000,
			Images:            []catalog.Image{{AssetRef: fmt.Sprintf("/images/products/page-%d.webp", i)}},
		})
		if err != nil {
			t.Fatalf("create pagination product %d: %v", i, err)
		}
	}
	firstPage, nextCursor, err := service.ListPublishedProducts(ctx, catalog.ProductFilter{CategorySlug: jerseys.Slug, Limit: 1})
	if err != nil {
		t.Fatalf("list first product page: %v", err)
	}
	if len(firstPage) != 1 || nextCursor == "" {
		t.Fatalf("first page has %d products and cursor %q", len(firstPage), nextCursor)
	}
	cursor, err := catalog.DecodeCursor(nextCursor)
	if err != nil {
		t.Fatalf("decode next cursor: %v", err)
	}
	secondPage, _, err := service.ListPublishedProducts(ctx, catalog.ProductFilter{CategorySlug: jerseys.Slug, Limit: 1, After: &cursor})
	if err != nil {
		t.Fatalf("list second product page: %v", err)
	}
	if len(secondPage) != 1 || secondPage[0].ID == firstPage[0].ID {
		t.Fatalf("pagination repeated or omitted rows: first=%#v second=%#v", firstPage, secondPage)
	}
	firstHTTPPage := httptest.NewRecorder()
	handler.ServeHTTP(firstHTTPPage, httptest.NewRequest(http.MethodGet, "/api/v1/products?category=football-jerseys&limit=1", nil))
	var firstHTTPEnvelope struct {
		Products   []catalog.PublicProduct `json:"products"`
		NextCursor string                  `json:"next_cursor"`
	}
	if firstHTTPPage.Code != http.StatusOK || json.Unmarshal(firstHTTPPage.Body.Bytes(), &firstHTTPEnvelope) != nil {
		t.Fatalf("first public HTTP page = %d %s", firstHTTPPage.Code, firstHTTPPage.Body.String())
	}
	if len(firstHTTPEnvelope.Products) != 1 || firstHTTPEnvelope.NextCursor == "" {
		t.Fatalf("first public HTTP page has %d products and cursor %q", len(firstHTTPEnvelope.Products), firstHTTPEnvelope.NextCursor)
	}
	secondHTTPPage := httptest.NewRecorder()
	secondPath := "/api/v1/products?category=football-jerseys&limit=1&cursor=" + firstHTTPEnvelope.NextCursor
	handler.ServeHTTP(secondHTTPPage, httptest.NewRequest(http.MethodGet, secondPath, nil))
	var secondHTTPEnvelope struct {
		Products []catalog.PublicProduct `json:"products"`
	}
	if secondHTTPPage.Code != http.StatusOK || json.Unmarshal(secondHTTPPage.Body.Bytes(), &secondHTTPEnvelope) != nil {
		t.Fatalf("second public HTTP page = %d %s", secondHTTPPage.Code, secondHTTPPage.Body.String())
	}
	if len(secondHTTPEnvelope.Products) != 1 || secondHTTPEnvelope.Products[0].ID == firstHTTPEnvelope.Products[0].ID {
		t.Fatalf("public HTTP pagination repeated or omitted rows: first=%#v second=%#v", firstHTTPEnvelope, secondHTTPEnvelope)
	}

	inactive, err := service.CreateCategory(ctx, catalog.Category{Name: "Inactive parent", Slug: "inactive-parent", Active: true})
	if err != nil {
		t.Fatalf("create category for visibility test: %v", err)
	}
	child, err := service.CreateCategory(ctx, catalog.Category{
		Name: "Active child", Slug: "active-child", ParentID: &inactive.ID, Active: true,
	})
	if err != nil {
		t.Fatalf("create child for visibility test: %v", err)
	}
	visibilityProduct, err := createPublishedProduct(ctx, service, catalog.Product{
		Slug: "category-visibility-shirt", Name: "Category Visibility Shirt", Description: "Visible through active ancestors.",
		PrimaryCategory: child.ID, RegularPriceToman: 75000,
		Images: []catalog.Image{{AssetRef: "/images/products/visibility.webp"}},
	})
	if err != nil {
		t.Fatalf("create product for category visibility: %v", err)
	}
	if _, err := repository.GetPublishedProductBySlug(ctx, visibilityProduct.Slug); err != nil {
		t.Fatalf("product in active category is unavailable: %v", err)
	}
	inactive.Active = false
	if _, err = service.EditCategory(ctx, inactive, inactive.Version); !errors.Is(err, catalog.ErrConflict) {
		t.Fatalf("deactivate category ancestor with published product error = %v, want conflict", err)
	}
	if _, err := repository.GetPublishedProductBySlug(ctx, visibilityProduct.Slug); err != nil {
		t.Fatalf("rejected category deactivation hid a published product: %v", err)
	}
	activeCategories, err := service.ListActiveCategories(ctx)
	if err != nil {
		t.Fatalf("list active categories: %v", err)
	}
	seen := map[catalog.ID]bool{}
	for _, category := range activeCategories {
		seen[category.ID] = true
	}
	if !seen[child.ID] || !seen[inactive.ID] {
		t.Fatalf("rejected category deactivation changed public category visibility: %#v", activeCategories)
	}
}

func createPublishedProduct(ctx context.Context, service *catalog.Service, product catalog.Product) (catalog.Product, error) {
	draft, err := service.CreateProductDraft(ctx, product)
	if err != nil {
		return catalog.Product{}, err
	}
	return service.ChangePublication(ctx, draft.ID, draft.Version, catalog.Published)
}

func stringPointer(value string) *string {
	return &value
}

func publicVariantForSize(t *testing.T, product catalog.PublicProduct, size string) catalog.PublicVariant {
	t.Helper()
	for _, variant := range product.Variants {
		if variant.Size != nil && *variant.Size == size {
			return variant
		}
	}
	t.Fatalf("public product %q has no variant with size %q", product.Slug, size)
	return catalog.PublicVariant{}
}
