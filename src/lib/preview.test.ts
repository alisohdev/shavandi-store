import { describe, expect, test } from "bun:test";
import { products } from "../data/catalog";
import {
  addCartLine,
  cartTotals,
  parseCart,
  resolveCart,
} from "./preview-cart";
import {
  emptyContact,
  normalizeDigits,
  sampleContact,
  validateContact,
} from "./preview-checkout";
import {
  createProductDraft,
  parseDraftPrice,
  validateProductDraft,
} from "./preview-editor";

describe("preview cart storage boundary", () => {
  test("recovers from malformed and unsupported storage", () => {
    expect(parseCart(null)).toEqual({ kind: "ready", lines: [] });
    for (const raw of [
      "{broken",
      "null",
      "[]",
      '{"version":2,"lines":[]}',
      '{"version":1,"lines":{}}',
    ]) {
      expect(parseCart(raw)).toEqual({ kind: "repaired", lines: [] });
    }
  });

  test("merges and clamps quantities while dropping unavailable or unknown combinations", () => {
    const result = parseCart(
      JSON.stringify({
        version: 1,
        lines: [
          { sku: "SH101-WH-S", quantity: 7 },
          { sku: "SH101-WH-S", quantity: 8 },
          { sku: "SH101-WH-M", quantity: -3 },
          { sku: "SH108-NV-M", quantity: 2.8 },
          { sku: "SH101-WH-L", quantity: 1 },
          { sku: "unknown", quantity: 1 },
          { sku: "SH104-RU-S", quantity: "3" },
          null,
        ],
      }),
    );
    expect(result).toEqual({
      kind: "repaired",
      lines: [
        { sku: "SH101-WH-S", quantity: 9 },
        { sku: "SH101-WH-M", quantity: 1 },
        { sku: "SH108-NV-M", quantity: 2 },
      ],
    });
  });

  test("resolves names, images and prices from fixtures and totals integer toman", () => {
    const cart = parseCart(
      JSON.stringify({
        version: 1,
        lines: [
          {
            sku: "SH101-WH-M",
            quantity: 2,
            priceToman: 1,
            name: "Injected name",
          },
          { sku: "SH108-NV-M", quantity: 1 },
        ],
      }),
    );
    expect(cart.kind).toBe("ready");
    expect(cart.lines).toEqual([
      { sku: "SH101-WH-M", quantity: 2 },
      { sku: "SH108-NV-M", quantity: 1 },
    ]);
    const resolved = resolveCart(cart.lines);
    expect(resolved[0]?.product.name).toBe("تاپ نخی مینیمال");
    expect(resolved[0]?.colorway.images[0].src).toBe("/images/cotton-top.webp");
    expect(resolved[1]?.colorway.label).toBe("سرمه‌ای");
    expect(cartTotals(resolved)).toEqual({
      subtotalToman: 7470000,
      shippingToman: 79000,
      totalToman: 7549000,
    });
    expect(cartTotals([])).toEqual({
      subtotalToman: 0,
      shippingToman: 0,
      totalToman: 0,
    });
  });

  test("adding selections respects availability and the per-combination limit", () => {
    const lines = [{ sku: "SH101-WH-S", quantity: 9 }];
    expect(addCartLine(lines, "SH101-WH-S")).toEqual(lines);
    expect(addCartLine(lines, "SH101-WH-L")).toEqual(lines);
    expect(addCartLine(lines, "SH101-WH-M")).toEqual([
      ...lines,
      { sku: "SH101-WH-M", quantity: 1 },
    ]);
  });
});

describe("preview contact details", () => {
  test("accepts Persian and Arabic digits and trims required text", () => {
    expect(normalizeDigits("۰۱۲۳۴۵۶۷۸۹ / ٠١٢٣٤٥٦٧٨٩")).toBe(
      "0123456789 / 0123456789",
    );
    expect(
      validateContact({
        ...sampleContact,
        recipient: "  مشتری نمونه  ",
        phone: " ٠٩١٢٣٤٥٦٧٨٩ ",
      }),
    ).toEqual({
      kind: "valid",
      details: {
        ...sampleContact,
        recipient: "مشتری نمونه",
        phone: "09123456789",
        postalCode: "1234567890",
      },
    });
  });

  test("identifies required fields, invalid mobile numbers and postcodes", () => {
    const empty = validateContact(emptyContact);
    expect(empty.kind).toBe("invalid");
    if (empty.kind === "invalid") {
      expect(empty.firstField).toBe("recipient");
      expect(Object.keys(empty.errors).length).toBe(5);
    }
    for (const phone of [
      "02112345678",
      "0912345678",
      "+989123456789",
      "0912sample89",
    ]) {
      const result = validateContact({ ...sampleContact, phone });
      expect(result.kind).toBe("invalid");
      if (result.kind === "invalid") expect(result.errors.phone).toBeDefined();
    }
    expect(
      validateContact({ ...sampleContact, postalCode: "۱۲۳۴۵" }).kind,
    ).toBe("invalid");
  });
});

describe("local product drafts", () => {
  test("accepts positive whole toman and rejects ambiguous or unsafe price values", () => {
    expect(parseDraftPrice(" ۱۲۳۴۵۶۷ ")).toBe(1234567);
    expect(parseDraftPrice("١٢٣٤٥٦٧")).toBe(1234567);
    for (const price of [
      "",
      "0",
      "-10",
      "12.5",
      "1e6",
      "1,000",
      "۱۲٬۰۰۰",
      "9007199254740992",
    ]) {
      expect(parseDraftPrice(price)).toBeNull();
    }
  });

  test("applies editable fields while preserving routes, SKUs and image identities", () => {
    const original = products.find((product) => product.slug === "city-coat");
    if (!original) throw new Error("Coat fixture is missing");
    const draft = createProductDraft(original);
    const result = validateProductDraft(original, {
      ...draft,
      name: "  نام ویرایش‌شده  ",
      description: "  توضیح نمونهٔ تازه  ",
      priceToman: "۳۰۰۰۰۰۰",
      availability: {
        ...draft.availability,
        "SH108-BL-S": "sold-out",
        "SH108-NV-S": "available",
      },
    });
    expect(result.kind).toBe("valid");
    if (result.kind !== "valid") return;
    expect(result.product.name).toBe("نام ویرایش‌شده");
    expect(result.product.description).toBe("توضیح نمونهٔ تازه");
    expect(result.product.priceToman).toBe(3000000);
    expect(result.product.slug).toBe(original.slug);
    expect(result.product.id).toBe(original.id);
    expect(result.product.colorways.map((colorway) => colorway.id)).toEqual(
      original.colorways.map((colorway) => colorway.id),
    );
    expect(
      result.product.colorways.flatMap((colorway) =>
        colorway.variants.map((variant) => [variant.sku, variant.size]),
      ),
    ).toEqual(
      original.colorways.flatMap((colorway) =>
        colorway.variants.map((variant) => [variant.sku, variant.size]),
      ),
    );
    expect(result.product.colorways[0].images).toBe(
      original.colorways[0].images,
    );
    expect(result.product.colorways[0].variants[0].availability).toBe(
      "sold-out",
    );
    expect(original.colorways[0].variants[0].availability).toBe("available");
  });

  test("rejects blank product copy and invalid prices before application", () => {
    const original = products[0];
    if (!original) throw new Error("Product fixture is missing");
    const result = validateProductDraft(original, {
      ...createProductDraft(original),
      name: " ",
      description: " ",
      priceToman: "-1",
    });
    expect(result.kind).toBe("invalid");
    if (result.kind === "invalid") {
      expect(result.firstField).toBe("name");
      expect(Object.keys(result.errors)).toEqual([
        "name",
        "description",
        "priceToman",
      ]);
    }
  });
});
