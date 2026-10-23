package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Run in a fresh process so globals and fatal exits cannot conceal a refusal.
func TestExampleProcess(t *testing.T) {
	if mode := os.Getenv("GOQUENT_EXAMPLE_TEST_MODE"); mode != "" {
		os.Exit(run(mode))
	}
	for _, tc := range []struct {
		mode   string
		code   int
		marker string
	}{
		{"plan", 0, "validated_read_diagnostic"},
		{"refuse-tenant", 3, "EXAMPLE_TENANT_REFUSED"},
		{"refuse-authorization", 4, "EXAMPLE_AUTHORIZATION_DENIED_CONSUMED"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestExampleProcess$")
			cmd.Env = append(os.Environ(), "GOQUENT_EXAMPLE_TEST_MODE="+tc.mode)
			output, err := cmd.CombinedOutput()
			code := 0
			if err != nil {
				if exit, ok := err.(*exec.ExitError); ok {
					code = exit.ExitCode()
				} else {
					t.Fatal("example process unavailable")
				}
			}
			if code != tc.code || !strings.Contains(string(output), tc.marker) {
				t.Fatal("example classification mismatch; source diagnostics omitted")
			}
			for _, secret := range []string{"SELECT ", "tenant_id", "alice@example.test", "fictional-subject", "example-only", "support ticket", "\"sql\"", "\"params\"", "PUBLIC_OUTPUT"} {
				if strings.Contains(string(output), secret) {
					t.Fatal("example public-output boundary failed")
				}
			}
		})
	}
}
