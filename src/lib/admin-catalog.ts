export type AdminCategory = {
  readonly ID: string;
  readonly Name: string;
  readonly Slug: string;
  readonly ParentID: string | null;
  readonly DisplayOrder: number;
  readonly Active: boolean;
};

export type AdminVariant = {
  readonly StockOnHand: number;
  readonly StockVersion: number;
  readonly ID: string;
  readonly Size: string | null;
  readonly Color: string | null;
  readonly Active: boolean;
  readonly RegularPriceOverrideToman: number | null;
  readonly SalePriceOverrideToman: number | null;
};

export type AdminImage = {
  readonly ID: string;
  readonly AssetRef: string;
  readonly SortOrder: number;
  readonly AltText: string | null;
};

export type AdminProduct = {
  readonly ID: string;
  readonly Slug: string;
  readonly Name: string;
  readonly Description: string;
  readonly PrimaryCategory: string;
  readonly RegularPriceToman: number;
  readonly SalePriceToman: number | null;
  readonly PublicationState: "draft" | "published" | "hidden";
  readonly Version: number;
  readonly Variants: readonly AdminVariant[];
  readonly Images: readonly AdminImage[];
};

export function effectiveVariantPrice(
  product: AdminProduct,
  variant: AdminVariant,
) {
  const regular =
    variant.RegularPriceOverrideToman ?? product.RegularPriceToman;
  const sale =
    variant.SalePriceOverrideToman ??
    (variant.RegularPriceOverrideToman === null
      ? product.SalePriceToman
      : null);
  return { regular, sale };
}

export class AdminCatalogError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
  ) {
    super(code);
  }
}

function record(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

async function getJSON(path: string, signal?: AbortSignal): Promise<unknown> {
  const response = await fetch(path, {
    credentials: "include",
    cache: "no-store",
    ...(signal ? { signal } : {}),
    headers: { Accept: "application/json" },
  });
  if (!response.ok) {
    let code = "unavailable";
    const body: unknown = await response.json().catch(() => null);
    if (
      record(body) &&
      record(body.error) &&
      typeof body.error.code === "string"
    ) {
      code = body.error.code;
    }
    throw new AdminCatalogError(response.status, code);
  }
  return response.json();
}

function parseProduct(value: unknown): AdminProduct {
  if (
    !record(value) ||
    typeof value.ID !== "string" ||
    typeof value.Slug !== "string" ||
    typeof value.Name !== "string" ||
    typeof value.Description !== "string" ||
    typeof value.PrimaryCategory !== "string" ||
    typeof value.RegularPriceToman !== "number" ||
    typeof value.Version !== "number" ||
    !["draft", "published", "hidden"].includes(
      String(value.PublicationState),
    ) ||
    !Array.isArray(value.Variants) ||
    !Array.isArray(value.Images)
  )
    throw new Error("ساختار اطلاعات محصول از سرور قابل خواندن نیست.");
  return value as unknown as AdminProduct;
}

export async function listAdminProducts(
  signal?: AbortSignal,
): Promise<readonly AdminProduct[]> {
  const value = await getJSON("/api/v1/admin/products", signal);
  if (!record(value) || !Array.isArray(value.products))
    throw new Error("فهرست محصولات از سرور قابل خواندن نیست.");
  return value.products.map(parseProduct);
}

export async function getAdminProduct(
  id: string,
  signal?: AbortSignal,
): Promise<AdminProduct> {
  const value = await getJSON(
    `/api/v1/admin/products/${encodeURIComponent(id)}`,
    signal,
  );
  if (!record(value) || !("product" in value))
    throw new Error("اطلاعات محصول از سرور قابل خواندن نیست.");
  return parseProduct(value.product);
}

export async function listAdminCategories(
  signal?: AbortSignal,
): Promise<readonly AdminCategory[]> {
  const value = await getJSON("/api/v1/admin/categories", signal);
  if (!record(value) || !Array.isArray(value.categories))
    throw new Error("فهرست دسته‌بندی‌ها از سرور قابل خواندن نیست.");
  return value.categories as AdminCategory[];
}

export async function mutateAdminProduct(
  path: string,
  method: "POST" | "PATCH" | "DELETE",
  body: object | FormData,
): Promise<AdminProduct> {
  const csrfResponse = await fetch("/api/v1/auth/csrf", {
    credentials: "include",
    cache: "no-store",
  });
  if (!csrfResponse.ok)
    throw new AdminCatalogError(csrfResponse.status, "csrf_unavailable");
  const csrf: unknown = await csrfResponse.json();
  if (!record(csrf) || typeof csrf.csrf_token !== "string")
    throw new Error("Invalid CSRF response");
  const multipart = body instanceof FormData;
  const response = await fetch(`/api/v1/admin/products${path}`, {
    method,
    credentials: "include",
    cache: "no-store",
    headers: {
      "X-CSRF-Token": csrf.csrf_token,
      ...(multipart ? {} : { "Content-Type": "application/json" }),
    },
    body: multipart ? body : JSON.stringify(body),
  });
  const value: unknown = await response.json().catch(() => null);
  if (!response.ok)
    throw new AdminCatalogError(
      response.status,
      record(value) &&
        record(value.error) &&
        typeof value.error.code === "string"
        ? value.error.code
        : "unavailable",
    );
  if (!record(value)) throw new Error("Invalid product response");
  return parseProduct(value.product);
}
