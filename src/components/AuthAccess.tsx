import {
  lazy,
  Suspense,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";
import AuthForm from "./AuthForm";
import { Button } from "./ui/button";
import {
  authErrorMessage,
  createAuthClient,
  type AuthSession,
} from "../lib/auth";
const AdminCatalogRead = lazy(() => import("./AdminCatalogRead"));

type Props = { readonly mode: "customer" } | { readonly mode: "admin" };

type SessionState =
  | { readonly kind: "checking" }
  | { readonly kind: "guest"; readonly notice: string }
  | { readonly kind: "authenticated"; readonly session: AuthSession }
  | { readonly kind: "unavailable"; readonly message: string };

export default function AuthAccess(props: Props) {
  const [client] = useState(createAuthClient);
  const [state, setState] = useState<SessionState>({ kind: "checking" });
  const [loggingOut, setLoggingOut] = useState(false);
  const [logoutError, setLogoutError] = useState("");
  const request = useRef<AbortController | null>(null);

  const refreshSession = useCallback(async () => {
    request.current?.abort();
    const controller = new AbortController();
    request.current = controller;
    try {
      const session = await client.session(controller.signal);
      if (controller.signal.aborted) return;
      setState(
        session && session.expiresAt > Date.now()
          ? { kind: "authenticated", session }
          : { kind: "guest", notice: "" },
      );
    } catch (failure) {
      if (!controller.signal.aborted) {
        setState({ kind: "unavailable", message: authErrorMessage(failure) });
      }
    }
  }, [client]);

  const requireLogin = useCallback(() => {
    request.current?.abort();
    setState({
      kind: "guest",
      notice: "نشست مدیریت پایان یافته است؛ دوباره وارد شو.",
    });
  }, []);

  useEffect(() => {
    void refreshSession();
    const recheck = () => {
      if (document.visibilityState === "visible") void refreshSession();
    };
    window.addEventListener("focus", recheck);
    document.addEventListener("visibilitychange", recheck);
    return () => {
      request.current?.abort();
      window.removeEventListener("focus", recheck);
      document.removeEventListener("visibilitychange", recheck);
    };
  }, [refreshSession]);

  useEffect(() => {
    if (state.kind !== "authenticated") return;
    const timer = window.setTimeout(
      () => {
        request.current?.abort();
        setState({
          kind: "guest",
          notice: "زمان ورودت تمام شده است. دوباره وارد شو.",
        });
      },
      Math.max(0, state.session.expiresAt - Date.now()),
    );
    return () => window.clearTimeout(timer);
  }, [state]);

  async function logout() {
    setLoggingOut(true);
    setLogoutError("");
    request.current?.abort();
    try {
      await client.logout();
      request.current?.abort();
      setState({ kind: "guest", notice: "از حساب خارج شدی." });
    } catch (failure) {
      setLogoutError(authErrorMessage(failure));
    } finally {
      setLoggingOut(false);
    }
  }

  if (state.kind === "checking") {
    return (
      <div className="auth-status" role="status">
        در حال بررسی ورود…
      </div>
    );
  }

  if (state.kind === "unavailable") {
    return (
      <section className="auth-card auth-unavailable">
        <p className="eyebrow">ارتباط با شواندی</p>
        <h1>بررسی ورود ممکن نشد</h1>
        <p className="auth-error" role="alert">
          {state.message}
        </p>
        <Button
          className="preview-primary"
          onClick={() => {
            setState({ kind: "checking" });
            void refreshSession();
          }}
        >
          تلاش دوباره
        </Button>
        <a className="auth-back-link" href="/">
          بازگشت به فروشگاه ↗
        </a>
      </section>
    );
  }

  if (state.kind === "guest") {
    return (
      <div className="auth-entry">
        {state.notice ? (
          <p className="auth-notice" role="status">
            {state.notice}
          </p>
        ) : null}
        <AuthForm
          mode={props.mode}
          client={client}
          onSignedIn={(session) => {
            request.current?.abort();
            setState({ kind: "authenticated", session });
          }}
        />
      </div>
    );
  }

  const { session } = state;
  const adminAllowed = props.mode === "admin" && session.user.role === "admin";
  return (
    <>
      <section
        className={adminAllowed ? "auth-admin-session" : "auth-card"}
        aria-labelledby="session-heading"
      >
        <div>
          <p className="eyebrow">
            {session.user.role === "admin" ? "حساب مدیر شواندی" : "حساب شواندی"}
          </p>
          <h1 id="session-heading">
            {props.mode === "admin" && !adminAllowed
              ? "ورود مدیر لازم است"
              : adminAllowed
                ? "خوش آمدی، مدیر شواندی"
                : "وارد حسابت شدی"}
          </h1>
          <p className="auth-intro">
            {props.mode === "admin" && !adminAllowed
              ? "این حساب دسترسی مدیریت ندارد. برای ادامه، خارج شو و با حساب مدیر وارد شو."
              : "ورودت با شمارهٔ همراه تأیید شده است."}
          </p>
          <p className="auth-session-phone" dir="ltr">
            {session.user.phone}
          </p>
        </div>
        <div className="auth-session-actions">
          <Button
            className="preview-secondary"
            onClick={logout}
            disabled={loggingOut}
          >
            {loggingOut ? "در حال خروج…" : "خروج از حساب"}
          </Button>
          {props.mode === "customer" ? (
            <a className="auth-back-link" href="/collections/">
              دیدن مجموعه‌ها ↗
            </a>
          ) : null}
          {props.mode === "customer" && session.user.role === "admin" ? (
            <a className="auth-back-link" href="/admin/">
              ورود به مدیریت ↗
            </a>
          ) : null}
        </div>
        {logoutError ? (
          <p className="auth-error" role="alert">
            {logoutError}
          </p>
        ) : null}
      </section>
      {adminAllowed && props.mode === "admin" ? (
        <>
          <Suspense fallback={<p role="status">در حال آماده‌سازی فهرست…</p>}>
            <AdminCatalogRead onUnauthorized={requireLogin} />
          </Suspense>
        </>
      ) : null}
    </>
  );
}
