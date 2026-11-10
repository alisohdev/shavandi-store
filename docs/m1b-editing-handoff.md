# M1b admin editing

The authenticated admin catalog now supports creating product drafts, editing name/slug/description/category/base price, adding active size/color variants, uploading JPEG/PNG/WebP images, moving images earlier (first is cover), removing images, and publishing/hiding products. Existing sale prices and variant price overrides are preserved; editing those values and category management remain outside this UI slice.

Every mutation obtains a signed CSRF token and includes session credentials. Product versions come from the latest server response. A conflict preserves the entered form, disables further writes, and offers a fresh version; refreshing replaces unsaved input. Duplicate in-flight writes are blocked. The server continues to enforce MFA, authorization, validation, image limits and publication prerequisites.

Verification: ten mocked-API browser tests pass in installed Chrome, including MFA/logout/expiry and product save/conflict/refresh/publication at 360px. Type checks and lint pass; the design detector reports no findings. The mobile editor screenshot was visually inspected. These browser tests do not prove live database/SMS interaction. Image lifecycle behavior has separate Go HTTP tests. A real administrator walkthrough still needs local provisioning or real SMS configuration.

Public storefront pages still use the existing static fixtures. Publishing through the admin API changes the backend catalog; wiring storefront rendering to that catalog is a separate next phase.
