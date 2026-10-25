package mcp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/publicoutput"
)

type auditReadError struct{ calls *int }

func (e auditReadError) Error() string          { *e.calls++; return publicCanary }
func (e auditReadError) Unwrap() error          { *e.calls++; panic("unexpected unwrap") }
func (e auditReadError) Format(fmt.State, rune) { *e.calls++; panic("unexpected format") }

type auditErrorReader struct{ cause error }

func (r auditErrorReader) Read([]byte) (int, error) { return 0, r.cause }

func TestSecurityOutputMCPSourceErrorBoundary(t *testing.T) {
	// Cover no bytes, a partial line, a partial header, and a partial framed body.
	for _, prefix := range []string{"", "{", "Content-Length: 10\r\n", "Content-Length: 10\r\n\r\n{"} {
		calls := 0
		cause := auditReadError{&calls}
		var out bytes.Buffer
		err := NewServer(Options{}).Serve(context.Background(), io.MultiReader(strings.NewReader(prefix), auditErrorReader{cause}), &out)
		if err != publicoutput.ErrOutput || errors.Unwrap(err) != nil || errors.Is(err, cause) || out.Len() != 0 {
			t.Fatal("source failure boundary changed")
		}
		assertPublic(t, fmt.Sprintf("%v %+v %#v", err, err, err))
		if calls != 0 {
			t.Fatal("source error evaluated")
		}
	}
	var out bytes.Buffer
	if NewServer(Options{}).Serve(context.Background(), strings.NewReader(""), &out) != nil || out.Len() != 0 {
		t.Fatal("normal EOF changed")
	}
}
