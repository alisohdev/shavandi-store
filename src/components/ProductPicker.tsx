import { useState } from "react";
import { Button } from "@/components/ui/button";
import CatalogImage from "./CatalogImage";
import {
  formatPrice,
  isAvailable,
  priceLabel,
  provenanceLabel,
  type Colorway,
  type Product,
  type Variant,
} from "@/data/catalog";
import { cn } from "@/lib/utils";
import {
  addCartLine,
  loadCart,
  maxCartQuantity,
  saveCart,
} from "@/lib/preview-cart";

type Selection = {
  readonly colorway: Colorway;
  readonly variant: Variant | null;
};

export default function ProductPicker({ product }: { product: Product }) {
  const [selection, setSelection] = useState<Selection>({
    colorway: product.colorways[0],
    variant: null,
  });
  const [cartMessage, setCartMessage] = useState("");
  const { colorway, variant: selectedVariant } = selection;
  const image = colorway.images[0];
  const colorAvailable = colorway.variants.some(
    (variant) => variant.availability === "available",
  );

  function addToCart() {
    if (!selectedVariant || selectedVariant.availability !== "available")
      return;
    const cart = loadCart();
    if (cart.kind === "unavailable") {
      setCartMessage(
        "ذخیرهٔ محلی در این مرورگر در دسترس نیست. در صفحهٔ تسویه می‌توانی با یک سبد نمونه تمرین کنی.",
      );
      return;
    }
    const current = cart.lines.find((line) => line.sku === selectedVariant.sku);
    if (current?.quantity === maxCartQuantity) {
      setCartMessage(
        "از این رنگ و اندازه، ۹ عدد در سبد نمایشی داری. تعداد را در صفحهٔ تسویه تغییر بده.",
      );
      return;
    }
    const saved = saveCart(addCartLine(cart.lines, selectedVariant.sku));
    setCartMessage(
      saved
        ? `${product.name}، ${colorway.label}، اندازهٔ ${selectedVariant.size} به سبد نمایشی اضافه شد.${cart.kind === "repaired" ? " بخش نامعتبر سبد قبلی پاک شد." : ""}`
        : "مرورگر اجازهٔ ذخیرهٔ سبد را نداد. در صفحهٔ تسویه می‌توانی با یک سبد نمونه تمرین کنی.",
    );
  }

  return (
    <div className="product-detail-grid">
      <figure className="detail-gallery">
        <CatalogImage
          image={image}
          sizes="(max-width: 540px) 92vw, 50vw"
          eager
        />
        <figcaption>
          {colorway.imageNote ??
            "عکس مرجع است؛ مشخصات این محصول در پیش‌نمایش نمونه‌اند."}
        </figcaption>
      </figure>
      <div className="product-detail-copy">
        <p className="eyebrow">{product.category} / مجموعهٔ شواندی</p>
        <h1>{product.name}</h1>
        <p className="product-provenance">
          {provenanceLabel(product.provenance)}
        </p>
        <p className="detail-price">
          <span>{formatPrice(product.priceToman)}</span>
          <span>تومان</span>
        </p>
        <p className="detail-description">{product.description}</p>
        <p className="price-provenance">{priceLabel(product.provenance)}</p>
        {product.provenance.kind === "sourced" && (
          <div className="source-facts">
            <p>اطلاعات ثبت‌شده از منبع</p>
            <ul>
              {product.provenance.verifiedFacts.map((fact) => (
                <li key={fact}>{fact}</li>
              ))}
            </ul>
            <a
              href={product.provenance.sourcePage}
              target="_blank"
              rel="noreferrer"
            >
              صفحهٔ محصول در فروشگاه منبع ↗
            </a>
            <p>{product.provenance.demoFacts.join(" ")}</p>
          </div>
        )}
        {product.provenance.kind === "temporary" && (
          <p className="purchase-notice">{product.provenance.note}</p>
        )}
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
                onClick={() => {
                  setSelection({ colorway: color, variant: null });
                  setCartMessage("");
                }}
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
                onClick={() => {
                  setSelection({ colorway, variant });
                  setCartMessage("");
                }}
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
        <div className="product-cart-actions">
          <Button
            type="button"
            size="lg"
            className="preview-primary"
            disabled={
              !selectedVariant || selectedVariant.availability !== "available"
            }
            onClick={addToCart}
          >
            افزودن به سبد نمایشی
          </Button>
          <a className="text-link" href="/checkout/">
            دیدن سبد و تسویهٔ نمایشی ←
          </a>
        </div>
        <p
          className="preview-feedback"
          role="status"
          aria-live="polite"
          aria-atomic="true"
        >
          {cartMessage}
        </p>
        <p className="purchase-notice">
          {isAvailable(product)
            ? "فقط رنگ، اندازه و تعداد در مرورگر ذخیره می‌شود. تسویه یک تمرین است؛ سفارش و پرداخت واقعی انجام نمی‌شود."
            : "این محصول در دادهٔ نمونه ناموجود است و به سبد اضافه نمی‌شود. سفارش و پرداخت واقعی انجام نمی‌شود."}
        </p>
        <dl className="product-facts">
          <div>
            <dt>
              {product.provenance.kind === "sourced" ? "جنس" : "جنس نمونه"}
            </dt>
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
