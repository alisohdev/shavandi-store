import { describe, expect, test } from "bun:test";
import { availableSizes, collections, isAvailable, products } from "./catalog";

describe("sample catalog", () => {
  test("every route and purchasable combination has a unique identity", () => {
    expect(new Set(products.map((product) => product.id)).size).toBe(
      products.length,
    );
    expect(new Set(products.map((product) => product.slug)).size).toBe(
      products.length,
    );
    const skus = products.flatMap((product) =>
      product.colorways.flatMap((colorway) =>
        colorway.variants.map((variant) => variant.sku),
      ),
    );
    expect(new Set(skus).size).toBe(skus.length);
    for (const product of products) {
      expect(
        Number.isSafeInteger(product.priceToman) && product.priceToman > 0,
      ).toBe(true);
      expect(
        new Set(product.colorways.map((colorway) => colorway.id)).size,
      ).toBe(product.colorways.length);
      for (const colorway of product.colorways) {
        expect(
          new Set(colorway.variants.map((variant) => variant.size)).size,
        ).toBe(colorway.variants.length);
      }
    }
  });

  test("availability follows actual color and size combinations", () => {
    const soldOut = products.find(
      (product) => product.slug === "violet-lace-dress",
    );
    const coat = products.find((product) => product.slug === "city-coat");
    if (!soldOut || !coat) throw new Error("Availability fixtures are missing");
    expect(isAvailable(soldOut)).toBe(false);
    expect(availableSizes(soldOut)).toEqual([]);
    expect(isAvailable(coat)).toBe(true);
    expect(availableSizes(coat)).toEqual(["S", "M", "L"]);
    expect(
      coat.colorways
        .flatMap((colorway) => colorway.variants)
        .some((variant) => variant.availability === "sold-out"),
    ).toBe(true);
  });

  test("all editorial collection references resolve to real product routes", () => {
    const ids = new Set(products.map((product) => product.id));
    for (const collection of collections) {
      expect(collection.productIds.length).toBeGreaterThan(0);
      expect(collection.productIds.every((id) => ids.has(id))).toBe(true);
    }
  });

  test("all images ship locally", async () => {
    const paths = new Set([
      ...products.flatMap((product) =>
        product.colorways.flatMap((colorway) =>
          colorway.images.map((image) => image.src),
        ),
      ),
      ...collections.map((collection) => collection.image),
    ]);
    for (const path of paths) {
      expect(path.startsWith("/images/")).toBe(true);
      expect(
        await Bun.file(
          new URL(`../../public${path}`, import.meta.url),
        ).exists(),
      ).toBe(true);
    }
  });
});
