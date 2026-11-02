# Shavandi preview architecture and design

## Scope

The Persian RTL storefront uses audited source records alongside static sample data, a browser-local preview cart, a simulated checkout, and a public local product editor. It has no database, API routes, server adapter, authentication, account, real order, payment, SMS, or operational admin access. Persian notices distinguish source prices from sample prices, identify demo inventory, shipping, simulation, and the editor's local scope. Only SKU/quantity pairs persist between pages; personal details and product edits stay in memory.

## Design direction

The storefront brings cozy wearable blankets from Shavandi Store and football clothing from Shavandi Sport into one experience, with clearly temporary women's clothing and a men's collection awaiting real products. The home page introduces all four categories and gives the eight sourced products the main product sections. Ordinary links lead to collection and product pages.

The palette is paper `#f7f7f2`, graphite `#232621`, olive `#576047`, mist `#eceee6`, muted gray `#6c7066`, and divider `#d9dcd2`. Local Vazirmatn carries Persian display, body, and utility text; an understated Arial wordmark supplies the Latin identity. The provisional name is SHAVANDI / شواندی.

The signature is the actual shark wearable-blanket photo beside a two-line Persian headline and a secondary football photo. Four category tiles sit near the hero, followed by cozy and sport grids. Desktop uses four product and category columns; narrow screens use two. Local responsive images preserve the full sourced garments without cropping or upscaling. Mobile navigation uses a native disclosure, with Escape and navigation closing it. The interface includes visible focus, a skip link, text labels, explicit sold-out sizes, and reduced-motion support.

Two structures were compared:

| Candidate                          | Structure                                                                                | Tradeoff                                                                                                |
| ---------------------------------- | ---------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| A: Astro pages with a small picker | Static home, collections, cards, content, and admin; one React island per product detail | Ordinary links remain usable without client routing; JS stays with the interaction that needs it        |
| B: One React catalog island        | Catalog, filters, selection, and detail state share a React owner                        | Makes cross-catalog interactions easier, but adds hydration and state coordination before F0 needs them |

The initial storefront uses A, with B's explicit unique SKUs and one island owning product selection. The commerce preview extends A with separate checkout and editor islands. It borrows explicit checkout phases and boundary validation from a shared-state alternative while keeping the public catalog static. A catalog-wide React root, filter/sort system, intercepted detail routes, and quick-view modal remain unnecessary. Product details are ordinary static routes.

## Data boundary

`src/data/catalog.ts` is the single typed source for products and editorial collections. `Product` owns colorways; each colorway has a non-empty image list and a non-empty list of explicit SKU/size/availability combinations. Product and card availability derive from those combinations. A product with all variants sold out is displayed as unavailable.

`ProductProvenance` is a discriminated union: sourced data carries its product-page URL, audit date, confirmed reuse rights, verified facts, and demo facts; temporary data carries its image credit and notice. The local editor adds a local-edit variant retaining original attribution whenever a name, description, or price changes. Source verification therefore cannot silently carry over to edited copy or prices. Availability remains demo for every product. The original eight product identities and SKUs are retained alongside eight sourced records. Four primary curated collections coexist with the original three sample collection routes; the men's category is intentionally empty.

`docs/asset-manifest.json` records the source product pages, original image URLs, prices, verified attributes, demo fields, and responsive derivative dimensions. The raw source HTML and JPEGs are development evidence outside the repository. Astro and React image renderers share `ProductImage` metadata and explicit sizes. Sourced photos use native dimensions and contain fitting; temporary references retain their existing presentation.

Prices are positive integer **toman**, never rial. All variants of a product share its product price in F0. Future per-SKU pricing belongs at that boundary when it is required. There is no parallel stock counter or independent list of available sizes.

`ProductPicker.tsx` owns the selected colorway and optional variant. A color change clears the size, derives the gallery from the new colorway, and offers only that colorway's defined size combinations. Unavailable combinations cannot be selected. The sample city coat intentionally has different available sizes across two colors. The navy option explicitly explains that the blue reference photo is being shown.

A future data adapter can replace fixture construction with validated external data while preserving this product shape. F0 does not introduce an unused repository abstraction or backend framework. External data will need validation at that future boundary.

## Local cart and checkout

`src/lib/preview-cart.ts` owns the versioned `shavandi.preview-cart.v1` storage boundary. The stored shape contains only `{ version: 1, lines: [{ sku, quantity }] }`. JSON enters as `unknown`; parsing accepts known available SKUs, merges duplicate lines, and clamps whole quantities to 1–9. Display data always resolves from the original fixtures. Malformed, stale, or unavailable storage is announced in the interface. The empty state offers a sample basket that also works in page memory when storage cannot be written.

The product island adds only a selected available SKU. A color change clears the size. The shared header links to checkout without a separate cart-count island or global context.

`CheckoutPreview.tsx` loads browser storage after hydration. A discriminated union models recipient details, review, and completion. Quantity/removal changes update the local cart and integer-toman totals; sample shipping is a fixed 79,000 toman. The contact boundary trims required fields and normalizes Persian/Arabic digits before validating an 11-digit `09` mobile number and ten-digit postcode. Inline errors identify fields and focus the first invalid input. Sample information can fill the form.

Recipient details and receipts exist only in React state. Review preserves the recipient details when going back to edit. Completion captures the lines, contact, and totals before clearing the cart, so the receipt still shows the completed simulation. No request, order, payment, card entry, or SMS is made. Contact details do not enter storage, URLs, analytics, or network payloads.

## Local product editor

`AdminEditor.tsx` owns an applied copy of the seed catalog and one selected product draft. Name, description, price, and existing SKU availability feed a live product preview. Positive whole-toman prices are parsed from input strings, including Persian and Arabic digits, before application. `preview-editor.ts` reconstructs non-empty colorways/variants while preserving IDs, slugs, SKUs, sizes, images, and other fixture fields.

Applying a valid draft updates only this island's table and availability metrics. Changed name, description, or price receives a local-edit provenance label in both live preview and the applied table. Reset actions restore the selected product or all seed data, including original provenance. Neither drafts nor applied changes are persisted, and the public catalog/cart continue to use the original fixtures. Switching the selected product discards unapplied form changes; the interface explains this behavior. No product creation/deletion, authentication, remote save, or global catalog provider is introduced.

## Rendering and components

- `StoreLayout.astro`: Persian document metadata, local fonts, shared header/footer, skip link.
- `AdminLayout.astro`: separate public local editor frame and noindex metadata.
- Astro product and collection cards: server-rendered, with ordinary links and no hydration.
- `ProductPicker.tsx`: one React island on each product page, including local cart actions.
- `CheckoutPreview.tsx`: one React island on the checkout route; browser storage is loaded in an effect so the static render needs no `window`.
- `AdminEditor.tsx`: one React island within the admin frame, with an isolated in-memory catalog.
- `styles/preview.css`: responsive commerce/editor styles using the storefront tokens and local typography.
- `components/ui/button.tsx`: genuine shadcn/ui Button obtained from its [official New York registry](https://ui.shadcn.com/r/styles/new-york/button.json), used by the size picker. `components.json`, `cn`, Tailwind tokens, and Radix Slot retain the normal shadcn integration.

Images and fonts are served locally. No runtime image API, external font service, analytics, or third-party request is needed.

## Tooling decisions

Astro is configured for static output with the official React integration and Tailwind's Vite plugin. TypeScript extends Astro's `strictest` configuration. TypeScript is pinned to major 6 because the installed Astro checker explicitly does not support TypeScript 7 yet.

The verified Bun canary provides native `bun lint` and `bun fmt`. `--no-script` prevents the native commands from recursively invoking identically named package scripts. Native lint covers TypeScript/TSX and configuration JavaScript. Native format uses explicit supported file patterns and excludes `.astro`, generated output, and binary assets.

Bun's native formatter currently has no Astro formatter. The official `prettier-plugin-astro` handles **only `.astro` files**, with a separate explicit config so Bun never tries to load that plugin. VS Code uses the official Astro extension as the Astro formatter. This narrow fallback preserves Bun as the linter and formatter for the supported source formats.

Fixture tests protect route/SKU uniqueness, integer toman, real color-size availability, editorial references, and local images. Pure preview tests cover malformed/stale storage, duplicate/quantity recovery, catalog-derived totals, contact validation/digit normalization, and product edits preserving stable identities. `astro check`, native lint/format checks, the Astro formatting check, and a production build cover source validation.

Browser checks exercise mobile navigation, unavailable options, cart persistence across ordinary links, empty/invalid forms, sample fill, quantity changes, review/back, receipt/cart clearing, storage failures, editor validation/apply/reset, and refresh resetting edits. Responsive checks use 360, 768, and 1440 pixels. Personal data must stay out of storage and URLs; the preview must make no network mutations.
