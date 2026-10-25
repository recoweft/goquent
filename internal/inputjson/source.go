package inputjson

import (
	"io"
	"os"
)

// MaxSourceBytes is the raw file budget for legacy schema, policy, manifest,
// review configuration and review plan loaders. It is not a decoded-memory or
// aggregate budget and does not change their format-specific decoding rules.
const MaxSourceBytes = 8 << 20

// ReadSourceFile bounds actual reads, including whitespace, before JSON decoding.
// It does not impose a deadline on a blocking file or promise filesystem isolation.
func ReadSourceFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readSource(f)
}

func readSource(r io.Reader) ([]byte, error) {
	b, err := io.ReadAll(io.LimitReader(r, MaxSourceBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > MaxSourceBytes {
		return nil, ErrInvalid
	}
	return b, nil
}
