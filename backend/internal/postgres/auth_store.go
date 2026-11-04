package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shavandi/store/backend/internal/auth"
	"github.com/shavandi/store/backend/internal/dbgen"
)

type AuthStore struct{ pool *pgxpool.Pool }
type authTx struct{ queries *dbgen.Queries }

func NewAuthStore(pool *pgxpool.Pool) *AuthStore { return &AuthStore{pool: pool} }

func (s *AuthStore) WithTx(ctx context.Context, action func(auth.Tx) error) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_ = tx.Rollback(rollbackCtx)
	}()
	if err := action(&authTx{queries: dbgen.New(tx)}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func authError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.ErrNotFound
	}
	return err
}
func timestamp(at time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: at, Valid: true} }
func nullableTimestamp(at *time.Time) pgtype.Timestamptz {
	if at == nil {
		return pgtype.Timestamptz{}
	}
	return timestamp(*at)
}
func authUUID(id string) pgtype.UUID { var value pgtype.UUID; _ = value.Scan(id); return value }
func authUser(user dbgen.ShavandiUser) auth.User {
	return auth.User{ID: uuidString(user.ID), Phone: user.Phone, Role: user.Role, PasswordHash: user.PasswordHash.String, Active: user.Active}
}

func (tx *authTx) UserByPhone(ctx context.Context, phone string) (auth.User, error) {
	user, err := tx.queries.AuthUserByPhone(ctx, phone)
	return authUser(user), authError(err)
}
func (tx *authTx) UserByID(ctx context.Context, id string) (auth.User, error) {
	user, err := tx.queries.AuthUserByID(ctx, authUUID(id))
	return authUser(user), authError(err)
}
func (tx *authTx) CreateCustomer(ctx context.Context, phone string) (auth.User, error) {
	user, err := tx.queries.AuthCreateCustomer(ctx, phone)
	return authUser(user), authError(err)
}
func (tx *authTx) ProvisionAdmin(ctx context.Context, phone, hash string) (auth.User, error) {
	user, err := tx.queries.AuthProvisionAdmin(ctx, dbgen.AuthProvisionAdminParams{Phone: phone, PasswordHash: pgtype.Text{String: hash, Valid: true}})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "users_single_admin_idx" {
		return auth.User{}, auth.ErrAdminExists
	}
	return authUser(user), authError(err)
}
func (tx *authTx) TakeRate(ctx context.Context, key []byte, now time.Time, window time.Duration) (time.Time, int32, error) {
	rate, err := tx.queries.AuthTakeRate(ctx, dbgen.AuthTakeRateParams{BucketKey: key, NowAt: timestamp(now), WindowSeconds: window.Seconds()})
	return rate.WindowStartedAt.Time, rate.Attempts, err
}
func (tx *authTx) LockTarget(ctx context.Context, phone, purpose string, now time.Time) (time.Time, error) {
	if err := tx.queries.AuthEnsureOTPTarget(ctx, dbgen.AuthEnsureOTPTargetParams{Phone: phone, Purpose: purpose, RequestedAt: timestamp(now.Add(-time.Hour))}); err != nil {
		return time.Time{}, err
	}
	at, err := tx.queries.AuthLockOTPTarget(ctx, dbgen.AuthLockOTPTargetParams{Phone: phone, Purpose: purpose})
	return at.Time, err
}
func (tx *authTx) TouchTarget(ctx context.Context, phone, purpose string, at time.Time) error {
	return tx.queries.AuthTouchOTPTarget(ctx, dbgen.AuthTouchOTPTargetParams{Phone: phone, Purpose: purpose, RequestedAt: timestamp(at)})
}
func (tx *authTx) InvalidateOTPs(ctx context.Context, phone, purpose string, at time.Time) error {
	return tx.queries.AuthInvalidateOTPs(ctx, dbgen.AuthInvalidateOTPsParams{Phone: phone, Purpose: purpose, ConsumedAt: timestamp(at)})
}
func (tx *authTx) CreateOTP(ctx context.Context, otp auth.OTP, now time.Time) error {
	return tx.queries.AuthCreateOTP(ctx, dbgen.AuthCreateOTPParams{IDVerifier: otp.IDVerifier, Phone: otp.Phone, Purpose: otp.Purpose, UserID: authUUID(otp.UserID), CodeVerifier: otp.CodeVerifier, CreatedAt: timestamp(now), ExpiresAt: timestamp(otp.ExpiresAt)})
}
func (tx *authTx) LockOTP(ctx context.Context, key []byte, now time.Time) (auth.OTP, error) {
	// The target lock is shared by resend, verify, and trusted role/password changes.
	candidate, err := tx.queries.AuthGetOTP(ctx, key)
	if err != nil {
		return auth.OTP{}, authError(err)
	}
	if _, err := tx.LockTarget(ctx, candidate.Phone, candidate.Purpose, now); err != nil {
		return auth.OTP{}, err
	}
	row, err := tx.queries.AuthLockOTP(ctx, key)
	if err != nil {
		return auth.OTP{}, authError(err)
	}
	otp := auth.OTP{IDVerifier: row.IDVerifier, Phone: row.Phone, Purpose: row.Purpose, UserID: uuidString(row.UserID), CodeVerifier: row.CodeVerifier, Attempts: row.Attempts, Delivered: row.Delivered, ExpiresAt: row.ExpiresAt.Time}
	if row.ConsumedAt.Valid {
		otp.ConsumedAt = &row.ConsumedAt.Time
	}
	return otp, nil
}
func (tx *authTx) FinishSMS(ctx context.Context, key []byte, delivered bool, consumed *time.Time) error {
	return tx.queries.AuthFinishSMS(ctx, dbgen.AuthFinishSMSParams{IDVerifier: key, Delivered: delivered, ConsumedAt: nullableTimestamp(consumed)})
}
func (tx *authTx) AttemptOTP(ctx context.Context, key []byte, consumed *time.Time) error {
	return tx.queries.AuthAttemptOTP(ctx, dbgen.AuthAttemptOTPParams{IDVerifier: key, ConsumedAt: nullableTimestamp(consumed)})
}
func (tx *authTx) CreateSession(ctx context.Context, key []byte, session auth.Session, now time.Time) error {
	return tx.queries.AuthCreateSession(ctx, dbgen.AuthCreateSessionParams{TokenVerifier: key, UserID: authUUID(session.User.ID), AuthMethod: session.Method, CreatedAt: timestamp(now), ExpiresAt: timestamp(session.ExpiresAt)})
}
func (tx *authTx) RevokeSession(ctx context.Context, key []byte, now time.Time) error {
	return tx.queries.AuthRevokeSession(ctx, dbgen.AuthRevokeSessionParams{TokenVerifier: key, RevokedAt: timestamp(now)})
}
func (tx *authTx) RevokeUserSessions(ctx context.Context, id string, now time.Time) error {
	return tx.queries.AuthRevokeUserSessions(ctx, dbgen.AuthRevokeUserSessionsParams{UserID: authUUID(id), RevokedAt: timestamp(now)})
}
func (tx *authTx) Audit(ctx context.Context, event auth.Audit) error {
	return tx.queries.AuthAudit(ctx, dbgen.AuthAuditParams{EventType: event.Event, UserID: authUUID(event.UserID), SubjectKey: event.SubjectKey, IpKey: event.IPKey, CreatedAt: timestamp(event.At)})
}
func (s *AuthStore) Session(ctx context.Context, key []byte, now time.Time) (auth.Session, error) {
	row, err := dbgen.New(s.pool).AuthSession(ctx, dbgen.AuthSessionParams{TokenVerifier: key, NowAt: timestamp(now)})
	if err != nil {
		return auth.Session{}, authError(err)
	}
	return auth.Session{User: auth.User{ID: uuidString(row.ID), Phone: row.Phone, Role: row.Role, Active: row.Active}, ExpiresAt: row.ExpiresAt.Time, Method: row.AuthMethod}, nil
}
