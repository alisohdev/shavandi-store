package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shavandi/store/backend/internal/catalog"
)

const (
	catalogMutationLockA int32 = 20261103
	catalogMutationLockB int32 = 170001
)

// CatalogRepository owns catalog persistence. Public callers should receive a
// catalog.Reader instead of this command-capable repository.
type CatalogRepository struct {
	pool *pgxpool.Pool
}

func NewCatalogRepository(pool *pgxpool.Pool) *CatalogRepository {
	return &CatalogRepository{pool: pool}
}

func (r *CatalogRepository) ListAllCategories(ctx context.Context) ([]catalog.Category, error) {
	rows, err := r.pool.Query(ctx, `SELECT id::text, name, slug, parent_id::text, display_order, active, version, created_at, updated_at FROM shavandi.categories ORDER BY display_order, name, id`)
	if err != nil {
		return nil, fmt.Errorf("list all catalog categories: %w", err)
	}
	defer rows.Close()
	result := make([]catalog.Category, 0)
	for rows.Next() {
		category, err := scanCategory(rows)
		if err != nil {
			return nil, mapCatalogDatabaseError(err)
		}
		result = append(result, category)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read catalog categories: %w", err)
	}
	return result, nil
}

func (r *CatalogRepository) GetCategory(ctx context.Context, id catalog.ID) (catalog.Category, error) {
	category, err := scanCategory(r.pool.QueryRow(ctx, `
		SELECT id::text, name, slug, parent_id::text, display_order, active, version, created_at, updated_at
		FROM shavandi.categories
		WHERE id = $1::uuid
	`, string(id)))
	return category, mapCatalogDatabaseError(err)
}

func (r *CatalogRepository) SaveCategory(ctx context.Context, category catalog.Category, expectedVersion int64) (catalog.Category, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return catalog.Category{}, fmt.Errorf("begin category transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockCatalogMutations(ctx, tx); err != nil {
		return catalog.Category{}, err
	}
	if expectedVersion > 0 && !category.Active {
		var hasPublished bool
		if err := tx.QueryRow(ctx, `WITH RECURSIVE subtree(id) AS (SELECT $1::uuid UNION ALL SELECT c.id FROM shavandi.categories c JOIN subtree s ON c.parent_id=s.id) SELECT EXISTS(SELECT 1 FROM shavandi.products p JOIN subtree s ON p.primary_category_id=s.id WHERE p.publication_state='published')`, string(category.ID)).Scan(&hasPublished); err != nil {
			return catalog.Category{}, fmt.Errorf("check published products before category deactivation: %w", err)
		}
		if hasPublished {
			return catalog.Category{}, catalog.ErrConflict
		}
	}
	if err := validateCategoryParent(ctx, tx, category); err != nil {
		return catalog.Category{}, err
	}
	var saved catalog.Category
	if expectedVersion == 0 {
		if category.Version != 1 {
			return catalog.Category{}, fmt.Errorf("%w: new category version must be one", catalog.ErrInvalid)
		}
		saved, err = scanCategory(tx.QueryRow(ctx, `
			INSERT INTO shavandi.categories (id, name, slug, parent_id, display_order, active, version)
			VALUES ($1::uuid, $2, $3, $4::uuid, $5, $6, 1)
			RETURNING id::text, name, slug, parent_id::text, display_order, active, version, created_at, updated_at
		`, string(category.ID), category.Name, category.Slug, idArgument(category.ParentID), category.DisplayOrder, category.Active))
	} else {
		saved, err = scanCategory(tx.QueryRow(ctx, `
			UPDATE shavandi.categories
			SET name = $2, slug = $3, parent_id = $4::uuid, display_order = $5,
			    active = $6, version = version + 1, updated_at = now()
			WHERE id = $1::uuid AND version = $7
			RETURNING id::text, name, slug, parent_id::text, display_order, active, version, created_at, updated_at
		`, string(category.ID), category.Name, category.Slug, idArgument(category.ParentID), category.DisplayOrder, category.Active, expectedVersion))
		if errors.Is(err, pgx.ErrNoRows) {
			err = versionOrNotFound(ctx, tx, "shavandi.categories", category.ID)
		}
	}
	if err != nil {
		return catalog.Category{}, mapCatalogDatabaseError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return catalog.Category{}, fmt.Errorf("commit category transaction: %w", mapCatalogDatabaseError(err))
	}
	return saved, nil
}

func (r *CatalogRepository) GetProduct(ctx context.Context, id catalog.ID) (catalog.Product, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return catalog.Product{}, fmt.Errorf("begin product read: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	product, err := readProduct(ctx, tx, id)
	if err != nil {
		return catalog.Product{}, mapCatalogDatabaseError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return catalog.Product{}, fmt.Errorf("finish product read: %w", err)
	}
	return product, nil
}

func (r *CatalogRepository) ListAllProducts(ctx context.Context) ([]catalog.Product, error) {
	rows, err := r.pool.Query(ctx, `SELECT id::text FROM shavandi.products ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list catalog product IDs: %w", err)
	}
	ids := make([]catalog.ID, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, catalog.ID(id))
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	products := make([]catalog.Product, 0, len(ids))
	for _, id := range ids {
		product, err := r.GetProduct(ctx, id)
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}
	return products, nil
}

func (r *CatalogRepository) SaveProduct(ctx context.Context, product catalog.Product, expectedVersion int64) (catalog.Product, error) {
	if len(product.Variants) == 0 {
		return catalog.Product{}, fmt.Errorf("%w: at least one variant is required", catalog.ErrInvalid)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return catalog.Product{}, fmt.Errorf("begin product transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockCatalogMutations(ctx, tx); err != nil {
		return catalog.Product{}, err
	}
	var categoryAvailable bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM shavandi.effective_active_categories WHERE id = $1::uuid
		)
	`, string(product.PrimaryCategory)).Scan(&categoryAvailable); err != nil {
		return catalog.Product{}, fmt.Errorf("check product category: %w", err)
	}
	if !categoryAvailable {
		return catalog.Product{}, fmt.Errorf("%w: primary category is missing or inactive", catalog.ErrInvalid)
	}
	if expectedVersion == 0 {
		if product.Version != 1 {
			return catalog.Product{}, fmt.Errorf("%w: new product version must be one", catalog.ErrInvalid)
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO shavandi.products
				(id, primary_category_id, name, description, regular_price_toman, sale_price_toman, publication_state, version)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, 1)
		`, string(product.ID), string(product.PrimaryCategory), product.Name, product.Description,
			product.RegularPriceToman, product.SalePriceToman, string(product.PublicationState))
		if err != nil {
			return catalog.Product{}, mapCatalogDatabaseError(err)
		}
	} else {
		command, updateErr := tx.Exec(ctx, `
			UPDATE shavandi.products
			SET primary_category_id = $2::uuid, name = $3, description = $4,
			    regular_price_toman = $5, sale_price_toman = $6,
			    publication_state = $7, version = version + 1, updated_at = now()
			WHERE id = $1::uuid AND version = $8
		`, string(product.ID), string(product.PrimaryCategory), product.Name, product.Description,
			product.RegularPriceToman, product.SalePriceToman, string(product.PublicationState), expectedVersion)
		if updateErr != nil {
			return catalog.Product{}, mapCatalogDatabaseError(updateErr)
		}
		if command.RowsAffected() == 0 {
			return catalog.Product{}, versionOrNotFound(ctx, tx, "shavandi.products", product.ID)
		}
	}
	if err := saveCurrentSlug(ctx, tx, product.ID, product.Slug); err != nil {
		return catalog.Product{}, mapCatalogDatabaseError(err)
	}
	if err := saveVariants(ctx, tx, product); err != nil {
		return catalog.Product{}, mapCatalogDatabaseError(err)
	}
	if err := saveImages(ctx, tx, product); err != nil {
		return catalog.Product{}, mapCatalogDatabaseError(err)
	}
	saved, err := readProduct(ctx, tx, product.ID)
	if err != nil {
		return catalog.Product{}, mapCatalogDatabaseError(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return catalog.Product{}, fmt.Errorf("commit product transaction: %w", mapCatalogDatabaseError(err))
	}
	return saved, nil
}

func lockCatalogMutations(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1, $2)", catalogMutationLockA, catalogMutationLockB); err != nil {
		return fmt.Errorf("lock catalog mutation: %w", err)
	}
	return nil
}

func validateCategoryParent(ctx context.Context, tx pgx.Tx, category catalog.Category) error {
	if category.ParentID == nil {
		return nil
	}
	var cycle bool
	err := tx.QueryRow(ctx, `
		WITH RECURSIVE descendants(id) AS (
			SELECT id FROM shavandi.categories WHERE id = $1::uuid
			UNION
			SELECT child.id
			FROM shavandi.categories AS child
			JOIN descendants AS parent ON child.parent_id = parent.id
		)
		SELECT EXISTS (SELECT 1 FROM descendants WHERE id = $2::uuid)
	`, string(category.ID), string(*category.ParentID)).Scan(&cycle)
	if err != nil {
		return fmt.Errorf("check category tree: %w", err)
	}
	if cycle {
		return fmt.Errorf("%w: category cannot be its own ancestor", catalog.ErrInvalid)
	}
	var parentExists bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM shavandi.categories WHERE id = $1::uuid)", string(*category.ParentID)).Scan(&parentExists); err != nil {
		return fmt.Errorf("check category parent: %w", err)
	}
	if !parentExists {
		return fmt.Errorf("%w: parent category does not exist", catalog.ErrInvalid)
	}
	return nil
}

func saveCurrentSlug(ctx context.Context, tx pgx.Tx, productID catalog.ID, slug string) error {
	if _, err := tx.Exec(ctx, `
		UPDATE shavandi.product_slugs SET is_current = false
		WHERE product_id = $1::uuid AND is_current AND slug <> $2
	`, string(productID), slug); err != nil {
		return err
	}
	var saved string
	err := tx.QueryRow(ctx, `
		INSERT INTO shavandi.product_slugs (slug, product_id, is_current)
		VALUES ($1, $2::uuid, true)
		ON CONFLICT (slug) DO UPDATE
		SET is_current = true
		WHERE shavandi.product_slugs.product_id = EXCLUDED.product_id
	RETURNING slug
	`, slug, string(productID)).Scan(&saved)
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.ErrConflict
	}
	return err
}

func saveVariants(ctx context.Context, tx pgx.Tx, product catalog.Product) error {
	rows, err := tx.Query(ctx, `
		SELECT id::text, sku
		FROM shavandi.variants
		WHERE product_id = $1::uuid
	`, string(product.ID))
	if err != nil {
		return err
	}
	existingSKUs := make(map[string]string)
	for rows.Next() {
		var id, sku string
		if err := rows.Scan(&id, &sku); err != nil {
			rows.Close()
			return err
		}
		existingSKUs[id] = sku
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if _, err := tx.Exec(ctx, "UPDATE shavandi.variants SET active = false WHERE product_id = $1::uuid", string(product.ID)); err != nil {
		return err
	}
	for _, variant := range product.Variants {
		if existingSKU, exists := existingSKUs[strings.ToLower(string(variant.ID))]; exists && existingSKU != variant.SKU {
			return fmt.Errorf("%w: variant SKUs are immutable", catalog.ErrInvalid)
		}
		command, err := tx.Exec(ctx, `
			UPDATE shavandi.variants
			SET sku = $3, size = $4, color = $5, regular_price_override_toman = $6,
			    sale_price_override_toman = $7, active = $8, updated_at = now()
			WHERE id = $1::uuid AND product_id = $2::uuid
		`, string(variant.ID), string(product.ID), variant.SKU, variant.Size, variant.Color,
			variant.RegularPriceOverrideToman, variant.SalePriceOverrideToman, variant.Active)
		if err != nil {
			return err
		}
		if command.RowsAffected() == 0 {
			if _, err := tx.Exec(ctx, `
				INSERT INTO shavandi.variants
					(id, product_id, sku, size, color, regular_price_override_toman, sale_price_override_toman, active)
				VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8)
			`, string(variant.ID), string(product.ID), variant.SKU, variant.Size, variant.Color,
				variant.RegularPriceOverrideToman, variant.SalePriceOverrideToman, variant.Active); err != nil {
				return err
			}
		}
	}
	return nil
}

func saveImages(ctx context.Context, tx pgx.Tx, product catalog.Product) error {
	if _, err := tx.Exec(ctx, "DELETE FROM shavandi.product_images WHERE product_id = $1::uuid", string(product.ID)); err != nil {
		return err
	}
	for _, image := range product.Images {
		if _, err := tx.Exec(ctx, `
			INSERT INTO shavandi.product_images (id, product_id, asset_ref, sort_order, alt_text)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5)
		`, string(image.ID), string(product.ID), image.AssetRef, image.SortOrder, image.AltText); err != nil {
			return err
		}
	}
	return nil
}

func scanCategory(row pgx.Row) (catalog.Category, error) {
	var id string
	var parentID *string
	var category catalog.Category
	err := row.Scan(&id, &category.Name, &category.Slug, &parentID, &category.DisplayOrder,
		&category.Active, &category.Version, &category.CreatedAt, &category.UpdatedAt)
	if err != nil {
		return catalog.Category{}, err
	}
	category.ID = catalog.ID(id)
	if parentID != nil {
		parent := catalog.ID(*parentID)
		category.ParentID = &parent
	}
	return category, nil
}

func readProduct(ctx context.Context, queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, id catalog.ID) (catalog.Product, error) {
	var product catalog.Product
	var productID, categoryID string
	var salePrice *int64
	err := queryer.QueryRow(ctx, `
		SELECT product.id::text, current_slug.slug, product.name, product.description,
		       product.primary_category_id::text, product.regular_price_toman,
		       product.sale_price_toman, product.publication_state, product.version,
		       product.created_at, product.updated_at
		FROM shavandi.products AS product
		JOIN shavandi.product_slugs AS current_slug
		  ON current_slug.product_id = product.id AND current_slug.is_current
		WHERE product.id = $1::uuid
	`, string(id)).Scan(&productID, &product.Slug, &product.Name, &product.Description,
		&categoryID, &product.RegularPriceToman, &salePrice, &product.PublicationState,
		&product.Version, &product.CreatedAt, &product.UpdatedAt)
	if err != nil {
		return catalog.Product{}, err
	}
	product.ID = catalog.ID(productID)
	product.PrimaryCategory = catalog.ID(categoryID)
	product.SalePriceToman = salePrice

	rows, err := queryer.Query(ctx, `
		SELECT id::text, sku, size, color, regular_price_override_toman,
		       sale_price_override_toman, active
		FROM shavandi.variants
		WHERE product_id = $1::uuid
		ORDER BY id
	`, string(id))
	if err != nil {
		return catalog.Product{}, err
	}
	product.Variants = make([]catalog.Variant, 0)
	for rows.Next() {
		var variant catalog.Variant
		if err := rows.Scan(&variant.ID, &variant.SKU, &variant.Size, &variant.Color,
			&variant.RegularPriceOverrideToman, &variant.SalePriceOverrideToman, &variant.Active); err != nil {
			rows.Close()
			return catalog.Product{}, err
		}
		product.Variants = append(product.Variants, variant)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return catalog.Product{}, err
	}
	rows.Close()

	rows, err = queryer.Query(ctx, `
		SELECT id::text, asset_ref, sort_order, alt_text
		FROM shavandi.product_images
		WHERE product_id = $1::uuid
		ORDER BY sort_order, id
	`, string(id))
	if err != nil {
		return catalog.Product{}, err
	}
	product.Images = make([]catalog.Image, 0)
	for rows.Next() {
		var image catalog.Image
		if err := rows.Scan(&image.ID, &image.AssetRef, &image.SortOrder, &image.AltText); err != nil {
			rows.Close()
			return catalog.Product{}, err
		}
		product.Images = append(product.Images, image)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return catalog.Product{}, err
	}
	rows.Close()
	return product, nil
}

func idArgument(id *catalog.ID) any {
	if id == nil {
		return nil
	}
	return string(*id)
}

func versionOrNotFound(ctx context.Context, tx pgx.Tx, table string, id catalog.ID) error {
	if table != "shavandi.products" && table != "shavandi.categories" {
		return errors.New("invalid catalog version table")
	}
	var exists bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM "+table+" WHERE id = $1::uuid)", string(id)).Scan(&exists); err != nil {
		return fmt.Errorf("check catalog record version: %w", err)
	}
	if !exists {
		return catalog.ErrNotFound
	}
	return catalog.ErrVersionConflict
}

func mapCatalogDatabaseError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return catalog.ErrNotFound
	}
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) {
		switch pgError.Code {
		case "23505":
			return catalog.ErrConflict
		case "23502", "23503", "23514", "22P02":
			return fmt.Errorf("%w: database constraint rejected catalog data", catalog.ErrInvalid)
		}
	}
	if errors.Is(err, catalog.ErrNotFound) || errors.Is(err, catalog.ErrConflict) ||
		errors.Is(err, catalog.ErrVersionConflict) || errors.Is(err, catalog.ErrInvalid) {
		return err
	}
	return err
}
