package storedmodel

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/migration"
	"github.com/recoweft/goquent/orm/query"
)

func TestSavedModelExpectations(t *testing.T) {
	t.Chdir("../../..")
	const root = "examples/ai-safe-orm/"
	const dir = root + "storedmodel/"
	read := func(path string, out any) {
		t.Helper()
		b, e := os.ReadFile(path)
		if e != nil || json.Unmarshal(b, out) != nil {
			t.Fatal("saved input unavailable")
		}
	}
	var schema migration.Schema
	var policy []query.TablePolicy
	read(root+"schema.json", &schema)
	read(root+"policies.json", &policy)
	input, e := manifest.Load(dir + "generator-input.json")
	if e != nil {
		t.Fatal("generator input unavailable")
	}
	current, e := manifest.Generate(manifest.Options{Dialect: "mysql", Schema: &schema, Policies: policy})
	if e != nil {
		t.Fatal("generation failed")
	}
	if input.GeneratedCodeFingerprint != "" || input.DatabaseFingerprint != "" || !manifest.Verify(input, current, time.Time{}).Fresh {
		t.Fatal("generator input no longer matches saved owner")
	}
	opts := manifest.RepositorySkeletonOptions{PackageName: "storedmodel", TableName: "users", RowTypeName: "StoredUserRow", RepositoryTypeName: "StoredUserRepository", Typed: true}
	generated, e := manifest.GenerateRepositorySkeleton(input, opts)
	if e != nil {
		t.Fatal("regeneration failed")
	}
	committed, e := os.ReadFile(dir + "users_generated.go")
	if e != nil || !bytes.Equal(generated, committed) {
		t.Fatal("generated source stale")
	}
	// A changed generator input must change actual output; no expected file is rewritten.
	changed := *input
	changed.Tables = append([]manifest.Table(nil), input.Tables...)
	changed.Tables[0].Columns = append([]manifest.Column(nil), input.Tables[0].Columns...)
	for i := range changed.Tables[0].Columns {
		if changed.Tables[0].Columns[i].Name == "tenant_id" {
			changed.Tables[0].Columns[i].Type = "bigint"
		}
	}
	drift, e := manifest.GenerateRepositorySkeleton(&changed, opts)
	if e != nil || bytes.Equal(drift, committed) {
		t.Fatal("generator drift not detected")
	}
	for _, dialect := range []string{"mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var db migration.Schema
			read(dir+dialect+"-database.json", &db)
			if !reflect.DeepEqual(db, ExpectedDatabase(dialect, CanonicalTable)) {
				t.Fatal("independent declaration stale")
			}
			path := root + "goquent.manifest.json"
			if dialect == "postgres" {
				path = dir + "postgres.manifest.json"
			}
			stored, e := manifest.Load(path)
			if e != nil {
				t.Fatal("expectation unavailable")
			}
			actual, e := manifest.Generate(manifest.Options{Dialect: dialect, Schema: &schema, Policies: policy, GeneratedCodePaths: []string{dir + "users_generated.go"}, DatabaseSchema: &db})
			if e != nil {
				t.Fatal("current input unavailable")
			}
			v := manifest.Verify(stored, actual, time.Time{})
			if !v.Fresh || len(v.Checks) != 4 {
				t.Fatal("saved expectations stale")
			}
			for _, c := range v.Checks {
				if c.Status != "ok" {
					t.Fatal("axis unverified")
				}
			}
			for _, mutate := range []func(*migration.Schema){
				func(s *migration.Schema) { s.Context.ServerVersion += "-changed" },
				func(s *migration.Schema) { s.Context.Coverage.Columns = "complete" },
				func(s *migration.Schema) { s.Context.Catalog = "different" },
			} {
				altered := ExpectedDatabase(dialect, CanonicalTable)
				mutate(&altered)
				m, e := manifest.Generate(manifest.Options{DatabaseSchema: &altered})
				if e != nil || m.DatabaseFingerprint == stored.DatabaseFingerprint {
					t.Fatal("database distinction erased")
				}
			}
		})
	}
	// The generated model deliberately retains the saved string tenant and unknown timestamp type.
	var row StoredUserRow
	var tenant string = row.TenantID
	_ = tenant
	if reflect.TypeOf(row).Field(0).Type.Kind() != reflect.Interface {
		t.Fatal("saved timestamp mapping changed")
	}
}

func TestSavedModelRealCLI(t *testing.T) {
	root, e := filepath.Abs("../../..")
	if e != nil {
		t.Fatal("root unavailable")
	}
	out := t.TempDir()
	if os.Chmod(out, 0700) != nil {
		t.Fatal("private directory failed")
	}
	script := `import json, os, sys
from pathlib import Path
sys.path.insert(0, 'scripts/release_validation')
import run
os.umask(0o077)
c = run.Collector(Path.cwd(), Path(sys.argv[1]))
r = c.manifest_evidence(original=True)
(Path(sys.argv[1]) / 'result.json').write_text(json.dumps(r))
assert r['status'] == 'pass'
assert r['counts'] == dict(passed=48, failed=0)
assert c.stage('manifest')['status'] == 'pass'
# Actual registry rejection for mutated generated code, using a private copy.
import shutil
area = Path(sys.argv[1]) / 'regen'
shutil.copytree('examples/ai-safe-orm/storedmodel', area)
for p in area.rglob('*'):
    p.chmod(0o700 if p.is_dir() else 0o600)
source = area / 'users_generated.go'
source.write_bytes(source.read_bytes() + b'\n// CANARY_REGEN_DRIFT\n')
code, log = c.command([str(c.cli), 'manifest', 'check-repositories', '--config', str(area / 'repositories.json')])
assert code == 1 and 'CANARY_' not in log.read_text()
# A changed fixed generator input must also be rejected, independently of code drift.
source.write_bytes(Path('examples/ai-safe-orm/storedmodel/users_generated.go').read_bytes())
input_path = area / 'generator-input.json'
v = json.loads(input_path.read_text())
for column in v['tables'][0]['columns']:
    if column['name'] == 'tenant_id':
        column['type'] = 'bigint'
input_path.write_text(json.dumps(v))
code, log = c.command([str(c.cli), 'manifest', 'check-repositories', '--config', str(area / 'repositories.json')])
assert code == 1 and 'CANARY_' not in log.read_text()
for p in Path(sys.argv[1]).rglob('*'):
    assert p.stat().st_mode & 0o077 == 0
`
	cmd := exec.Command("python3", "-c", script, out)
	cmd.Dir = root
	if _, e = cmd.CombinedOutput(); e != nil {
		t.Fatal("saved model real CLI regression failed; details omitted")
	}
}
