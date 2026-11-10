import { expect, test } from "@playwright/test";
test.beforeEach(async ({ page }) => {
  await page.route("**/api/v1/payments/config", (route) =>
    route.fulfill({ json: { enabled: false, sandbox: false } }),
  );
  await page.route("**/api/v1/shipping-methods", (route) =>
    route.fulfill({ json: { methods: [] } }),
  );
});

test("a customer retries an uncertain order request with the same key then cancels the reservation", async ({
  page,
}) => {
  const order = {
    delivery: null,
    payment_status: "none",
    payment_environment: "production",
    payment_reference: null,
    id: "0192bbd1-9e2d-4a45-9baf-37f4d6280701",
    status: "pending_confirmation",
    subtotal_toman: 160000,
    created_at: new Date().toISOString(),
    expires_at: new Date(Date.now() + 900000).toISOString(),
    lines: [
      {
        variant_id: "variant-1",
        name: "پیراهن کاتالوگ",
        color: "آبی",
        size: "M",
        quantity: 1,
        unit_price_toman: 160000,
      },
    ],
  };
  let key = "";
  let attempts = 0;
  await page.route("**/api/v1/auth/csrf", (route) =>
    route.fulfill({ json: { csrf_token: "order-csrf" } }),
  );
  await page.route("**/api/v1/cart", (route) =>
    route.fulfill({
      json: {
        quote_token: "a".repeat(64),
        lines: [
          {
            variant_id: "variant-1",
            product_slug: "live-shirt",
            name: "پیراهن کاتالوگ",
            color: "آبی",
            size: "M",
            quantity: 1,
            available: true,
            available_quantity: 2,
            unit_price_toman: 160000,
            line_total_toman: 160000,
          },
        ],
        subtotal_toman: 160000,
        has_unavailable: false,
      },
    }),
  );
  await page.route("**/api/v1/orders**", async (route) => {
    const request = route.request();
    if (request.method() === "GET") {
      await route.fulfill({ json: { orders: attempts ? [order] : [] } });
      return;
    }
    expect(request.headers()["x-csrf-token"]).toBe("order-csrf");
    if (new URL(request.url()).pathname.endsWith("/cancel")) {
      expect(request.postDataJSON()).toEqual({});
      order.status = "cancelled";
      await route.fulfill({ json: { order } });
      return;
    }
    const body = request.postDataJSON();
    expect(body.quote_token).toBe("a".repeat(64));
    expect(Object.keys(body).sort()).toEqual(["key", "quote_token"]);
    if (attempts === 0) {
      key = body.key;
      attempts++;
      await route.abort();
      return;
    }
    expect(body.key).toBe(key);
    attempts++;
    await route.fulfill({ json: { order } });
  });
  await page.goto("/cart/");
  await page
    .getByRole("button", { name: "تأیید و رزرو سفارش", exact: true })
    .click();
  await expect(page.getByRole("status")).toContainText("پاسخ سفارش دریافت نشد");
  await page
    .getByRole("button", { name: "تأیید و رزرو سفارش", exact: true })
    .click();
  await expect(page.getByRole("status")).toContainText("سفارش رزرو شد");
  await page.getByRole("link", { name: "دیدن سفارش‌ها", exact: false }).click();
  await expect(page.getByText("رزرو موقت", { exact: true })).toBeVisible();
  await expect(page.getByText("جمع کالاها: ۱۶۰٬۰۰۰ تومان")).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: "output/playwright/customer-orders-360.png",
    fullPage: true,
  });
  await page
    .getByRole("button", { name: `لغو رزرو ${order.id}`, exact: true })
    .click();
  await expect(page.getByText("لغوشده", { exact: true })).toBeVisible();
  expect(attempts).toBe(2);
});

test("orders require sign-in", async ({ page }) => {
  await page.route("**/api/v1/orders", (route) =>
    route.fulfill({
      status: 401,
      json: { error: { code: "authentication_required" } },
    }),
  );
  await page.goto("/orders/");
  await expect(
    page.getByRole("link", { name: "ورود به حساب", exact: true }),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: /لغو رزرو/ })).toHaveCount(0);
});

test("delivery details persist, keep drafts on conflict, and stay out of browser storage", async ({
  page,
}) => {
  const recipient = "گیرنده آزمایشی";
  await page.route("**/api/v1/shipping-methods", (route) =>
    route.fulfill({
      json: {
        methods: [
          {
            id: "shipping-1",
            name: "پست",
            fee_toman: 75000,
            version: 1,
            active: true,
          },
        ],
      },
    }),
  );
  const order = {
    payment_status: "none",
    payment_environment: "production",
    payment_reference: null,
    id: "0192bbd1-9e2d-4a45-9baf-37f4d6280702",
    status: "pending_confirmation",
    subtotal_toman: 160000,
    created_at: new Date().toISOString(),
    expires_at: new Date(Date.now() + 900000).toISOString(),
    lines: [],
    delivery: null as null | {
      recipient: string;
      phone: string;
      province: string;
      city: string;
      address: string;
      postal_code: string;
      version: number;
      shipping_method_id: string;
      shipping_version: number;
      shipping_name: string;
      shipping_fee_toman: number;
    },
  };
  let conflict = false;
  await page.route("**/api/v1/auth/csrf", (route) =>
    route.fulfill({ json: { csrf_token: "delivery-csrf" } }),
  );
  await page.route("**/api/v1/orders**", async (route) => {
    if (route.request().method() === "GET") {
      await route.fulfill({ json: { orders: [order] } });
      return;
    }
    expect(route.request().headers()["x-csrf-token"]).toBe("delivery-csrf");
    const body = route.request().postDataJSON();
    expect(Object.keys(body).sort()).toEqual([
      "address",
      "city",
      "phone",
      "postal_code",
      "province",
      "recipient",
      "shipping_method_id",
      "shipping_version",
      "version",
    ]);
    if (conflict) {
      await route.fulfill({
        status: 409,
        json: { error: { code: "order_conflict" } },
      });
      return;
    }
    expect(body.version).toBe(0);
    expect(body.shipping_method_id).toBe("shipping-1");
    expect(body.shipping_version).toBe(1);
    order.delivery = {
      ...body,
      phone: "+989123456789",
      postal_code: "1234567890",
      shipping_name: "پست",
      shipping_fee_toman: 75000,
      version: 1,
    };
    await route.fulfill({ json: { order } });
  });
  await page.goto("/orders/");
  await page
    .getByLabel("روش ارسال", { exact: true })
    .selectOption("shipping-1");
  await page.getByLabel("نام گیرنده", { exact: true }).fill(recipient);
  await page.getByLabel("همراه گیرنده", { exact: true }).fill("۰۹۱۲۳۴۵۶۷۸۹");
  await page.getByLabel("استان", { exact: true }).fill("تهران");
  await page.getByLabel("شهر", { exact: true }).fill("تهران");
  await page
    .getByLabel("نشانی کامل", { exact: true })
    .fill("خیابان آزمایشی، پلاک ۱۲");
  await page.getByLabel("کد پستی", { exact: true }).fill("۱۲۳۴۵۶۷۸۹۰");
  await page
    .getByRole("button", { name: "ذخیره اطلاعات گیرنده", exact: true })
    .click();
  await expect(page.getByRole("alert")).toContainText(
    "اطلاعات گیرنده و روش ارسال ذخیره شد",
  );
  await expect(page.getByText("جمع کالا و ارسال: ۲۳۵٬۰۰۰ تومان")).toBeVisible();
  await page.reload();
  await expect(page.getByLabel("نام گیرنده", { exact: true })).toHaveValue(
    recipient,
  );
  await expect(page.getByLabel("کد پستی", { exact: true })).toHaveValue(
    "1234567890",
  );
  expect(
    await page.evaluate(() =>
      JSON.stringify({
        local: { ...localStorage },
        session: { ...sessionStorage },
      }),
    ),
  ).not.toContain(recipient);
  expect(page.url()).not.toContain(encodeURIComponent(recipient));
  conflict = true;
  await page.getByLabel("نام گیرنده", { exact: true }).fill("نوشته ذخیره نشده");
  await page
    .getByRole("button", { name: "ذخیره اطلاعات گیرنده", exact: true })
    .click();
  await expect(page.getByRole("alert")).toContainText("نسخهٔ نشانی تغییر کرده");
  await expect(page.getByLabel("نام گیرنده", { exact: true })).toHaveValue(
    "نوشته ذخیره نشده",
  );
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: "output/playwright/delivery-details-360.png",
    fullPage: true,
  });
});
