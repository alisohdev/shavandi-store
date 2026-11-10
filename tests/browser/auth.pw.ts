import { expect, test, type Page } from "@playwright/test";

type Role = "customer" | "admin";
type StubBackend = {
  role: Role | null;
  expiresIn: number;
  resendAfter: number;
  challenges: number;
  requestError: string;
  requests: { readonly path: string; readonly body: unknown }[];
};

// These browser tests exercise the UI against an explicit API stub. Backend tests
// cover real OTP delivery boundaries, cookies, authorization and persistence.
async function stubAuth(page: Page, initialRole: Role | null = null) {
  const backend: StubBackend = {
    role: initialRole,
    expiresIn: 300,
    resendAfter: 60,
    challenges: 0,
    requestError: "",
    requests: [],
  };
  await page.route("**/api/v1/auth/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    const request = route.request();
    const body: unknown =
      request.method() === "POST" ? request.postDataJSON() : null;
    backend.requests.push({ path, body });
    const session = () => ({
      user: { id: "browser-user", phone: "+989123456789", role: backend.role },
      expires_at: new Date(Date.now() + 3600000).toISOString(),
    });
    if (path.endsWith("/csrf")) {
      await route.fulfill({ json: { csrf_token: "browser-csrf" } });
    } else if (path.endsWith("/session")) {
      await route.fulfill(
        backend.role
          ? { json: session() }
          : {
              status: 401,
              json: { error: { code: "authentication_required" } },
            },
      );
    } else if (path.endsWith("/logout")) {
      backend.role = null;
      await route.fulfill({ status: 204 });
    } else if (path.endsWith("/request") || path.endsWith("/login")) {
      if (backend.requestError) {
        await route.fulfill({
          status: backend.requestError === "rate_limited" ? 429 : 503,
          headers: { "Retry-After": "3" },
          json: {
            error: {
              code: backend.requestError,
              message: "provider-internal-detail",
            },
          },
        });
      } else if (
        path.endsWith("/login") &&
        (typeof body !== "object" ||
          body === null ||
          !("password" in body) ||
          body.password !== "browser-password")
      ) {
        await route.fulfill({
          status: 401,
          json: { error: { code: "invalid_credentials" } },
        });
      } else {
        backend.challenges++;
        await route.fulfill({
          status: 202,
          json: {
            challenge_id: `browser-challenge-${backend.challenges}`,
            expires_in: backend.expiresIn,
            resend_after: backend.resendAfter,
          },
        });
      }
    } else if (path.endsWith("/verify")) {
      if (
        typeof body !== "object" ||
        body === null ||
        !("code" in body) ||
        body.code !== "123456"
      ) {
        await route.fulfill({
          status: 401,
          json: { error: { code: "invalid_otp" } },
        });
      } else {
        backend.role = path.includes("/admin/") ? "admin" : "customer";
        await route.fulfill({ json: session() });
      }
    } else {
      await route.fulfill({ status: 404 });
    }
  });
  await page.route("**/api/v1/admin/**", async (route) => {
    if (backend.role !== "admin") {
      await route.fulfill({
        status: backend.role ? 403 : 401,
        json: { error: { code: "authentication_required" } },
      });
      return;
    }
    const path = new URL(route.request().url()).pathname;
    await route.fulfill({
      json: path.endsWith("/categories")
        ? { categories: [] }
        : { products: [] },
    });
  });
  return backend;
}

async function expectNoOverflow(page: Page) {
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
}

test("customer phone, invalid code, returning session and logout at 360px", async ({
  page,
}) => {
  const backend = await stubAuth(page);
  await page.goto("/login/");
  await expect(
    page.getByRole("heading", { name: "خوش آمدی", exact: true }),
  ).toBeVisible();
  await page.screenshot({
    path: "output/playwright/auth-customer-entry-360.png",
    fullPage: true,
  });
  await page.getByRole("button", { name: "دریافت کد ورود" }).click();
  await expect(page.getByLabel("شمارهٔ همراه", { exact: true })).toBeFocused();
  await expect(page.getByRole("alert")).toContainText("شمارهٔ همراه معتبر");
  await page.getByLabel("شمارهٔ همراه", { exact: true }).fill("۰۹۱۲۳۴۵۶۷۸۹");
  await page.getByRole("button", { name: "دریافت کد ورود" }).click();
  await expect(page.getByLabel("کد ورود ۶ رقمی")).toBeFocused();
  await expect(
    page.getByRole("button", { name: /ارسال دوباره تا/ }),
  ).toBeDisabled();
  expect(
    backend.requests.find((item) => item.path.endsWith("/request"))?.body,
  ).toEqual({ phone: "+989123456789" });
  await page.getByLabel("کد ورود ۶ رقمی").fill("000000");
  await page.getByRole("button", { name: "تأیید و ورود" }).click();
  await expect(page.getByRole("alert")).toContainText("کد درست نیست");
  await expect(page.getByLabel("کد ورود ۶ رقمی")).toBeFocused();
  await expectNoOverflow(page);
  await page.screenshot({
    path: "output/playwright/auth-customer-360.png",
    fullPage: true,
  });
  await page.getByLabel("کد ورود ۶ رقمی").fill("۱۲۳۴۵۶");
  await page.getByRole("button", { name: "تأیید و ورود" }).click();
  await expect(
    page.getByRole("heading", { name: "وارد حسابت شدی" }),
  ).toBeVisible();
  expect(await page.evaluate(() => Object.keys(localStorage))).toEqual([]);
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "وارد حسابت شدی" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "خروج از حساب" }).click();
  await expect(
    page.getByText("از حساب خارج شدی.", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "خوش آمدی", exact: true }),
  ).toBeVisible();
  expect(backend.role).toBeNull();
});

test("expired code can be replaced only after the resend cooldown", async ({
  page,
}) => {
  await page.clock.install();
  const backend = await stubAuth(page);
  backend.expiresIn = 2;
  backend.resendAfter = 3;
  await page.goto("/login/");
  await page.getByLabel("شمارهٔ همراه", { exact: true }).fill("09123456789");
  await page.getByRole("button", { name: "دریافت کد ورود" }).click();
  await expect(page.getByLabel("کد ورود ۶ رقمی")).toBeVisible();
  await page.clock.fastForward(2200);
  await expect(page.getByLabel("کد ورود ۶ رقمی")).toBeDisabled();
  await expect(
    page.getByText("اعتبار این کد تمام شد. یک کد تازه بگیر."),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: /ارسال دوباره تا/ }),
  ).toBeDisabled();
  await page.clock.fastForward(1200);
  await page.getByRole("button", { name: "ارسال دوبارهٔ کد" }).click();
  await expect(page.getByLabel("کد ورود ۶ رقمی")).toBeEnabled();
  expect(backend.challenges).toBe(2);
  await page.getByRole("button", { name: "ویرایش شمارهٔ همراه" }).click();
  await expect(page.getByLabel("شمارهٔ همراه", { exact: true })).toHaveValue(
    "09123456789",
  );
});

test("SMS unavailability stays signed out and does not expose provider details", async ({
  page,
}) => {
  const backend = await stubAuth(page);
  backend.requestError = "sms_unavailable";
  await page.goto("/login/");
  await page.getByLabel("شمارهٔ همراه", { exact: true }).fill("09123456789");
  await page.getByRole("button", { name: "دریافت کد ورود" }).click();
  await expect(page.getByRole("alert")).toContainText(
    "ارسال پیامک فعلاً ممکن نیست",
  );
  await expect(page.getByLabel("کد ورود ۶ رقمی")).toHaveCount(0);
  await expect(page.getByText("provider-internal-detail")).toHaveCount(0);
  expect(backend.role).toBeNull();
});

test("rate limiting disables a new request until Retry-After elapses", async ({
  page,
}) => {
  await page.clock.install();
  const backend = await stubAuth(page);
  backend.requestError = "rate_limited";
  await page.goto("/login/");
  await page.getByLabel("شمارهٔ همراه", { exact: true }).fill("09123456789");
  await page.getByRole("button", { name: "دریافت کد ورود" }).click();
  await expect(page.getByRole("alert")).toContainText("تعداد تلاش‌ها زیاد شده");
  await expect(
    page.getByRole("button", { name: "دریافت کد ورود" }),
  ).toBeDisabled();
  backend.requestError = "";
  await page.clock.fastForward(3200);
  await page.getByRole("button", { name: "دریافت کد ورود" }).click();
  await expect(page.getByLabel("کد ورود ۶ رقمی")).toBeVisible();
});

test("admin catalog stays gated through password and SMS and is hidden after logout", async ({
  page,
}) => {
  const backend = await stubAuth(page);
  await page.goto("/admin/");
  await expect(
    page.getByRole("heading", { name: "ورود به مدیریت", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "هنوز محصولی ثبت نشده است." }),
  ).toHaveCount(0);
  await page.getByLabel("شمارهٔ همراه", { exact: true }).fill("09123456789");
  await page
    .getByLabel("رمز عبور مدیر", { exact: true })
    .fill("wrong-password");
  await page.getByRole("button", { name: "دریافت کد ورود" }).click();
  await expect(page.getByRole("alert")).toContainText("رمز عبور درست نیست");
  await page
    .getByLabel("رمز عبور مدیر", { exact: true })
    .fill("browser-password");
  await page.getByRole("button", { name: "دریافت کد ورود" }).click();
  await expect(page.getByLabel("کد ورود ۶ رقمی")).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "هنوز محصولی ثبت نشده است." }),
  ).toHaveCount(0);
  await page.screenshot({
    path: "output/playwright/auth-admin-360.png",
    fullPage: true,
  });
  await page.getByLabel("کد ورود ۶ رقمی").fill("123456");
  await page.getByRole("button", { name: "تأیید و ورود" }).click();
  await expect(
    page.getByRole("heading", { name: "هنوز محصولی ثبت نشده است." }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "خوش آمدی، مدیر شواندی" }),
  ).toBeVisible();
  await expectNoOverflow(page);
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "هنوز محصولی ثبت نشده است." }),
  ).toBeVisible();
  await page.getByRole("button", { name: "خروج از حساب" }).click();
  await expect(
    page.getByRole("heading", { name: "هنوز محصولی ثبت نشده است." }),
  ).toHaveCount(0);
  await expect(page.getByLabel("رمز عبور مدیر", { exact: true })).toBeVisible();
  expect(backend.role).toBeNull();
});

test("a customer session cannot mount the admin catalog", async ({ page }) => {
  await stubAuth(page, "customer");
  await page.goto("/admin/");
  await expect(
    page.getByRole("heading", { name: "ورود مدیر لازم است" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "هنوز محصولی ثبت نشده است." }),
  ).toHaveCount(0);
  await expectNoOverflow(page);
});

test("an expired admin session removes the catalog and asks for a new login", async ({
  page,
}) => {
  await page.clock.install();
  const backend = await stubAuth(page, "admin");
  await page.goto("/admin/");
  await expect(
    page.getByRole("heading", { name: "هنوز محصولی ثبت نشده است." }),
  ).toBeVisible();
  backend.role = null;
  await page.clock.fastForward(3601000);
  await expect(
    page.getByRole("heading", { name: "هنوز محصولی ثبت نشده است." }),
  ).toHaveCount(0);
  await expect(
    page.getByText("زمان ورودت تمام شده است. دوباره وارد شو."),
  ).toBeVisible();
  await expect(page.getByLabel("رمز عبور مدیر", { exact: true })).toBeVisible();
});

test("customer entry uses the existing store framing at desktop size", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await stubAuth(page);
  await page.goto("/login/");
  await expect(page.getByLabel("شمارهٔ همراه", { exact: true })).toBeVisible();
  await expectNoOverflow(page);
  await page.screenshot({
    path: "output/playwright/auth-customer-desktop.png",
    fullPage: true,
  });
});

test("unavailable session check fails closed with a retry", async ({
  page,
}) => {
  let unavailable = true;
  await page.route("**/api/v1/auth/session", (route) =>
    unavailable
      ? route.abort("failed")
      : route.fulfill({
          status: 401,
          json: { error: { code: "authentication_required" } },
        }),
  );
  await page.goto("/admin/");
  await expect(
    page.getByRole("heading", { name: "بررسی ورود ممکن نشد" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "هنوز محصولی ثبت نشده است." }),
  ).toHaveCount(0);
  await expect(page.getByRole("button", { name: "تلاش دوباره" })).toBeVisible();
  unavailable = false;
  await page.getByRole("button", { name: "تلاش دوباره" }).click();
  await expect(page.getByLabel("رمز عبور مدیر", { exact: true })).toBeVisible();
});

test("admin saves a product, handles a conflict, and publishes the fresh version", async ({
  page,
}) => {
  await stubAuth(page, "admin");
  let product = {
    ID: "product-1",
    Slug: "shirt",
    Name: "پیراهن",
    Description: "توضیحات محصول",
    PrimaryCategory: "category-1",
    RegularPriceToman: 1000,
    SalePriceToman: null,
    PublicationState: "draft",
    Version: 1,
    Variants: [
      {
        ID: "stock-variant",
        Size: "M",
        Color: "Blue",
        Active: true,
        RegularPriceOverrideToman: null,
        SalePriceOverrideToman: null,
        StockOnHand: 0,
        StockVersion: 1,
      },
    ],
    Images: [],
  };
  let conflict = false;
  await page.route("**/api/v1/admin/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname;
    if (path.endsWith("/categories")) {
      await route.fulfill({
        json: {
          categories: [
            {
              ID: "category-1",
              Name: "پوشاک",
              Slug: "clothing",
              Active: true,
              ParentID: null,
              DisplayOrder: 0,
            },
          ],
        },
      });
      return;
    }
    if (request.method() === "GET") {
      await route.fulfill({
        json: path.endsWith("/products")
          ? { products: [product] }
          : { product },
      });
      return;
    }
    expect(request.headers()["x-csrf-token"]).toBe("browser-csrf");
    const body = request.postDataJSON();
    if (path.endsWith("/stock")) {
      expect(body).toEqual({
        version: 1,
        quantity: 4,
        reason: "Initial count",
      });
      product = {
        ...product,
        Variants: product.Variants.map((v) => ({
          ...v,
          StockOnHand: 4,
          StockVersion: 2,
        })),
      };
      await route.fulfill({ json: { product } });
      return;
    }
    expect(body.version).toBe(product.Version);
    if (conflict) {
      conflict = false;
      product = { ...product, Version: product.Version + 1 };
      await route.fulfill({
        status: 409,
        json: { error: { code: "version_conflict" } },
      });
      return;
    }
    product = {
      ...product,
      Name: body.name ?? product.Name,
      Version: product.Version + 1,
      PublicationState: path.endsWith("/publish")
        ? "published"
        : product.PublicationState,
    };
    await route.fulfill({ json: { product } });
  });
  await page.goto("/admin/");
  await page.getByLabel("نام محصول", { exact: true }).fill("پیراهن تازه");
  await page
    .getByRole("button", { name: "ذخیره تغییرات", exact: true })
    .click();
  await expect(page.getByLabel("نام محصول", { exact: true })).toHaveValue(
    "پیراهن تازه",
  );
  conflict = true;
  await page.getByLabel("نام محصول", { exact: true }).fill("تغییر هم‌زمان");
  await page
    .getByRole("button", { name: "ذخیره تغییرات", exact: true })
    .click();
  await expect(page.getByRole("status")).toContainText(
    "نسخهٔ محصول تغییر کرده است",
  );
  await expect(page.getByLabel("نام محصول", { exact: true })).toHaveValue(
    "تغییر هم‌زمان",
  );
  await page
    .getByRole("button", { name: "دریافت نسخهٔ تازه", exact: true })
    .click();
  await expect(page.getByLabel("نام محصول", { exact: true })).toHaveValue(
    "پیراهن تازه",
  );
  await page.getByRole("button", { name: "انتشار محصول", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "پنهان کردن محصول", exact: true }),
  ).toBeVisible();
  await page.getByLabel("موجودی stock-variant", { exact: true }).fill("4");
  await page
    .getByLabel("دلیل تغییر stock-variant", { exact: true })
    .fill("Initial count");
  await page
    .getByRole("button", { name: "ثبت موجودی stock-variant", exact: true })
    .click();
  await expect(
    page.getByLabel("موجودی stock-variant", { exact: true }),
  ).toHaveValue("4");
  await expectNoOverflow(page);
  await page.screenshot({
    path: "output/playwright/admin-editor-360.png",
    fullPage: true,
  });
});
