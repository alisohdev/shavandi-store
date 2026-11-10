import AdminProductEditor from "./AdminProductEditor";
import { useEffect, useMemo, useState } from "react";
import {
  AdminCatalogError,
  effectiveVariantPrice,
  getAdminProduct,
  listAdminCategories,
  listAdminProducts,
  type AdminCategory,
  type AdminProduct,
} from "@/lib/admin-catalog";

type Props = { readonly onUnauthorized: () => void };
const money = new Intl.NumberFormat("fa-IR");
const stateLabel: Record<AdminProduct["PublicationState"], string> = {
  draft: "پیش‌نویس",
  published: "منتشرشده",
  hidden: "پنهان",
};

function errorMessage(error: unknown): string {
  if (error instanceof AdminCatalogError) {
    if (error.status === 403) return "این حساب به فهرست مدیریت دسترسی ندارد.";
    if (error.status === 404) return "این محصول دیگر در فهرست نیست.";
  }
  return "دریافت اطلاعات از سرور ممکن نشد. اتصال API را بررسی و دوباره تلاش کن.";
}

export default function AdminCatalogRead({ onUnauthorized }: Props) {
  const [products, setProducts] = useState<readonly AdminProduct[] | null>(
    null,
  );
  const [categories, setCategories] = useState<readonly AdminCategory[]>([]);
  const [selectedID, setSelectedID] = useState("");
  const [detail, setDetail] = useState<AdminProduct | null>(null);
  const [loadingDetail, setLoadingDetail] = useState(false);
  const [error, setError] = useState("");
  const [creating, setCreating] = useState(false);
  const [retry, setRetry] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    setProducts(null);
    setError("");
    void Promise.all([
      listAdminProducts(controller.signal),
      listAdminCategories(controller.signal),
    ])
      .then(([items, actualCategories]) => {
        if (controller.signal.aborted) return;
        setProducts(items);
        setCategories(actualCategories);
        setSelectedID((current) =>
          current && items.some((item) => item.ID === current)
            ? current
            : (items[0]?.ID ?? ""),
        );
      })
      .catch((failure: unknown) => {
        if (controller.signal.aborted) return;
        if (failure instanceof AdminCatalogError && failure.status === 401) {
          onUnauthorized();
          return;
        }
        setError(errorMessage(failure));
      });
    return () => controller.abort();
  }, [onUnauthorized, retry]);

  useEffect(() => {
    if (!selectedID) {
      setDetail(null);
      return;
    }
    const controller = new AbortController();
    setLoadingDetail(true);
    setDetail(null);
    void getAdminProduct(selectedID, controller.signal)
      .then((product) => {
        if (!controller.signal.aborted) setDetail(product);
      })
      .catch((failure: unknown) => {
        if (controller.signal.aborted) return;
        if (failure instanceof AdminCatalogError && failure.status === 401) {
          onUnauthorized();
          return;
        }
        setError(errorMessage(failure));
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoadingDetail(false);
      });
    return () => controller.abort();
  }, [selectedID, onUnauthorized]);

  const categoryByID = useMemo(
    () => new Map(categories.map((category) => [category.ID, category.Name])),
    [categories],
  );

  if (error)
    return (
      <section className="auth-card" role="alert">
        <p className="eyebrow">کاتالوگ مدیریت</p>
        <h2>دریافت اطلاعات انجام نشد</h2>
        <p className="auth-error">{error}</p>
        <button
          className="preview-secondary"
          type="button"
          onClick={() => setRetry((value) => value + 1)}
        >
          تلاش دوباره
        </button>
      </section>
    );
  if (!products)
    return (
      <p className="auth-status" role="status">
        در حال دریافت محصولات و دسته‌بندی‌ها…
      </p>
    );

  return (
    <div className="admin-editor-preview">
      <p className="preview-notice admin-scope-note">
        تغییرات پس از ذخیره در کاتالوگ ثبت می‌شوند. پیش‌نویس‌ها تا زمان انتشار
        در دسترس مشتری نیستند.
      </p>
      <button
        type="button"
        className="preview-secondary"
        onClick={() => setCreating((value) => !value)}
      >
        {creating ? "بستن محصول جدید" : "محصول جدید"}
      </button>
      {creating && (
        <AdminProductEditor
          product={null}
          categories={categories}
          onUnauthorized={onUnauthorized}
          onSaved={(saved) => {
            setCreating(false);
            setProducts((items) => [saved, ...(items ?? [])]);
            setSelectedID(saved.ID);
            setDetail(saved);
          }}
        />
      )}
      <div className="admin-metrics">
        <div>
          <span>محصول‌ها</span>
          <strong>{money.format(products.length)}</strong>
        </div>
        <div>
          <span>دسته‌بندی‌های فعال</span>
          <strong>
            {money.format(categories.filter((item) => item.Active).length)}
          </strong>
        </div>
        <div>
          <span>دسته‌بندی‌های غیرفعال</span>
          <strong>
            {money.format(categories.filter((item) => !item.Active).length)}
          </strong>
        </div>
      </div>
      {products.length === 0 ? (
        <section className="admin-editor-section">
          <p className="eyebrow">کاتالوگ PostgreSQL</p>
          <h2>هنوز محصولی ثبت نشده است.</h2>
          <p>
            پس از افزودن محصول، فهرست و جزئیات آن در همین بخش نمایش داده می‌شود.
          </p>
        </section>
      ) : (
        <>
          <section
            className="admin-products"
            aria-labelledby="admin-catalog-heading"
          >
            <div className="section-heading">
              <div>
                <p className="eyebrow">کاتالوگ PostgreSQL</p>
                <h2 id="admin-catalog-heading">محصول‌ها</h2>
              </div>
            </div>
            <div
              className="admin-table-wrap"
              role="region"
              aria-label="فهرست محصولات"
              tabIndex={0}
            >
              <table className="admin-table">
                <thead>
                  <tr>
                    <th scope="col">نام محصول</th>
                    <th scope="col">قیمت پایه</th>
                    <th scope="col">وضعیت</th>
                    <th scope="col">جزئیات</th>
                  </tr>
                </thead>
                <tbody>
                  {products.map((product) => (
                    <tr
                      key={product.ID}
                      data-selected={product.ID === selectedID}
                    >
                      <th scope="row">
                        {product.Name}
                        <small>
                          <bdi>{product.ID}</bdi>
                        </small>
                      </th>
                      <td>{money.format(product.RegularPriceToman)} تومان</td>
                      <td>
                        <span className="admin-availability">
                          {stateLabel[product.PublicationState]}
                        </span>
                      </td>
                      <td>
                        <button
                          type="button"
                          className="preview-secondary"
                          aria-pressed={product.ID === selectedID}
                          onClick={() => setSelectedID(product.ID)}
                        >
                          مشاهده
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </section>
          <section
            className="admin-editor-section"
            aria-labelledby="admin-product-detail-heading"
          >
            <div className="section-heading admin-editor-heading">
              <div>
                <p className="eyebrow">جزئیات از سرور</p>
                <h2 id="admin-product-detail-heading">
                  {loadingDetail
                    ? "در حال دریافت جزئیات…"
                    : (detail?.Name ?? "محصول")}
                </h2>
              </div>
            </div>
            {detail ? (
              <div>
                <AdminProductEditor
                  key={`${detail.ID}:${detail.Version}`}
                  product={detail}
                  categories={categories}
                  onUnauthorized={onUnauthorized}
                  onSaved={(saved) => {
                    setDetail(saved);
                    setProducts(
                      (items) =>
                        items?.map((item) =>
                          item.ID === saved.ID ? saved : item,
                        ) ?? null,
                    );
                  }}
                />
                <div className="admin-edit-grid">
                  <div className="admin-edit-form">
                    <p>
                      <strong>شناسه:</strong> <bdi>{detail.ID}</bdi>
                    </p>
                    <p>
                      <strong>نسخه:</strong> {money.format(detail.Version)}
                    </p>
                    <p>
                      <strong>نشانی:</strong> <bdi>{detail.Slug}</bdi>
                    </p>
                    <p>
                      <strong>دسته‌بندی:</strong>{" "}
                      {categoryByID.get(detail.PrimaryCategory) ??
                        "دسته‌بندی نامشخص"}
                    </p>
                    <p>
                      <strong>قیمت پایه:</strong>{" "}
                      {money.format(detail.RegularPriceToman)} تومان
                    </p>
                    {detail.SalePriceToman !== null && (
                      <p>
                        <strong>قیمت فروش ویژه:</strong>{" "}
                        {money.format(detail.SalePriceToman)} تومان
                      </p>
                    )}
                    <p>
                      <strong>وضعیت:</strong>{" "}
                      {stateLabel[detail.PublicationState]}
                    </p>
                    <p>{detail.Description || "توضیحی ثبت نشده است."}</p>
                    <h3>ترکیب‌ها</h3>
                    {detail.Variants.length ? (
                      detail.Variants.map((variant) => {
                        const price = effectiveVariantPrice(detail, variant);
                        return (
                          <p key={variant.ID}>
                            {variant.Size ?? "بدون اندازه"} ·{" "}
                            {variant.Color ?? "بدون رنگ"} —{" "}
                            {variant.Active ? "فعال" : "غیرفعال"}
                            {` · قیمت مؤثر: ${money.format(price.regular)} تومان`}
                            {price.sale !== null
                              ? ` · فروش ویژه: ${money.format(price.sale)} تومان`
                              : ""}
                            {variant.RegularPriceOverrideToman !== null
                              ? ` · قیمت پایهٔ اختصاصی: ${money.format(variant.RegularPriceOverrideToman)} تومان`
                              : ""}
                            {variant.SalePriceOverrideToman !== null
                              ? ` · فروش ویژهٔ اختصاصی: ${money.format(variant.SalePriceOverrideToman)} تومان`
                              : ""}
                          </p>
                        );
                      })
                    ) : (
                      <p>ترکیبی ثبت نشده است.</p>
                    )}
                  </div>
                  <aside className="admin-live-preview">
                    <h3>تصاویر</h3>
                    {detail.Images.length ? (
                      detail.Images.map((image) => (
                        <figure key={image.ID}>
                          <img
                            src={image.AssetRef}
                            alt={image.AltText ?? detail.Name}
                            loading="lazy"
                          />
                          <figcaption>
                            {image.AltText ?? detail.Name}
                          </figcaption>
                        </figure>
                      ))
                    ) : (
                      <p>تصویری ثبت نشده است.</p>
                    )}
                  </aside>
                </div>
              </div>
            ) : null}
          </section>
        </>
      )}
      <section
        className="admin-products"
        aria-labelledby="admin-categories-heading"
      >
        <div className="section-heading">
          <div>
            <p className="eyebrow">از PostgreSQL</p>
            <h2 id="admin-categories-heading">دسته‌بندی‌ها</h2>
          </div>
        </div>
        {categories.length ? (
          <ul>
            {categories.map((category) => (
              <li key={category.ID}>
                {category.Name}{" "}
                <small>({category.Active ? "فعال" : "غیرفعال"})</small>
              </li>
            ))}
          </ul>
        ) : (
          <p>دسته‌بندی‌ای ثبت نشده است.</p>
        )}
      </section>
    </div>
  );
}
