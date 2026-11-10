import { expect, test } from "@playwright/test";

test("customer saves a selected variant, changes quantity and removes it", async ({
  page,
}) => {
  let quantity = 0;
  await page.route("**/api/v1/auth/csrf", (route) =>
    route.fulfill({ json: { csrf_token: "cart-csrf" } }),
  );
  await page.route("**/api/v1/cart**", async (route) => {
    const request = route.request();
    if (request.method() === "POST") {
      expect(request.headers()["x-csrf-token"]).toBe("cart-csrf");
      expect(request.postDataJSON()).toEqual({
        quantity: request.postDataJSON().quantity,
      });
      quantity = request.postDataJSON().quantity;
    }
    await route.fulfill({
      json: {
        lines: quantity
          ? [
              {
                variant_id: "variant-1",
                product_slug: "live-shirt",
                name: "پیراهن کاتالوگ",
                size: "M",
                color: "آبی",
                quantity,
                available: true,
                available_quantity: 5,
                unit_price_toman: 160000,
                line_total_toman: 160000 * quantity,
              },
            ]
          : [],
        subtotal_toman: 160000 * quantity,
        has_unavailable: false,
        available_quantity: 0,
      },
    });
  });
  await page.goto("/products/live-shirt/");
  await page
    .getByLabel("رنگ و اندازه", { exact: true })
    .selectOption("variant-1");
  await page.getByLabel("تعداد در سبد", { exact: true }).fill("2");
  await page.getByRole("button", { name: "ذخیره در سبد", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("در سبد ذخیره شد");
  await page.getByRole("link", { name: "دیدن سبد" }).click();
  await expect(
    page.getByText("جمع کالاهای در دسترس: ۳۲۰٬۰۰۰ تومان"),
  ).toBeVisible();
  await page.getByLabel("تعداد پیراهن کاتالوگ").selectOption("3");
  await expect(
    page.getByText("جمع کالاهای در دسترس: ۴۸۰٬۰۰۰ تومان"),
  ).toBeVisible();
  await page.reload();
  await expect(page.getByLabel("تعداد پیراهن کاتالوگ")).toHaveValue("3");
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: "output/playwright/customer-cart-360.png",
    fullPage: true,
  });
  await page.getByRole("button", { name: "حذف پیراهن کاتالوگ" }).click();
  await expect(page.getByText("سبدت هنوز خالی است.")).toBeVisible();
});

test("signed-out and unavailable cart states require explicit recovery", async ({
  page,
}) => {
  await page.route("**/api/v1/cart**", (route) =>
    route.fulfill({
      status: 401,
      json: { error: { code: "authentication_required" } },
    }),
  );
  await page.goto("/cart/");
  await expect(
    page.getByRole("link", { name: "ورود به حساب", exact: true }),
  ).toBeVisible();
  await page.unroute("**/api/v1/cart**");
  await page.route("**/api/v1/cart**", (route) =>
    route.fulfill({
      json: {
        lines: [
          {
            variant_id: "variant-1",
            product_slug: "hidden",
            name: "کالای پنهان",
            size: null,
            color: null,
            quantity: 2,
            available: false,
            available_quantity: 0,
            unit_price_toman: null,
            line_total_toman: null,
          },
        ],
        subtotal_toman: 0,
        has_unavailable: true,
        available_quantity: 5,
      },
    }),
  );
  await page.reload();
  await expect(
    page.getByText(
      "تعداد انتخاب‌شده در دسترس نیست؛ تعداد را کم کن یا از سبد حذفش کن.",
    ),
  ).toBeVisible();
  await expect(page.getByLabel("تعداد کالای پنهان")).toBeDisabled();
  await expect(
    page.getByRole("button", { name: "حذف کالای پنهان" }),
  ).toBeEnabled();
  await expect(page.getByText("جمع کالاهای در دسترس: ۰ تومان")).toBeVisible();
});

test("a stock reduction lets a customer lower an existing cart quantity", async ({
  page,
}) => {
  let quantity = 3;
  await page.route("**/api/v1/auth/csrf", (route) =>
    route.fulfill({ json: { csrf_token: "cart-csrf" } }),
  );
  await page.route("**/api/v1/cart**", async (route) => {
    if (route.request().method() === "POST") {
      expect(route.request().postDataJSON()).toEqual({ quantity: 1 });
      quantity = 1;
    }
    await route.fulfill({
      json: {
        lines: [
          {
            variant_id: "variant-1",
            product_slug: "live-shirt",
            name: "پیراهن کاتالوگ",
            size: "M",
            color: "آبی",
            quantity,
            available: quantity === 1,
            available_quantity: 1,
            unit_price_toman: quantity === 1 ? 160000 : null,
            line_total_toman: quantity === 1 ? 160000 : null,
          },
        ],
        subtotal_toman: quantity === 1 ? 160000 : 0,
        has_unavailable: quantity !== 1,
      },
    });
  });
  await page.goto("/cart/");
  await expect(page.getByLabel("تعداد پیراهن کاتالوگ")).toHaveValue("3");
  await page.getByLabel("تعداد پیراهن کاتالوگ").selectOption("1");
  await expect(
    page.getByText("جمع کالاهای در دسترس: ۱۶۰٬۰۰۰ تومان"),
  ).toBeVisible();
});
