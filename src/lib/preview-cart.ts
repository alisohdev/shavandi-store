import {
  products,
  type Colorway,
  type Product,
  type Variant,
} from "../data/catalog";

export const cartStorageKey = "shavandi.preview-cart.v1";
export const maxCartQuantity = 9;

export type CartLine = {
  readonly sku: string;
  readonly quantity: number;
};

export type ResolvedCartLine = CartLine & {
  readonly product: Product;
  readonly colorway: Colorway;
  readonly variant: Variant;
};

export type CartSnapshot = {
  readonly kind: "ready" | "repaired" | "unavailable";
  readonly lines: readonly CartLine[];
};

function findVariant(sku: string) {
  for (const product of products) {
    for (const colorway of product.colorways) {
      const variant = colorway.variants.find((item) => item.sku === sku);
      if (variant?.availability === "available") {
        return { product, colorway, variant };
      }
    }
  }
  return null;
}

export function parseCart(raw: string | null): CartSnapshot {
  if (raw === null) return { kind: "ready", lines: [] };

  let value: unknown;
  try {
    value = JSON.parse(raw);
  } catch {
    return { kind: "repaired", lines: [] };
  }
  if (
    typeof value !== "object" ||
    value === null ||
    !("version" in value) ||
    value.version !== 1 ||
    !("lines" in value) ||
    !Array.isArray(value.lines)
  ) {
    return { kind: "repaired", lines: [] };
  }

  const quantities = new Map<string, number>();
  let repaired = false;
  const entries: readonly unknown[] = value.lines;
  for (const entry of entries) {
    if (
      typeof entry !== "object" ||
      entry === null ||
      !("sku" in entry) ||
      typeof entry.sku !== "string" ||
      !findVariant(entry.sku) ||
      !("quantity" in entry) ||
      typeof entry.quantity !== "number" ||
      !Number.isFinite(entry.quantity)
    ) {
      repaired = true;
      continue;
    }
    const quantity = Math.min(
      maxCartQuantity,
      Math.max(1, Math.floor(entry.quantity)),
    );
    const previous = quantities.get(entry.sku);
    if (quantity !== entry.quantity || previous !== undefined) repaired = true;
    quantities.set(
      entry.sku,
      Math.min(maxCartQuantity, (previous ?? 0) + quantity),
    );
  }

  return {
    kind: repaired ? "repaired" : "ready",
    lines: Array.from(quantities, ([sku, quantity]) => ({ sku, quantity })),
  };
}

export function loadCart(): CartSnapshot {
  try {
    return parseCart(window.localStorage.getItem(cartStorageKey));
  } catch {
    return { kind: "unavailable", lines: [] };
  }
}

export function saveCart(lines: readonly CartLine[]): boolean {
  try {
    window.localStorage.setItem(
      cartStorageKey,
      JSON.stringify({
        version: 1,
        lines: lines.map(({ sku, quantity }) => ({ sku, quantity })),
      }),
    );
    return true;
  } catch {
    return false;
  }
}

export function addCartLine(
  lines: readonly CartLine[],
  sku: string,
): readonly CartLine[] {
  if (!findVariant(sku)) return lines;
  const existing = lines.find((line) => line.sku === sku);
  return existing
    ? lines.map((line) =>
        line.sku === sku
          ? { sku, quantity: Math.min(maxCartQuantity, line.quantity + 1) }
          : line,
      )
    : [...lines, { sku, quantity: 1 }];
}

export function resolveCart(
  lines: readonly CartLine[],
): readonly ResolvedCartLine[] {
  return lines.flatMap((line) => {
    const match = findVariant(line.sku);
    return match ? [{ ...line, ...match }] : [];
  });
}

export function cartTotals(lines: readonly ResolvedCartLine[]) {
  const subtotalToman = lines.reduce(
    (total, line) => total + line.quantity * line.product.priceToman,
    0,
  );
  const shippingToman = lines.length > 0 ? 79000 : 0;
  return {
    subtotalToman,
    shippingToman,
    totalToman: subtotalToman + shippingToman,
  };
}
