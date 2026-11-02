import { useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  formatPrice,
  isAvailable,
  priceLabel,
  provenanceLabel,
  type Product,
} from "@/data/catalog";
import CatalogImage from "./CatalogImage";
import {
  createProductDraft,
  parseDraftPrice,
  previewProduct,
  validateProductDraft,
  type ProductDraft,
  type ProductErrors,
  type ProductField,
} from "@/lib/preview-editor";

type Selection = {
  readonly productId: string;
  readonly draft: ProductDraft;
};

export default function AdminEditor({
  seedProducts,
}: {
  seedProducts: readonly Product[];
}) {
  const [appliedProducts, setAppliedProducts] = useState(seedProducts);
  const [selection, setSelection] = useState<Selection | null>(() => {
    const first = seedProducts[0];
    return first
      ? { productId: first.id, draft: createProductDraft(first) }
      : null;
  });
  const [errors, setErrors] = useState<ProductErrors>({});
  const [notice, setNotice] = useState("");
  const editorHeading = useRef<HTMLHeadingElement>(null);
  const selectedProduct = appliedProducts.find(
    (product) => product.id === selection?.productId,
  );

  function selectProduct(product: Product) {
    setSelection({ productId: product.id, draft: createProductDraft(product) });
    setErrors({});
    setNotice("");
  }

  function updateField(field: ProductField, value: string) {
    setSelection((current) =>
      current
        ? { ...current, draft: { ...current.draft, [field]: value } }
        : null,
    );
    setErrors((current) => {
      const next = { ...current };
      delete next[field];
      return next;
    });
    setNotice("");
  }

  if (!selection || !selectedProduct) {
    return (
      <p className="preview-notice">محصول نمونه‌ای برای ویرایش وجود ندارد.</p>
    );
  }

  const liveProduct = previewProduct(selectedProduct, selection.draft);
  const image = liveProduct.colorways[0].images[0];
  const allVariants = appliedProducts.flatMap((product) =>
    product.colorways.flatMap((colorway) => colorway.variants),
  );
  const liveVariants = liveProduct.colorways.flatMap(
    (colorway) => colorway.variants,
  );

  return (
    <div className="admin-editor-preview">
      <p className="preview-notice admin-scope-note">
        ویرایش‌ها فقط در همین صفحه‌اند. «اعمال در پیش‌نمایش» جدول و آمار اینجا
        را تغییر می‌دهد؛ فروشگاه و سبد از کاتالوگ اولیه استفاده می‌کنند. با
        تازه‌کردن صفحه، همهٔ ویرایش‌ها پاک می‌شوند.
      </p>
      <div className="admin-metrics">
        <div>
          <span>محصول در پیش‌نمایش</span>
          <strong>{formatPrice(appliedProducts.length)}</strong>
        </div>
        <div>
          <span>ترکیب رنگ و اندازه</span>
          <strong>{formatPrice(allVariants.length)}</strong>
        </div>
        <div>
          <span>ترکیب موجود در پیش‌نمایش</span>
          <strong>
            {formatPrice(
              allVariants.filter(
                (variant) => variant.availability === "available",
              ).length,
            )}
          </strong>
        </div>
      </div>
      <section
        className="admin-editor-section"
        aria-labelledby="editor-heading"
        id="product-editor"
      >
        <div className="section-heading admin-editor-heading">
          <div>
            <p className="eyebrow">ویرایش محلی / دادهٔ نمونه</p>
            <h2 ref={editorHeading} tabIndex={-1} id="editor-heading">
              محصول را امتحان کن.
            </h2>
          </div>
          <div className="admin-product-select form-field">
            <label htmlFor="selected-product">انتخاب محصول</label>
            <select
              id="selected-product"
              value={selectedProduct.id}
              onChange={(event) => {
                const product = appliedProducts.find(
                  (item) => item.id === event.currentTarget.value,
                );
                if (product) selectProduct(product);
              }}
            >
              {appliedProducts.map((product) => (
                <option key={product.id} value={product.id}>
                  {product.name}
                </option>
              ))}
            </select>
          </div>
        </div>
        <div className="admin-edit-grid">
          <form
            className="preview-form admin-edit-form"
            noValidate
            onSubmit={(event) => {
              event.preventDefault();
              const result = validateProductDraft(
                selectedProduct,
                selection.draft,
              );
              if (result.kind === "invalid") {
                setErrors(result.errors);
                const field = event.currentTarget.elements.namedItem(
                  result.firstField,
                );
                if (field instanceof HTMLElement) field.focus();
                return;
              }
              setAppliedProducts((current) =>
                current.map((product) =>
                  product.id === result.product.id ? result.product : product,
                ),
              );
              setSelection({
                productId: result.product.id,
                draft: createProductDraft(result.product),
              });
              setErrors({});
              setNotice(
                `«${result.product.name}» در پیش‌نمایش همین صفحه اعمال شد. جدول و آمار به‌روز شدند.`,
              );
            }}
          >
            <div className="form-field">
              <label htmlFor="product-name">نام محصول</label>
              <input
                id="product-name"
                name="name"
                maxLength={120}
                required
                value={selection.draft.name}
                onChange={(event) =>
                  updateField("name", event.currentTarget.value)
                }
                aria-invalid={Boolean(errors.name)}
                aria-describedby={
                  errors.name ? "product-name-error" : undefined
                }
              />
              {errors.name && (
                <p className="field-error" id="product-name-error">
                  {errors.name}
                </p>
              )}
            </div>
            <div className="form-field">
              <label htmlFor="product-description">توضیح محصول</label>
              <textarea
                id="product-description"
                name="description"
                rows={4}
                maxLength={1000}
                required
                value={selection.draft.description}
                onChange={(event) =>
                  updateField("description", event.currentTarget.value)
                }
                aria-invalid={Boolean(errors.description)}
                aria-describedby={
                  errors.description ? "product-description-error" : undefined
                }
              />
              {errors.description && (
                <p className="field-error" id="product-description-error">
                  {errors.description}
                </p>
              )}
            </div>
            <div className="form-field">
              <label htmlFor="product-price">قیمت در ویرایشگر (تومان)</label>
              <input
                id="product-price"
                name="priceToman"
                inputMode="numeric"
                dir="ltr"
                maxLength={16}
                required
                value={selection.draft.priceToman}
                onChange={(event) =>
                  updateField("priceToman", event.currentTarget.value)
                }
                aria-invalid={Boolean(errors.priceToman)}
                aria-describedby={
                  errors.priceToman
                    ? "product-price-error"
                    : "product-price-hint"
                }
              />
              {errors.priceToman ? (
                <p className="field-error" id="product-price-error">
                  {errors.priceToman}
                </p>
              ) : (
                <p className="field-hint" id="product-price-hint">
                  عدد صحیح و مثبت؛ رقم‌های فارسی و انگلیسی پذیرفته می‌شوند.
                </p>
              )}
            </div>
            <div className="admin-variant-fields">
              <h3>موجودی رنگ و اندازه</h3>
              <p className="field-hint">
                فقط وضعیت ترکیب‌های فعلی قابل تغییر است.
              </p>
              {selectedProduct.colorways.map((colorway) => (
                <fieldset key={colorway.id} className="admin-colorway-fieldset">
                  <legend>
                    <span
                      className="color-swatch"
                      style={{ backgroundColor: colorway.swatch }}
                    />
                    {colorway.label}
                  </legend>
                  {colorway.variants.map((variant) => (
                    <div key={variant.sku} className="admin-variant-row">
                      <label htmlFor={`availability-${variant.sku}`}>
                        اندازهٔ <bdi>{variant.size}</bdi>
                        <small>
                          <bdi>{variant.sku}</bdi>
                        </small>
                      </label>
                      <select
                        id={`availability-${variant.sku}`}
                        value={
                          selection.draft.availability[variant.sku] ??
                          variant.availability
                        }
                        onChange={(event) => {
                          const availability = event.currentTarget.value;
                          if (
                            availability !== "available" &&
                            availability !== "sold-out"
                          )
                            return;
                          setSelection((current) =>
                            current
                              ? {
                                  ...current,
                                  draft: {
                                    ...current.draft,
                                    availability: {
                                      ...current.draft.availability,
                                      [variant.sku]: availability,
                                    },
                                  },
                                }
                              : null,
                          );
                          setNotice("");
                        }}
                      >
                        <option value="available">موجود</option>
                        <option value="sold-out">ناموجود</option>
                      </select>
                    </div>
                  ))}
                </fieldset>
              ))}
            </div>
            <div className="form-actions">
              <Button type="submit" className="preview-primary" size="lg">
                اعمال در پیش‌نمایش
              </Button>
              <Button
                type="button"
                variant="outline"
                className="preview-secondary"
                onClick={() => {
                  const original = seedProducts.find(
                    (product) => product.id === selectedProduct.id,
                  );
                  if (!original) return;
                  setAppliedProducts((current) =>
                    current.map((product) =>
                      product.id === original.id ? original : product,
                    ),
                  );
                  setSelection({
                    productId: original.id,
                    draft: createProductDraft(original),
                  });
                  setErrors({});
                  setNotice(
                    "این محصول به دادهٔ اولیه برگشت؛ جدول و آمار هم به‌روز شدند.",
                  );
                }}
              >
                بازنشانی این محصول
              </Button>
            </div>
            <p
              className="preview-feedback"
              role="status"
              aria-live="polite"
              aria-atomic="true"
            >
              {notice}
            </p>
            <p className="field-hint">
              با انتخاب محصول دیگر، تغییرات اعمال‌نشدهٔ فرم کنار گذاشته می‌شوند.
            </p>
          </form>
          <aside
            className="admin-live-preview"
            aria-labelledby="live-preview-heading"
          >
            <div className="admin-live-preview-heading">
              <p className="eyebrow">هم‌زمان با تایپ</p>
              <h3 id="live-preview-heading">نمای زندهٔ محصول</h3>
            </div>
            <CatalogImage
              image={image}
              sizes="(max-width: 540px) 90vw, 350px"
            />
            <div className="admin-live-copy">
              <span
                className={`admin-availability${isAvailable(liveProduct) ? "" : " is-unavailable"}`}
              >
                {isAvailable(liveProduct)
                  ? "موجود در پیش‌نمایش"
                  : "ناموجود در پیش‌نمایش"}
              </span>
              <h3>{liveProduct.name}</h3>
              <p className="product-provenance">
                {provenanceLabel(liveProduct.provenance)}
              </p>
              <p className="admin-live-price">
                {formatPrice(liveProduct.priceToman)} تومان
              </p>
              <p className="price-provenance">
                {priceLabel(liveProduct.provenance)}
              </p>
              <p className="admin-live-description">
                {liveProduct.description}
              </p>
              <p className="field-hint">
                {formatPrice(
                  liveVariants.filter(
                    (variant) => variant.availability === "available",
                  ).length,
                )}{" "}
                ترکیب موجود از {formatPrice(liveVariants.length)} ترکیب رنگ و
                اندازه
              </p>
              {parseDraftPrice(selection.draft.priceToman) === null && (
                <p className="field-hint">
                  تا واردکردن قیمت معتبر، آخرین قیمت اعمال‌شده نمایش داده
                  می‌شود.
                </p>
              )}
              <p className="field-hint">
                برچسب منبع برای دادهٔ اولیه است. با تغییر نام، توضیح یا قیمت،
                اطلاعات این نما «ویرایش محلی» می‌شود و تأیید منبع ندارد.
              </p>
            </div>
          </aside>
        </div>
      </section>
      <section
        className="admin-products"
        aria-labelledby="product-table-heading"
      >
        <div className="section-heading">
          <div>
            <p className="eyebrow">پس از اعمال تغییرات</p>
            <h2 id="product-table-heading">محصولات پیش‌نمایش</h2>
          </div>
          <Button
            type="button"
            variant="outline"
            className="preview-secondary"
            onClick={() => {
              setAppliedProducts(seedProducts);
              const original = seedProducts.find(
                (product) => product.id === selectedProduct.id,
              );
              if (original)
                setSelection({
                  productId: original.id,
                  draft: createProductDraft(original),
                });
              setErrors({});
              setNotice("همهٔ محصولات به دادهٔ اولیه برگشتند.");
            }}
          >
            بازنشانی همه
          </Button>
        </div>
        <div
          className="admin-table-wrap"
          role="region"
          aria-label="جدول محصولات پیش‌نمایش؛ برای دیدن همهٔ ستون‌ها افقی پیمایش کنید"
          tabIndex={0}
        >
          <table className="admin-table" id="product-table">
            <caption className="sr-only">
              نام، قیمت و موجودی محصولات پس از اعمال تغییرات محلی
            </caption>
            <thead>
              <tr>
                <th scope="col">محصول</th>
                <th scope="col">قیمت (تومان)</th>
                <th scope="col">رنگ‌ها</th>
                <th scope="col">وضعیت پیش‌نمایش</th>
                <th scope="col">ویرایش</th>
              </tr>
            </thead>
            <tbody>
              {appliedProducts.map((product) => (
                <tr
                  key={product.id}
                  data-selected={product.id === selectedProduct.id}
                >
                  <th scope="row">
                    <div className="admin-product-identity">
                      <CatalogImage
                        image={product.colorways[0].images[0]}
                        sizes="40px"
                      />
                      <span>
                        {product.name}
                        <small>
                          <bdi>{product.id.toUpperCase()}</bdi>
                        </small>
                        <small className="product-provenance">
                          {provenanceLabel(product.provenance)}
                        </small>
                      </span>
                    </div>
                  </th>
                  <td>
                    {formatPrice(product.priceToman)}
                    <p className="price-provenance">
                      {priceLabel(product.provenance)}
                    </p>
                  </td>
                  <td>{formatPrice(product.colorways.length)}</td>
                  <td>
                    <span
                      className={`admin-availability${isAvailable(product) ? "" : " is-unavailable"}`}
                    >
                      {isAvailable(product) ? "موجود" : "ناموجود"}
                    </span>
                  </td>
                  <td>
                    <Button
                      type="button"
                      variant="outline"
                      className="preview-secondary"
                      aria-label={`ویرایش ${product.name}`}
                      aria-pressed={product.id === selectedProduct.id}
                      onClick={() => {
                        selectProduct(product);
                        editorHeading.current?.focus();
                      }}
                    >
                      ویرایش
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
      <p className="admin-end-note">
        این ویرایشگر عمومی به حساب کاربری یا دادهٔ واقعی متصل نیست. نام، قیمت،
        توضیح و وضعیت نمونه را می‌توانی همین‌جا امتحان کنی.
      </p>
    </div>
  );
}
