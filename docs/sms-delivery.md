# SMS delivery

The normal API build supports the same IranPayamak and MeliPayamak request contracts as better-buy. Set SMS_PROVIDER to iranpayamak or melipayamak and supply the matching backend-only variables in backend/.env.example to the API process environment. No credentials were copied from the reference project.

Set OTP_SMS_DOMAIN to this storefront's hostname and SMS_TEMPLATE_WEBOTP_APPROVED=true only after approving its exact template with the provider. IranPayamak receives code, code2, and domain attributes; MeliPayamak receives code;code and requires a successful status plus a message identifier of at least 16 digits. The MeliPayamak template itself must contain this store's hostname. Do not reuse better-buy branding or domain.

An empty SMS_PROVIDER or unavailable retains safe unavailable delivery. Incomplete provider configuration stops startup. Requests use a 12-second timeout, reject redirects, bound response decoding, and return generic delivery errors without exposing provider responses or credentials. Existing OTP expiry, attempts, rate limits, and administrator MFA remain in the auth service.

The explicit devsms build continues to use the existing loopback-only terminal sender. Use the ordinary build for real delivery. Provider contract tests use an HTTP transport double; live delivery requires this store's credentials and approved template and has not been tested.
