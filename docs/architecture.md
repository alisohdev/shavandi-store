# Shavandi F0 architecture and design

## Scope

F0 establishes the Persian RTL storefront and a public, read-only admin shell. It contains static fixture data and a local, temporary color/size selection. It has no database, API routes, server adapter, authentication, account, basket, order, payment, SMS, or real admin operations. Persian notices identify the preview, sample prices/inventory, and disabled purchasing. No selections persist between visits.

## Design direction

The subject is a contemporary women's clothing store for people building a practical everyday wardrobe. The home page helps visitors discover the collection and follow ordinary product links.

The palette is paper `#f7f7f2`, graphite `#232621`, olive `#576047`, mist `#eceee6`, muted gray `#6c7066`, and divider `#d9dcd2`. Local Vazirmatn carries Persian display, body, and utility text; an understated Arial wordmark supplies the Latin identity. The provisional name is SHAVANDI / شواندی.

The signature is a large street-style photograph beside an oversized, two-line Persian headline. Quiet product grids and unboxed captions leave the clothing in focus. Desktop uses four product columns; narrow screens use two. Mobile navigation uses a native disclosure, with Escape and navigation closing it. The interface includes visible focus, a skip link, text labels, explicit sold-out sizes, and reduced-motion support.

Two structures were compared:

| Candidate                          | Structure                                                                                | Tradeoff                                                                                                |
| ---------------------------------- | ---------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------- |
| A: Astro pages with a small picker | Static home, collections, cards, content, and admin; one React island per product detail | Ordinary links remain usable without client routing; JS stays with the interaction that needs it        |
| B: One React catalog island        | Catalog, filters, selection, and detail state share a React owner                        | Makes cross-catalog interactions easier, but adds hydration and state coordination before F0 needs them |

The final structure uses A, with B's explicit unique SKUs and one island owning product selection. A catalog-wide React root, filter/sort system, intercepted detail routes, and quick-view modal were rejected for F0. Product details are ordinary static routes.

## Data boundary

`src/data/catalog.ts` is the single typed source for products and editorial collections. `Product` owns colorways; each colorway has a non-empty image list and a non-empty list of explicit SKU/size/availability combinations. Product and card availability derive from those combinations. A product with all variants sold out is displayed as unavailable.

Prices are positive integer **toman**, never rial. All variants of a product share its product price in F0. Future per-SKU pricing belongs at that boundary when it is required. There is no parallel stock counter or independent list of available sizes.

`ProductPicker.tsx` owns the selected colorway and optional variant. A color change clears the size, derives the gallery from the new colorway, and offers only that colorway's defined size combinations. Unavailable combinations cannot be selected. The sample city coat intentionally has different available sizes across two colors. The navy option explicitly explains that the blue reference photo is being shown.

A future data adapter can replace fixture construction with validated external data while preserving this product shape. F0 does not introduce an unused repository abstraction or backend framework. External data will need validation at that future boundary.

## Rendering and components

- `StoreLayout.astro`: Persian document metadata, local fonts, shared header/footer, skip link.
- `AdminLayout.astro`: separate public admin frame and noindex metadata.
- Astro product and collection cards: server-rendered, with ordinary links and no hydration.
- `ProductPicker.tsx`: the only React island, hydrated with `client:load` on detail pages.
- `components/ui/button.tsx`: genuine shadcn/ui Button obtained from its [official New York registry](https://ui.shadcn.com/r/styles/new-york/button.json), used by the size picker. `components.json`, `cn`, Tailwind tokens, and Radix Slot retain the normal shadcn integration.

Images and fonts are served locally. No runtime image API, external font service, analytics, or third-party request is needed.

## Tooling decisions

Astro is configured for static output with the official React integration and Tailwind's Vite plugin. TypeScript extends Astro's `strictest` configuration. TypeScript is pinned to major 6 because the installed Astro checker explicitly does not support TypeScript 7 yet.

The verified Bun canary provides native `bun lint` and `bun fmt`. `--no-script` prevents the native commands from recursively invoking identically named package scripts. Native lint covers TypeScript/TSX and configuration JavaScript. Native format uses explicit supported file patterns and excludes `.astro`, generated output, and binary assets.

Bun's native formatter currently has no Astro formatter. The official `prettier-plugin-astro` handles **only `.astro` files**, with a separate explicit config so Bun never tries to load that plugin. VS Code uses the official Astro extension as the Astro formatter. This narrow fallback preserves Bun as the linter and formatter for the supported source formats.

Fixture tests protect route/SKU uniqueness, integer toman, real color-size availability, editorial references, and the presence of local images. `astro check`, native lint/format checks, the Astro formatting check, and a production build cover the remaining source validation. Browser checks should exercise mobile navigation, color changes clearing sizes, unavailable options, direct navigation, and page layout at 360, 768, and 1440 pixels.
