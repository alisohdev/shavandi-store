import { expect, test } from "@playwright/test";

function paymentOrder() {
  return {
    id: "0192bbd1-9e2d-4a45-9baf-37f4d6280801",
    status: "pending_confirmation",
    payment_status: "none",
    payment_environment: "production",
    payment_reference: null as string | null,
    subtotal_toman: 160000,
    created_at: new Date().toISOString(),
    expires_at: new Date(Date.now() + 900000).toISOString(),
    lines: [
      {
        variant_id: "variant-1",
        name: "پیراهن",
        color: "آبی",
        size: "M",
        quantity: 1,
        unit_price_toman: 160000,
      },
    ],
    delivery: {
      recipient: "گیرنده آزمایشی",
      phone: "+989123456789",
      province: "تهران",
      city: "تهران",
      address: "خیابان آزمایشی پلاک ۱۲",
      postal_code: "1234567890",
      version: 1,
      shipping_method_id: "shipping-1",
      shipping_version: 1,
      shipping_name: "پست",
      shipping_fee_toman: 75000,
    },
  };
}

test("reviewed total starts Zarinpal and gateway return requires verification", async ({
  page,
}) => {
  const order = paymentOrder();
  const authority = "A" + "0".repeat(35);
  let starts = 0;
  await page.route("**/api/v1/payments/config", (route) =>
    route.fulfill({ json: { enabled: true, sandbox: false } }),
  );
  await page.route("**/api/v1/shipping-methods", (route) =>
    route.fulfill({ json: { methods: [] } }),
  );
  await page.route("**/api/v1/auth/csrf", (route) =>
    route.fulfill({ json: { csrf_token: "payment-csrf" } }),
  );
  await page.route("**/api/v1/orders**", async (route) => {
    const request = route.request();
    if (request.method() === "GET") {
      await route.fulfill({ json: { orders: [order] } });
      return;
    }
    expect(request.headers()["x-csrf-token"]).toBe("payment-csrf");
    if (new URL(request.url()).pathname.endsWith("/verify")) {
      expect(request.postDataJSON()).toEqual({});
      order.status = "paid";
      order.payment_status = "succeeded";
      order.payment_reference = "123456789012345678";
      await route.fulfill({ json: { status: "checked" } });
      return;
    }
    expect(request.postDataJSON()).toEqual({
      delivery_version: 1,
      expected_total_toman: 235000,
    });
    starts++;
    order.payment_status = "ready";
    await route.fulfill({
      json: {
        redirect_url: `https://payment.zarinpal.com/pg/StartPay/${authority}`,
      },
    });
  });
  await page.route("https://payment.zarinpal.com/pg/StartPay/**", (route) =>
    route.fulfill({ contentType: "text/html", body: "<h1>Test gateway</h1>" }),
  );
  await page.goto("/orders/");
  await page
    .getByRole("button", { name: "پرداخت با زرین‌پال", exact: true })
    .click();
  await expect(page).toHaveURL(
    `https://payment.zarinpal.com/pg/StartPay/${authority}`,
  );
  await page.goto("http://127.0.0.1:4322/orders/?payment=returned&Status=OK");
  await expect(page.getByText("رزرو موقت", { exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: /لغو رزرو/ })).toBeDisabled();
  await expect(page.getByLabel("نام گیرنده", { exact: true })).toHaveCount(0);
  await page
    .getByRole("button", { name: "بررسی نتیجه پرداخت", exact: true })
    .click();
  await expect(
    page.getByText("پرداخت تأییدشده", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("123456789012345678", { exact: true }),
  ).toBeVisible();
  expect(starts).toBe(1);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: "output/playwright/paid-order-360.png",
    fullPage: true,
  });
});

test("late verified payment is shown for manual review without another charge", async ({
  page,
}) => {
  const order = paymentOrder();
  order.status = "payment_review";
  order.payment_status = "succeeded";
  order.payment_reference = "12345";
  await page.route("**/api/v1/orders", (route) =>
    route.fulfill({ json: { orders: [order] } }),
  );
  await page.goto("/orders/");
  await expect(
    page.getByText("پرداخت نیازمند بررسی فروشگاه", { exact: true }),
  ).toBeVisible();
  await expect(page.getByText(/فروشگاه باید پرداخت را بررسی/)).toBeVisible();
  order.payment_environment = "sandbox";
  await page.reload();
  await expect(
    page.getByText("پرداخت آزمایشی؛ وجه واقعی دریافت نشده است.", {
      exact: true,
    }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "پرداخت با زرین‌پال", exact: true }),
  ).toHaveCount(0);
});

test("payment rejects a redirect outside the gateway", async ({ page }) => {
  const order = paymentOrder();
  await page.route("**/api/v1/payments/config", (route) =>
    route.fulfill({ json: { enabled: true, sandbox: false } }),
  );
  await page.route("**/api/v1/shipping-methods", (route) =>
    route.fulfill({ json: { methods: [] } }),
  );
  await page.route("**/api/v1/auth/csrf", (route) =>
    route.fulfill({ json: { csrf_token: "payment-csrf" } }),
  );
  await page.route("**/api/v1/orders**", (route) =>
    route.fulfill({
      json:
        route.request().method() === "GET"
          ? { orders: [order] }
          : { redirect_url: "https://evil.example/steal" },
    }),
  );
  await page.goto("/orders/");
  await page
    .getByRole("button", { name: "پرداخت با زرین‌پال", exact: true })
    .click();
  await expect(page.getByRole("status")).toContainText(
    "ارتباط با پرداخت برقرار نشد",
  );
  await expect(page).toHaveURL(/127\.0\.0\.1:4322\/orders/);
});
