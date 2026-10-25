package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
)

type auditCLIReadError struct{ calls *int }

func (e auditCLIReadError) Error() string          { *e.calls++; return cliCanary }
func (e auditCLIReadError) Unwrap() error          { *e.calls++; panic("unexpected unwrap") }
func (e auditCLIReadError) Format(fmt.State, rune) { *e.calls++; panic("unexpected format") }

type auditCLIReader struct{ cause error }

func (r auditCLIReader) Read([]byte) (int, error) { return 0, r.cause }

func TestSecurityOutputCLIMCPReadFailure(t *testing.T) {
	for _, prefix := range []string{"", "{", "Content-Length: 10\r\n\r\n{"} {
		calls := 0
		var out, stderr bytes.Buffer
		input := io.MultiReader(strings.NewReader(prefix), auditCLIReader{auditCLIReadError{&calls}})
		if runMCP(nil, input, &out, &stderr) != 2 || out.Len() != 0 || !strings.Contains(stderr.String(), "PUBLIC_OUTPUT") {
			t.Fatal("read failure lost")
		}
		checkCLIOutput(t, &out, &stderr)
		if calls != 0 {
			t.Fatal("CLI evaluated source error")
		}
	}
}
