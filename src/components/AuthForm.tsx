import { useEffect, useRef, useState, type SubmitEvent } from "react";
import { Button } from "./ui/button";
import {
  AuthRequestError,
  authErrorMessage,
  normalizeAuthPhone,
  normalizeOtpCode,
  secondsUntil,
  type AuthClient,
  type AuthCredentials,
  type AuthSession,
  type OtpChallenge,
} from "../lib/auth";

type FormStep =
  | { readonly kind: "credentials" }
  | {
      readonly kind: "code";
      readonly challenge: OtpChallenge;
      readonly credentials: AuthCredentials;
    };

type FieldErrors = Partial<Record<"phone" | "password" | "code", string>>;
const digits = new Intl.NumberFormat("fa-IR");

export default function AuthForm({
  mode,
  client,
  onSignedIn,
}: {
  readonly mode: "customer" | "admin";
  readonly client: AuthClient;
  readonly onSignedIn: (session: AuthSession) => void;
}) {
  const [step, setStep] = useState<FormStep>({ kind: "credentials" });
  const [phone, setPhone] = useState("");
  const [password, setPassword] = useState("");
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [fields, setFields] = useState<FieldErrors>({});
  const [retryAt, setRetryAt] = useState(0);
  const [now, setNow] = useState(Date.now);
  const phoneInput = useRef<HTMLInputElement>(null);
  const passwordInput = useRef<HTMLInputElement>(null);
  const codeInput = useRef<HTMLInputElement>(null);
  const retryRemaining = secondsUntil(retryAt, now);
  const resendRemaining =
    step.kind === "code" ? secondsUntil(step.challenge.resendAt, now) : 0;
  const expired = step.kind === "code" && step.challenge.expiresAt <= now;

  useEffect(() => {
    if (busy) return;
    if (step.kind === "code") {
      codeInput.current?.focus();
      if (fields.code) codeInput.current?.select();
    } else if (fields.password) passwordInput.current?.focus();
    else phoneInput.current?.focus();
  }, [busy, fields, step.kind]);

  useEffect(() => {
    if (step.kind !== "code" && retryAt <= Date.now()) return;
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [step.kind, retryAt]);

  function showRequestError(failure: unknown) {
    setError(authErrorMessage(failure));
    if (failure instanceof AuthRequestError && failure.retryAfter > 0) {
      const currentTime = Date.now();
      setNow(currentTime);
      setRetryAt(currentTime + failure.retryAfter * 1000);
    }
    if (failure instanceof AuthRequestError && failure.code === "invalid_otp") {
      setFields({ code: authErrorMessage(failure) });
      setError("");
    }
  }

  async function sendCode(credentials: AuthCredentials) {
    const challenge = await client.requestCode(credentials);
    setStep({ kind: "code", challenge, credentials });
    setPassword("");
    setCode("");
    setNow(Date.now());
    setRetryAt(0);
  }

  async function submit(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy || retryRemaining > 0) return;
    setError("");
    setFields({});
    if (step.kind === "credentials") {
      const normalizedPhone = normalizeAuthPhone(phone);
      if (!normalizedPhone) {
        setFields({ phone: "شمارهٔ همراه معتبر وارد کن؛ مثلاً ۰۹۱۲۳۴۵۶۷۸۹." });
        return;
      }
      if (mode === "admin" && !password) {
        setFields({ password: "رمز عبور مدیر را وارد کن." });
        return;
      }
      const credentials: AuthCredentials =
        mode === "admin"
          ? { kind: "admin", phone: normalizedPhone, password }
          : { kind: "customer", phone: normalizedPhone };
      setBusy(true);
      try {
        await sendCode(credentials);
      } catch (failure) {
        showRequestError(failure);
      } finally {
        setBusy(false);
      }
      return;
    }
    const normalizedCode = normalizeOtpCode(code);
    if (!normalizedCode) {
      setFields({ code: "کد ۶ رقمی پیامک‌شده را وارد کن." });
      return;
    }
    if (step.challenge.expiresAt <= Date.now()) {
      setNow(Date.now());
      return;
    }
    setBusy(true);
    try {
      onSignedIn(
        await client.verifyCode({
          mode,
          challenge: step.challenge,
          code: normalizedCode,
        }),
      );
    } catch (failure) {
      showRequestError(failure);
    } finally {
      setBusy(false);
    }
  }

  async function resend() {
    if (step.kind !== "code" || busy || resendRemaining || retryRemaining)
      return;
    setError("");
    setFields({});
    setBusy(true);
    try {
      await sendCode(step.credentials);
    } catch (failure) {
      showRequestError(failure);
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="auth-card" aria-labelledby="auth-heading">
      <p className="eyebrow">
        {mode === "admin" ? "ورود مدیر / دو مرحله" : "حساب شواندی"}
      </p>
      <h1 id="auth-heading">
        {step.kind === "code"
          ? "کد ورود را وارد کن"
          : mode === "admin"
            ? "ورود به مدیریت"
            : "خوش آمدی"}
      </h1>
      {step.kind === "credentials" ? (
        <p className="auth-intro">
          {mode === "admin"
            ? "با شمارهٔ همراه و رمز عبور مدیر شروع کن. سپس کد پیامک‌شده را تأیید کن."
            : "شمارهٔ همراهت را وارد کن تا کد ورود برایت پیامک شود."}
        </p>
      ) : (
        <p className="auth-intro" role="status">
          کد ورود به <bdi>{step.credentials.phone}</bdi> پیامک شد.
        </p>
      )}
      <form className="auth-form" onSubmit={submit} noValidate aria-busy={busy}>
        {step.kind === "credentials" ? (
          <>
            <div className="form-field">
              <label htmlFor="auth-phone">شمارهٔ همراه</label>
              <input
                ref={phoneInput}
                id="auth-phone"
                name="phone"
                type="tel"
                inputMode="tel"
                autoComplete="tel"
                dir="ltr"
                maxLength={32}
                placeholder="09123456789"
                value={phone}
                disabled={busy}
                aria-invalid={Boolean(fields.phone)}
                aria-describedby={fields.phone ? "auth-phone-error" : undefined}
                onChange={(event) => setPhone(event.currentTarget.value)}
              />
              {fields.phone ? (
                <p id="auth-phone-error" className="field-error" role="alert">
                  {fields.phone}
                </p>
              ) : null}
            </div>
            {mode === "admin" ? (
              <div className="form-field">
                <label htmlFor="auth-password">رمز عبور مدیر</label>
                <input
                  ref={passwordInput}
                  id="auth-password"
                  name="password"
                  type="password"
                  autoComplete="current-password"
                  dir="ltr"
                  maxLength={1024}
                  value={password}
                  disabled={busy}
                  aria-invalid={Boolean(fields.password)}
                  aria-describedby={
                    fields.password ? "auth-password-error" : undefined
                  }
                  onChange={(event) => setPassword(event.currentTarget.value)}
                />
                {fields.password ? (
                  <p
                    id="auth-password-error"
                    className="field-error"
                    role="alert"
                  >
                    {fields.password}
                  </p>
                ) : null}
              </div>
            ) : null}
          </>
        ) : (
          <div className="form-field">
            <label htmlFor="auth-code">کد ورود ۶ رقمی</label>
            <input
              ref={codeInput}
              id="auth-code"
              name="code"
              type="text"
              inputMode="numeric"
              autoComplete="one-time-code"
              dir="ltr"
              maxLength={6}
              className="auth-code-input"
              value={code}
              disabled={busy || expired}
              aria-invalid={Boolean(fields.code)}
              aria-describedby={
                fields.code ? "auth-code-error" : "auth-code-hint"
              }
              onChange={(event) => setCode(event.currentTarget.value)}
            />
            {fields.code ? (
              <p id="auth-code-error" className="field-error" role="alert">
                {fields.code}
              </p>
            ) : null}
            <p id="auth-code-hint" className="field-hint">
              {expired
                ? "اعتبار این کد تمام شد. یک کد تازه بگیر."
                : "فقط آخرین کد پیامک‌شده معتبر است."}
            </p>
          </div>
        )}
        {error ? (
          <p className="auth-error" role="alert">
            {error}
          </p>
        ) : null}
        {retryRemaining > 0 ? (
          <p className="field-hint" aria-live="off">
            تلاش دوباره تا {digits.format(retryRemaining)} ثانیه
          </p>
        ) : null}
        <Button
          type="submit"
          className="preview-primary auth-submit"
          disabled={busy || retryRemaining > 0 || expired}
        >
          {busy
            ? "در حال بررسی…"
            : step.kind === "credentials"
              ? "دریافت کد ورود"
              : "تأیید و ورود"}
        </Button>
        {step.kind === "code" ? (
          <div className="auth-code-actions">
            <button
              type="button"
              className="auth-text-button"
              disabled={busy || resendRemaining > 0 || retryRemaining > 0}
              onClick={resend}
            >
              {resendRemaining > 0
                ? `ارسال دوباره تا ${digits.format(resendRemaining)} ثانیه`
                : "ارسال دوبارهٔ کد"}
            </button>
            <button
              type="button"
              className="auth-text-button"
              disabled={busy}
              onClick={() => {
                setStep({ kind: "credentials" });
                setPassword("");
                setCode("");
                setFields({});
                setError("");
              }}
            >
              {mode === "admin" ? "ویرایش اطلاعات ورود" : "ویرایش شمارهٔ همراه"}
            </button>
          </div>
        ) : null}
      </form>
      <a className="auth-back-link" href="/">
        بازگشت به فروشگاه ↗
      </a>
    </section>
  );
}
