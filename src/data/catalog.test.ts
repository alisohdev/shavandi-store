import { describe, expect, test } from "bun:test";
import {
  availableSizes,
  collections,
  homeCollections,
  isAvailable,
  priceLabel,
  products,
  provenanceLabel,
  realProducts,
} from "./catalog";
import assetManifest from "../../docs/asset-manifest.json";

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
      if (collection.slug === "men") {
        expect(collection.productIds).toEqual([]);
        expect(collection.note).toContain("موقت");
      } else expect(collection.productIds.length).toBeGreaterThan(0);
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
      ...collections.map((collection) => collection.image.src),
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

  test("source products keep audited prices and separate unverified information", () => {
    expect(realProducts).toHaveLength(8);
    expect(homeCollections.map((collection) => collection.slug)).toEqual([
      "wearable-blankets",
      "football",
      "men",
      "women",
    ]);
    expect(
      products.filter((product) => product.provenance.kind === "temporary"),
    ).toHaveLength(8);
    for (const product of realProducts) {
      expect(product.provenance.kind).toBe("sourced");
      if (product.provenance.kind !== "sourced")
        throw new Error("Missing source provenance");
      expect(product.provenance.rights).toBe("confirmed-by-user");
      expect(product.provenance.checkedAt).toBe("2026-11-02");
      const provenance = product.provenance;
      const source = assetManifest.assets.find(
        (asset) =>
          "sourcePage" in asset && asset.sourcePage === provenance.sourcePage,
      );
      if (!source || !("priceToman" in source))
        throw new Error(`Missing audited source for ${product.id}`);
      expect(product.priceToman).toBe(source.priceToman);
      expect(product.material).toContain("مشخص نشده");
      expect(product.provenance.demoFacts.join(" ")).toContain("موجودی");
      expect(provenanceLabel(product.provenance)).toContain("شواندی");
      expect(priceLabel(product.provenance)).toContain("منبع");
    }
    expect(
      realProducts.find(
        (product) => product.slug === "dinosaur-wearable-blanket",
      )?.priceToman,
    ).toBe(5380000);
    expect(
      realProducts.find((product) => product.slug === "real-madrid-white-shirt")
        ?.description,
    ).toContain("نسخهٔ هواداری و مجوز رسمی تأیید نشده");
    const france = realProducts.find(
      (product) => product.slug === "france-third-player-world-cup-kit",
    );
    if (france?.provenance.kind !== "sourced")
      throw new Error("France source product is missing");
    expect(france.provenance.verifiedFacts.join(" ")).toContain("پلیری");
  });

  test("responsive source photos ship without crop or upscaling and match the manifest", async () => {
    const sourcedAssets = assetManifest.assets.filter(
      (asset) => asset.kind === "sourced",
    );
    expect(sourcedAssets).toHaveLength(realProducts.length);
    for (const product of realProducts) {
      for (const colorway of product.colorways) {
        for (const image of colorway.images) {
          expect(image.fit).toBe("contain");
          const source = sourcedAssets.find(
            (asset) =>
              "derivatives" in asset &&
              asset.derivatives?.some(
                (derivative) => derivative.path === image.src,
              ),
          );
          if (
            !source ||
            !("sourceDimensions" in source) ||
            !source.sourceDimensions ||
            !("derivatives" in source) ||
            !source.derivatives
          )
            throw new Error(`Image source missing: ${image.src}`);
          expect(image.width).toBe(source.sourceDimensions.width);
          expect(image.height).toBe(source.sourceDimensions.height);
          expect(source.rights).toBe("confirmed-by-user-2026-11-02");
          expect(image.srcSet).toBe(
            source.derivatives
              .map((derivative) => `${derivative.path} ${derivative.width}w`)
              .join(", "),
          );
          for (const derivative of source.derivatives) {
            expect(derivative.width).toBeLessThanOrEqual(image.width);
            expect(
              Math.abs(
                derivative.width / derivative.height -
                  image.width / image.height,
              ),
            ).toBeLessThan(0.005);
            expect(
              await Bun.file(
                new URL(`../../public${derivative.path}`, import.meta.url),
              ).exists(),
            ).toBe(true);
          }
        }
      }
    }
  });
});
