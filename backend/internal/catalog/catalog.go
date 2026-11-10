package catalog

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ID string

type PublicationState string

const (
	Draft     PublicationState = "draft"
	Published PublicationState = "published"
	Hidden    PublicationState = "hidden"
)

var (
	ErrNotFound          = errors.New("catalog record not found")
	ErrConflict          = errors.New("catalog record conflicts with an existing record")
	ErrVersionConflict   = errors.New("catalog record was changed by another editor")
	ErrInvalid           = errors.New("catalog record is invalid")
	ErrInvalidTransition = errors.New("publication transition is not allowed")
	ErrNotPublished      = errors.New("product is not publicly available")

	categorySlugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	productSlugPattern  = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	uuidPattern         = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
)

type Category struct {
	ID           ID
	Name         string
	Slug         string
	ParentID     *ID
	DisplayOrder int32
	Active       bool
	Version      int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Product struct {
	ID                ID
	Slug              string
	Name              string
	Description       string
	PrimaryCategory   ID
	RegularPriceToman int64
	SalePriceToman    *int64
	PublicationState  PublicationState
	Version           int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Variants          []Variant
	Images            []Image
}

type Variant struct {
	StockOnHand               int   `json:"StockOnHand"`
	StockVersion              int64 `json:"StockVersion"`
	ID                        ID
	SKU                       string
	Size                      *string
	Color                     *string
	RegularPriceOverrideToman *int64
	SalePriceOverrideToman    *int64
	Active                    bool
}

type Image struct {
	ID        ID
	AssetRef  string
	SortOrder int32
	AltText   *string
}

type EffectivePrice struct {
	RegularToman int64  `json:"regular_toman"`
	SaleToman    *int64 `json:"sale_toman"`
}

type PublicVariant struct {
	AvailableQuantity int            `json:"available_quantity"`
	ID                ID             `json:"id"`
	Size              *string        `json:"size"`
	Color             *string        `json:"color"`
	Price             EffectivePrice `json:"price"`
}

type PublicImage struct {
	ID        ID      `json:"id"`
	AssetRef  string  `json:"asset_ref"`
	SortOrder int32   `json:"sort_order"`
	AltText   *string `json:"alt_text"`
}

type PublicProduct struct {
	ID                ID              `json:"id"`
	Slug              string          `json:"slug"`
	Name              string          `json:"name"`
	Description       string          `json:"description"`
	CategoryID        ID              `json:"category_id"`
	RegularPriceToman int64           `json:"regular_price_toman"`
	SalePriceToman    *int64          `json:"sale_price_toman"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	Variants          []PublicVariant `json:"variants"`
	Images            []PublicImage   `json:"images"`
}

type ProductFilter struct {
	CategorySlug string
	After        *Cursor
	Limit        int
}

type Cursor struct {
	CreatedAt time.Time
	ID        ID
}

type ProductPage struct {
	Products   []PublicProduct `json:"products"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

// Reader is the public, read-only boundary passed to HTTP handlers.
type Reader interface {
	ListActiveCategories(context.Context) ([]Category, error)
	ListAllCategories(context.Context) ([]Category, error)
	ListPublishedProducts(context.Context, ProductFilter) ([]PublicProduct, string, error)
	GetPublishedProductBySlug(context.Context, string) (PublicProduct, error)
}

// Repository contains write operations and private management reads. It is not
// passed to the public HTTP layer.
type Repository interface {
	Reader
	GetCategory(context.Context, ID) (Category, error)
	SaveCategory(context.Context, Category, int64) (Category, error)
	GetProduct(context.Context, ID) (Product, error)
	ListAllProducts(context.Context) ([]Product, error)
	SaveProduct(context.Context, Product, int64) (Product, error)
}

// ListAllProducts is restricted to the authenticated administration API.
func (s *Service) ListAllProducts(ctx context.Context) ([]Product, error) {
	return s.repository.ListAllProducts(ctx)
}

// GetProduct returns the management representation, including unpublished state.
func (s *Service) GetProduct(ctx context.Context, id ID) (Product, error) {
	return s.repository.GetProduct(ctx, id)
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) ListActiveCategories(ctx context.Context) ([]Category, error) {
	return s.repository.ListActiveCategories(ctx)
}

func (s *Service) ListAllCategories(ctx context.Context) ([]Category, error) {
	return s.repository.ListAllCategories(ctx)
}

func (s *Service) GetCategory(ctx context.Context, id ID) (Category, error) {
	return s.repository.GetCategory(ctx, id)
}

func (s *Service) ListPublishedProducts(ctx context.Context, filter ProductFilter) ([]PublicProduct, string, error) {
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if filter.Limit < 1 || filter.Limit > 50 {
		return nil, "", invalid("page size must be between 1 and 50")
	}
	if filter.CategorySlug != "" && !categorySlugPattern.MatchString(filter.CategorySlug) {
		return nil, "", invalid("category slug is malformed")
	}
	return s.repository.ListPublishedProducts(ctx, filter)
}

func (s *Service) GetPublishedProductBySlug(ctx context.Context, slug string) (PublicProduct, error) {
	if !productSlugPattern.MatchString(slug) {
		return PublicProduct{}, ErrNotFound
	}
	return s.repository.GetPublishedProductBySlug(ctx, slug)
}

func (s *Service) CreateCategory(ctx context.Context, category Category) (Category, error) {
	if category.ID == "" {
		category.ID = newID()
	}
	category.Version = 1
	if err := ValidateCategory(category); err != nil {
		return Category{}, err
	}
	return s.repository.SaveCategory(ctx, category, 0)
}

func (s *Service) EditCategory(ctx context.Context, category Category, expectedVersion int64) (Category, error) {
	if expectedVersion < 1 {
		return Category{}, fmt.Errorf("%w: expected category version must be positive", ErrInvalid)
	}
	category.Version = expectedVersion + 1
	if err := ValidateCategory(category); err != nil {
		return Category{}, err
	}
	return s.repository.SaveCategory(ctx, category, expectedVersion)
}

func (s *Service) CreateProductDraft(ctx context.Context, product Product) (Product, error) {
	if product.ID == "" {
		product.ID = newID()
	}
	product.PublicationState = Draft
	product.Version = 1
	ensureVariantIDs(&product)
	if err := ValidateProduct(product); err != nil {
		return Product{}, err
	}
	return s.repository.SaveProduct(ctx, product, 0)
}

// EditProduct keeps the current publication state. An edit cannot publish a
// draft or republish a hidden product by setting a field in the submitted form.
func (s *Service) EditProduct(ctx context.Context, id ID, expectedVersion int64, changes Product) (Product, error) {
	if expectedVersion < 1 {
		return Product{}, fmt.Errorf("%w: expected product version must be positive", ErrInvalid)
	}
	current, err := s.repository.GetProduct(ctx, id)
	if err != nil {
		return Product{}, err
	}
	if current.Version != expectedVersion {
		return Product{}, ErrVersionConflict
	}
	if changes.Variants != nil && len(changes.Variants) == 0 {
		return Product{}, invalid("product edits must retain or supply at least one variant")
	}
	changes.ID = current.ID
	changes.Version = current.Version + 1
	changes.PublicationState = current.PublicationState
	if changes.Variants == nil {
		changes.Variants = cloneVariants(current.Variants)
	} else {
		changes.Variants, err = mergeVariantChanges(current.Variants, changes.Variants)
		if err != nil {
			return Product{}, err
		}
	}
	if changes.Images == nil {
		changes.Images = cloneImages(current.Images)
	}
	ensureImageIDs(&changes)
	if err := ValidateProduct(changes); err != nil {
		return Product{}, err
	}
	if changes.PublicationState == Published {
		if err := ValidatePublishedProduct(changes); err != nil {
			return Product{}, err
		}
	}
	return s.repository.SaveProduct(ctx, changes, expectedVersion)
}

func (s *Service) ChangePublication(ctx context.Context, id ID, expectedVersion int64, target PublicationState) (Product, error) {
	if expectedVersion < 1 {
		return Product{}, fmt.Errorf("%w: expected product version must be positive", ErrInvalid)
	}
	product, err := s.repository.GetProduct(ctx, id)
	if err != nil {
		return Product{}, err
	}
	if product.Version != expectedVersion {
		return Product{}, ErrVersionConflict
	}
	if !allowedTransition(product.PublicationState, target) {
		return Product{}, ErrInvalidTransition
	}
	if target == Published {
		if err := ValidatePublishedProduct(product); err != nil {
			return Product{}, err
		}
	}
	product.PublicationState = target
	product.Version++
	return s.repository.SaveProduct(ctx, product, expectedVersion)
}

func ValidateCategory(category Category) error {
	if category.ID == "" || !uuidPattern.MatchString(string(category.ID)) {
		return invalid("category id must be a UUID")
	}
	if strings.TrimSpace(category.Name) == "" {
		return invalid("category name is required")
	}
	if !categorySlugPattern.MatchString(category.Slug) {
		return invalid("category slug must use lowercase letters, numbers, and hyphens")
	}
	if category.DisplayOrder < 0 {
		return invalid("category display order cannot be negative")
	}
	if category.Version < 1 {
		return invalid("category version must be positive")
	}
	if category.ParentID != nil && (category.ID == *category.ParentID || !uuidPattern.MatchString(string(*category.ParentID))) {
		return invalid("category parent must be another valid category")
	}
	return nil
}

func ValidateProduct(product Product) error {
	if product.ID == "" || !uuidPattern.MatchString(string(product.ID)) {
		return invalid("product id must be a UUID")
	}
	if !productSlugPattern.MatchString(product.Slug) {
		return invalid("product slug must use lowercase letters, numbers, and hyphens")
	}
	if strings.TrimSpace(product.Name) == "" {
		return invalid("product name is required")
	}
	if !uuidPattern.MatchString(string(product.PrimaryCategory)) {
		return invalid("primary category must be a valid category id")
	}
	if product.RegularPriceToman < 0 {
		return invalid("regular price cannot be negative")
	}
	if product.SalePriceToman != nil && (*product.SalePriceToman < 0 || *product.SalePriceToman >= product.RegularPriceToman) {
		return invalid("product sale price must be nonnegative and below the regular price")
	}
	if product.Version < 1 {
		return invalid("product version must be positive")
	}
	if product.PublicationState != Draft && product.PublicationState != Published && product.PublicationState != Hidden {
		return invalid("publication state is invalid")
	}
	if len(product.Variants) == 0 {
		return invalid("at least one variant is required")
	}
	seenIDs := make(map[ID]struct{}, len(product.Variants))
	seenSKUs := make(map[string]struct{}, len(product.Variants))
	seenCombinations := make(map[string]struct{}, len(product.Variants))
	activeVariants := 0
	for _, variant := range product.Variants {
		if variant.ID == "" || !uuidPattern.MatchString(string(variant.ID)) {
			return invalid("variant id must be a UUID")
		}
		if _, exists := seenIDs[variant.ID]; exists {
			return invalid("variant ids must be unique within a product")
		}
		seenIDs[variant.ID] = struct{}{}
		if strings.TrimSpace(variant.SKU) == "" {
			return invalid("variant SKU is required")
		}
		if strings.TrimSpace(variant.SKU) != variant.SKU {
			return invalid("variant SKU cannot begin or end with whitespace")
		}
		if _, exists := seenSKUs[variant.SKU]; exists {
			return invalid("variant SKUs must be unique within a product")
		}
		seenSKUs[variant.SKU] = struct{}{}
		if variant.Size != nil && strings.TrimSpace(*variant.Size) == "" {
			return invalid("variant size cannot be blank")
		}
		if variant.Color != nil && strings.TrimSpace(*variant.Color) == "" {
			return invalid("variant color cannot be blank")
		}
		if variant.RegularPriceOverrideToman != nil && *variant.RegularPriceOverrideToman < 0 {
			return invalid("variant regular-price override cannot be negative")
		}
		if variant.SalePriceOverrideToman != nil && *variant.SalePriceOverrideToman < 0 {
			return invalid("variant sale-price override cannot be negative")
		}
		if variant.Active {
			activeVariants++
			combination := combinationKey(variant.Size, variant.Color)
			if _, exists := seenCombinations[combination]; exists {
				return invalid("active variants must have unique size/color combinations")
			}
			seenCombinations[combination] = struct{}{}
		}
		price, err := PriceFor(product, variant)
		if err != nil {
			return err
		}
		if price.SaleToman != nil && *price.SaleToman >= price.RegularToman {
			return invalid("effective variant sale price must be below its regular price")
		}
	}
	if activeVariants == 0 {
		return invalid("at least one variant must be active")
	}
	for _, image := range product.Images {
		if image.ID == "" || !uuidPattern.MatchString(string(image.ID)) {
			return invalid("image id must be a UUID")
		}
		if !strings.HasPrefix(image.AssetRef, "/images/") || strings.Contains(image.AssetRef, "..") || strings.ContainsAny(image.AssetRef, "\\?#") {
			return invalid("image asset reference must be a safe /images/ path")
		}
		if image.SortOrder < 0 {
			return invalid("image sort order cannot be negative")
		}
	}
	return nil
}

func ValidatePublishedProduct(product Product) error {
	if err := ValidateProduct(product); err != nil {
		return err
	}
	if strings.TrimSpace(product.Description) == "" {
		return invalid("published products require a description")
	}
	if len(product.Images) == 0 {
		return invalid("published products require at least one image")
	}
	return nil
}

// PriceFor applies variant sale overrides first. A product sale is inherited
// only by variants that do not override the product's regular price.
func PriceFor(product Product, variant Variant) (EffectivePrice, error) {
	regular := product.RegularPriceToman
	if variant.RegularPriceOverrideToman != nil {
		regular = *variant.RegularPriceOverrideToman
	}
	var sale *int64
	if variant.SalePriceOverrideToman != nil {
		sale = variant.SalePriceOverrideToman
	} else if variant.RegularPriceOverrideToman == nil {
		sale = product.SalePriceToman
	}
	if regular < 0 || (sale != nil && (*sale < 0 || *sale >= regular)) {
		return EffectivePrice{}, invalid("effective sale price must be nonnegative and below the effective regular price")
	}
	return EffectivePrice{RegularToman: regular, SaleToman: cloneInt64(sale)}, nil
}

func ToPublicProduct(product Product) (PublicProduct, error) {
	result := PublicProduct{
		ID: product.ID, Slug: product.Slug, Name: product.Name, Description: product.Description,
		CategoryID: product.PrimaryCategory, RegularPriceToman: product.RegularPriceToman,
		SalePriceToman: cloneInt64(product.SalePriceToman), CreatedAt: product.CreatedAt, UpdatedAt: product.UpdatedAt,
		Variants: make([]PublicVariant, 0, len(product.Variants)), Images: make([]PublicImage, 0, len(product.Images)),
	}
	for _, variant := range product.Variants {
		if !variant.Active {
			continue
		}
		price, err := PriceFor(product, variant)
		if err != nil {
			return PublicProduct{}, err
		}
		result.Variants = append(result.Variants, PublicVariant{AvailableQuantity: variant.StockOnHand, ID: variant.ID, Size: cloneString(variant.Size), Color: cloneString(variant.Color), Price: price})
	}
	for _, image := range product.Images {
		result.Images = append(result.Images, PublicImage{ID: image.ID, AssetRef: image.AssetRef, SortOrder: image.SortOrder, AltText: cloneString(image.AltText)})
	}
	return result, nil
}

func EncodeCursor(cursor Cursor) string {
	payload := strconv.FormatInt(cursor.CreatedAt.UTC().UnixNano(), 10) + ":" + string(cursor.ID)
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}

func DecodeCursor(value string) (Cursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return Cursor{}, invalid("page cursor is malformed")
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 || !uuidPattern.MatchString(parts[1]) {
		return Cursor{}, invalid("page cursor is malformed")
	}
	nanos, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return Cursor{}, invalid("page cursor is malformed")
	}
	return Cursor{CreatedAt: time.Unix(0, nanos).UTC(), ID: ID(parts[1])}, nil
}

func allowedTransition(current, target PublicationState) bool {
	switch current {
	case Draft:
		return target == Published
	case Published:
		return target == Hidden
	case Hidden:
		return target == Published
	default:
		return false
	}
}

func ensureVariantIDs(product *Product) {
	if len(product.Variants) == 0 {
		product.Variants = []Variant{{ID: newID(), SKU: newSKU(), Active: true}}
	} else {
		for i := range product.Variants {
			if product.Variants[i].ID == "" {
				product.Variants[i].ID = newID()
			}
			if strings.TrimSpace(product.Variants[i].SKU) == "" {
				product.Variants[i].SKU = newSKU()
			}
		}
	}
	ensureImageIDs(product)
}

func ensureImageIDs(product *Product) {
	for i := range product.Images {
		if product.Images[i].ID == "" {
			product.Images[i].ID = newID()
		}
	}
}

func mergeVariantChanges(current, changes []Variant) ([]Variant, error) {
	merged := cloneVariants(current)
	indexByID := make(map[string]int, len(merged))
	for index, variant := range merged {
		indexByID[strings.ToLower(string(variant.ID))] = index
	}
	seen := make(map[string]struct{}, len(changes))
	for _, change := range changes {
		if change.ID == "" {
			change.ID = newID()
			if strings.TrimSpace(change.SKU) == "" {
				change.SKU = newSKU()
			}
		} else if index, exists := indexByID[strings.ToLower(string(change.ID))]; exists {
			if strings.TrimSpace(change.SKU) == "" {
				change.SKU = merged[index].SKU
			}
		} else if strings.TrimSpace(change.SKU) == "" {
			change.SKU = newSKU()
		}
		key := strings.ToLower(string(change.ID))
		if _, duplicate := seen[key]; duplicate {
			return nil, invalid("a variant can only be changed once per product edit")
		}
		seen[key] = struct{}{}
		if index, exists := indexByID[key]; exists {
			merged[index] = change
			continue
		}
		indexByID[key] = len(merged)
		merged = append(merged, change)
	}
	return merged, nil
}

func cloneVariants(variants []Variant) []Variant {
	cloned := make([]Variant, len(variants))
	for index, variant := range variants {
		cloned[index] = Variant{
			ID: variant.ID, SKU: variant.SKU, Size: cloneString(variant.Size), Color: cloneString(variant.Color),
			RegularPriceOverrideToman: cloneInt64(variant.RegularPriceOverrideToman),
			SalePriceOverrideToman:    cloneInt64(variant.SalePriceOverrideToman), Active: variant.Active,
		}
	}
	return cloned
}

func cloneImages(images []Image) []Image {
	cloned := make([]Image, len(images))
	for index, image := range images {
		cloned[index] = Image{ID: image.ID, AssetRef: image.AssetRef, SortOrder: image.SortOrder, AltText: cloneString(image.AltText)}
	}
	return cloned
}

func combinationKey(size, color *string) string {
	value := func(input *string) string {
		if input == nil {
			return "<nil>"
		}
		return "<value>" + *input
	}
	return value(size) + "\x00" + value(color)
}

func invalid(message string) error {
	return fmt.Errorf("%w: %s", ErrInvalid, message)
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func newID() ID {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	raw[6] = raw[6]&0x0f | 0x40
	raw[8] = raw[8]&0x3f | 0x80
	text := hex.EncodeToString(raw[:])
	return ID(text[:8] + "-" + text[8:12] + "-" + text[12:16] + "-" + text[16:20] + "-" + text[20:])
}

func newSKU() string {
	var raw [6]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return "SHV-" + strings.ToUpper(hex.EncodeToString(raw[:]))
}
