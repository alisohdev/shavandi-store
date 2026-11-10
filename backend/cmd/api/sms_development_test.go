//go:build devsms

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestDevelopmentSMSRequiresDevelopmentAndLoopback(t *testing.T) {
	for _, tc := range []struct{ env, addr string }{{"production", "127.0.0.1:8080"}, {"development", "0.0.0.0:8080"}, {"development", "192.0.2.1:8080"}} {
		if _, err := newSMSSender(tc.env, tc.addr, &bytes.Buffer{}); err == nil {
			t.Fatalf("accepted env=%s addr=%s", tc.env, tc.addr)
		}
	}
	var output bytes.Buffer
	s, err := newSMSSender("development", "127.0.0.1:8080", &output)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SendOTP(context.Background(), "+989120000000", "583194"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "583194") {
		t.Fatal("OTP not written to local terminal output")
	}
}
