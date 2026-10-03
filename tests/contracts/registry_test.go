package contracts_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

const repo = "../.."
const specification = "specs/goquent_ai_contracts_v3.md"

type contractCase struct {
	ID             string   `json:"id"`
	Input          string   `json:"input"`
	Current        string   `json:"current"`
	Required       string   `json:"required"`
	Owners         []string `json:"owners"`
	AnalysisLimit  string   `json:"analysis_limit"`
	Coverage       string   `json:"coverage"`
	Evidence       []string `json:"evidence"`
	AssertionScope string   `json:"assertion_scope"`
}
type apiCoverage struct {
	Section       string   `json:"section"`
	Entries       []string `json:"entries"`
	EvidenceFiles []string `json:"evidence_files"`
	Coverage      string   `json:"coverage"`
	AnalysisLimit string   `json:"analysis_limit"`
}
type registry struct {
	Baseline    string         `json:"baseline"`
	Defaults    string         `json:"defaults"`
	Cases       []contractCase `json:"cases"`
	APICoverage []apiCoverage  `json:"api_coverage"`
}

func readJSON(t *testing.T, path string, dst any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		t.Fatalf("%s: trailing JSON: %v", path, err)
	}
}
func loadRegistry(t *testing.T) registry {
	t.Helper()
	var r registry
	readJSON(t, "testdata/cases.json", &r)
	return r
}
func requireCase(t *testing.T, id string) {
	t.Helper()
	for _, c := range loadRegistry(t).Cases {
		if c.ID == id {
			return
		}
	}
	t.Fatalf("unregistered executable case %s", id)
}

// Only registration/document correspondence is checked here. A test reference
// is not proof it ran, nor proof of anything outside its assertion_scope.
func validateRegistry(r registry, document string) error {
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(r.Baseline) || r.Defaults == "" {
		return fmt.Errorf("missing baseline/defaults")
	}
	documented := map[string][]string{}
	api := map[string][]string{}
	section := ""
	for _, line := range strings.Split(document, "\n") {
		if strings.HasPrefix(line, "### 2.") {
			section = strings.Fields(line)[1]
		}
		if strings.HasPrefix(line, "## 3.") {
			section = ""
		}
		if !strings.HasPrefix(line, "| ") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		if regexp.MustCompile(`^C\d\d `).MatchString(cells[0]) {
			id, input, _ := strings.Cut(cells[0], " ")
			if _, exists := documented[id]; exists {
				return fmt.Errorf("duplicate document ID %s", id)
			}
			if len(cells) != 4 {
				return fmt.Errorf("invalid document row %s", id)
			}
			documented[id] = []string{input, cells[1], cells[2], cells[3]}
		}
		if section != "" && !strings.HasPrefix(cells[0], "---") && cells[0] != "Public entry" && cells[0] != "Surface" {
			api[section] = append(api[section], cells[0])
		}
	}
	if len(r.Cases) != 28 || len(documented) != 28 {
		return fmt.Errorf("expected C01-C28 in registry and document")
	}
	seen := map[string]bool{}
	for _, c := range r.Cases {
		if seen[c.ID] {
			return fmt.Errorf("duplicate case %s", c.ID)
		}
		seen[c.ID] = true
		fields := []string{c.Input, c.Current, c.Required, c.AnalysisLimit}
		if !reflect.DeepEqual(fields, documented[c.ID]) {
			return fmt.Errorf("%s differs from specification", c.ID)
		}
		for _, field := range append(fields, c.AssertionScope) {
			if strings.TrimSpace(field) == "" {
				return fmt.Errorf("%s missing required field", c.ID)
			}
		}
		ownerMatch := regexp.MustCompile(`\((\d\d(?:/\d\d)*)\)`).FindStringSubmatch(c.Required)
		if len(ownerMatch) != 2 {
			return fmt.Errorf("%s missing owners in specification", c.ID)
		}
		var owners []string
		for _, owner := range strings.Split(ownerMatch[1], "/") {
			owners = append(owners, "GQ-AI-"+owner)
		}
		if !reflect.DeepEqual(c.Owners, owners) {
			return fmt.Errorf("%s owner mismatch", c.ID)
		}
		switch c.Coverage {
		case "documentary":
			if len(c.Evidence) != 0 {
				return fmt.Errorf("%s documentary case has executable mapping", c.ID)
			}
		case "partial", "executable":
			if len(c.Evidence) == 0 {
				return fmt.Errorf("%s missing executable mapping", c.ID)
			}
		default:
			return fmt.Errorf("%s invalid coverage %q", c.ID, c.Coverage)
		}
		unique := map[string]bool{}
		for _, ref := range c.Evidence {
			if unique[ref] {
				return fmt.Errorf("%s duplicate evidence %s", c.ID, ref)
			}
			unique[ref] = true
			path, name, ok := strings.Cut(ref, "#")
			if !ok || !strings.HasPrefix(name, "Test") {
				return fmt.Errorf("invalid evidence %s", ref)
			}
			file, err := parseTestFile(path)
			if err != nil {
				return err
			}
			found := false
			for _, decl := range file.Decls {
				if f, ok := decl.(*ast.FuncDecl); ok && f.Recv == nil && f.Name.Name == name {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("missing test %s", ref)
			}
		}
	}
	for i := 1; i <= 28; i++ {
		if !seen[fmt.Sprintf("C%02d", i)] {
			return fmt.Errorf("missing C%02d", i)
		}
	}
	if len(r.APICoverage) != len(api) {
		return fmt.Errorf("API section count differs")
	}
	seen = map[string]bool{}
	for _, area := range r.APICoverage {
		if seen[area.Section] || !reflect.DeepEqual(area.Entries, api[area.Section]) {
			return fmt.Errorf("API inventory mismatch %s", area.Section)
		}
		seen[area.Section] = true
		if area.Coverage != "partial" || area.AnalysisLimit == "" || len(area.EvidenceFiles) == 0 {
			return fmt.Errorf("API scope missing %s", area.Section)
		}
		for _, path := range area.EvidenceFiles {
			if _, err := parseTestFile(path); err != nil {
				return err
			}
		}
	}
	return nil
}
func parseTestFile(path string) (*ast.File, error) {
	if !filepath.IsLocal(path) || !strings.HasSuffix(path, "_test.go") {
		return nil, fmt.Errorf("invalid test path %q", path)
	}
	return parser.ParseFile(token.NewFileSet(), filepath.Join(repo, path), nil, 0)
}
func TestSharedCaseRegistry(t *testing.T) {
	doc, err := os.ReadFile(filepath.Join(repo, specification))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateRegistry(loadRegistry(t), string(doc)); err != nil {
		t.Fatal(err)
	}
}
func TestRegistryRejectsBrokenCorrespondence(t *testing.T) {
	doc, err := os.ReadFile(filepath.Join(repo, specification))
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*registry){
		"missing":       func(r *registry) { r.Cases = r.Cases[1:] },
		"duplicate":     func(r *registry) { r.Cases[1] = r.Cases[0] },
		"unknown ID":    func(r *registry) { r.Cases[0].ID = "C99" },
		"verdict drift": func(r *registry) { r.Cases[0].Current = "approved" },
		"wrong owner":   func(r *registry) { r.Cases[0].Owners = []string{"GQ-AI-99"} },
		"missing test":  func(r *registry) { r.Cases[0].Evidence = []string{"orm/query/plan_test.go#TestDoesNotExist"} },
		"bad status":    func(r *registry) { r.Cases[0].Coverage = "safe" },
		"wrong API":     func(r *registry) { r.APICoverage[0].Entries[0] = "StrictPlan" },
	} {
		t.Run(name, func(t *testing.T) {
			r := loadRegistry(t)
			mutate(&r)
			if validateRegistry(r, string(doc)) == nil {
				t.Fatal("invalid registration accepted")
			}
		})
	}
}
