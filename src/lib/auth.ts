import { normalizeDigits } from "./preview-checkout";

export type PhoneNumber = string & { readonly __brand: "PhoneNumber" };
export type OtpCode = string & { readonly __brand: "OtpCode" };

export type AuthSession = {
  readonly user: {
    readonly id: string;
    readonly phone: string;
    readonly role: "customer" | "admin";
  };
  readonly expiresAt: number;
};

export type AuthCredentials =
  | { readonly kind: "customer"; readonly phone: PhoneNumber }
  | {
      readonly kind: "admin";
      readonly phone: PhoneNumber;
      readonly password: string;
    };

export type OtpChallenge = {
  readonly id: string;
  readonly expiresAt: number;
  readonly resendAt: number;
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isPhoneNumber(value: string): value is PhoneNumber {
  return /^\+989\d{9}$/.test(value);
}

export function normalizeAuthPhone(value: string): PhoneNumber | null {
  let phone = normalizeDigits(value).replace(/[\s()-]/g, "");
  if (phone.startsWith("0098")) phone = `+98${phone.slice(4)}`;
  else if (phone.startsWith("09")) phone = `+98${phone.slice(1)}`;
  else if (/^9\d{9}$/.test(phone)) phone = `+98${phone}`;
  return isPhoneNumber(phone) ? phone : null;
}

function isOtpCode(value: string): value is OtpCode {
  return /^\d{6}$/.test(value);
}

export function normalizeOtpCode(value: string): OtpCode | null {
  const code = normalizeDigits(value).trim();
  return isOtpCode(code) ? code : null;
}

export function secondsUntil(deadline: number, now: number): number {
  return Math.max(0, Math.ceil((deadline - now) / 1000));
}

export function parseAuthSession(value: unknown): AuthSession {
  if (
    !isRecord(value) ||
    !isRecord(value.user) ||
    typeof value.user.id !== "string" ||
    !value.user.id ||
    typeof value.user.phone !== "string" ||
    !/^\+989\d{9}$/.test(value.user.phone) ||
    (value.user.role !== "customer" && value.user.role !== "admin") ||
    typeof value.expires_at !== "string"
  ) {
    throw new Error("Invalid authentication response");
  }
  const expiresAt = Date.parse(value.expires_at);
  if (!Number.isFinite(expiresAt)) {
    throw new Error("Invalid authentication response");
  }
  return {
    user: {
      id: value.user.id,
      phone: value.user.phone,
      role: value.user.role,
    },
    expiresAt,
  };
}

function parseChallenge(value: unknown): OtpChallenge {
  if (
    !isRecord(value) ||
    typeof value.challenge_id !== "string" ||
    !value.challenge_id ||
    typeof value.expires_in !== "number" ||
    !Number.isSafeInteger(value.expires_in) ||
    value.expires_in <= 0 ||
    typeof value.resend_after !== "number" ||
    !Number.isSafeInteger(value.resend_after) ||
    value.resend_after < 0
  ) {
    throw new Error("Invalid authentication response");
  }
  const now = Date.now();
  return {
    id: value.challenge_id,
    expiresAt: now + value.expires_in * 1000,
    resendAt: now + value.resend_after * 1000,
  };
}

export class AuthRequestError extends Error {
  readonly code: string;
  readonly retryAfter: number;

  constructor(code: string, retryAfter = 0) {
    super(code);
    this.name = "AuthRequestError";
    this.code = code;
    this.retryAfter = retryAfter;
  }
}

export function authErrorMessage(error: unknown): string {
  if (error instanceof AuthRequestError) {
    switch (error.code) {
      case "invalid_otp":
        return "کد درست نیست یا اعتبار آن تمام شده است. دوباره تلاش کن یا کد تازه بگیر.";
      case "invalid_credentials":
        return "شمارهٔ همراه یا رمز عبور درست نیست.";
      case "rate_limited":
        return "تعداد تلاش‌ها زیاد شده است. کمی صبر کن و دوباره تلاش کن.";
      case "sms_unavailable":
        return "ارسال پیامک فعلاً ممکن نیست. کمی بعد دوباره تلاش کن.";
      case "authentication_required":
        return "زمان ورودت تمام شده است. دوباره وارد شو.";
      case "csrf_invalid":
      case "origin_not_allowed":
        return "ارتباط امن برقرار نشد. صفحه را تازه کن و دوباره تلاش کن.";
      case "invalid_request":
        return "اطلاعات واردشده را بررسی کن و دوباره تلاش کن.";
    }
  }
  return "ارتباط با فروشگاه برقرار نشد. دوباره تلاش کن.";
}

async function responseError(response: Response): Promise<AuthRequestError> {
  let code = "unavailable";
  const value: unknown = await response.json().catch(() => null);
  if (
    isRecord(value) &&
    isRecord(value.error) &&
    typeof value.error.code === "string"
  ) {
    code = value.error.code;
  }
  const retryHeader = response.headers.get("Retry-After") ?? "";
  const retryAfter = /^\d+$/.test(retryHeader) ? Number(retryHeader) : 0;
  return new AuthRequestError(
    code,
    Number.isSafeInteger(retryAfter) ? retryAfter : 0,
  );
}

type AuthFetch = (input: string, init?: RequestInit) => Promise<Response>;

export function createAuthClient(fetcher: AuthFetch = fetch) {
  let csrfToken: string | null = null;

  async function csrf(): Promise<string> {
    if (csrfToken) return csrfToken;
    const response = await fetcher("/api/v1/auth/csrf", {
      credentials: "include",
      cache: "no-store",
    });
    if (!response.ok) throw await responseError(response);
    const value: unknown = await response.json();
    if (
      !isRecord(value) ||
      typeof value.csrf_token !== "string" ||
      !value.csrf_token
    ) {
      throw new Error("Invalid authentication response");
    }
    csrfToken = value.csrf_token;
    return csrfToken;
  }

  async function post(path: string, body: object, retryCsrf = true) {
    const token = await csrf();
    const response = await fetcher(`/api/v1/auth/${path}`, {
      method: "POST",
      credentials: "include",
      cache: "no-store",
      headers: { "Content-Type": "application/json", "X-CSRF-Token": token },
      body: JSON.stringify(body),
    });
    if (response.ok) return response;
    const error = await responseError(response);
    if (retryCsrf && error.code === "csrf_invalid") {
      csrfToken = null;
      return post(path, body, false);
    }
    throw error;
  }

  return {
    async session(signal?: AbortSignal): Promise<AuthSession | null> {
      const response = await fetcher("/api/v1/auth/session", {
        credentials: "include",
        cache: "no-store",
        ...(signal ? { signal } : {}),
      });
      if (!response.ok) {
        const error = await responseError(response);
        if (
          response.status === 401 &&
          error.code === "authentication_required"
        ) {
          return null;
        }
        throw error;
      }
      const value: unknown = await response.json();
      return parseAuthSession(value);
    },
    async requestCode(credentials: AuthCredentials): Promise<OtpChallenge> {
      const response = await post(
        credentials.kind === "admin" ? "admin/login" : "customer/otp/request",
        credentials.kind === "admin"
          ? { phone: credentials.phone, password: credentials.password }
          : { phone: credentials.phone },
      );
      const value: unknown = await response.json();
      return parseChallenge(value);
    },
    async verifyCode({
      mode,
      challenge,
      code,
    }: {
      readonly mode: "customer" | "admin";
      readonly challenge: OtpChallenge;
      readonly code: OtpCode;
    }): Promise<AuthSession> {
      const response = await post(`${mode}/otp/verify`, {
        challenge_id: challenge.id,
        code,
      });
      const value: unknown = await response.json();
      return parseAuthSession(value);
    },
    async logout(): Promise<void> {
      await post("logout", {});
      csrfToken = null;
    },
  };
}

export type AuthClient = ReturnType<typeof createAuthClient>;
