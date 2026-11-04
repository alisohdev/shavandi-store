-- +goose Up
CREATE TABLE shavandi.users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    phone text NOT NULL UNIQUE CHECK (phone ~ '^\+989[0-9]{9}$'),
    role text NOT NULL CHECK (role IN ('customer', 'admin')),
    password_hash text,
    active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_password_role CHECK (
        (role = 'customer' AND password_hash IS NULL) OR
        (role = 'admin' AND password_hash IS NOT NULL AND password_hash LIKE '$argon2id$v=19$%')
    )
);
CREATE UNIQUE INDEX users_single_admin_idx ON shavandi.users (role) WHERE role = 'admin';

CREATE TABLE shavandi.auth_otp_targets (
    phone text NOT NULL CHECK (phone ~ '^\+989[0-9]{9}$'),
    purpose text NOT NULL CHECK (purpose IN ('customer', 'admin')),
    requested_at timestamptz NOT NULL,
    PRIMARY KEY (phone, purpose)
);

CREATE TABLE shavandi.auth_otp_challenges (
    id_verifier bytea PRIMARY KEY CHECK (octet_length(id_verifier) = 32),
    phone text NOT NULL CHECK (phone ~ '^\+989[0-9]{9}$'),
    purpose text NOT NULL CHECK (purpose IN ('customer', 'admin')),
    user_id uuid REFERENCES shavandi.users(id),
    code_verifier bytea NOT NULL CHECK (octet_length(code_verifier) = 32),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 5),
    delivered boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    CHECK (expires_at > created_at AND expires_at <= created_at + interval '5 minutes'),
    CHECK ((purpose = 'customer' AND user_id IS NULL) OR (purpose = 'admin' AND user_id IS NOT NULL))
);
CREATE INDEX auth_otp_challenges_phone_purpose_idx ON shavandi.auth_otp_challenges (phone, purpose) WHERE consumed_at IS NULL;

CREATE TABLE shavandi.auth_sessions (
    token_verifier bytea PRIMARY KEY CHECK (octet_length(token_verifier) = 32),
    user_id uuid NOT NULL REFERENCES shavandi.users(id),
    auth_method text NOT NULL CHECK (auth_method IN ('customer_sms', 'admin_password_sms')),
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    CHECK (expires_at > created_at AND expires_at <= created_at + interval '12 hours')
);
CREATE INDEX auth_sessions_user_id_idx ON shavandi.auth_sessions (user_id);

CREATE TABLE shavandi.auth_rate_limits (
    bucket_key bytea PRIMARY KEY CHECK (octet_length(bucket_key) = 32),
    window_started_at timestamptz NOT NULL,
    attempts integer NOT NULL CHECK (attempts > 0)
);

CREATE TABLE shavandi.auth_audit_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_type text NOT NULL CHECK (event_type IN ('password_failed', 'otp_requested', 'otp_sent', 'sms_failed', 'otp_failed', 'login_succeeded', 'logout', 'rate_limited', 'admin_provisioned')),
    user_id uuid REFERENCES shavandi.users(id),
    subject_key bytea NOT NULL CHECK (octet_length(subject_key) = 32),
    ip_key bytea NOT NULL CHECK (octet_length(ip_key) = 32),
    created_at timestamptz NOT NULL
);
CREATE INDEX auth_audit_events_created_at_idx ON shavandi.auth_audit_events (created_at);

-- +goose Down
DROP TABLE shavandi.auth_audit_events;
DROP TABLE shavandi.auth_rate_limits;
DROP TABLE shavandi.auth_sessions;
DROP TABLE shavandi.auth_otp_challenges;
DROP TABLE shavandi.auth_otp_targets;
DROP TABLE shavandi.users;
