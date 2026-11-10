# M1b-C local development authentication

The ordinary API build always wires `auth.UnavailableSMS`. A terminal SMS sender is compiled only with the explicit `devsms` Go build tag (`cmd/api/sms_development.go`). That build refuses to start the sender unless `APP_ENV=development` and `HTTP_ADDR` names a loopback listener. It writes generated OTP messages to stderr; the existing auth service still generates and verifies codes, stores only verifiers, enforces expiry/attempt/resend limits, and issues the normal MFA session. There is no OTP HTTP endpoint.

## Windows PowerShell walkthrough

Start local PostgreSQL and apply the existing migrations from `backend` after setting `DATABASE_URL`:

```powershell
$env:DATABASE_URL = 'postgres://<local-user>@127.0.0.1:5432/shavandi?sslmode=disable'
go run github.com/pressly/goose/v3/cmd/goose@v3.29.0 -dir db/migrations postgres $env:DATABASE_URL up
```

Set development auth configuration. Keep the generated secret stable while the local API is running:

```powershell
$env:APP_ENV = 'development'
$env:HTTP_ADDR = '127.0.0.1:8080'
$env:AUTH_COOKIE_SECURE = 'false'
$authBytes = [byte[]]::new(32)
$rng = [Security.Cryptography.RandomNumberGenerator]::Create()
try { $rng.GetBytes($authBytes) } finally { $rng.Dispose() }
$env:AUTH_SECRET = [Convert]::ToBase64String($authBytes)
```

Provision Sajad using the existing `adminctl`. The password is prompted as a secure string and piped through stdin, never supplied as a command argument:

```powershell
$phone = Read-Host 'Sajad phone number'
$securePassword = Read-Host 'New administrator password' -AsSecureString
$passwordPointer = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($securePassword)
try {
  $password = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($passwordPointer)
  $password | go run ./cmd/adminctl --phone $phone --password-stdin
} finally {
  [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($passwordPointer)
  Remove-Variable password, securePassword -ErrorAction SilentlyContinue
}
```

In one terminal, start only the development-tagged, loopback API:

```powershell
go run -tags devsms ./cmd/api
```

In another terminal, start Astro with `bun run dev`. Its existing `/api` proxy targets `127.0.0.1:8080`; open the local admin login page, enter Sajad's phone and password, then enter the one-time code printed in the API terminal. The browser uses the normal development session/CSRF cookies. Admin APIs still require the persisted admin role and password-plus-OTP assurance. Use the dashboard logout action to revoke the session; the API's `POST /api/v1/auth/logout` route also revokes sessions and requires the usual CSRF token.

## Production isolation

Build production without `-tags devsms`; that binary contains only `UnavailableSMS` and safely returns `sms_unavailable` when no real provider is configured. The development sender is absent from production builds. Even a dev-tagged binary cannot activate terminal delivery with production `APP_ENV` or a non-loopback `HTTP_ADDR`. Never expose a dev-tagged binary to a network interface.

Focused Go auth tests, normal production build/tests, and `-tags devsms` sender gate tests should be run independently. A complete account walkthrough requires local PostgreSQL and Sajad's phone/password; no administrator was provisioned by this implementation.
