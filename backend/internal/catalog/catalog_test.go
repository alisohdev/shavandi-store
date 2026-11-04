package catalog

import (
	"errors"
	"testing"
	"time"
)

const (
	testProductID  ID = "0192bbd1-9e2d-4a45-9baf-37f4d6280001"
	testCategoryID ID = "0192bbd1-9e2d-4a45-9baf-37f4d6280002"
	testVariantID  ID = "0192bbd1-9e2d-4a45-9baf-37f4d6280003"
	testImageID    ID = "0192bbd1-9e2d-4a45-9baf-37f4d6280004"
)

func TestPriceForPrecedence(t *testing.T) {
	productRegular, productSale := int64(1000), int64(800)
	variantRegular, variantSale := int64(1300), int64(900)
	product := Product{RegularPriceToman: productRegular, SalePriceToman: &productSale}

	tests := []struct {
		name        string
		variant     Variant
		wantRegular int64
		wantSale    *int64
	}{
		{name: "inherits product sale", wantRegular: productRegular, wantSale: &productSale},
		{name: "regular override disables product sale", variant: Variant{RegularPriceOverrideToman: &variantRegular}, wantRegular: variantRegular},
		{name: "variant sale overrides inherited sale", variant: Variant{SalePriceOverrideToman: &variantSale}, wantRegular: productRegular, wantSale: &variantSale},
		{name: "both variant overrides", variant: Variant{RegularPriceOverrideToman: &variantRegular, SalePriceOverrideToman: &variantSale}, wantRegular: variantRegular, wantSale: &variantSale},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := PriceFor(product, test.variant)
			if err != nil {
				t.Fatalf("PriceFor returned error: %v", err)
			}
			if got.RegularToman != test.wantRegular || !sameInt64(got.SaleToman, test.wantSale) {
				t.Fatalf("PriceFor = %#v, want regular %d sale %v", got, test.wantRegular, test.wantSale)
			}
		})
	}

	tooHigh := int64(1400)
	if _, err := PriceFor(product, Variant{RegularPriceOverrideToman: &variantRegular, SalePriceOverrideToman: &tooHigh}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid effective sale price error = %v, want ErrInvalid", err)
	}
}

func TestValidateProductRejectsInvalidPricesAndDuplicateActiveCombinations(t *testing.T) {
	valid := Product{
		ID: testProductID, Slug: "test-shirt", Name: "Test Shirt", PrimaryCategory: testCategoryID,
		RegularPriceToman: 1000, PublicationState: Draft, Version: 1,
		Variants: []Variant{{ID: testVariantID, SKU: "SHV-TEST", Active: true}},
	}
	negative := valid
	negative.RegularPriceToman = -1
	duplicate := valid
	duplicate.Variants = []Variant{
		{ID: testVariantID, SKU: "SHV-TEST-A", Active: true},
		{ID: "0192bbd1-9e2d-4a45-9baf-37f4d6280005", SKU: "SHV-TEST-B", Active: true},
	}
	negativeOverride := valid
	negativeOverride.Variants = []Variant{{ID: testVariantID, SKU: "SHV-TEST", RegularPriceOverrideToman: pointer(int64(-1)), Active: true}}
	spacedSKU := valid
	spacedSKU.Variants = []Variant{{ID: testVariantID, SKU: " SHV-TEST", Active: true}}
	unsafeImage := valid
	unsafeImage.Images = []Image{{ID: testImageID, AssetRef: "/images/../private.jpg"}}

	for name, product := range map[string]Product{
		"negative price":                 negative,
		"duplicate default combinations": duplicate,
		"negative variant override":      negativeOverride,
		"whitespace variant SKU":          spacedSKU,
		"unsafe asset path":              unsafeImage,
	} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateProduct(product); !errors.Is(err, ErrInvalid) {
				t.Fatalf("ValidateProduct error = %v, want ErrInvalid", err)
			}
		})
	}
	if err := ValidateProduct(valid); err != nil {
		t.Fatalf("valid product rejected: %v", err)
	}
}

func TestPublishedProductRequiresCompleteData(t *testing.T) {
	product := Product{
		ID: testProductID, Slug: "test-shirt", Name: "Test Shirt", PrimaryCategory: testCategoryID,
		RegularPriceToman: 0, PublicationState: Published, Version: 1,
		Variants: []Variant{{ID: testVariantID, SKU: "SHV-TEST", Active: true}},
	}
	if err := ValidatePublishedProduct(product); !errors.Is(err, ErrInvalid) {
		t.Fatalf("published incomplete product error = %v, want ErrInvalid", err)
	}
	product.Description = "A complete description."
	product.Images = []Image{{ID: testImageID, AssetRef: "/images/products/test-shirt.webp"}}
	if err := ValidatePublishedProduct(product); err != nil {
		t.Fatalf("complete product rejected: %v", err)
	}
}

func TestCursorRoundTrip(t *testing.T) {
	want := Cursor{CreatedAt: time.Date(2026, 11, 3, 17, 0, 0, 123, time.FixedZone("test", 3*60*60+30*60)), ID: testProductID}
	encoded := EncodeCursor(want)
	got, err := DecodeCursor(encoded)
	if err != nil {
		t.Fatalf("DecodeCursor returned error: %v", err)
	}
	if !got.CreatedAt.Equal(want.CreatedAt) || got.ID != want.ID {
		t.Fatalf("cursor = %#v, want %#v", got, want)
	}
	if _, err := DecodeCursor("bad"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("malformed cursor error = %v, want ErrInvalid", err)
	}
}

func pointer(value int64) *int64 {
	return &value
}

func sameInt64(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
