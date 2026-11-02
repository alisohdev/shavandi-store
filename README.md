# Shavandi store — F0

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

| Command                | Purpose                                                                 |
| ---------------------- | ----------------------------------------------------------------------- |
| `bun run dev`          | Start Astro development server                                          |
| `bun run check`        | Astro and TypeScript diagnostics                                        |
| `bun run lint`         | Native Bun lint for TS, TSX, and JavaScript config                      |
| `bun run format`       | Native Bun formatting, then official Astro formatting                   |
| `bun run format:check` | Check both formatting passes without writing                            |
| `bun test`             | Validate fixture identities, availability, references, and local assets |
| `bun run build`        | Generate the static website in `dist/`                                  |
| `bun run preview`      | Serve the production build locally                                      |
| `bun run validate`     | Run every source check, fixture tests, and production build             |

Bun's native formatter does not currently format Astro files. Only `.astro` files use the official `prettier-plugin-astro`, through a separate config and explicit file pattern. The official Astro VS Code extension is the default formatter for Astro. There is no ESLint or Biome substitution. Native commands include `--no-script` to avoid package-script recursion.

## Routes

- `/`: editorial hero, eight product cards, three collections, brand story.
- `/collections/`: the complete sample catalog.
- `/collections/everyday/`, `/collections/soft-layers/`, `/collections/out-in-the-city/`: static curated collections.
- `/products/[slug]/`: eight static product pages with a small React color/size picker. Try `/products/city-coat/` to see color-dependent availability and selection reset; `/products/violet-lace-dress/` is entirely sold out.
- `/about/`: brand direction and a clear explanation of the preview.
- `/guide/`: sample size guide and clothing-care content.
- `/admin/`: public, static admin shell with computed sample metrics and a read-only product table.
- `/404.html`: custom missing-page content.

## F0 boundaries

All product details, fabrics, prices, and stock are fixtures. Prices are integer **toman**, shared across each product's color/size variants. Persian labels identify the preview and sample data. Selection is temporary and does not persist.

This phase has no backend, database, API routes, login, account, basket, order flow, payment, SMS, or operational admin actions. The admin route is a visual shell, not an authenticated administration system. Product names and default colors match the reference photography; the navy coat option explicitly identifies its missing photo.

The home page, navigation, collection cards, product cards, content pages, and admin use Astro components. Only the product picker hydrates React. Navigation is ordinary links; the mobile menu uses native disclosure. Fonts and photographs are local, with no runtime external requests.

See [the design and architecture brief](docs/architecture.md) for the rendering comparison, data model, and future boundary; [asset credits](docs/assets.md) record image and font provenance.
