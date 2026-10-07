package driver

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

type loggerBomb struct{}

func (loggerBomb) String() string { panic("unexpected String") }
func (loggerBomb) Error() string  { panic("unexpected Error") }
func TestPublicDriverLoggerDoesNotFormatPayload(t *testing.T) {
	var out bytes.Buffer
	old := log.Writer()
	log.SetOutput(&out)
	defer log.SetOutput(old)
	publicDriverLogger{}.Print("driver-secret@example.test", loggerBomb{})
	if strings.Contains(out.String(), "driver-secret") || !strings.Contains(out.String(), "details omitted") {
		t.Fatal("unsafe driver log")
	}
}
