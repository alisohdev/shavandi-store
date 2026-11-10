# Shavandi store — local commerce preview

A mobile-first Persian RTL clothing storefront built with Astro, Bun, React islands, shadcn/ui, Tailwind CSS, and strict TypeScript. SHAVANDI / شواندی is the provisional identity.

## Run locally

The complete toolchain currently needs a Bun build containing **both native `bun lint` and native `bun fmt`**. It was verified with **Bun 1.4.4-canary.1, revision `271b65ec6`** on Windows. The installed stable Bun 1.4.2 has neither command; stable 1.4.4 has formatting but does not provide the native linter used here. Use that canary or a newer release that exposes both commands. `packageManager` records the tested canary; it does not install it automatically.

Check your executable before installing:

```sh
bun --revision
bun lint --help
bun fmt --help
bun install --frozen-lockfile
bun run dev
```

Open the local URL printed by Astro, normally `http://localhost:4321`. Scripts explicitly run Astro through Bun. Astro's dev server also supports background mode:

```sh
bun --bun astro dev --background
bun --bun astro dev status
bun --bun astro dev stop
```

On Windows, prepend your chosen Bun executable directory to `PATH` so subprocesses use the same version. For example, with this session's verified binary:

```powershell
$env:PATH = 'C:/Users/soheili/AppData/Local/Temp/shavandi-bun-canary/bun-windows-x64;' + $env:PATH
bun run dev
```

That temporary binary path is machine-specific. Keep your own Bun installation in a durable directory. The scaffold declares Node `>=22.23.4` for tooling run through Node; the supplied scripts use Bun. TypeScript stays on major 6 because the installed Astro checker does not yet support TypeScript 7.

## Commands

| Command                | Purpose                                                                          |
| ---------------------- | -------------------------------------------------------------------------------- |
| `bun run dev`          | Start Astro development server                                                   |
| `bun run check`        | Astro and TypeScript diagnostics                                                 |
| `bun run lint`         | Native Bun lint for TS, TSX, and JavaScript config                               |
| `bun run format`       | Native Bun formatting, then official Astro formatting                            |
| `bun run format:check` | Check both formatting passes without writing                                     |
| `bun test`             | Validate fixtures, cart parsing/totals, contact fields, and local product drafts |
| `bun run build`        | Generate the static website in `dist/`                                           |
| `bun run preview`      | Serve the production build locally                                               |
| `bun run validate`     | Run every source check, tests, and production build                              |

Bun's native formatter does not currently format Astro files. Only `.astro` files use the official `prettier-plugin-astro`, through a separate config and explicit file pattern. The official Astro VS Code extension is the default formatter for Astro. There is no ESLint or Biome substitution. Native commands include `--no-script` to avoid package-script recursion.

## Routes

- `/`: real shark-blanket hero, four category tiles, four sourced cozy products, four sourced sports products, and two clearly temporary everyday examples.
- `/collections/`: the complete catalog, with sourced products first and temporary products labeled.
- `/collections/wearable-blankets/`, `/collections/football/`: eight sourced Shavandi products; `/collections/women/`: temporary sample clothing; `/collections/men/`: honest empty collection with an original temporary illustration.
- `/collections/everyday/`, `/collections/soft-layers/`, `/collections/out-in-the-city/`: retained temporary sample collections.
- `/products/[slug]/`: sixteen static product pages with a React color/size picker and add-to-preview-cart action. Try `/products/shark-wearable-blanket/` or `/products/real-madrid-white-shirt/` for sourced products. The original eight routes and SKUs still work; `/products/city-coat/` demonstrates color-dependent availability and selection reset; `/products/violet-lace-dress/` is entirely sold out.
- `/checkout/`: local basket, recipient details, review, and a simulated receipt. A sample-basket button and sample recipient details make the full path easy to try.
- `/about/`: brand direction and a clear explanation of the preview.
- `/guide/`: sample size guide and clothing-care content.
- `/admin/`: public local product editor with live preview, applied table/metrics, and reset actions.
- `/404.html`: custom missing-page content.

## Preview boundaries

Eight products use real primary photos, names, and prices audited from fresh product pages at Shavandi Store and Shavandi Sport on **2026-11-02**. The user confirmed photo reuse rights that day. Listed height, colors, sports sizes, and the France kit's player-version title are recorded where verified. No fan-version, official-license, season, fabric, or care claim is invented. All availability and checkout remain simulated. Unknown blanket sizing is explicitly demo; the shark source specifies height 140–175 cm. Prices are integer **toman**, shared across each product's variants.

The original eight sample products retain their IDs, slugs, SKUs, and cart behavior. Their Unsplash photos and every associated product field are clearly temporary, including the women's collection. Men's clothing has an original temporary SVG and an empty collection; no women's garment has been relabeled as men's inventory. `ProductProvenance` distinguishes sourced, temporary, and local-editor content. Persian labels identify source prices, sample prices, demo stock, and locally edited fields.

The preview cart persists only versioned SKU/quantity pairs in `localStorage` under `shavandi.preview-cart.v1`. Names, images, availability, and prices resolve from the original catalog. Parsing removes unknown/unavailable combinations, merges duplicates, and clamps quantities to 1–9. Storage problems produce a visible notice; a sample basket lets checkout work within the current page when persistence is unavailable.

Checkout accepts Persian, Arabic, and Latin digits for a basic Iranian mobile number and ten-digit postcode. Recipient details and the receipt stay in component memory. Completing the simulation snapshots the receipt and clears the preview cart; it creates no order and makes no payment or SMS request. Refreshing or leaving the page discards recipient details.

The public admin editor changes name, description, positive whole-toman price, and availability of existing color/size combinations. Editing name, description, or price replaces the source label with a local-edit label while retaining original attribution. Applying updates changes only the editor's table and computed metrics. It does not change the public storefront or checkout catalog. All editor changes reset on refresh; reset actions restore one product or all fixtures. IDs, routes, SKUs, images, and color/size identities are fixed.

The Astro frontend remains static. Its cart, checkout, and public editor do not call the API. The Go API provides liveness, PostgreSQL readiness, and read-only M1a catalog endpoints. There is no login, account, real order, payment, SMS, or operational admin access. The public editor is an isolated demonstration. Use sample recipient information when trying checkout.

For Windows API setup, PostgreSQL migrations, sqlc generation, Go tests, and API builds, see [the backend instructions](backend/README.md). The [M1a handoff](docs/m1a-handoff.md) describes the catalog schema, service boundaries, API contract, pricing, and fixture mapping. The frontend commands remain at the repository root.

The home page, navigation, collection cards, product cards, and content pages use Astro components. React hydrates only product controls, checkout, and the admin editor, one island on each relevant page. Navigation uses ordinary links; the mobile menu uses native disclosure. Fonts and photographs are local, with no runtime external requests.

See [the design and architecture brief](docs/architecture.md) for the rendering comparison, data model, and future boundary; [asset credits](docs/assets.md) and the [asset manifest](docs/asset-manifest.json) record photo sources, verified facts, demo fields, rights, and every responsive derivative. Both Astro and React image renderers preserve the full sourced photo with `object-fit: contain`, real intrinsic dimensions, and local width-based `srcset`; no source photo is upscaled.

## Live catalog runtime

Catalog routes now use the Go public API at request time. Build with `bun run build`, then start the Node runtime with `node dist/server/entry.mjs`. Set the server-only `CATALOG_API_ORIGIN` to the Go API origin (defaults to `http://127.0.0.1:8080`). Route `/api/*` to Go at the production reverse proxy, including uploaded image URLs; route frontend pages to Astro. Static-only deployment is no longer sufficient. See [public catalog handoff](docs/public-catalog-handoff.md) for behavior and verification limits.

## Customer cart

The signed-in account cart is at `/cart/`; product pages save selected variant IDs and absolute quantities to PostgreSQL. Apply migration `20261110120000` through the existing Goose workflow before starting the updated API. This cart recomputes catalog prices and does not reserve stock, create orders, or accept payment. The legacy `/checkout/` page remains a separate fixture demo. See [customer cart handoff](docs/customer-cart-handoff.md).

## Temporary orders

Customers can reserve their current account cart for 15 minutes and view/cancel reservations at `/orders/`. Apply migration `20261110140000` before starting the updated API. Reservations do not accept payment or arrange shipping; their displayed total includes only merchandise. See [order reservations](docs/orders-handoff.md) for retry, expiry and verification behavior.
