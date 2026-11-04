package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shavandi/store/backend/internal/catalog"
	"github.com/shavandi/store/backend/internal/dbgen"
)

type publicBase struct {
	id          pgtype.UUID
	slug        string
	name        string
	description string
	categoryID  pgtype.UUID
	regular     int64
	sale        pgtype.Int8
	createdAt   pgtype.Timestamptz
	updatedAt   pgtype.Timestamptz
}

func (r *CatalogRepository) ListActiveCategories(ctx context.Context) ([]catalog.Category, error) {
	rows, err := dbgen.New(r.pool).ListActiveCategories(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active catalog categories: %w", err)
	}
	categories := make([]catalog.Category, 0, len(rows))
	for _, row := range rows {
		category := catalog.Category{
			ID: catalog.ID(uuidString(row.ID)), Name: row.Name, Slug: row.Slug,
			DisplayOrder: row.DisplayOrder, Active: row.Active, Version: row.Version,
			CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
		}
		if row.ParentID.Valid {
			parentID := catalog.ID(uuidString(row.ParentID))
			category.ParentID = &parentID
		}
		categories = append(categories, category)
	}
	return categories, nil
}

func (r *CatalogRepository) ListPublishedProducts(ctx context.Context, filter catalog.ProductFilter) ([]catalog.PublicProduct, string, error) {
	categorySlug := pgtype.Text{}
	if filter.CategorySlug != "" {
		categorySlug = pgtype.Text{String: filter.CategorySlug, Valid: true}
	}
	afterCreatedAt, afterID := pgtype.Timestamptz{}, pgtype.UUID{}
	if filter.After != nil {
		afterCreatedAt = pgtype.Timestamptz{Time: filter.After.CreatedAt, Valid: true}
		parsed, err := uuidValue(filter.After.ID)
		if err != nil {
			return nil, "", catalog.ErrInvalid
		}
		afterID = parsed
	}
	limit := filter.Limit
	if limit < 1 || limit > 50 {
		return nil, "", catalog.ErrInvalid
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, "", fmt.Errorf("begin catalog page read: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	rows, err := queries.ListPublishedProducts(ctx, dbgen.ListPublishedProductsParams{
		CategorySlug: categorySlug, AfterCreatedAt: afterCreatedAt, AfterID: afterID, PageLimit: int32(limit + 1),
	})
	if err != nil {
		return nil, "", fmt.Errorf("list published catalog products: %w", err)
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	bases := make([]publicBase, 0, len(rows))
	for _, row := range rows {
		bases = append(bases, publicBase{
			id: row.ID, slug: row.Slug, name: row.Name, description: row.Description,
			categoryID: row.PrimaryCategoryID, regular: row.RegularPriceToman,
			sale: row.SalePriceToman, createdAt: row.CreatedAt, updatedAt: row.UpdatedAt,
		})
	}
	products, err := loadPublicProducts(ctx, queries, bases)
	if err != nil {
		return nil, "", err
	}
	nextCursor := ""
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		nextCursor = catalog.EncodeCursor(catalog.Cursor{CreatedAt: last.CreatedAt.Time, ID: catalog.ID(uuidString(last.ID))})
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, "", fmt.Errorf("finish catalog page read: %w", err)
	}
	return products, nextCursor, nil
}

func (r *CatalogRepository) GetPublishedProductBySlug(ctx context.Context, slug string) (catalog.PublicProduct, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return catalog.PublicProduct{}, fmt.Errorf("begin public product read: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	queries := dbgen.New(tx)
	row, err := queries.GetPublishedProductBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return catalog.PublicProduct{}, catalog.ErrNotFound
		}
		return catalog.PublicProduct{}, fmt.Errorf("get published catalog product: %w", err)
	}
	products, err := loadPublicProducts(ctx, queries, []publicBase{{
		id: row.ID, slug: row.Slug, name: row.Name, description: row.Description,
		categoryID: row.PrimaryCategoryID, regular: row.RegularPriceToman,
		sale: row.SalePriceToman, createdAt: row.CreatedAt, updatedAt: row.UpdatedAt,
	}})
	if err != nil {
		return catalog.PublicProduct{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return catalog.PublicProduct{}, fmt.Errorf("finish public product read: %w", err)
	}
	return products[0], nil
}

func loadPublicProducts(ctx context.Context, queries *dbgen.Queries, bases []publicBase) ([]catalog.PublicProduct, error) {
	products := make([]catalog.PublicProduct, len(bases))
	if len(bases) == 0 {
		return products, nil
	}
	productIDs := make([]pgtype.UUID, 0, len(bases))
	indexByID := make(map[pgtype.UUID]int, len(bases))
	for index, base := range bases {
		productIDs = append(productIDs, base.id)
		indexByID[base.id] = index
		products[index] = catalog.PublicProduct{
			ID: catalog.ID(uuidString(base.id)), Slug: base.slug, Name: base.name, Description: base.description,
			CategoryID: catalog.ID(uuidString(base.categoryID)), RegularPriceToman: base.regular,
			SalePriceToman: int64Pointer(base.sale), CreatedAt: base.createdAt.Time, UpdatedAt: base.updatedAt.Time,
			Variants: make([]catalog.PublicVariant, 0), Images: make([]catalog.PublicImage, 0),
		}
	}
	variants, err := queries.ListPublicVariants(ctx, productIDs)
	if err != nil {
		return nil, fmt.Errorf("load public catalog variants: %w", err)
	}
	for _, row := range variants {
		index, exists := indexByID[row.ProductID]
		if !exists {
			continue
		}
		base := bases[index]
		product := catalog.Product{RegularPriceToman: base.regular, SalePriceToman: int64Pointer(base.sale)}
		variant := catalog.Variant{
			RegularPriceOverrideToman: int64Pointer(row.RegularPriceOverrideToman),
			SalePriceOverrideToman:    int64Pointer(row.SalePriceOverrideToman),
		}
		price, err := catalog.PriceFor(product, variant)
		if err != nil {
			return nil, fmt.Errorf("calculate public variant price: %w", err)
		}
		products[index].Variants = append(products[index].Variants, catalog.PublicVariant{
			ID: catalog.ID(uuidString(row.ID)), Size: textPointer(row.Size), Color: textPointer(row.Color), Price: price,
		})
	}
	images, err := queries.ListPublicImages(ctx, productIDs)
	if err != nil {
		return nil, fmt.Errorf("load public catalog images: %w", err)
	}
	for _, row := range images {
		index, exists := indexByID[row.ProductID]
		if !exists {
			continue
		}
		products[index].Images = append(products[index].Images, catalog.PublicImage{
			ID: catalog.ID(uuidString(row.ID)), AssetRef: row.AssetRef, SortOrder: row.SortOrder, AltText: textPointer(row.AltText),
		})
	}
	return products, nil
}

func uuidString(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}
	bytes := value.Bytes
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

func uuidValue(value catalog.ID) (pgtype.UUID, error) {
	var parsed pgtype.UUID
	if err := parsed.Scan(string(value)); err != nil {
		return pgtype.UUID{}, err
	}
	return parsed, nil
}

func textPointer(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	copy := value.String
	return &copy
}

func int64Pointer(value pgtype.Int8) *int64 {
	if !value.Valid {
		return nil
	}
	copy := value.Int64
	return &copy
}
