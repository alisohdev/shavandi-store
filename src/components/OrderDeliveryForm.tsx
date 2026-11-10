import { useEffect, useState, type SubmitEvent } from "react";
import { parseShippingMethods, type ShippingMethod } from "../lib/shipping";
import type { OrderDelivery } from "../lib/order-delivery";

type Props = {
  delivery: OrderDelivery | null;
  busy: boolean;
  onSave: (delivery: OrderDelivery) => Promise<void>;
};
export default function OrderDeliveryForm({ delivery, busy, onSave }: Props) {
  const [methods, setMethods] = useState<ShippingMethod[] | null>(null);
  const [error, setError] = useState("");
  async function load() {
    try {
      const response = await fetch("/api/v1/shipping-methods", {
        cache: "no-store",
      });
      if (!response.ok) throw new Error();
      setMethods(parseShippingMethods(await response.json()));
      setError("");
    } catch {
      setError("دریافت روش‌های ارسال انجام نشد. دوباره تلاش کن.");
    }
  }
  useEffect(() => {
    void load();
  }, []);
  function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const method = methods?.find((item) => item.id === data.get("shipping"));
    void onSave({
      shipping_method_id: method?.id ?? null,
      shipping_version: method?.version ?? null,
      recipient: String(data.get("recipient")).trim(),
      phone: String(data.get("phone")).trim(),
      province: String(data.get("province")).trim(),
      city: String(data.get("city")).trim(),
      address: String(data.get("address")).trim(),
      postal_code: String(data.get("postal_code")).trim(),
      version: delivery?.version ?? 0,
    });
  }
  return (
    <form className="live-delivery-form" onSubmit={submit}>
      <fieldset disabled={busy || !methods}>
        <legend>اطلاعات گیرنده</legend>
        <p>
          این اطلاعات برای همین سفارش در سرور ذخیره می‌شود. ذخیرهٔ نشانی، مهلت
          رزرو را تمدید نمی‌کند. هزینهٔ روش انتخاب‌شده با ذخیرهٔ اطلاعات ثبت
          می‌شود.
        </p>
        {methods && methods.length > 0 ? (
          <label>
            روش ارسال
            <select
              aria-label="روش ارسال"
              name="shipping"
              required
              defaultValue={delivery?.shipping_method_id ?? ""}
            >
              <option value="" disabled>
                انتخاب روش ارسال
              </option>
              {methods.map((method) => (
                <option key={method.id} value={method.id}>
                  {method.name} —{" "}
                  {new Intl.NumberFormat("fa-IR").format(method.fee_toman)}{" "}
                  تومان
                </option>
              ))}
            </select>
          </label>
        ) : (
          <p>
            هنوز روش ارسال فعالی تعریف نشده است؛ هزینهٔ ارسال تعیین نشده است.
          </p>
        )}
        <label>
          نام گیرنده
          <input
            name="recipient"
            autoComplete="shipping name"
            required
            minLength={2}
            maxLength={100}
            defaultValue={delivery?.recipient ?? ""}
          />
        </label>
        <label>
          همراه گیرنده
          <input
            name="phone"
            type="tel"
            inputMode="tel"
            autoComplete="shipping tel"
            required
            maxLength={30}
            defaultValue={delivery?.phone ?? ""}
          />
        </label>
        <label>
          استان
          <input
            name="province"
            autoComplete="shipping address-level1"
            required
            minLength={2}
            maxLength={100}
            defaultValue={delivery?.province ?? ""}
          />
        </label>
        <label>
          شهر
          <input
            name="city"
            autoComplete="shipping address-level2"
            required
            minLength={2}
            maxLength={100}
            defaultValue={delivery?.city ?? ""}
          />
        </label>
        <label>
          نشانی کامل
          <textarea
            name="address"
            autoComplete="shipping street-address"
            required
            minLength={10}
            maxLength={500}
            defaultValue={delivery?.address ?? ""}
          />
        </label>
        <label>
          کد پستی
          <input
            name="postal_code"
            inputMode="numeric"
            autoComplete="shipping postal-code"
            required
            minLength={10}
            maxLength={10}
            defaultValue={delivery?.postal_code ?? ""}
          />
        </label>
        <button type="submit" className="primary-link">
          {busy ? "در حال ذخیره…" : "ذخیره اطلاعات گیرنده"}
        </button>
      </fieldset>
      {error && <p role="alert">{error}</p>}
      {!methods && (
        <button
          type="button"
          className="preview-secondary"
          disabled={busy}
          onClick={() => void load()}
        >
          دریافت دوباره روش‌های ارسال
        </button>
      )}
    </form>
  );
}
