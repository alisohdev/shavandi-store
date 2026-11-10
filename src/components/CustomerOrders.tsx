import OrderDeliveryForm from "./OrderDeliveryForm";
import { isOrderDelivery, type OrderDelivery } from "../lib/order-delivery";
import { useEffect, useRef, useState } from "react";

type Order = {
  delivery: OrderDelivery | null;
  id: string;
  status: "pending_confirmation" | "cancelled" | "expired";
  subtotal_toman: number;
  created_at: string;
  expires_at: string;
  lines: {
    variant_id: string;
    name: string;
    size: string | null;
    color: string | null;
    quantity: number;
    unit_price_toman: number;
  }[];
};
class OrderError extends Error {
  constructor(readonly status: number) {
    super("Order request failed");
  }
}
function record(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}
function parseOrder(value: unknown): Order {
  if (
    !record(value) ||
    typeof value.id !== "string" ||
    !(value.delivery === null || isOrderDelivery(value.delivery)) ||
    !["pending_confirmation", "cancelled", "expired"].includes(
      String(value.status),
    ) ||
    typeof value.subtotal_toman !== "number" ||
    !Number.isSafeInteger(value.subtotal_toman) ||
    value.subtotal_toman < 0 ||
    typeof value.created_at !== "string" ||
    typeof value.expires_at !== "string" ||
    !Number.isFinite(Date.parse(value.expires_at)) ||
    !Array.isArray(value.lines) ||
    !value.lines.every(
      (line) =>
        record(line) &&
        typeof line.name === "string" &&
        typeof line.variant_id === "string" &&
        typeof line.quantity === "number" &&
        Number.isInteger(line.quantity) &&
        line.quantity >= 1 &&
        line.quantity <= 9 &&
        typeof line.unit_price_toman === "number" &&
        Number.isSafeInteger(line.unit_price_toman) &&
        line.unit_price_toman >= 0 &&
        (line.color === null || typeof line.color === "string") &&
        (line.size === null || typeof line.size === "string"),
    )
  )
    throw new Error("Invalid order response");
  return value as Order;
}
async function orderRequest(path: string, body?: object): Promise<unknown> {
  let headers: Record<string, string> = {};
  if (body) {
    const csrfResponse = await fetch("/api/v1/auth/csrf", {
      credentials: "include",
      cache: "no-store",
    });
    if (!csrfResponse.ok) throw new OrderError(csrfResponse.status);
    const csrf: unknown = await csrfResponse.json();
    if (!record(csrf) || typeof csrf.csrf_token !== "string")
      throw new Error("Invalid CSRF response");
    headers = {
      "Content-Type": "application/json",
      "X-CSRF-Token": csrf.csrf_token,
    };
  }
  const response = await fetch(`/api/v1/orders${path}`, {
    method: body ? "POST" : "GET",
    credentials: "include",
    cache: "no-store",
    headers,
    ...(body ? { body: JSON.stringify(body) } : {}),
  });
  if (!response.ok) throw new OrderError(response.status);
  return response.json();
}
function errorMessage(error: unknown) {
  return error instanceof OrderError && error.status === 401
    ? "برای دیدن یا رزرو سفارش وارد حساب شو."
    : error instanceof OrderError && error.status === 409
      ? "سبد یا قیمت‌ها تغییر کرده یا سفارش رزروشده‌ای داری. سفارش‌ها را بررسی و سبد را تازه کن."
      : "پاسخ سفارش دریافت نشد. ابتدا سفارش‌ها را بررسی کن؛ تلاش دوباره با همین درخواست، سفارش تکراری نمی‌سازد.";
}
const money = new Intl.NumberFormat("fa-IR");
export function ReserveOrder({
  quote,
  onCreated,
}: {
  quote: string;
  onCreated: (status: string) => void;
}) {
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const [created, setCreated] = useState(false);
  const lock = useRef(false);
  const request = useRef<{ quote: string; key: string } | null>(null);
  async function reserve() {
    if (lock.current) return;
    lock.current = true;
    setBusy(true);
    setMessage("");
    try {
      if (request.current?.quote !== quote)
        request.current = { quote, key: crypto.randomUUID() };
      const body = await orderRequest("", {
        quote_token: quote,
        key: request.current.key,
      });
      if (!record(body)) throw new Error("Invalid order response");
      const order = parseOrder(body.order);
      setCreated(true);
      setMessage(
        order.status === "pending_confirmation"
          ? "سفارش رزرو شد. پرداخت و ارسال هنوز فعال نیست؛ رزرو پس از ۱۵ دقیقه آزاد می‌شود."
          : "این درخواست قبلاً ثبت شده است. وضعیت آن را در سفارش‌ها ببین.",
      );
      onCreated(order.status);
    } catch (error) {
      setMessage(errorMessage(error));
    } finally {
      lock.current = false;
      setBusy(false);
    }
  }
  return (
    <section>
      <p>
        با تأیید، کالاها برای ۱۵ دقیقه رزرو و سبد خالی می‌شود. این مبلغ فقط جمع
        کالاهاست؛ هزینهٔ ارسال و پرداخت هنوز فعال نیست.
      </p>
      <button
        type="button"
        className="primary-link"
        disabled={busy || created}
        onClick={() => void reserve()}
      >
        {busy ? "در حال رزرو…" : "تأیید و رزرو سفارش"}
      </button>
      {message && <p role="status">{message}</p>}
      <a className="text-link" href="/orders/">
        دیدن سفارش‌ها ←
      </a>
    </section>
  );
}
export default function CustomerOrders() {
  const [orders, setOrders] = useState<Order[] | null>(null);
  const [message, setMessage] = useState("");
  const [login, setLogin] = useState(false);
  const [busy, setBusy] = useState(false);
  const lock = useRef(false);
  async function load() {
    if (lock.current) return;
    lock.current = true;
    setBusy(true);
    setMessage("");
    try {
      const value = await orderRequest("");
      if (!record(value) || !Array.isArray(value.orders))
        throw new Error("Invalid orders response");
      setOrders(value.orders.map(parseOrder));
      setLogin(false);
    } catch (error) {
      setOrders(null);
      setMessage(errorMessage(error));
      setLogin(error instanceof OrderError && error.status === 401);
    } finally {
      lock.current = false;
      setBusy(false);
    }
  }
  useEffect(() => {
    void load();
  }, []);
  async function cancel(id: string) {
    if (lock.current) return;
    lock.current = true;
    setBusy(true);
    setMessage("");
    try {
      const value = await orderRequest(`/${encodeURIComponent(id)}/cancel`, {});
      if (!record(value)) throw new Error("Invalid order response");
      const order = parseOrder(value.order);
      setOrders(
        (current) =>
          current?.map((item) => (item.id === id ? order : item)) ?? null,
      );
    } catch (error) {
      setMessage(errorMessage(error));
    } finally {
      lock.current = false;
      setBusy(false);
    }
  }

  async function saveDelivery(id: string, delivery: OrderDelivery) {
    if (lock.current) return;
    lock.current = true;
    setBusy(true);
    setMessage("");
    try {
      const value = await orderRequest(
        `/${encodeURIComponent(id)}/delivery`,
        delivery,
      );
      if (!record(value)) throw new Error("Invalid order response");
      const updated = parseOrder(value.order);
      setOrders(
        (current) =>
          current?.map((item) => (item.id === id ? updated : item)) ?? null,
      );
      setMessage("اطلاعات گیرنده و روش ارسال ذخیره شد. پرداخت هنوز فعال نیست.");
    } catch (error) {
      setMessage(
        error instanceof OrderError && error.status === 400
          ? "اطلاعات گیرنده معتبر نیست. نام، همراه، نشانی و کد پستی را بررسی کن."
          : error instanceof OrderError && error.status === 409
            ? "نسخهٔ نشانی تغییر کرده یا مهلت رزرو تمام شده است. سفارش‌ها را تازه کن؛ با تازه‌کردن، نوشته‌های ذخیره‌نشده جایگزین می‌شوند."
            : errorMessage(error),
      );
    } finally {
      lock.current = false;
      setBusy(false);
    }
  }
  const status = {
    pending_confirmation: "رزرو موقت",
    cancelled: "لغوشده",
    expired: "منقضی‌شده",
  };
  return (
    <section className="live-cart">
      <p>۲۰ سفارش اخیر حساب. پرداخت و ارسال هنوز فعال نیست.</p>
      {message && <p role="alert">{message}</p>}
      {login && <a href="/login/">ورود به حساب</a>}
      <button
        className="preview-secondary"
        disabled={busy}
        onClick={() => void load()}
      >
        تازه‌کردن سفارش‌ها
      </button>
      {busy && <p role="status">در حال دریافت…</p>}
      {orders?.length === 0 && <p>هنوز سفارشی ثبت نکرده‌ای.</p>}
      <ul>
        {orders?.map((order) => (
          <li key={order.id}>
            <h2>
              سفارش <bdi>{order.id}</bdi>
            </h2>
            <p>{status[order.status]}</p>
            <p>جمع کالاها: {money.format(order.subtotal_toman)} تومان</p>
            <ul>
              {order.lines.map((line) => (
                <li key={line.variant_id}>
                  {line.name} · {line.color ?? "بدون رنگ"} ·{" "}
                  {line.size ?? "بدون اندازه"} · {money.format(line.quantity)}{" "}
                  عدد · هر واحد {money.format(line.unit_price_toman)} تومان
                </li>
              ))}
            </ul>
            {order.delivery && (
              <section aria-label="گیرندهٔ ذخیره‌شده">
                <h3>گیرندهٔ ذخیره‌شده</h3>
                <p>
                  {order.delivery.recipient} · <bdi>{order.delivery.phone}</bdi>
                </p>
                <p>
                  {order.delivery.province}، {order.delivery.city}،{" "}
                  {order.delivery.address}
                </p>
                <p>
                  کد پستی: <bdi>{order.delivery.postal_code}</bdi>
                </p>
                {order.delivery.shipping_fee_toman != null ? (
                  <>
                    <p>
                      {order.delivery.shipping_name} · هزینهٔ ارسال:{" "}
                      {money.format(order.delivery.shipping_fee_toman)} تومان
                    </p>
                    <p>
                      جمع کالا و ارسال:{" "}
                      {money.format(
                        order.subtotal_toman +
                          order.delivery.shipping_fee_toman,
                      )}{" "}
                      تومان
                    </p>
                  </>
                ) : (
                  <p>هزینهٔ ارسال تعیین نشده است.</p>
                )}
              </section>
            )}
            {order.status === "pending_confirmation" && (
              <>
                <OrderDeliveryForm
                  key={`${order.id}:${order.delivery?.version ?? 0}`}
                  delivery={order.delivery}
                  busy={busy}
                  onSave={(delivery) => saveDelivery(order.id, delivery)}
                />
                <p>
                  مهلت رزرو:{" "}
                  <time dateTime={order.expires_at}>
                    {new Date(order.expires_at).toLocaleString("fa-IR")}
                  </time>
                </p>
                <button
                  className="preview-secondary"
                  disabled={busy}
                  onClick={() => void cancel(order.id)}
                >
                  لغو رزرو {order.id}
                </button>
              </>
            )}
          </li>
        ))}
      </ul>
      <a className="text-link" href="/cart/">
        دیدن سبد ←
      </a>
    </section>
  );
}
