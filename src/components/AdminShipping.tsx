import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type SubmitEvent,
} from "react";
import { parseShippingMethods, type ShippingMethod } from "../lib/shipping";
export default function AdminShipping({
  onUnauthorized,
}: {
  onUnauthorized: () => void;
}) {
  const [methods, setMethods] = useState<ShippingMethod[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const lock = useRef(false);
  const load = useCallback(async () => {
    try {
      const response = await fetch("/api/v1/admin/shipping-methods", {
        credentials: "include",
        cache: "no-store",
      });
      if (response.status === 401) {
        onUnauthorized();
        return;
      }
      if (!response.ok) throw new Error();
      setMethods(parseShippingMethods(await response.json()));
      setMessage("");
    } catch {
      setMessage("دریافت روش‌های ارسال انجام نشد. دوباره تلاش کن.");
    }
  }, [onUnauthorized]);
  useEffect(() => {
    void load();
  }, [load]);
  async function save(
    event: SubmitEvent<HTMLFormElement>,
    method?: ShippingMethod,
  ) {
    event.preventDefault();
    if (lock.current) return;
    const form = event.currentTarget;
    const data = new FormData(form);
    const fee = Number(data.get("fee"));
    if (!Number.isSafeInteger(fee) || fee < 0 || fee > 1000000000) {
      setMessage("هزینهٔ ارسال باید عدد صحیح بین صفر و یک میلیارد تومان باشد.");
      return;
    }
    lock.current = true;
    setBusy(true);
    setMessage("");
    try {
      const csrf = await fetch("/api/v1/auth/csrf", {
        credentials: "include",
        cache: "no-store",
      });
      const token: unknown = await csrf.json();
      if (
        !csrf.ok ||
        typeof token !== "object" ||
        token === null ||
        !("csrf_token" in token) ||
        typeof token.csrf_token !== "string"
      )
        throw new Error();
      const response = await fetch("/api/v1/admin/shipping-methods", {
        method: "POST",
        credentials: "include",
        cache: "no-store",
        headers: {
          "Content-Type": "application/json",
          "X-CSRF-Token": token.csrf_token,
        },
        body: JSON.stringify({
          id: method?.id ?? "",
          version: method?.version ?? 0,
          name: String(data.get("name")).trim(),
          fee_toman: fee,
          active: data.get("active") === "on",
        }),
      });
      if (response.status === 401) {
        onUnauthorized();
        return;
      }
      if (response.status === 409) {
        setMessage(
          "این روش تغییر کرده است. فهرست را تازه کن و تغییرات را دوباره وارد کن.",
        );
        return;
      }
      if (!response.ok) throw new Error();
      if (!method) form.reset();
      await load();
      setMessage("روش ارسال ذخیره شد.");
    } catch {
      setMessage("ذخیرهٔ روش ارسال انجام نشد. اطلاعات و اتصال را بررسی کن.");
    } finally {
      lock.current = false;
      setBusy(false);
    }
  }
  return (
    <section className="admin-editor-section" aria-label="تنظیمات ارسال">
      <h2>روش‌ها و هزینهٔ ارسال</h2>
      <p>
        برای هر روش یک نام و هزینه به تومان تعریف کن. صفر یعنی ارسال رایگان. روش
        غیرفعال برای انتخاب تازه نمایش داده نمی‌شود.
      </p>
      {message && <p role="status">{message}</p>}
      <button
        type="button"
        className="preview-secondary"
        disabled={busy}
        onClick={() => void load()}
      >
        تازه‌کردن روش‌های ارسال
      </button>
      {methods?.map((method) => (
        <form
          key={`${method.id}:${method.version}`}
          onSubmit={(event) => void save(event, method)}
        >
          <fieldset disabled={busy}>
            <legend>{method.name}</legend>
            <label>
              نام روش {method.id}
              <input
                name="name"
                required
                minLength={2}
                maxLength={100}
                defaultValue={method.name}
              />
            </label>
            <label>
              هزینهٔ ارسال {method.id}
              <input
                name="fee"
                type="number"
                min="0"
                max="1000000000"
                step="1"
                required
                defaultValue={method.fee_toman}
              />
            </label>
            <label>
              <input
                name="active"
                type="checkbox"
                defaultChecked={method.active}
              />
              فعال
            </label>
            <button type="submit" className="preview-secondary">
              ذخیره روش {method.id}
            </button>
          </fieldset>
        </form>
      ))}
      {methods && (
        <form onSubmit={(event) => void save(event)}>
          <fieldset disabled={busy}>
            <legend>روش ارسال جدید</legend>
            <label>
              نام روش جدید
              <input name="name" required minLength={2} maxLength={100} />
            </label>
            <label>
              هزینهٔ روش جدید به تومان
              <input
                name="fee"
                type="number"
                min="0"
                max="1000000000"
                step="1"
                required
              />
            </label>
            <label>
              <input name="active" type="checkbox" defaultChecked />
              فعال
            </label>
            <button className="preview-primary" type="submit">
              افزودن روش ارسال
            </button>
          </fieldset>
        </form>
      )}
    </section>
  );
}
