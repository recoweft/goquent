package review

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/analysis"
	"github.com/recoweft/goquent/orm/query"
)

// These source counterexamples test the design premises, not a new classifier.
// Every fixture is compiled independently of review's loader and every expected
// outcome is specified by the case, not calculated from comparison predicates.
func TestBoundedSelectIdentitySourceCounterexamples(t *testing.T) {
	const source = "package fixture\nimport \"github.com/recoweft/goquent/orm\"\nfunc sample(db *orm.DB) { var rows []map[string]any; _ = db.Table(\"PRIVATE_TABLE_CANARY\").Select(\"*\").Where(\"id\", 1).Get(&rows) }\n"
	const dependency = "package fixture\nconst bound = 1\n"
	for _, mode := range []string{"same", "document", "dependency", "build_selection", "moved", "second_location"} {
		t.Run(mode, func(t *testing.T) {
			base, head := ciFixture(t)
			old := map[string]string{"sample.go": source, "value.go": dependency}
			next := map[string]string{"sample.go": source, "value.go": dependency}
			switch mode {
			case "document":
				next["note.txt"] = "bounded context edit\n"
			case "dependency":
				next["value.go"] = strings.Replace(dependency, "= 1", "= 2", 1)
			case "build_selection":
				// Same terminal and constant value; a different selected file set.
				next["value.go"] = "//go:build ignore\n\n" + dependency
				next["alternate.go"] = dependency
			case "moved":
				next["sample.go"] = strings.Replace(source, "func sample", "\nfunc sample", 1)
			case "second_location":
				next["sample.go"] = source + strings.Replace(strings.Split(source, "\n")[2], "sample", "second", 1) + "\n"
			}
			for root, files := range map[string]map[string]string{base: old, head: next} {
				for name, content := range files {
					if os.WriteFile(filepath.Join(root, name), []byte(content), 0600) != nil {
						t.Fatal("fixture write")
					}
				}
				cmd := exec.Command("go", "test", "-run", "^$", ".")
				cmd.Dir = root
				if err := cmd.Run(); err != nil {
					t.Fatal("independent fixture compilation failed")
				}
			}
			ci := CIOptions{Root: head, BaseRoot: base, Diff: changedFixturePatch(old, next), FailOn: query.RiskMedium, FailOnPrecision: query.AnalysisPartial}
			r, tr := observedCI(t, Options{}, ci)
			if r.invalid || !r.risk || !r.precision || r.ExitCode() != 1 {
				t.Fatal("fixture input or full gates changed")
			}
			count := 1
			if mode == "second_location" {
				count = 2
			}
			if len(r.selected.privateOperations) != len(tr.headReport.privateOperations) {
				t.Fatal("operation coverage lost")
			}
			selects := 0
			for _, op := range tr.headReport.privateOperations {
				if op.operation != query.OperationSelect {
					continue
				}
				i := selects
				selects++
				line := 3 + i
				if mode == "moved" {
					line = 4
				}
				lines := strings.Split(next["sample.go"], "\n")
				if op.operation != query.OperationSelect || op.Location.Line != line || op.Location.Column != strings.Index(lines[line-1], "Get(&rows)")+1 {
					t.Fatal("physical terminal identity differs from source expectation")
				}
				prefix := strings.Join(lines[:line-1], "\n") + "\n"
				if op.Source.Offset != len(prefix)+strings.Index(lines[line-1], "db.Table") || op.Source.File != "sample.go" || op.Source.Version == "" {
					t.Fatal("source reference does not identify the chain start")
				}
				if op.Detail.SQL != analysis.Known || op.Detail.Columns != analysis.Known || op.Detail.Bindings != analysis.Known || op.Detail.Policy == analysis.Known || op.Correspondence.Source == analysis.Matched {
					t.Fatal("static reconstruction confused with policy or runtime proof")
				}
			}
			if selects != count {
				t.Fatal("SELECT operation population differs")
			}
			want := "unknown"
			if mode == "same" {
				want = "unchanged"
			} else if mode == "moved" || mode == "second_location" {
				want = "new"
			}
			stars := 0
			keys := map[string]bool{}
			for i, f := range tr.head {
				if f.f.Code != query.WarningSelectStarUsed {
					continue
				}
				stars++
				if !f.known || f.operation != "select" || tr.states[i] != want || keys[f.key()] {
					t.Fatal("independent identity/context counterexample failed")
				}
				keys[f.key()] = true
			}
			if stars != count {
				t.Fatal("missing diagnostic control")
			}
			t.Logf("buildable=true select_operations=%d select_star_state=%s input_failed=false run_ci_exit=1", count, want)
		})
	}
}
