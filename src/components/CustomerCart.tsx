import { useEffect, useRef, useState, type SubmitEvent } from "react";
import type { PublicProduct } from "../lib/public-catalog";

type CartLine = {
  variant_id: string;
  product_slug: string;
  name: string;
  size: string | null;
  color: string | null;
  quantity: number;
  available: boolean;
  unit_price_toman: number | null;
  line_total_toman: number | null;
};
type Cart = {
  lines: CartLine[];
  subtotal_toman: number;
  has_unavailable: boolean;
};
function record(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}
function nullableText(value: unknown) {
  return value === null || typeof value === "string";
}
function validPrice(value: unknown) {
  return typeof value === "number" && Number.isSafeInteger(value) && value >= 0;
}
function validLine(value: unknown) {
  return (
    record(value) &&
    typeof value.variant_id === "string" &&
    typeof value.product_slug === "string" &&
    typeof value.name === "string" &&
    nullableText(value.size) &&
    nullableText(value.color) &&
    typeof value.quantity === "number" &&
    Number.isInteger(value.quantity) &&
    value.quantity >= 1 &&
    value.quantity <= 9 &&
    typeof value.available === "boolean" &&
    (value.available
      ? validPrice(value.unit_price_toman) && validPrice(value.line_total_toman)
      : value.unit_price_toman === null && value.line_total_toman === null)
  );
}
class CartError extends Error {
  constructor(readonly status: number) {
    super("Cart request failed");
  }
}
async function requestCart(
  variantID?: string,
  quantity?: number,
): Promise<Cart> {
  let headers: Record<string, string> = {};
  if (variantID) {
    const response = await fetch("/api/v1/auth/csrf", {
      credentials: "include",
      cache: "no-store",
    });
    if (!response.ok) throw new CartError(response.status);
    const token: unknown = await response.json();
    if (
      typeof token !== "object" ||
      token === null ||
      !("csrf_token" in token) ||
      typeof token.csrf_token !== "string"
    )
      throw new Error("Invalid CSRF response");
    headers = {
      "Content-Type": "application/json",
      "X-CSRF-Token": token.csrf_token,
    };
  }
  const response = await fetch(
    `/api/v1/cart${variantID ? `/lines/${encodeURIComponent(variantID)}` : ""}`,
    {
      credentials: "include",
      cache: "no-store",
      method: variantID ? "POST" : "GET",
      headers,
      ...(variantID ? { body: JSON.stringify({ quantity }) } : {}),
    },
  );
  if (!response.ok) throw new CartError(response.status);
  const cart: unknown = await response.json();
  if (
    typeof cart !== "object" ||
    cart === null ||
    !("lines" in cart) ||
    !Array.isArray(cart.lines) ||
    cart.lines.length > 50 ||
    !cart.lines.every(validLine) ||
    !("has_unavailable" in cart) ||
    typeof cart.has_unavailable !== "boolean" ||
    !("subtotal_toman" in cart) ||
    typeof cart.subtotal_toman !== "number" ||
    !validPrice(cart.subtotal_toman)
  )
    throw new Error("Invalid cart response");
  return cart as Cart;
}
const money = new Intl.NumberFormat("fa-IR");
function failureMessage(error: unknown) {
  return error instanceof CartError && error.status === 401
    ? "برای استفاده از سبد، وارد حساب شو."
    : error instanceof CartError && error.status === 409
      ? "این انتخاب دیگر قابل افزودن نیست یا سبد به سقف تعداد رسیده است. سبد را تازه کن."
      : "دریافت یا ذخیرهٔ سبد انجام نشد. دوباره تلاش کن.";
}

export function AddToCart({ product }: { product: PublicProduct }) {
  const [ready, setReady] = useState(false);
  useEffect(() => setReady(true), []);
  const [selected, setSelected] = useState("");
  const [quantity, setQuantity] = useState(1);
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const [login, setLogin] = useState(false);
  const lock = useRef(false);
  async function add(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    if (lock.current || !selected) return;
    lock.current = true;
    setBusy(true);
    setMessage("");
    setLogin(false);
    try {
      await requestCart(selected, quantity);
      setMessage("این انتخاب با تعداد مشخص‌شده در سبد ذخیره شد.");
    } catch (error) {
      setMessage(failureMessage(error));
      setLogin(error instanceof CartError && error.status === 401);
    } finally {
      lock.current = false;
      setBusy(false);
    }
  }
  return (
    <form onSubmit={add} className="live-cart-form">
      <fieldset disabled={busy || !ready}>
        <legend>انتخاب برای سبد</legend>
        <label>
          رنگ و اندازه
          <select
            aria-label="رنگ و اندازه"
            required
            value={selected}
            onChange={(event) => setSelected(event.target.value)}
          >
            <option value="" disabled>
              یک ترکیب انتخاب کن
            </option>
            {product.variants.map((v) => (
              <option key={v.id} value={v.id}>
                {v.color ?? "بدون رنگ"} · {v.size ?? "بدون اندازه"} —{" "}
                {money.format(v.price.sale_toman ?? v.price.regular_toman)}{" "}
                تومان
              </option>
            ))}
          </select>
        </label>
        <label>
          تعداد در سبد
          <input
            type="number"
            min="1"
            max="9"
            step="1"
            required
            value={quantity}
            onChange={(event) => setQuantity(Number(event.target.value))}
          />
        </label>
        <button className="primary-link" type="submit">
          {busy ? "در حال ذخیره…" : "ذخیره در سبد"}
        </button>
      </fieldset>
      {message && <p role="status">{message}</p>}
      {login && <a href="/login/">ورود به حساب</a>}
      <a className="text-link" href="/cart/">
        دیدن سبد ←
      </a>
      <p>ذخیره دوبارهٔ همین ترکیب، تعداد آن را جایگزین می‌کند.</p>
    </form>
  );
}

export default function CustomerCart() {
  const [cart, setCart] = useState<Cart | null>(null);
  const [error, setError] = useState("");
  const [login, setLogin] = useState(false);
  const [busy, setBusy] = useState(false);
  const lock = useRef(false);
  async function load() {
    if (lock.current) return;
    lock.current = true;
    setBusy(true);
    setError("");
    try {
      setCart(await requestCart());
      setLogin(false);
    } catch (error) {
      setCart(null);
      setError(failureMessage(error));
      setLogin(error instanceof CartError && error.status === 401);
    } finally {
      lock.current = false;
      setBusy(false);
    }
  }
  useEffect(() => {
    void load();
  }, []);
  async function update(id: string, quantity: number) {
    if (lock.current) return;
    lock.current = true;
    setBusy(true);
    setError("");
    try {
      setCart(await requestCart(id, quantity));
    } catch (error) {
      setCart(null);
      setError(failureMessage(error));
      setLogin(error instanceof CartError && error.status === 401);
    } finally {
      lock.current = false;
      setBusy(false);
    }
  }
  return (
    <section className="live-cart">
      <p>
        قیمت‌ها با هر بار دریافت یا تغییر سبد از کاتالوگ محاسبه می‌شوند. سبد،
        رزرو موجودی یا سفارش نیست.
      </p>
      {error && <p role="alert">{error}</p>}
      {login ? (
        <a className="primary-link" href="/login/">
          ورود به حساب
        </a>
      ) : (
        <button
          disabled={busy}
          className="preview-secondary"
          onClick={() => void load()}
        >
          تازه‌کردن سبد
        </button>
      )}
      {busy && <p role="status">در حال دریافت سبد…</p>}
      {cart && (
        <>
          {cart.lines.length === 0 ? (
            <p>سبدت هنوز خالی است.</p>
          ) : (
            <ul>
              {cart.lines.map((line) => (
                <li key={line.variant_id}>
                  <h2>
                    <a
                      href={`/products/${encodeURIComponent(line.product_slug)}/`}
                    >
                      {line.name}
                    </a>
                  </h2>
                  <p>
                    {line.color ?? "بدون رنگ"} · {line.size ?? "بدون اندازه"}
                  </p>
                  {line.available ? (
                    <p>{money.format(line.line_total_toman ?? 0)} تومان</p>
                  ) : (
                    <p>این انتخاب دیگر در دسترس نیست؛ از سبد حذفش کن.</p>
                  )}
                  <label>
                    تعداد {line.name}
                    <select
                      value={line.quantity}
                      disabled={busy || !line.available}
                      onChange={(event) =>
                        void update(line.variant_id, Number(event.target.value))
                      }
                    >
                      {Array.from({ length: 9 }, (_, index) => (
                        <option key={index + 1} value={index + 1}>
                          {index + 1}
                        </option>
                      ))}
                    </select>
                  </label>
                  <button
                    className="preview-secondary"
                    disabled={busy}
                    onClick={() => void update(line.variant_id, 0)}
                  >
                    حذف {line.name}
                  </button>
                </li>
              ))}
            </ul>
          )}
          <p>جمع کالاهای در دسترس: {money.format(cart.subtotal_toman)} تومان</p>
          {cart.has_unavailable && (
            <p role="status">کالاهای ناموجود در جمع حساب نشده‌اند.</p>
          )}
          <p className="purchase-notice">
            ارسال، ثبت سفارش و پرداخت هنوز فعال نیست.
          </p>
        </>
      )}
      <a className="text-link" href="/collections/">
        ادامهٔ انتخاب محصولات ←
      </a>
    </section>
  );
}
