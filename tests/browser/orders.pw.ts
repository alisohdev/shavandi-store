import { expect, test } from "@playwright/test";

test("a customer retries an uncertain order request with the same key then cancels the reservation", async ({
  page,
}) => {
  const order = {
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
