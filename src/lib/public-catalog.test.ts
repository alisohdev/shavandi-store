import { expect, test } from "bun:test";
import { createPublicCatalog, parsePublicProduct } from "./public-catalog";

test("public API client encodes category and cursor and sends no browser credentials", async () => {
  const client = createPublicCatalog(
    "http://127.0.0.1:8080",
    async (input, init) => {
      const url = new URL(String(input));
      expect(url.searchParams.get("category")).toBe("sports");
      expect(url.searchParams.get("cursor")).toBe("a+b/=");
      expect(init?.cache).toBe("no-store");
      expect(init?.headers).toEqual({ Accept: "application/json" });
      return Response.json({ products: [], next_cursor: "next" });
    },
  );
  expect(await client.products("sports", "a+b/=")).toEqual({
    products: [],
    nextCursor: "next",
  });
});
test("public API failures and malformed data never become fixture products", async () => {
  const client = createPublicCatalog("http://127.0.0.1:8080", async () =>
    Response.json({}, { status: 503 }),
  );
  await expect(client.products()).rejects.toMatchObject({ status: 503 });
  expect(() => parsePublicProduct({ id: "bad" })).toThrow();
  expect(() =>
    createPublicCatalog("https://user:secret@example.test"),
  ).toThrow();
});
