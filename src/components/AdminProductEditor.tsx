import { useRef, useState, type SubmitEvent } from "react";
import {
  AdminCatalogError,
  getAdminProduct,
  mutateAdminProduct,
  type AdminCategory,
  type AdminProduct,
} from "@/lib/admin-catalog";

type Props = {
  product: AdminProduct | null;
  categories: readonly AdminCategory[];
  onSaved: (product: AdminProduct) => void;
  onUnauthorized: () => void;
};

export default function AdminProductEditor({
  product,
  categories,
  onSaved,
  onUnauthorized,
}: Props) {
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [conflict, setConflict] = useState(false);
  const lock = useRef(false);
  const base = product ? `/${encodeURIComponent(product.ID)}` : "";
  async function perform(action: () => Promise<AdminProduct>) {
    if (lock.current) return;
    lock.current = true;
    setBusy(true);
    setMessage("");
    try {
      const saved = await action();
      onSaved(saved);
      setConflict(false);
      setMessage("تغییرات ذخیره شد.");
    } catch (error: unknown) {
      if (error instanceof AdminCatalogError && error.status === 401) {
        onUnauthorized();
        return;
      }
      const stale = error instanceof AdminCatalogError && error.status === 409;
      setConflict(stale);
      setMessage(
        stale
          ? "نسخهٔ محصول تغییر کرده است. نسخهٔ تازه را دریافت کن و تغییرات را دوباره وارد کن."
          : error instanceof AdminCatalogError && error.status === 400
            ? "اطلاعات معتبر نیست. قیمت، دسته‌بندی و شرایط انتشار (تصویر و ترکیب فعال) را بررسی کن."
            : "ذخیره انجام نشد. اتصال و دسترسی را بررسی و دوباره تلاش کن.",
      );
    } finally {
      lock.current = false;
      setBusy(false);
    }
  }
  function save(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const price = Number(data.get("price"));
    if (!Number.isSafeInteger(price) || price <= 0) {
      setMessage("قیمت باید عدد صحیح مثبت به تومان باشد.");
      return;
    }
    void perform(() =>
      mutateAdminProduct(base, product ? "PATCH" : "POST", {
        ...(product ? { version: product.Version } : {}),
        name: String(data.get("name")).trim(),
        slug: String(data.get("slug")).trim(),
        description: String(data.get("description")).trim(),
        category_id: data.get("category"),
        regular_price_toman: price,
      }),
    );
  }
  return (
    <section
      className="admin-editor-section"
      aria-label={product ? "ویرایش محصول" : "محصول جدید"}
    >
      <h3>{product ? "ویرایش محصول" : "محصول جدید"}</h3>
      <form onSubmit={save} className="admin-edit-form">
        <fieldset disabled={busy || conflict}>
          <label>
            نام محصول
            <input
              name="name"
              required
              maxLength={200}
              defaultValue={product?.Name ?? ""}
            />
          </label>
          <label>
            نشانی محصول
            <input
              name="slug"
              required
              dir="ltr"
              defaultValue={product?.Slug ?? ""}
            />
          </label>
          <label>
            توضیحات
            <textarea
              name="description"
              defaultValue={product?.Description ?? ""}
            />
          </label>
          <label>
            دسته‌بندی
            <select
              name="category"
              required
              defaultValue={product?.PrimaryCategory ?? ""}
            >
              <option value="" disabled>
                انتخاب دسته‌بندی
              </option>
              {categories
                .filter((c) => c.Active || c.ID === product?.PrimaryCategory)
                .map((c) => (
                  <option key={c.ID} value={c.ID}>
                    {c.Name}
                  </option>
                ))}
            </select>
          </label>
          <label>
            قیمت پایه به تومان
            <input
              name="price"
              type="number"
              min="1"
              step="1"
              required
              defaultValue={product?.RegularPriceToman ?? ""}
            />
          </label>
          <button className="preview-primary" type="submit">
            {busy
              ? "در حال ذخیره…"
              : product
                ? "ذخیره تغییرات"
                : "ساخت پیش‌نویس"}
          </button>
        </fieldset>
      </form>
      {message && (
        <p role="status" className="auth-status">
          {message}
        </p>
      )}
      {conflict && product && (
        <button
          className="preview-secondary"
          disabled={busy}
          onClick={() => void perform(() => getAdminProduct(product.ID))}
        >
          دریافت نسخهٔ تازه
        </button>
      )}
      {product && (
        <fieldset disabled={busy || conflict}>
          <legend>انتشار و تصاویر</legend>
          <h4>موجودی ترکیب‌ها</h4>
          <p>موجودی اولیه صفر است. تعداد واقعی را همراه دلیل ثبت کن.</p>
          {product.Variants.map((variant) => (
            <form
              key={`${variant.ID}:${variant.StockVersion}`}
              onSubmit={(event) => {
                event.preventDefault();
                const data = new FormData(event.currentTarget);
                const quantity = Number(data.get("quantity"));
                if (
                  !Number.isSafeInteger(quantity) ||
                  quantity < 0 ||
                  quantity > 1000000
                ) {
                  setMessage("موجودی باید عدد صحیح بین صفر و یک میلیون باشد.");
                  return;
                }
                void perform(() =>
                  mutateAdminProduct(
                    `${base}/variants/${encodeURIComponent(variant.ID)}/stock`,
                    "PATCH",
                    {
                      version: variant.StockVersion,
                      quantity,
                      reason: String(data.get("reason")).trim(),
                    },
                  ),
                );
              }}
            >
              <p>
                {variant.Color ?? "بدون رنگ"} · {variant.Size ?? "بدون اندازه"}{" "}
                · رزروشده: {variant.ReservedQuantity ?? 0}
              </p>
              <label>
                موجودی {variant.ID}
                <input
                  name="quantity"
                  type="number"
                  min="0"
                  max="1000000"
                  step="1"
                  required
                  defaultValue={variant.StockOnHand}
                />
              </label>
              <label>
                دلیل تغییر {variant.ID}
                <input name="reason" required maxLength={300} />
              </label>
              <button className="preview-secondary" type="submit">
                ثبت موجودی {variant.ID}
              </button>
            </form>
          ))}

          <p>انتشار به تصویر، ترکیب فعال و اطلاعات کامل محصول نیاز دارد.</p>
          <button
            className="preview-primary"
            type="button"
            onClick={() =>
              void perform(() =>
                mutateAdminProduct(
                  `${base}/${product.PublicationState === "published" ? "hide" : "publish"}`,
                  "POST",
                  { version: product.Version },
                ),
              )
            }
          >
            {product.PublicationState === "published"
              ? "پنهان کردن محصول"
              : "انتشار محصول"}
          </button>
          <form
            onSubmit={(event) => {
              event.preventDefault();
              const form = event.currentTarget;
              const body = new FormData(form);
              const files = body.getAll("images");
              if (
                files.some(
                  (f) =>
                    f instanceof File &&
                    (f.size > 10 * 1024 * 1024 || f.size === 0),
                ) ||
                files.length + product.Images.length > 10
              ) {
                setMessage("حداکثر ۱۰ تصویر؛ هر تصویر حداکثر ۱۰ مگابایت.");
                return;
              }
              body.set("version", String(product.Version));
              void perform(() =>
                mutateAdminProduct(`${base}/images`, "POST", body).then(
                  (saved) => {
                    form.reset();
                    return saved;
                  },
                ),
              );
            }}
          >
            <label>
              افزودن تصاویر
              <input
                name="images"
                type="file"
                accept="image/jpeg,image/png,image/webp"
                multiple
                required
              />
            </label>
            <label>
              توضیح تصاویر
              <input name="alt_text" />
            </label>
            <button className="preview-secondary" type="submit">
              بارگذاری تصاویر
            </button>
          </form>
          <ol>
            {product.Images.map((image, index) => (
              <li key={image.ID}>
                <span>
                  {image.AltText ?? `تصویر ${index + 1}`}
                  {index === 0 ? " (جلد)" : ""}
                </span>{" "}
                {index > 0 && (
                  <button
                    type="button"
                    className="preview-secondary"
                    onClick={() => {
                      const ids = product.Images.map((i) => i.ID);
                      [ids[index - 1], ids[index]] = [
                        ids[index]!,
                        ids[index - 1]!,
                      ];
                      void perform(() =>
                        mutateAdminProduct(`${base}/images`, "PATCH", {
                          version: product.Version,
                          image_ids: ids,
                        }),
                      );
                    }}
                  >
                    انتقال تصویر {index + 1} به قبل
                  </button>
                )}
                <button
                  type="button"
                  className="preview-secondary"
                  onClick={() =>
                    void perform(() =>
                      mutateAdminProduct(
                        `${base}/images/${encodeURIComponent(image.ID)}`,
                        "DELETE",
                        { version: product.Version },
                      ),
                    )
                  }
                >
                  حذف تصویر {index + 1}
                </button>
              </li>
            ))}
          </ol>
          <form
            onSubmit={(event) => {
              event.preventDefault();
              const form = event.currentTarget;
              const data = new FormData(form);
              void perform(() =>
                mutateAdminProduct(`${base}/variants`, "POST", {
                  version: product.Version,
                  size: String(data.get("size")).trim() || null,
                  color: String(data.get("color")).trim() || null,
                  active: true,
                }).then((saved) => {
                  form.reset();
                  return saved;
                }),
              );
            }}
          >
            <h4>افزودن ترکیب فعال</h4>
            <label>
              اندازه
              <input name="size" />
            </label>
            <label>
              رنگ
              <input name="color" />
            </label>
            <button className="preview-secondary" type="submit">
              افزودن ترکیب
            </button>
          </form>
        </fieldset>
      )}
    </section>
  );
}
