export type PublicCategory = { id: string; name: string; slug: string };
export type PublicProduct = {
  id: string;
  slug: string;
  name: string;
  description: string;
  category_id: string;
  regular_price_toman: number;
  sale_price_toman: number | null;
  variants: {
    available_quantity: number;
    id: string;
    size: string | null;
    color: string | null;
    price: { regular_toman: number; sale_toman: number | null };
  }[];
  images: {
    id: string;
    asset_ref: string;
    sort_order: number;
    alt_text: string | null;
  }[];
};
export class CatalogRequestError extends Error {
  constructor(readonly status: number) {
    super("Catalog request failed");
  }
}
function record(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
function text(value: unknown): value is string {
  return typeof value === "string";
}
function nullableText(value: unknown) {
  return value === null || text(value);
}
function price(value: unknown) {
  return typeof value === "number" && Number.isSafeInteger(value) && value >= 0;
}
function optionalPrice(value: unknown) {
  return value === null || price(value);
}
export function parsePublicProduct(value: unknown): PublicProduct {
  if (
    !record(value) ||
    !text(value.id) ||
    !text(value.slug) ||
    !text(value.name) ||
    !text(value.description) ||
    !text(value.category_id) ||
    !price(value.regular_price_toman) ||
    !optionalPrice(value.sale_price_toman) ||
    !Array.isArray(value.variants) ||
    !Array.isArray(value.images) ||
    !value.variants.every(
      (v) =>
        record(v) &&
        text(v.id) &&
        typeof v.available_quantity === "number" &&
        Number.isSafeInteger(v.available_quantity) &&
        v.available_quantity >= 0 &&
        nullableText(v.size) &&
        nullableText(v.color) &&
        record(v.price) &&
        price(v.price.regular_toman) &&
        optionalPrice(v.price.sale_toman),
    ) ||
    !value.images.every(
      (i) =>
        record(i) &&
        text(i.id) &&
        text(i.asset_ref) &&
        /^\/(?!\/)/.test(i.asset_ref) &&
        typeof i.sort_order === "number" &&
        nullableText(i.alt_text),
    )
  )
    throw new CatalogRequestError(502);
  return value as PublicProduct;
}
export function createPublicCatalog(
  origin: string,
  fetcher: (
    input: URL | RequestInfo,
    init?: RequestInit,
  ) => Promise<Response> = fetch,
) {
  const base = new URL(origin);
  if (
    !["http:", "https:"].includes(base.protocol) ||
    base.username ||
    base.password ||
    base.pathname !== "/" ||
    base.search ||
    base.hash
  )
    throw new CatalogRequestError(503);
  async function request(path: string): Promise<unknown> {
    let response: Response;
    try {
      response = await fetcher(new URL(`/api/v1/${path}`, base), {
        signal: AbortSignal.timeout(5000),
        cache: "no-store",
        redirect: "error",
        headers: { Accept: "application/json" },
      });
    } catch {
      throw new CatalogRequestError(503);
    }
    if (!response.ok) throw new CatalogRequestError(response.status);
    try {
      return await response.json();
    } catch {
      throw new CatalogRequestError(502);
    }
  }
  return {
    async categories(): Promise<PublicCategory[]> {
      const body = await request("categories");
      if (
        !record(body) ||
        !Array.isArray(body.categories) ||
        !body.categories.every(
          (c) => record(c) && text(c.id) && text(c.name) && text(c.slug),
        )
      )
        throw new CatalogRequestError(502);
      return body.categories as PublicCategory[];
    },
    async products(category = "", cursor = "") {
      const query = new URLSearchParams({ limit: "20" });
      if (category) query.set("category", category);
      if (cursor) query.set("cursor", cursor);
      const body = await request(`products?${query}`);
      if (
        !record(body) ||
        !Array.isArray(body.products) ||
        (body.next_cursor !== undefined && !text(body.next_cursor))
      )
        throw new CatalogRequestError(502);
      return {
        products: body.products.map(parsePublicProduct),
        nextCursor:
          typeof body.next_cursor === "string" ? body.next_cursor : "",
      };
    },
    async product(slug: string) {
      const body = await request(`products/${encodeURIComponent(slug)}`);
      if (!record(body)) throw new CatalogRequestError(502);
      return parsePublicProduct(body.product);
    },
  };
}
export const catalogOrigin = () =>
  process.env.CATALOG_API_ORIGIN || "http://127.0.0.1:8080";

export async function loadPublicListing(category: string, cursor: string) {
  let categories: PublicCategory[] = [];
  try {
    const api = createPublicCatalog(catalogOrigin());
    categories = await api.categories();
    if (category && !categories.some((c) => c.slug === category))
      throw new CatalogRequestError(404);
    return {
      categories,
      ...(await api.products(category, cursor)),
      status: 200,
    };
  } catch (error) {
    return {
      categories,
      products: [] as PublicProduct[],
      nextCursor: "",
      status:
        error instanceof CatalogRequestError &&
        [400, 404].includes(error.status)
          ? error.status
          : 503,
    };
  }
}
