import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { formatPrice } from "@/data/catalog";
import {
  cartTotals,
  loadCart,
  maxCartQuantity,
  resolveCart,
  saveCart,
  type CartLine,
  type ResolvedCartLine,
} from "@/lib/preview-cart";
import {
  emptyContact,
  sampleContact,
  validateContact,
  type ContactDetails,
  type ContactErrors,
  type ContactField,
} from "@/lib/preview-checkout";

type Receipt = {
  readonly details: ContactDetails;
  readonly lines: readonly ResolvedCartLine[];
  readonly totals: ReturnType<typeof cartTotals>;
};

type CheckoutStep =
  | { readonly kind: "details"; readonly draft: ContactDetails }
  | { readonly kind: "review"; readonly details: ContactDetails }
  | { readonly kind: "complete"; readonly receipt: Receipt };

type CartState =
  | { readonly kind: "loading" }
  | {
      readonly kind: "loaded";
      readonly lines: readonly CartLine[];
      readonly notice: string;
    };

type CartControls =
  | { readonly kind: "readonly" }
  | {
      readonly kind: "editable";
      readonly onQuantity: (sku: string, quantity: number) => void;
      readonly onRemove: (sku: string) => void;
    };

function OrderSummary({
  lines,
  totals,
  controls,
}: {
  lines: readonly ResolvedCartLine[];
  totals: ReturnType<typeof cartTotals>;
  controls: CartControls;
}) {
  return (
    <aside className="checkout-summary" aria-labelledby="summary-heading">
      <div className="checkout-summary-heading">
        <h2 id="summary-heading">سبد تمرین</h2>
        <span>
          {formatPrice(lines.reduce((total, line) => total + line.quantity, 0))}{" "}
          کالا
        </span>
      </div>
      <ul className="checkout-items">
        {lines.map((line) => (
          <li key={line.sku} className="checkout-item">
            <img
              src={line.colorway.images[0].src}
              alt={line.colorway.images[0].alt}
              width="76"
              height="101"
              style={{ objectPosition: line.colorway.images[0].position }}
            />
            <div className="checkout-item-copy">
              <a href={`/products/${line.product.slug}/`}>
                {line.product.name}
              </a>
              <p>
                {line.colorway.label} · اندازهٔ <bdi>{line.variant.size}</bdi>
              </p>
              <p>{formatPrice(line.product.priceToman)} تومان / هر عدد</p>
              {controls.kind === "editable" ? (
                <div className="cart-line-controls">
                  <label htmlFor={`quantity-${line.sku}`}>تعداد</label>
                  <select
                    id={`quantity-${line.sku}`}
                    value={line.quantity}
                    aria-label={`تعداد ${line.product.name}، ${line.colorway.label}، اندازهٔ ${line.variant.size}`}
                    onChange={(event) =>
                      controls.onQuantity(
                        line.sku,
                        Number(event.currentTarget.value),
                      )
                    }
                  >
                    {Array.from(
                      { length: maxCartQuantity },
                      (_, index) => index + 1,
                    ).map((quantity) => (
                      <option key={quantity} value={quantity}>
                        {formatPrice(quantity)}
                      </option>
                    ))}
                  </select>
                  <button
                    type="button"
                    className="plain-button remove-line"
                    aria-label={`حذف ${line.product.name}، ${line.colorway.label}، اندازهٔ ${line.variant.size}`}
                    onClick={() => controls.onRemove(line.sku)}
                  >
                    حذف
                  </button>
                </div>
              ) : (
                <p>تعداد: {formatPrice(line.quantity)}</p>
              )}
              <strong className="cart-line-total">
                {formatPrice(line.product.priceToman * line.quantity)} تومان
              </strong>
            </div>
          </li>
        ))}
      </ul>
      <dl className="checkout-totals">
        <div>
          <dt>جمع کالاها</dt>
          <dd>{formatPrice(totals.subtotalToman)} تومان</dd>
        </div>
        <div>
          <dt>ارسال نمونه</dt>
          <dd>{formatPrice(totals.shippingToman)} تومان</dd>
        </div>
        <div className="checkout-grand-total">
          <dt>جمع نمایشی</dt>
          <dd>{formatPrice(totals.totalToman)} تومان</dd>
        </div>
      </dl>
      <p className="field-hint">
        هزینهٔ ارسال برای تمرین، ثابت و ۷۹٬۰۰۰ تومان است. هیچ مبلغی دریافت
        نمی‌شود.
      </p>
    </aside>
  );
}

function ContactForm({
  initialDetails,
  onReview,
}: {
  initialDetails: ContactDetails;
  onReview: (details: ContactDetails) => void;
}) {
  const [draft, setDraft] = useState(initialDetails);
  const [errors, setErrors] = useState<ContactErrors>({});
  const [notice, setNotice] = useState("");

  function update(field: ContactField, value: string) {
    setDraft((current) => ({ ...current, [field]: value }));
    setErrors((current) => {
      const next = { ...current };
      delete next[field];
      return next;
    });
    setNotice("");
  }

  return (
    <form
      className="preview-form contact-form"
      noValidate
      onSubmit={(event) => {
        event.preventDefault();
        const result = validateContact(draft);
        if (result.kind === "invalid") {
          setErrors(result.errors);
          const field = event.currentTarget.elements.namedItem(
            result.firstField,
          );
          if (field instanceof HTMLElement) field.focus();
          return;
        }
        onReview(result.details);
      }}
    >
      <div className="form-intro">
        <p>برای امتحان مسیر، از اطلاعات ساختگی استفاده کن.</p>
        <Button
          type="button"
          variant="outline"
          className="preview-secondary"
          onClick={() => {
            setDraft(sampleContact);
            setErrors({});
            setNotice("اطلاعات ساختگی وارد شد. حالا می‌توانی خلاصه را ببینی.");
          }}
        >
          پرکردن با اطلاعات نمونه
        </Button>
      </div>
      <div className="form-fields">
        <div className="form-field">
          <label htmlFor="recipient">نام و نام خانوادگی گیرنده</label>
          <input
            id="recipient"
            name="recipient"
            autoComplete="name"
            required
            maxLength={120}
            value={draft.recipient}
            onChange={(event) => update("recipient", event.currentTarget.value)}
            aria-invalid={Boolean(errors.recipient)}
            aria-describedby={errors.recipient ? "recipient-error" : undefined}
          />
          {errors.recipient && (
            <p className="field-error" id="recipient-error">
              {errors.recipient}
            </p>
          )}
        </div>
        <div className="form-field">
          <label htmlFor="phone">شمارهٔ همراه</label>
          <input
            id="phone"
            name="phone"
            type="tel"
            inputMode="tel"
            autoComplete="tel-national"
            required
            maxLength={20}
            dir="ltr"
            placeholder="۰۹۱۲۳۴۵۶۷۸۹"
            value={draft.phone}
            onChange={(event) => update("phone", event.currentTarget.value)}
            aria-invalid={Boolean(errors.phone)}
            aria-describedby={errors.phone ? "phone-error" : "phone-hint"}
          />
          {errors.phone ? (
            <p className="field-error" id="phone-error">
              {errors.phone}
            </p>
          ) : (
            <p className="field-hint" id="phone-hint">
              ۱۱ رقم با پیش‌شمارهٔ ۰۹؛ پیامکی ارسال نمی‌شود.
            </p>
          )}
        </div>
        <div className="form-field">
          <label htmlFor="city">شهر</label>
          <input
            id="city"
            name="city"
            autoComplete="address-level2"
            required
            maxLength={100}
            value={draft.city}
            onChange={(event) => update("city", event.currentTarget.value)}
            aria-invalid={Boolean(errors.city)}
            aria-describedby={errors.city ? "city-error" : undefined}
          />
          {errors.city && (
            <p className="field-error" id="city-error">
              {errors.city}
            </p>
          )}
        </div>
        <div className="form-field">
          <label htmlFor="postalCode">کد پستی</label>
          <input
            id="postalCode"
            name="postalCode"
            inputMode="numeric"
            autoComplete="postal-code"
            required
            maxLength={16}
            dir="ltr"
            placeholder="۱۲۳۴۵۶۷۸۹۰"
            value={draft.postalCode}
            onChange={(event) =>
              update("postalCode", event.currentTarget.value)
            }
            aria-invalid={Boolean(errors.postalCode)}
            aria-describedby={
              errors.postalCode ? "postalCode-error" : undefined
            }
          />
          {errors.postalCode && (
            <p className="field-error" id="postalCode-error">
              {errors.postalCode}
            </p>
          )}
        </div>
        <div className="form-field form-field-wide">
          <label htmlFor="address">نشانی کامل، پلاک و واحد</label>
          <textarea
            id="address"
            name="address"
            autoComplete="street-address"
            required
            rows={3}
            maxLength={600}
            value={draft.address}
            onChange={(event) => update("address", event.currentTarget.value)}
            aria-invalid={Boolean(errors.address)}
            aria-describedby={errors.address ? "address-error" : undefined}
          />
          {errors.address && (
            <p className="field-error" id="address-error">
              {errors.address}
            </p>
          )}
        </div>
      </div>
      <p className="preview-feedback" role="status" aria-live="polite">
        {notice}
      </p>
      <div className="form-actions">
        <Button type="submit" size="lg" className="preview-primary">
          ادامه به مرور نهایی ←
        </Button>
        <a className="text-link" href="/collections/">
          ادامهٔ دیدن لباس‌ها
        </a>
      </div>
    </form>
  );
}

function ShippingDetails({ details }: { details: ContactDetails }) {
  return (
    <dl className="shipping-details">
      <div>
        <dt>گیرنده</dt>
        <dd>{details.recipient}</dd>
      </div>
      <div>
        <dt>همراه</dt>
        <dd>
          <bdi>{details.phone}</bdi>
        </dd>
      </div>
      <div>
        <dt>نشانی</dt>
        <dd>
          {details.city}، {details.address}
        </dd>
      </div>
      <div>
        <dt>کد پستی</dt>
        <dd>
          <bdi>{details.postalCode}</bdi>
        </dd>
      </div>
    </dl>
  );
}

export default function CheckoutPreview() {
  const [cart, setCart] = useState<CartState>({ kind: "loading" });
  const [step, setStep] = useState<CheckoutStep>({
    kind: "details",
    draft: emptyContact,
  });
  const stepHeading = useRef<HTMLHeadingElement>(null);

  useEffect(() => {
    const saved = loadCart();
    setCart({
      kind: "loaded",
      lines: saved.lines,
      notice:
        saved.kind === "unavailable"
          ? "ذخیرهٔ محلی در دسترس نیست. با سبد نمونه می‌توانی همین‌جا تمرین کنی؛ سبد با خروج از این صفحه نگه داشته نمی‌شود."
          : saved.kind === "repaired"
            ? "سبد قبلی بخش نامعتبر یا ناموجود داشت. فقط انتخاب‌های معتبر نگه داشته شدند."
            : "",
    });
  }, []);

  useEffect(() => {
    stepHeading.current?.focus();
  }, [step.kind]);

  function updateCart(lines: readonly CartLine[]) {
    const saved = saveCart(lines);
    setCart({
      kind: "loaded",
      lines,
      notice: saved
        ? ""
        : lines.length === 0
          ? "سبد این صفحه خالی شد؛ پاک‌کردن سبد ذخیره‌شده ممکن نشد. سبد قبلی ممکن است در مراجعهٔ بعدی دیده شود."
          : "سبد فقط در همین صفحه نگه داشته می‌شود؛ مرورگر اجازهٔ ذخیرهٔ محلی را نداد.",
    });
  }

  if (cart.kind === "loading") {
    return (
      <p className="checkout-loading" role="status">
        در حال آماده‌کردن سبد نمایشی…
      </p>
    );
  }

  const lines = resolveCart(cart.lines);
  const totals = cartTotals(lines);

  if (step.kind !== "complete" && lines.length === 0) {
    return (
      <div className="checkout-empty">
        <span className="eyebrow">یک انتخاب برای شروع</span>
        <h2>سبد نمایشی‌ات هنوز خالی است.</h2>
        <p>
          یک رنگ و اندازه از مجموعه انتخاب کن، یا با یک تاپ نمونه مسیر تسویه را
          امتحان کن.
        </p>
        {cart.notice && (
          <p className="preview-notice" role="status">
            {cart.notice}
          </p>
        )}
        <div className="form-actions">
          <a className="primary-link" href="/collections/">
            دیدن همهٔ لباس‌ها ←
          </a>
          <Button
            type="button"
            variant="outline"
            className="preview-secondary"
            onClick={() => updateCart([{ sku: "SH101-WH-M", quantity: 1 }])}
          >
            امتحان با سبد نمونه
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="checkout-preview">
      <ol className="checkout-steps" aria-label="مراحل تسویهٔ نمایشی">
        <li aria-current={step.kind === "details" ? "step" : undefined}>
          <span>۱</span> اطلاعات گیرنده
        </li>
        <li aria-current={step.kind === "review" ? "step" : undefined}>
          <span>۲</span> مرور نهایی
        </li>
        <li aria-current={step.kind === "complete" ? "step" : undefined}>
          <span>۳</span> پایان تمرین
        </li>
      </ol>
      {cart.notice && (
        <p className="preview-notice" role="status">
          {cart.notice}
        </p>
      )}
      {step.kind === "complete" ? (
        <div className="checkout-grid">
          <section className="checkout-main checkout-complete">
            <span className="completion-mark" aria-hidden="true">
              ✓
            </span>
            <p className="eyebrow">رسید تمرینی / فقط در همین صفحه</p>
            <h2 ref={stepHeading} tabIndex={-1}>
              تمرین تسویه کامل شد.
            </h2>
            <p className="completion-copy">
              هیچ سفارش واقعی ثبت نشده و هیچ پرداختی انجام نشده است. این خلاصه
              فقط برای بررسی تجربهٔ خرید نمایش داده می‌شود.
            </p>
            <ShippingDetails details={step.receipt.details} />
            <p className="field-hint">
              سبد نمایشی خالی شد. اطلاعات گیرنده و این رسید ذخیره نمی‌شوند و با
              تازه‌کردن صفحه از بین می‌روند.
            </p>
            <a className="primary-link" href="/collections/">
              بازگشت به مجموعه ←
            </a>
          </section>
          <OrderSummary
            lines={step.receipt.lines}
            totals={step.receipt.totals}
            controls={{ kind: "readonly" }}
          />
        </div>
      ) : (
        <div className="checkout-grid">
          <section className="checkout-main">
            <p className="eyebrow">
              {step.kind === "details"
                ? "گام اول / مشخصات نمونه"
                : "گام دوم / یک نگاه آخر"}
            </p>
            <h2 ref={stepHeading} tabIndex={-1}>
              {step.kind === "details"
                ? "بسته به دست چه کسی برسد؟"
                : "همه‌چیز آمادهٔ یک تمرین است."}
            </h2>
            {step.kind === "details" ? (
              <ContactForm
                initialDetails={step.draft}
                onReview={(details) => setStep({ kind: "review", details })}
              />
            ) : (
              <div className="checkout-review">
                <ShippingDetails details={step.details} />
                <Button
                  type="button"
                  variant="outline"
                  className="preview-secondary"
                  onClick={() =>
                    setStep({ kind: "details", draft: step.details })
                  }
                >
                  ویرایش اطلاعات گیرنده
                </Button>
                <div className="simulation-note">
                  <h3>پرداخت و سفارش، فقط شبیه‌سازی می‌شوند.</h3>
                  <p>
                    با دکمهٔ زیر، رسید تمرینی را می‌بینی. درگاه بانکی، ارسال
                    بسته و پیامک در این پیش‌نمایش وجود ندارد.
                  </p>
                </div>
                <Button
                  type="button"
                  size="lg"
                  className="preview-primary"
                  onClick={() => {
                    const receipt: Receipt = {
                      details: step.details,
                      lines,
                      totals,
                    };
                    setStep({ kind: "complete", receipt });
                    updateCart([]);
                  }}
                >
                  شبیه‌سازی پرداخت و پایان تمرین
                </Button>
              </div>
            )}
          </section>
          <OrderSummary
            lines={lines}
            totals={totals}
            controls={
              step.kind === "details"
                ? {
                    kind: "editable",
                    onQuantity: (sku, quantity) => {
                      if (
                        !Number.isSafeInteger(quantity) ||
                        quantity < 1 ||
                        quantity > maxCartQuantity
                      )
                        return;
                      updateCart(
                        cart.lines.map((line) =>
                          line.sku === sku ? { sku, quantity } : line,
                        ),
                      );
                    },
                    onRemove: (sku) =>
                      updateCart(cart.lines.filter((line) => line.sku !== sku)),
                  }
                : { kind: "readonly" }
            }
          />
        </div>
      )}
    </div>
  );
}
