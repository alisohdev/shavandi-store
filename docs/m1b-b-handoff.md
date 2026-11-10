# M1b-B image storage and API handoff

Product image files are stored outside the repository. Set `PRODUCT_IMAGE_DIR` to choose the directory; otherwise the API uses the OS user config directory under `shavandi-store/product-images`. The local store uses opaque random keys, restrictive file creation mode, and an allowlisted key format. PostgreSQL stores only `/api/v1/images/{key}` references and image metadata.

Admin endpoints:

- `POST /api/v1/admin/products/{id}/images` — multipart form with `version` and one or more `images` files. Supports JPEG, PNG, and WebP; each file is limited to 10 MiB, uploads to 10 images per product, and decoded dimensions to 25 million pixels.
- `PATCH /api/v1/admin/products/{id}/images` — JSON `{ "version": 3, "image_ids": ["…", "…"] }`; the first ordered image is the cover.
- `DELETE /api/v1/admin/products/{id}/images/{imageID}` — JSON `{ "version": 3 }`.
- `GET /api/v1/images/{key}` — serves images linked to published products publicly. Draft and hidden product images require an MFA-verified admin session.

Mutations use the existing admin guards and product version checks. Newly saved files are deleted if catalog persistence fails. After removing metadata, the API scans product references before deleting the file. A process crash between file save and catalog commit can leave an unreferenced file; a future maintenance task can compare local opaque keys against `product_images.asset_ref` and remove old unreferenced files. No automatic sweeping is enabled.

Current tests exercise PNG upload, safe opaque storage keys, retrieval, and path traversal rejection. JPEG/WebP, limit-boundary, reorder/removal, and injected database/storage failure cases are not covered by focused tests yet. PostgreSQL catalog integration tests continue to cover product/image metadata transaction behavior and publication requirements.
