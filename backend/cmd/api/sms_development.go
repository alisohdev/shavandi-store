//go:build devsms

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/shavandi/store/backend/internal/auth"
)

type terminalSMS struct {
	output io.Writer
	mu     sync.Mutex
}

func newSMSSender(environment, address string, output io.Writer) (auth.SMS, error) {
	if environment != "development" {
		return nil, errors.New("development SMS requires APP_ENV=development")
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, errors.New("development SMS requires a loopback HTTP_ADDR")
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return nil, errors.New("development SMS requires a loopback HTTP_ADDR")
	}
	if output == nil {
		return nil, errors.New("development SMS terminal output is unavailable")
	}
	return &terminalSMS{output: output}, nil
}

func (s *terminalSMS) SendOTP(_ context.Context, phone, code string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := fmt.Fprintf(s.output, "\n[DEVELOPMENT SMS] OTP for %s: %s\n\n", phone, code)
	return err
}
