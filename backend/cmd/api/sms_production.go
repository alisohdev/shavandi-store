//go:build !devsms

package main

import (
	"io"

	"github.com/shavandi/store/backend/internal/auth"
)

func newSMSSender(_, _ string, _ io.Writer) (auth.SMS, error) { return auth.SMSFromEnv() }
