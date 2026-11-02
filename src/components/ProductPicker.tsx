import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  formatPrice,
  isAvailable,
  type Colorway,
  type Product,
  type Variant,
} from "@/data/catalog";
import { cn } from "@/lib/utils";

type Selection = {
  readonly colorway: Colorway;
  readonly variant: Variant | null;
};

export default function ProductPicker({ product }: { product: Product }) {
  const [selection, setSelection] = useState<Selection>({
    colorway: product.colorways[0],
    variant: null,
  });
  const { colorway, variant: selectedVariant } = selection;
  const image = colorway.images[0];
  const colorAvailable = colorway.variants.some(
    (variant) => variant.availability === "available",
  );

  return (
    <div className="product-detail-grid">
      <figure className="detail-gallery">
        <img
          src={image.src}
          alt={image.alt}
          width="800"
          height="1067"
          fetchPriority="high"
          style={{ objectPosition: image.position }}
        />
        <figcaption>
          {colorway.imageNote ??
            "عکس مرجع است؛ مشخصات این محصول در پیش‌نمایش نمونه‌اند."}
        </figcaption>
      </figure>
      <div className="product-detail-copy">
        <p className="eyebrow">{product.category} / مجموعهٔ شواندی</p>
        <h1>{product.name}</h1>
        <p className="detail-price">
          <span>{formatPrice(product.priceToman)}</span>
          <span>تومان</span>
        </p>
        <p className="detail-description">{product.description}</p>
        <p className="selection-label">انتخاب رنگ و اندازه · نمایشی</p>
        <fieldset className="color-fieldset">
          <legend>
            رنگ: <strong>{colorway.label}</strong>
          </legend>
          <div className="color-options">
            {product.colorways.map((color) => (
              <button
                key={color.id}
                type="button"
                aria-pressed={colorway.id === color.id}
                className={cn(
                  "color-option",
                  colorway.id === color.id && "is-selected",
                )}
                onClick={() => setSelection({ colorway: color, variant: null })}
              >
                <span
                  className="color-swatch"
                  style={{ backgroundColor: color.swatch }}
                />
                <span>{color.label}</span>
              </button>
            ))}
          </div>
        </fieldset>
        <fieldset className="size-fieldset">
          <legend>اندازه</legend>
          <a className="size-guide-link" href="/guide/">
            راهنمای اندازه ↖
          </a>
          <div className="size-options">
            {colorway.variants.map((variant) => (
              <Button
                key={variant.sku}
                type="button"
                variant="outline"
                className={cn(
                  "size-option",
                  selectedVariant?.sku === variant.sku && "is-selected",
                )}
                disabled={variant.availability === "sold-out"}
                aria-pressed={selectedVariant?.sku === variant.sku}
                aria-label={`اندازهٔ ${variant.size}${variant.availability === "sold-out" ? "، ناموجود" : ""}`}
                onClick={() => setSelection({ colorway, variant })}
              >
                <bdi>{variant.size}</bdi>
                {variant.availability === "sold-out" && (
                  <span className="size-sold-out">ناموجود</span>
                )}
              </Button>
            ))}
          </div>
        </fieldset>
        <div
          className="selection-status"
          role="status"
          aria-live="polite"
          aria-atomic="true"
        >
          {selectedVariant ? (
            <>
              <span className="availability-dot" />
              {colorway.label}، اندازهٔ <bdi>{selectedVariant.size}</bdi> ·
              موجود در دادهٔ نمونه
            </>
          ) : colorAvailable ? (
            "برای دیدن موجودی نمونه، یک اندازه انتخاب کن."
          ) : (
            "همهٔ اندازه‌های این رنگ در دادهٔ نمونه ناموجودند."
          )}
        </div>
        <p className="purchase-notice">
          {isAvailable(product)
            ? "این یک پیش‌نمایش است. انتخاب شما ذخیره نمی‌شود و خرید و ثبت سفارش فعال نیست."
            : "این محصول در دادهٔ نمونه ناموجود است. خرید و ثبت سفارش در این پیش‌نمایش فعال نیست."}
        </p>
        <dl className="product-facts">
          <div>
            <dt>جنس نمونه</dt>
            <dd>{product.material}</dd>
          </div>
          <div>
            <dt>فرم لباس</dt>
            <dd>{product.fit}</dd>
          </div>
          <div>
            <dt>نگهداری</dt>
            <dd>{product.care}</dd>
          </div>
        </dl>
      </div>
    </div>
  );
}
