package payments

import (
	"context"
	"errors"
)

var ErrUnavailable = errors.New("payment service unavailable")
var ErrNotPaid = errors.New("payment not verified")
var ErrConflict = errors.New("payment state conflict")
var ErrNotFound = errors.New("payment not found")
var ErrInvalid = errors.New("invalid payment request")

type Attempt struct {
	ID, OrderID, Authority, Status, Environment, MerchantFingerprint string
	AmountRial                                                       int64
	RefID                                                            *int64
}
type Store interface {
	Begin(context.Context, string, string, int64, int64, string, string) (Attempt, bool, error)
	Bind(context.Context, string, string) error
	RequestFailed(context.Context, string, bool) error
	ByAuthority(context.Context, string) (Attempt, error)
	ByOrder(context.Context, string, string) (Attempt, error)
	Claim(context.Context, string) (bool, error)
	ReleaseClaim(context.Context, string) error
	Settle(context.Context, string, int64) error
}
type Gateway interface {
	Request(context.Context, int64, string) (string, error)
	Verify(context.Context, int64, string) (int64, error)
	Redirect(string) string
}
type Service struct {
	Store                            Store
	Gateway                          Gateway
	Environment, MerchantFingerprint string
}

func (s *Service) Start(ctx context.Context, user, order string, version, total int64) (string, error) {
	if s == nil || s.Gateway == nil {
		return "", ErrUnavailable
	}
	attempt, fresh, err := s.Store.Begin(ctx, user, order, version, total, s.Environment, s.MerchantFingerprint)
	if err != nil {
		return "", err
	}
	if !fresh {
		if attempt.Status != "ready" || attempt.Authority == "" {
			return "", ErrConflict
		}
		return s.Gateway.Redirect(attempt.Authority), nil
	}
	authority, err := s.Gateway.Request(ctx, attempt.AmountRial, attempt.OrderID)
	if err != nil {
		_ = s.Store.RequestFailed(ctx, attempt.ID, !errors.Is(err, ErrNotPaid))
		return "", ErrUnavailable
	}
	if err = s.Store.Bind(ctx, attempt.ID, authority); err != nil {
		return "", ErrUnavailable
	}
	return s.Gateway.Redirect(authority), nil
}
func (s *Service) CheckOwner(ctx context.Context, user, order string) error {
	if s == nil || s.Gateway == nil {
		return ErrUnavailable
	}
	attempt, err := s.Store.ByOrder(ctx, user, order)
	if err != nil {
		return err
	}
	return s.check(ctx, attempt)
}
func (s *Service) Callback(ctx context.Context, authority string) error {
	if s == nil || s.Gateway == nil {
		return ErrUnavailable
	}
	attempt, err := s.Store.ByAuthority(ctx, authority)
	if err != nil {
		return err
	}
	return s.check(ctx, attempt)
}
func (s *Service) check(ctx context.Context, attempt Attempt) error {
	if attempt.Environment != s.Environment || attempt.MerchantFingerprint != s.MerchantFingerprint {
		return ErrUnavailable
	}
	if attempt.Status == "succeeded" {
		return nil
	}
	if attempt.Status != "ready" || attempt.Authority == "" {
		return ErrConflict
	}
	claimed, err := s.Store.Claim(ctx, attempt.ID)
	if err != nil {
		return err
	}
	if !claimed {
		return ErrConflict
	}
	ref, err := s.Gateway.Verify(ctx, attempt.AmountRial, attempt.Authority)
	if err != nil {
		_ = s.Store.ReleaseClaim(ctx, attempt.ID)
		return err
	}
	if ref <= 0 {
		_ = s.Store.ReleaseClaim(ctx, attempt.ID)
		return ErrUnavailable
	}
	return s.Store.Settle(ctx, attempt.ID, ref)
}
