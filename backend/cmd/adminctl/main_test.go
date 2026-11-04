package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestProvisioningRequiresStdinAndBoundsPasswordBeforeDatabaseAccess(t *testing.T) {
	for _, test := range []struct {
		arguments []string
		password  string
	}{
		{[]string{"--phone", "09123456789"}, "unused-password"},
		{[]string{"--password-stdin"}, "unused-password"},
		{[]string{"--phone", "09123456789", "--password-stdin"}, strings.Repeat("x", 1025)},
	} {
		var output bytes.Buffer
		if err := run(test.arguments, strings.NewReader(test.password), &output); err == nil {
			t.Fatal("unsafe provisioning input accepted")
		}
		if output.Len() != 0 {
			t.Fatal("failed provisioning wrote output")
		}
	}
}
