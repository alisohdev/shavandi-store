import { describe, expect, test } from "bun:test";
import {
  AuthRequestError,
  authErrorMessage,
  createAuthClient,
  normalizeAuthPhone,
  normalizeOtpCode,
  parseAuthSession,
  secondsUntil,
  type AuthSession,
} from "./auth";

const sessionPayload = {
  user: { id: "user-1", phone: "+989123456789", role: "customer" },
  expires_at: "2030-01-01T00:00:00Z",
} satisfies { user: AuthSession["user"]; expires_at: string };

describe("authentication input boundary", () => {
  test("normalizes supported Iranian mobile formats and both digit alphabets", () => {
    for (const value of [
      "۰۹۱۲۳۴۵۶۷۸۹",
      "٠٩١٢٣٤٥٦٧٨٩",
      "+98 (912) 345-6789",
      "00989123456789",
      "9123456789",
    ]) {
      expect<string | null>(normalizeAuthPhone(value)).toBe("+989123456789");
    }
    for (const value of ["02112345678", "+19123456789", "0912x456789", ""]) {
      expect(normalizeAuthPhone(value)).toBeNull();
    }
    expect<string | null>(normalizeOtpCode(" ۱۲۳۴۵۶ ")).toBe("123456");
    expect<string | null>(normalizeOtpCode("١٢٣٤٥٦")).toBe("123456");
    for (const value of ["12345", "1234567", "12 456", "12345x"]) {
      expect(normalizeOtpCode(value)).toBeNull();
    }
  });

  test("rejects malformed sessions and unauthorized role values", () => {
    expect(parseAuthSession(sessionPayload)).toEqual({
      user: sessionPayload.user,
      expiresAt: Date.parse(sessionPayload.expires_at),
    });
    for (const value of [
      null,
      { ...sessionPayload, user: { ...sessionPayload.user, role: "owner" } },
      {
        ...sessionPayload,
        user: { ...sessionPayload.user, phone: "injected" },
      },
      { ...sessionPayload, expires_at: "unknown" },
    ]) {
      expect(() => parseAuthSession(value)).toThrow();
    }
    expect(secondsUntil(2001, 1000)).toBe(2);
    expect(secondsUntil(1000, 1000)).toBe(0);
    expect(secondsUntil(999, 1000)).toBe(0);
  });
});

describe("cookie authentication client", () => {
  test("uses included credentials and refreshes an invalid CSRF token once", async () => {
    const calls: { readonly path: string; readonly init?: RequestInit }[] = [];
    let csrfRequests = 0;
    let otpRequests = 0;
    const client = createAuthClient(async (path, init) => {
      calls.push({ path, ...(init ? { init } : {}) });
      if (path.endsWith("/csrf")) {
        csrfRequests++;
        return Response.json({ csrf_token: `csrf-${csrfRequests}` });
      }
      otpRequests++;
      if (otpRequests === 1) {
        return Response.json(
          { error: { code: "csrf_invalid" } },
          { status: 403 },
        );
      }
      return Response.json(
        { challenge_id: "challenge-1", expires_in: 300, resend_after: 60 },
        { status: 202 },
      );
    });
    const phone = normalizeAuthPhone("09123456789");
    if (!phone) throw new Error("Expected valid test phone");
    const challenge = await client.requestCode({ kind: "customer", phone });
    expect(challenge.id).toBe("challenge-1");
    expect(challenge.expiresAt - challenge.resendAt).toBe(240000);
    expect(csrfRequests).toBe(2);
    expect(otpRequests).toBe(2);
    expect(calls.every((call) => call.init?.credentials === "include")).toBe(
      true,
    );
    expect(calls.every((call) => call.init?.cache === "no-store")).toBe(true);
    const mutation = calls.at(-1);
    expect(mutation?.path).toBe("/api/v1/auth/customer/otp/request");
    expect(mutation?.init?.method).toBe("POST");
    expect(mutation?.init?.body).toBe('{"phone":"+989123456789"}');
    expect(new Headers(mutation?.init?.headers).get("X-CSRF-Token")).toBe(
      "csrf-2",
    );
  });

  test("does not keep retrying a rejected CSRF token", async () => {
    let mutations = 0;
    const client = createAuthClient(async (path) => {
      if (path.endsWith("/csrf")) return Response.json({ csrf_token: "token" });
      mutations++;
      return Response.json(
        { error: { code: "csrf_invalid" } },
        { status: 403 },
      );
    });
    const failure: unknown = await client
      .logout()
      .catch((error: unknown) => error);
    expect(failure).toMatchObject({ code: "csrf_invalid" });
    expect(mutations).toBe(2);
  });

  test("distinguishes a signed-out session from an unavailable backend", async () => {
    const guest = createAuthClient(async () =>
      Response.json(
        { error: { code: "authentication_required" } },
        { status: 401 },
      ),
    );
    expect(await guest.session()).toBeNull();
    const unavailable = createAuthClient(
      async () => new Response("", { status: 503 }),
    );
    const failure: unknown = await unavailable
      .session()
      .catch((error: unknown) => error);
    expect(failure).toBeInstanceOf(AuthRequestError);
  });

  test("uses the admin password only for the first step and verifies the challenge", async () => {
    const calls: { path: string; body: unknown }[] = [];
    const client = createAuthClient(async (path, init) => {
      if (path.endsWith("/csrf")) return Response.json({ csrf_token: "token" });
      const body: unknown =
        typeof init?.body === "string" ? JSON.parse(init.body) : null;
      calls.push({ path, body });
      if (path.endsWith("/login")) {
        return Response.json({
          challenge_id: "admin-1",
          expires_in: 300,
          resend_after: 60,
        });
      }
      if (path.endsWith("/logout")) return new Response(null, { status: 204 });
      return Response.json({
        ...sessionPayload,
        user: { ...sessionPayload.user, role: "admin" },
      });
    });
    const phone = normalizeAuthPhone("09123456789");
    const code = normalizeOtpCode("123456");
    if (!phone || !code) throw new Error("Expected valid test inputs");
    const challenge = await client.requestCode({
      kind: "admin",
      phone,
      password: "mock-password",
    });
    const session = await client.verifyCode({ mode: "admin", challenge, code });
    expect(session.user.role).toBe("admin");
    expect(calls).toEqual([
      {
        path: "/api/v1/auth/admin/login",
        body: { phone, password: "mock-password" },
      },
      {
        path: "/api/v1/auth/admin/otp/verify",
        body: { challenge_id: "admin-1", code },
      },
    ]);
    await client.logout();
    expect(calls.at(-1)).toEqual({ path: "/api/v1/auth/logout", body: {} });
  });

  test("preserves retry timing and shows fixed Persian errors without server details", async () => {
    const client = createAuthClient(async (path) => {
      if (path.endsWith("/csrf")) return Response.json({ csrf_token: "token" });
      return Response.json(
        { error: { code: "rate_limited", message: "private provider detail" } },
        { status: 429, headers: { "Retry-After": "60" } },
      );
    });
    const phone = normalizeAuthPhone("09123456789");
    if (!phone) throw new Error("Expected valid test phone");
    let failure: unknown;
    try {
      await client.requestCode({ kind: "customer", phone });
    } catch (error) {
      failure = error;
    }
    expect(failure).toMatchObject({ code: "rate_limited", retryAfter: 60 });
    expect(authErrorMessage(failure)).not.toContain("private provider detail");
    expect(authErrorMessage(new AuthRequestError("sms_unavailable"))).toContain(
      "ارسال پیامک فعلاً ممکن نیست",
    );
  });
});
