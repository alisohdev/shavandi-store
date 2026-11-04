-- name: AuthUserByPhone :one
SELECT * FROM shavandi.users WHERE phone = $1;

-- name: AuthUserByID :one
SELECT * FROM shavandi.users WHERE id = $1;

-- name: AuthCreateCustomer :one
INSERT INTO shavandi.users (phone, role) VALUES ($1, 'customer')
ON CONFLICT (phone) DO UPDATE SET phone = EXCLUDED.phone
RETURNING *;

-- name: AuthProvisionAdmin :one
INSERT INTO shavandi.users (phone, role, password_hash) VALUES ($1, 'admin', $2)
ON CONFLICT (phone) DO UPDATE SET role = 'admin', password_hash = EXCLUDED.password_hash, active = true, updated_at = now()
RETURNING *;

-- name: AuthTakeRate :one
INSERT INTO shavandi.auth_rate_limits (bucket_key, window_started_at, attempts)
VALUES (sqlc.arg(bucket_key), sqlc.arg(now_at), 1)
ON CONFLICT (bucket_key) DO UPDATE SET
    window_started_at = CASE WHEN auth_rate_limits.window_started_at + make_interval(secs => sqlc.arg(window_seconds)::double precision) <= sqlc.arg(now_at) THEN sqlc.arg(now_at) ELSE auth_rate_limits.window_started_at END,
    attempts = CASE WHEN auth_rate_limits.window_started_at + make_interval(secs => sqlc.arg(window_seconds)::double precision) <= sqlc.arg(now_at) THEN 1 ELSE LEAST(auth_rate_limits.attempts + 1, 1000000) END
RETURNING window_started_at, attempts;

-- name: AuthEnsureOTPTarget :exec
INSERT INTO shavandi.auth_otp_targets (phone, purpose, requested_at) VALUES ($1, $2, $3)
ON CONFLICT (phone, purpose) DO NOTHING;

-- name: AuthLockOTPTarget :one
SELECT requested_at FROM shavandi.auth_otp_targets WHERE phone = $1 AND purpose = $2 FOR UPDATE;

-- name: AuthTouchOTPTarget :exec
UPDATE shavandi.auth_otp_targets SET requested_at = $3 WHERE phone = $1 AND purpose = $2;

-- name: AuthInvalidateOTPs :exec
UPDATE shavandi.auth_otp_challenges SET consumed_at = $3 WHERE phone = $1 AND purpose = $2 AND consumed_at IS NULL;

-- name: AuthCreateOTP :exec
INSERT INTO shavandi.auth_otp_challenges (id_verifier, phone, purpose, user_id, code_verifier, created_at, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: AuthLockOTP :one
SELECT * FROM shavandi.auth_otp_challenges WHERE id_verifier = $1 FOR UPDATE;

-- name: AuthGetOTP :one
SELECT * FROM shavandi.auth_otp_challenges WHERE id_verifier = $1;

-- name: AuthFinishSMS :exec
UPDATE shavandi.auth_otp_challenges SET delivered = $2, consumed_at = $3 WHERE id_verifier = $1 AND consumed_at IS NULL;

-- name: AuthAttemptOTP :exec
UPDATE shavandi.auth_otp_challenges SET attempts = attempts + 1, consumed_at = $2 WHERE id_verifier = $1;

-- name: AuthCreateSession :exec
INSERT INTO shavandi.auth_sessions (token_verifier, user_id, auth_method, created_at, expires_at) VALUES ($1, $2, $3, $4, $5);

-- name: AuthSession :one
SELECT s.expires_at, s.auth_method, u.id, u.phone, u.role, u.active
FROM shavandi.auth_sessions s JOIN shavandi.users u ON u.id = s.user_id
WHERE s.token_verifier = sqlc.arg(token_verifier) AND s.revoked_at IS NULL AND s.expires_at > GREATEST(sqlc.arg(now_at)::timestamptz, statement_timestamp()) AND u.active;

-- name: AuthRevokeSession :exec
UPDATE shavandi.auth_sessions SET revoked_at = $2 WHERE token_verifier = $1 AND revoked_at IS NULL;

-- name: AuthRevokeUserSessions :exec
UPDATE shavandi.auth_sessions SET revoked_at = $2 WHERE user_id = $1 AND revoked_at IS NULL;

-- name: AuthAudit :exec
INSERT INTO shavandi.auth_audit_events (event_type, user_id, subject_key, ip_key, created_at) VALUES ($1, $2, $3, $4, $5);
