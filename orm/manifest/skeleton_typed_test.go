package manifest_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/tests/typedfixture"
)

func TestTypedRepositoryActualCompile(t *testing.T) {
	root, e := filepath.Abs("../..")
	if e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	gomod := "module fixturecompile\n\ngo 1.25.0\n\nrequire github.com/recoweft/goquent v0.0.0\nreplace github.com/recoweft/goquent => " + filepath.ToSlash(root) + "\n"
	if e = os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0600); e != nil {
		t.Fatal(e)
	}
	for _, d := range []string{"mysql", "postgres"} {
		src, err := manifest.GenerateRepositorySkeleton(typedfixture.Manifest(d), typedfixture.Options(d))
		if err != nil {
			t.Fatal("generation refused")
		}
		again, err := manifest.GenerateRepositorySkeleton(typedfixture.Manifest(d), typedfixture.Options(d))
		if err != nil || !bytes.Equal(src, again) {
			t.Fatal("nondeterministic source")
		}
		checked, err := os.ReadFile(filepath.Join(root, "tests/typedfixture", d+"_generated.go"))
		if err != nil || !bytes.Equal(src, checked) {
			t.Fatal("fictional fixture source differs")
		}
		if err = os.WriteFile(filepath.Join(dir, d+".go"), src, 0600); err != nil {
			t.Fatal(err)
		}
	}
	positive := `package typedfixture
import "context"
func positive(){
 c:=MySQLRecordColumns()
 _=c.ID.Eq(MySQLRecordIDKey(1))
 _=c.Status.In(MySQLRecordStatusValueChoice1)
 _=MySQLRecordKey{ID:MySQLRecordIDKey(0),Segment:MySQLRecordSegmentKey(0)}
 var r *MySQLRecordRepository
 _,_,_=r.PlanIdentity(context.Background(),MySQLRecordRead{Filters:[]MySQLRecordPredicate{c.ID.Eq(1)},OrderBy:[]MySQLRecordOrder{c.ID.Desc()}})
}
`
	path := filepath.Join(dir, "calls.go")
	compile := func(source string) bool {
		t.Helper()
		if err := os.WriteFile(path, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("go", "test", "-mod=mod", "-run=^$", ".")
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOWORK=off")
		_, err := cmd.CombinedOutput()
		return err == nil
	}
	if !compile(positive) {
		t.Fatal("positive generated Go compilation failed; source retained only in local test directory")
	}
	negatives := []string{
		`_ = MySQLRecordColumns().ID.Eq(PostgresRecordIDKey(1))`,
		`_ = MySQLRecordColumns().ID.Eq(int64(1))`,
		`_ = MySQLRecordColumns().ID.Eq(MySQLRecordSegmentKey(1))`,
		`_ = MySQLRecordKey{ID:MySQLRecordSegmentKey(1)}`,
		`_ = MySQLRecordRead{Filters:[]MySQLRecordPredicate{PostgresRecordColumns().ID.Eq(1)}}`,
		`_ = MySQLRecordRead{OrderBy:[]MySQLRecordOrder{PostgresRecordColumns().ID.Asc()}}`,
		`_ = MySQLRecordColumns().Status.Eq(PostgresRecordStatusValueChoice1)`,
		`_ = MySQLRecordColumns().Active.Eq("true")`,
	}
	for i, n := range negatives {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			if compile("package typedfixture\nfunc negative(){" + n + "}\n") {
				t.Fatal("type mismatch compiled")
			}
		})
	}
	// Exercise renamed fields, import collisions, keywords and a single nominal key.
	odd := manifest.Table{Name: "odd", Columns: []manifest.Column{{Name: "table_name", Type: "bigint", TypeSource: "sql", NullableKnown: true, Primary: true}, {Name: "string", Type: "text", TypeSource: "sql"}, {Name: "_", Type: "boolean", TypeSource: "sql"}, {Name: "a_b", Type: "text", TypeSource: "sql"}, {Name: "a__b", Type: "text", TypeSource: "sql"}, {Name: "type", Type: "date", TypeSource: "sql", NullableKnown: true}}}
	src, err := manifest.GenerateRepositorySkeletonForTable(odd, manifest.RepositorySkeletonOptions{Typed: true, Dialect: "postgres", PackageName: "typedfixture", RowTypeName: "OddRow", Projections: []manifest.RepositoryProjection{{Name: "Odd", Columns: []string{"type", "table_name"}}}})
	if err != nil {
		t.Fatal("odd names refused")
	}
	if err = os.WriteFile(filepath.Join(dir, "odd.go"), src, 0600); err != nil {
		t.Fatal(err)
	}
	if !compile(positive) {
		t.Fatal("collision and keyword fixture did not compile")
	}
}

func TestTypedRepositoryUnknownAndRefusals(t *testing.T) {
	m := typedfixture.Manifest("postgres")
	opts := typedfixture.Options("postgres")
	m.Tables[0].Columns = append(m.Tables[0].Columns, manifest.Column{Name: "mystery", Type: "int8", Primary: true}, manifest.Column{Name: "go_decl", Type: "int8", TypeSource: "go"}, manifest.Column{Name: "native_enum", Type: "custom_enum", TypeSource: "sql"}, manifest.Column{Name: "array_col", Type: "integer[]", TypeSource: "sql"})
	src, err := manifest.GenerateRepositorySkeleton(m, opts)
	if err != nil {
		t.Fatal("unknown model representation refused")
	}
	if strings.Contains(string(src), "func (c PostgresRecordMysteryColumn) Eq") || strings.Contains(string(src), "func (r *PostgresRecordRepository) FindByKey") {
		t.Fatal("unknown PK gained a typed guarantee")
	}
	for _, tc := range []string{"dialect", "projection_empty", "projection_unknown", "projection_forbidden", "projection_duplicate", "projection_collision", "identifier", "import_collision", "version"} {
		t.Run(tc, func(t *testing.T) {
			m := typedfixture.Manifest("postgres")
			o := typedfixture.Options("postgres")
			switch tc {
			case "dialect":
				o.Dialect = "mysql"
			case "projection_empty":
				o.Projections[0].Columns = nil
			case "projection_unknown":
				o.Projections[0].Columns = []string{"missing"}
			case "projection_forbidden":
				o.Projections[0].Columns = []string{"hidden"}
			case "projection_duplicate":
				o.Projections[0].Columns = []string{"id", "id"}
			case "projection_collision":
				o.Projections = append(o.Projections, o.Projections[0])
			case "identifier":
				m.Tables[0].Columns[0].Name = "id; payload"
			case "import_collision":
				o.RowTypeName = "orm"
			case "version":
				m.Version = "99"
			}
			_, err := manifest.GenerateRepositorySkeleton(m, o)
			if !errors.Is(err, manifest.ErrRepositoryGeneration) {
				t.Fatal("expected fixed refusal")
			}
			if strings.Contains(fmt.Sprintf("%+v %#v", err, err), "payload") {
				t.Fatal("error leaked source")
			}
		})
	}
	if _, err = manifest.GenerateRepositorySkeletonForTable(m.Tables[0], manifest.RepositorySkeletonOptions{Typed: true}); !errors.Is(err, manifest.ErrRepositoryGeneration) {
		t.Fatal("missing dialect accepted")
	}
}

type readonlyModel struct {
	ID     int64  `db:"id,pk"`
	Server string `db:"server,readonly,generated"`
}

func (readonlyModel) TableName() string { return "readonly_fixture" }
func TestReadonlyGeneratedManifestSource(t *testing.T) {
	m, err := manifest.Generate(manifest.Options{Models: []any{readonlyModel{}}})
	if err != nil {
		t.Fatal(err)
	}
	var c manifest.Column
	for _, v := range m.Tables[0].Columns {
		if v.Name == "server" {
			c = v
		}
	}
	if !c.Readonly || !c.Generated || c.TypeSource != "go" || c.NullableKnown {
		t.Fatal("source facts lost")
	}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var out manifest.Column
	if json.Unmarshal(b, &out) != nil || !out.Readonly || !out.Generated || out.NullableKnown {
		t.Fatal("roundtrip lost facts")
	}
	legacy := []byte(`{"name":"server","type_source":"sql","type":"text"}`)
	if json.Unmarshal(legacy, &out) != nil || out.Readonly || out.NullableKnown {
		t.Fatal("legacy presence changed")
	}
}

func TestTypedGeneratorMappingsAndSnapshotScope(t *testing.T) {
	cases := []struct{ dialect, typ, source, want string }{
		{"mysql", "tinyint", "sql", "int8"}, {"mysql", "mediumint unsigned", "sql", "uint32"}, {"mysql", "bigint unsigned", "sql", "uint64"},
		{"postgres", "int8", "sql", "int64"}, {"postgres", "int8", "go", "any"}, {"mysql", "tinyint(1)", "sql", "any"},
		{"postgres", "numeric(30,4)", "sql", "string"}, {"postgres", "uuid", "sql", "string"}, {"postgres", "custom_enum", "sql", "any"},
		{"mysql", "timestamp(6)", "sql", "any"}, {"postgres", "integer[]", "sql", "any"},
	}
	for _, tc := range cases {
		t.Run(tc.dialect+"/"+tc.typ+"/"+tc.source, func(t *testing.T) {
			table := manifest.Table{Name: "sample", Columns: []manifest.Column{{Name: "value", Type: tc.typ, TypeSource: tc.source, NullableKnown: true}}}
			b, e := manifest.GenerateRepositorySkeletonForTable(table, manifest.RepositorySkeletonOptions{Typed: true, Dialect: tc.dialect})
			if e != nil {
				t.Fatal(e)
			}
			if !bytes.Contains(b, []byte("Value "+tc.want+" ")) {
				t.Fatal("mapping changed")
			}
			if !bytes.Contains(b, []byte("Snapshot kind: table")) {
				t.Fatal("table snapshot promoted")
			}
		})
	}
	m := typedfixture.Manifest("postgres")
	m.SchemaFingerprint = ""
	m.PolicyFingerprint = ""
	b, e := manifest.GenerateRepositorySkeleton(m, typedfixture.Options("postgres"))
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Contains(b, []byte("Snapshot kind: manifest")) || !bytes.Contains(b, []byte("Schema fingerprint: \"\"")) {
		t.Fatal("missing whole snapshot fingerprint filled")
	}
	for _, raw := range []string{`{"name":"x","readonly":null}`, `{"name":"x","readonly":"true"}`, `{"name":"x","readonly":true,"Readonly":false}`} {
		var c manifest.Column
		if json.Unmarshal([]byte(raw), &c) == nil {
			t.Fatal("invalid readonly source accepted")
		}
	}
}
