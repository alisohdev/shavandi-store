import type { Product, Variant } from "../data/catalog";
import { normalizeDigits } from "./preview-checkout";

export type ProductDraft = {
  readonly name: string;
  readonly description: string;
  readonly priceToman: string;
  readonly availability: Readonly<Record<string, Variant["availability"]>>;
};

export type ProductField = "name" | "description" | "priceToman";
export type ProductErrors = Partial<Record<ProductField, string>>;

export function createProductDraft(product: Product): ProductDraft {
  return {
    name: product.name,
    description: product.description,
    priceToman: String(product.priceToman),
    availability: Object.fromEntries(
      product.colorways.flatMap((colorway) =>
        colorway.variants.map(
          (variant) =>
            [variant.sku, variant.availability] satisfies [
              string,
              Variant["availability"],
            ],
        ),
      ),
    ),
  };
}

export function parseDraftPrice(input: string): number | null {
  const normalized = normalizeDigits(input).trim();
  if (!/^\d+$/.test(normalized)) return null;
  const price = Number(normalized);
  return Number.isSafeInteger(price) && price > 0 ? price : null;
}

function mapNonEmpty<T, U>(
  items: readonly [T, ...T[]],
  map: (item: T) => U,
): [U, ...U[]] {
  const [first, ...rest] = items;
  return [map(first), ...rest.map(map)];
}

export function previewProduct(product: Product, draft: ProductDraft): Product {
  const changed =
    draft.name.trim() !== product.name ||
    draft.description.trim() !== product.description ||
    (parseDraftPrice(draft.priceToman) ?? product.priceToman) !==
      product.priceToman;
  return {
    ...product,
    provenance:
      changed && product.provenance.kind !== "local-edit"
        ? { kind: "local-edit", original: product.provenance }
        : product.provenance,
    name: draft.name.trim() || "نام محصول",
    description:
      draft.description.trim() || "توضیح کوتاه محصول اینجا نمایش داده می‌شود.",
    priceToman: parseDraftPrice(draft.priceToman) ?? product.priceToman,
    colorways: mapNonEmpty(product.colorways, (colorway) => ({
      ...colorway,
      variants: mapNonEmpty(colorway.variants, (variant) => ({
        ...variant,
        availability: draft.availability[variant.sku] ?? variant.availability,
      })),
    })),
  };
}

type ProductDraftResult =
  | { readonly kind: "valid"; readonly product: Product }
  | {
      readonly kind: "invalid";
      readonly errors: ProductErrors;
      readonly firstField: ProductField;
    };

export function validateProductDraft(
  product: Product,
  draft: ProductDraft,
): ProductDraftResult {
  const errors: ProductErrors = {};
  if (!draft.name.trim()) errors.name = "نام محصول را وارد کن.";
  if (!draft.description.trim()) errors.description = "توضیح محصول را وارد کن.";
  if (parseDraftPrice(draft.priceToman) === null) {
    errors.priceToman = "قیمت باید یک عدد صحیح و مثبت به تومان باشد.";
  }
  const fields: readonly ProductField[] = ["name", "description", "priceToman"];
  const firstField = fields.find((field) => errors[field]);
  return firstField
    ? { kind: "invalid", errors, firstField }
    : { kind: "valid", product: previewProduct(product, draft) };
}
