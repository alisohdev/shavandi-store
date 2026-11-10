import { expect, test } from "@playwright/test";

test("public catalog renders API products, categories, pagination and variant prices", async ({
  page,
}) => {
  await page.goto("/collections/");
  await expect(
    page.getByRole("heading", { name: "پیراهن کاتالوگ" }),
  ).toBeVisible();
  await page.getByRole("link", { name: "پوشاک زنده", exact: true }).click();
  await expect(page).toHaveURL(/collections\/live\//);
  await page.getByRole("link", { name: "محصولات بیشتر" }).click();
  await expect(
    page.getByText("هنوز محصولی در این بخش منتشر نشده است."),
  ).toBeVisible();
  const response = await page.goto("/products/live-shirt/");
  expect(response?.status()).toBe(200);
  await expect(
    page.getByRole("heading", { name: "پیراهن کاتالوگ" }),
  ).toBeVisible();
  await expect(page.getByText(/آبی · M/)).toContainText("۱۶۰٬۰۰۰");
  await expect(
    page.getByText("ثبت سفارش و پرداخت آنلاین هنوز فعال نیست."),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: "output/playwright/public-product-360.png",
    fullPage: true,
  });
});

test("missing products, unknown categories, empty categories and API failures stay explicit", async ({
  page,
}) => {
  expect((await page.goto("/products/hidden-product/"))?.status()).toBe(404);
  await expect(
    page.getByRole("heading", { name: "این محصول پیدا نشد." }),
  ).toBeVisible();
  expect((await page.goto("/collections/unknown/"))?.status()).toBe(404);
  expect((await page.goto("/collections/unavailable/"))?.status()).toBe(503);
  await expect(
    page.getByRole("heading", { name: "دریافت محصولات ممکن نشد." }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "پیراهن کاتالوگ" }),
  ).toHaveCount(0);
  await page.goto("/collections/empty/");
  await expect(
    page.getByText("هنوز محصولی در این بخش منتشر نشده است."),
  ).toBeVisible();
});
