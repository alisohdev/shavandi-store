import { useEffect, useRef, useState } from "react";

type Props = {
  id: string;
  version: number;
  total: number;
  state: string;
  onChanged: () => Promise<void>;
};
export default function OrderPayment({
  id,
  version,
  total,
  state,
  onChanged,
}: Props) {
  const [enabled, setEnabled] = useState(false);
  const [sandbox, setSandbox] = useState(false);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const lock = useRef(false);
  useEffect(() => {
    const controller = new AbortController();
    void fetch("/api/v1/payments/config", {
      cache: "no-store",
      signal: controller.signal,
    })
      .then((r) => r.json())
      .then((value: unknown) => {
        if (
          typeof value === "object" &&
          value !== null &&
          "enabled" in value &&
          value.enabled === true
        ) {
          setEnabled(true);
          setSandbox("sandbox" in value && value.sandbox === true);
        }
      })
      .catch(() => {});
    return () => controller.abort();
  }, []);
  async function action(verify: boolean) {
    if (lock.current) return;
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
      const response = await fetch(
        `/api/v1/orders/${encodeURIComponent(id)}/payment${verify ? "/verify" : ""}`,
        {
          method: "POST",
          credentials: "include",
          cache: "no-store",
          headers: {
            "Content-Type": "application/json",
            "X-CSRF-Token": token.csrf_token,
          },
          body: JSON.stringify(
            verify
              ? {}
              : { delivery_version: version, expected_total_toman: total },
          ),
        },
      );
      if (!response.ok) {
        setMessage(
          response.status === 409
            ? "پرداخت هنوز تأیید نشده یا وضعیت سفارش تغییر کرده است. سفارش‌ها را تازه کن."
            : "پاسخ پرداخت دریافت نشد. دوباره پرداخت نکن؛ وضعیت را بررسی و سپس سفارش‌ها را تازه کن.",
        );
        return;
      }
      if (verify) {
        await onChanged();
        setMessage("وضعیت پرداخت بررسی شد.");
        return;
      }
      const value: unknown = await response.json();
      if (
        typeof value !== "object" ||
        value === null ||
        !("redirect_url" in value) ||
        typeof value.redirect_url !== "string"
      )
        throw new Error();
      const url = new URL(value.redirect_url);
      if (
        ![
          "https://payment.zarinpal.com",
          "https://sandbox.zarinpal.com",
        ].includes(url.origin) ||
        !/^\/pg\/StartPay\/[AS][A-Za-z0-9]{35}$/.test(url.pathname) ||
        url.search ||
        url.hash ||
        url.username ||
        url.password
      )
        throw new Error();
      window.location.assign(url.href);
    } catch {
      setMessage(
        "ارتباط با پرداخت برقرار نشد. قبل از تلاش دوباره، وضعیت سفارش را بررسی کن.",
      );
    } finally {
      lock.current = false;
      setBusy(false);
    }
  }
  if (!enabled) return <p>پرداخت آنلاین هنوز فعال نشده است.</p>;
  return (
    <section aria-label="پرداخت سفارش">
      <p>
        {sandbox
          ? "محیط آزمایشی زرین‌پال؛ پرداخت واقعی انجام نمی‌شود."
          : "اطلاعات کارت فقط در درگاه زرین‌پال وارد می‌شود."}
      </p>
      {(state === "none" || state === "ready") && (
        <button
          className="primary-link"
          disabled={busy || total <= 0}
          onClick={() => void action(false)}
        >
          {state === "ready"
            ? "ادامه پرداخت در زرین‌پال"
            : sandbox
              ? "پرداخت آزمایشی زرین‌پال"
              : "پرداخت با زرین‌پال"}
        </button>
      )}
      {state === "ready" && (
        <button
          className="preview-secondary"
          disabled={busy}
          onClick={() => void action(true)}
        >
          بررسی نتیجه پرداخت
        </button>
      )}
      {(state === "requesting" || state === "unknown") && (
        <p>
          نتیجهٔ درخواست پرداخت نامشخص است. پرداخت تازه‌ای نساز؛ برای بررسی با
          فروشگاه تماس بگیر.
        </p>
      )}
      {state === "failed" && (
        <p>درخواست درگاه پذیرفته نشد. رزرو را لغو کن و سفارش تازه‌ای بساز.</p>
      )}
      {message && <p role="status">{message}</p>}
    </section>
  );
}
