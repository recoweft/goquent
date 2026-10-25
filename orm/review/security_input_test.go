package review

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/recoweft/goquent/internal/inputjson"
)

func TestSecurityInputPlanFileBoundaries(t *testing.T) {
	const limit = 8 << 20
	p := filepath.Join(t.TempDir(), "input.json")
	source := []byte(`{"version":1,"operation":"raw","sql":"DELETE FROM fixture WHERE id = ?","params":[9007199254740993]}`)
	write := func(b []byte) {
		t.Helper()
		if os.WriteFile(p, b, 0600) != nil {
			t.Fatal("fixture write failed")
		}
	}
	write(source)
	want, err := reviewPlanJSONFile(reviewContext{}, p)
	if err != nil || len(want) == 0 {
		t.Fatal("ordinary plan fixture failed")
	}
	for _, n := range []int{limit - 1, limit, limit + 1} {
		b := bytes.Repeat([]byte(" "), n)
		copy(b, source)
		write(b)
		got, err := reviewPlanJSONFile(reviewContext{}, p)
		if n > limit {
			if !errors.Is(err, inputjson.ErrInvalid) || got != nil {
				t.Fatal("oversized plan not refused")
			}
		} else if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatal("within-limit plan findings changed")
		}
	}
}
