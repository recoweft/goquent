package tests

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"time"
)

type boundaryWriteStep struct {
	data  []byte
	reply chan boundaryWriteResult
}

type boundaryWriteResult struct {
	n   int
	err error
}

type boundaryControlledWriter chan boundaryWriteStep

func (w boundaryControlledWriter) Write(b []byte) (int, error) {
	step := boundaryWriteStep{append([]byte(nil), b...), make(chan boundaryWriteResult, 1)}
	w <- step
	r := <-step.reply
	return r.n, r.err
}

func TestBoundaryCancellationForwarding(t *testing.T) {
	for _, scenario := range []string{"complete", "short_then_complete", "error", "partial_error", "full_error", "zero", "ordinary"} {
		t.Run(scenario, func(t *testing.T) {
			p := &boundaryProxy{cancellation: make(chan struct{})}
			writer := make(boundaryControlledWriter)
			frame := []byte{0, 0, 0, 16, 4, 210, 22, 46, 0, 0, 0, 1, 0, 0, 0, 2}
			done := make(chan error, 1)
			finished := make(chan struct{})
			var pending *boundaryWriteStep
			// Drain blocked writes on assertion failure so test workers cannot leak.
			t.Cleanup(func() {
				if pending != nil {
					select {
					case pending.reply <- boundaryWriteResult{err: io.ErrClosedPipe}:
					default:
					}
				}
				for {
					select {
					case step := <-writer:
						step.reply <- boundaryWriteResult{err: io.ErrClosedPipe}
					case <-finished:
						return
					case <-time.After(3 * time.Second):
						t.Error("forwarding worker cleanup watchdog exceeded")
						return
					}
				}
			})
			// The complete frame has been received before forwarding starts.
			go func() {
				defer close(finished)
				done <- p.forwardRequest(writer, frame, scenario != "ordinary")
			}()
			next := func() boundaryWriteStep {
				t.Helper()
				select {
				case step := <-writer:
					pending = &step
					select {
					case <-p.cancellation:
						t.Fatal("cancellation published before complete successful write")
					default:
					}
					return step
				case <-time.After(3 * time.Second):
					t.Fatal("forwarding write watchdog exceeded")
					return boundaryWriteStep{}
				}
			}
			step := next()
			if !bytes.Equal(step.data, frame) {
				t.Fatal("forwarded frame mismatch")
			}
			wantErr := error(nil)
			switch scenario {
			case "short_then_complete":
				step.reply <- boundaryWriteResult{n: 5}
				step = next()
				if !bytes.Equal(step.data, frame[5:]) {
					t.Fatal("short write remainder mismatch")
				}
				step.reply <- boundaryWriteResult{n: len(frame) - 5}
			case "error", "partial_error", "full_error":
				wantErr = io.ErrClosedPipe
				n := 0
				if scenario == "partial_error" {
					n = 5
				}
				if scenario == "full_error" {
					n = len(frame)
				}
				step.reply <- boundaryWriteResult{n: n, err: wantErr}
			case "zero":
				wantErr = io.ErrShortWrite
				step.reply <- boundaryWriteResult{}
			default:
				step.reply <- boundaryWriteResult{n: len(frame)}
			}
			select {
			case err := <-done:
				if !errors.Is(err, wantErr) {
					t.Fatal("unexpected forwarding error classification")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("forwarding result watchdog exceeded")
			}
			published := false
			select {
			case <-p.cancellation:
				published = true
			default:
			}
			if published != (wantErr == nil && scenario != "ordinary") {
				t.Fatal("unexpected cancellation publication")
			}
		})
	}
}
