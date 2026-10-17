package migration

import (
	"bytes"
	"encoding/json"
	"github.com/recoweft/goquent/orm/query"
	"reflect"
	"strings"
	"testing"
)

func completeFixtureCoverage() SchemaCoverage {
	return SchemaCoverage{Tables: CoverageComplete, Columns: CoverageComplete, Indexes: CoverageComplete, ForeignKeys: CoverageComplete, Checks: CoverageComplete, Identity: CoverageComplete}
}
func declaredFixture(s Schema) Schema {
	s.Context = &SchemaContext{Dialect: "postgres", Catalog: "testdb", Schema: "public", Scope: "namespace", Visibility: "declared", Coverage: completeFixtureCoverage()}
	for i := range s.Tables {
		s.Tables[i].Identity = ObjectIdentity{"testdb", "public", s.Tables[i].Name, CoverageComplete}
		s.Tables[i].Coverage = completeFixtureCoverage()
	}
	return s
}
func constraintFixture() Schema {
	s := comparisonFixture()
	t := &s.Tables[0]
	t.ForeignKeys = []ForeignKeySchema{{Name: "fk", Columns: []string{"a", "b"}, Reference: ObjectIdentity{"testdb", "other", "parent", CoverageComplete}, ReferencedColumns: []string{"x", "y"}, OnUpdate: "cascade", OnDelete: "restrict", Match: "simple", Validation: "validated", Enforcement: "enforced", Deferrability: "not_deferrable", Definition: CoverageComplete}}
	t.Checks = []CheckSchema{{Name: "ck", Inheritance: "inheritable", Expression: "a <> 'secret_canary'", Validation: "validated", Enforcement: "enforced", Definition: CoverageComplete}}
	return s
}
func TestExtendedDefinitionChanges(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Schema)
		kind   MigrationStepType
	}{
		{"fk_order", func(s *Schema) { s.Tables[0].ForeignKeys[0].Columns = []string{"b", "a"} }, AlterForeignKey},
		{"fk_reference_order", func(s *Schema) { s.Tables[0].ForeignKeys[0].ReferencedColumns = []string{"y", "x"} }, AlterForeignKey},
		{"fk_action", func(s *Schema) { s.Tables[0].ForeignKeys[0].OnDelete = "cascade" }, AlterForeignKey},
		{"fk_schema", func(s *Schema) { s.Tables[0].ForeignKeys[0].Reference.Schema = "elsewhere" }, AlterForeignKey},
		{"fk_table", func(s *Schema) { s.Tables[0].ForeignKeys[0].Reference.Name = "elsewhere" }, AlterForeignKey},
		{"fk_validation", func(s *Schema) { s.Tables[0].ForeignKeys[0].Validation = "not_validated" }, AlterForeignKey},
		{"fk_drop", func(s *Schema) { s.Tables[0].ForeignKeys = nil }, DropForeignKey},
		{"check_expression", func(s *Schema) { s.Tables[0].Checks[0].Expression = "a  <> 'secret_canary'" }, AlterCheck},
		{"check_validation", func(s *Schema) { s.Tables[0].Checks[0].Validation = "not_validated" }, AlterCheck},
		{"check_enforcement", func(s *Schema) { s.Tables[0].Checks[0].Enforcement = "not_enforced" }, AlterCheck},
		{"check_drop", func(s *Schema) { s.Tables[0].Checks = nil }, DropCheck},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, b := constraintFixture(), constraintFixture()
			tc.mutate(&b)
			p := DiffSchemas(a, b)
			r := CompareSchemaDrift(b, a)
			if p.ComparisonIncomplete || !r.Drifted || len(p.Steps) != 1 || p.Steps[0].Type != tc.kind || !reflect.DeepEqual(p.Steps, r.Steps) {
				t.Fatal("structural change missing", p.Steps)
			}
			p.Approval = &query.Approval{Reason: "approved"}
			if EnsureExecutable(p) == nil {
				t.Fatal("structural record executable")
			}
			v, _ := p.PublicView()
			raw, _ := v.ToJSON()
			if bytes.Contains(raw, []byte("secret_canary")) || bytes.Contains(raw, []byte("elsewhere")) {
				t.Fatal("public leak")
			}
		})
	}
	for _, kind := range []MigrationStepType{AddForeignKey, DropForeignKey, AlterForeignKey, AddCheck, DropCheck, AlterCheck, SchemaIdentityDifference} {
		p := PlanSteps([]MigrationStep{{Type: kind}})
		if EnsureExecutable(p) == nil || !p.Blocked {
			t.Fatal("empty recipe executable")
		}
	}
}
func TestCoverageAndIdentityFailClosed(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Schema)
	}{
		{"nil_context", func(s *Schema) { s.Context = nil }},
		{"unknown_inventory", func(s *Schema) { s.Context.Coverage.Tables = "future" }},
		{"partial_inventory", func(s *Schema) { s.Context.Coverage.Tables = CoveragePartial }},
		{"invisible", func(s *Schema) { s.Context.Visibility = "catalog_visible" }},
		{"scope", func(s *Schema) { s.Context.Scope = "tables"; s.Context.Tables = []string{"outside"} }},
		{"identity_conflict", func(s *Schema) { s.Tables[0].Identity.Schema = "other" }},
		{"duplicate", func(s *Schema) { s.Tables = append(s.Tables, s.Tables[0]) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := constraintFixture()
			tc.mutate(&a)
			b := declaredFixture(Schema{})
			for _, pair := range [][2]Schema{{a, b}, {b, a}} {
				p := DiffSchemas(pair[0], pair[1])
				if !p.ComparisonIncomplete {
					t.Fatal("incomplete lost")
				}
				for _, step := range p.Steps {
					if step.Type == AddTable || step.Type == DropTable {
						t.Fatal("unproved absence")
					}
				}
			}
		})
	}
	a, b := constraintFixture(), constraintFixture()
	b.Context.Schema = "other"
	b.Tables[0].Identity.Schema = "other"
	p := DiffSchemas(a, b)
	if !p.ComparisonIncomplete || !hasStep(p.Steps, SchemaIdentityDifference, "") {
		t.Fatal("namespace collision merged")
	}
	// Legacy records still expose local known differences, never complete identity.
	a, b = Schema{Tables: []TableSchema{{Name: "t", Columns: []ColumnSchema{{Name: "c", Type: "text"}}}}}, Schema{Tables: []TableSchema{{Name: "t", Columns: []ColumnSchema{{Name: "c", Type: "integer"}}}}}
	p = DiffSchemas(a, b)
	if !p.ComparisonIncomplete || !hasStep(p.Steps, AlterColumnType, "c") {
		t.Fatal("legacy local difference lost")
	}
	if !DiffSchemas(Schema{}, Schema{}).ComparisonIncomplete {
		t.Fatal("zero promoted")
	}
	if DiffSchemas(declaredFixture(Schema{}), declaredFixture(Schema{})).ComparisonIncomplete {
		t.Fatal("declared empty unavailable")
	}
}
func TestExtendedUnknownAndDetachment(t *testing.T) {
	for _, mutate := range []func(*Schema){
		func(s *Schema) { s.Tables[0].ForeignKeys[0].OnDelete = "future" },
		func(s *Schema) { s.Tables[0].ForeignKeys[0].ReferencedColumns = nil },
		func(s *Schema) { s.Tables[0].ForeignKeys[0].Reference.State = "future" },
		func(s *Schema) { s.Tables[0].Checks = append(s.Tables[0].Checks, s.Tables[0].Checks[0]) },
		func(s *Schema) { s.Tables[0].Coverage.Checks = CoverageUnsupported },
	} {
		a, b := constraintFixture(), constraintFixture()
		mutate(&b)
		b.Tables[0].Columns[0].Type = "integer"
		p := DiffSchemas(a, b)
		if !p.ComparisonIncomplete || !hasStep(p.Steps, AlterColumnType, "a") {
			t.Fatal("unknown hid known change")
		}
	}
	a, b := constraintFixture(), constraintFixture()
	b.Tables[0].ForeignKeys[0].OnDelete = "cascade"
	p := DiffSchemas(a, b)
	before, _ := p.ToJSON()
	b.Tables[0].ForeignKeys[0].Columns[0] = "mutated"
	b.Context.Tables = append(b.Context.Tables, "mutated")
	after, _ := p.ToJSON()
	if !bytes.Equal(before, after) {
		t.Fatal("source aliases input")
	}
	var decoded MigrationPlan
	if err := json.Unmarshal(before, &decoded); err != nil {
		t.Fatal(err)
	}
	wire, _ := decoded.ToJSON()
	if !bytes.Equal(before, wire) {
		t.Fatal("source roundtrip")
	}
}
func TestExtendedSourceJSON(t *testing.T) {
	for _, wire := range []string{`{"context":null}`, `{"context":{"Scope":"namespace"}}`, `{"context":{"scope":"tables","scope":"namespace"}}`, `{"tables":[{"name":"x","coverage":null}]}`, `{"tables":[{"name":"x","foreign_keys":[{"name":"f","reference":null}]}]}`} {
		var s Schema
		if json.Unmarshal([]byte(wire), &s) == nil {
			t.Fatal("ambiguous source accepted")
		}
	}
	var s Schema
	wire := `{"context":{"scope":"future","coverage":{"tables":"future"}},"tables":[]}`
	if json.Unmarshal([]byte(wire), &s) != nil {
		t.Fatal("future enums rejected")
	}
	b, _ := json.Marshal(s)
	if !strings.Contains(string(b), "future") || !DiffSchemas(s, s).ComparisonIncomplete {
		t.Fatal("future promoted or lost")
	}
}
func TestCapabilityProvenanceAndRedaction(t *testing.T) {
	steps := []MigrationStep{{Type: AddIndex, Concurrent: true}, {Type: AddCheck}}
	for _, version := range []string{"", "99.1", "secret_canary", "16.10"} {
		c := &SchemaContext{Dialect: "postgres", ServerVersion: version, VersionSource: "catalog_select", Coverage: SchemaCoverage{Version: CoverageComplete}}
		for _, v := range capabilitiesFor(c, steps) {
			if v.Transactional.Support != "unknown" {
				t.Fatal("external declaration became observation")
			}
		}
	}
	c := &SchemaContext{Dialect: "postgres", ServerVersion: "16.10", observedVersion: "16.10", VersionSource: "catalog_select", Coverage: SchemaCoverage{Version: CoverageComplete}}
	v := capabilitiesFor(c, steps)
	if v[0].Transactional.Support != "unsupported" || v[0].Concurrent.Conditions != "unknown" || v[0].LockFree.Support != "unsupported" {
		t.Fatal("operation conditions wrong")
	}
	b, _ := json.Marshal(c)
	var d SchemaContext
	if json.Unmarshal(b, &d) != nil {
		t.Fatal("decode")
	}
	if capabilitiesFor(&d, steps)[0].Source != "unverified" {
		t.Fatal("JSON fabricated private observation")
	}
	p := PlanSteps(steps)
	p.Capabilities = []OperationCapability{{Operation: "secret_canary", Source: "secret_canary", LockFree: CapabilityAssessment{Support: "secret_canary", Conditions: "secret_canary"}}}
	view, _ := p.PublicView()
	raw, _ := view.ToJSON()
	if bytes.Contains(raw, []byte("secret_canary")) {
		t.Fatal("capability leak")
	}
}

func TestIndexExtendedAttributes(t *testing.T) {
	a0, b0 := comparisonFixture(), comparisonFixture()
	b0.Tables[0].Indexes[0].Keys = []IndexKey{}
	if CompareSchemaDrift(b0, a0).Drifted {
		t.Fatal("empty key list changed legacy column identity")
	}
	base := func() Schema {
		s := comparisonFixture()
		s.Tables[0].Indexes[0] = IndexSchema{Name: "ix", Keys: []IndexKey{{Expression: "length(a)", Order: "asc", Nulls: "last"}, {Column: "b", Order: "desc", Nulls: "first"}}, Predicate: ExpressionDefinition{"expression", "b IS NOT NULL"}, Method: "btree", Visibility: "visible", Attributes: CoverageComplete}
		return s
	}
	for _, tc := range []struct {
		name    string
		mutate  func(*IndexSchema)
		unknown bool
	}{
		{"expression", func(i *IndexSchema) { i.Keys[0].Expression = "length(a) + 1" }, false},
		{"predicate", func(i *IndexSchema) { i.Predicate.SQL = " b IS NOT NULL" }, false},
		{"order", func(i *IndexSchema) { i.Keys[0], i.Keys[1] = i.Keys[1], i.Keys[0] }, false},
		{"include", func(i *IndexSchema) { i.Include = []string{"a"} }, false},
		{"visibility", func(i *IndexSchema) { i.Visibility = "invisible" }, false},
		{"key_union", func(i *IndexSchema) { i.Keys[0].Column = "a" }, true},
		{"legacy_conflict", func(i *IndexSchema) { i.Columns = []string{"a", "b"} }, true},
		{"unknown_attr", func(i *IndexSchema) { i.Keys[0].Nulls = "future" }, true},
		{"unknown_method", func(i *IndexSchema) { i.Method = "future" }, true},
		{"predicate_contradiction", func(i *IndexSchema) { i.Predicate.State = "none" }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, b := base(), base()
			tc.mutate(&b.Tables[0].Indexes[0])
			p := DiffSchemas(a, b)
			r := CompareSchemaDrift(b, a)
			if p.ComparisonIncomplete != tc.unknown || r.Drifted == tc.unknown {
				t.Fatal("index classification")
			}
			if !tc.unknown && !hasStep(p.Steps, AlterIndex, "ix") {
				t.Fatal("index change lost")
			}
		})
	}
	a, b := base(), base()
	b.Tables[0].Indexes[0].Include = []string{"a"}
	p := DiffSchemas(a, b)
	raw, _ := p.ToJSON()
	b.Tables[0].Indexes[0].Keys[0].Expression = "mutated"
	b.Tables[0].Indexes[0].Include[0] = "mutated"
	now, _ := p.ToJSON()
	if !bytes.Equal(raw, now) {
		t.Fatal("index alias")
	}
}
func TestWholeTableConstraintUnknown(t *testing.T) {
	for _, kind := range []string{"partial_fk", "unknown_validation", "unknown_check", "unknown_attributes"} {
		s := constraintFixture()
		switch kind {
		case "partial_fk":
			s.Tables[0].ForeignKeys[0].Definition = CoveragePartial
		case "unknown_validation":
			s.Tables[0].ForeignKeys[0].Validation = "unknown"
		case "unknown_check":
			s.Tables[0].Checks[0].Definition = CoverageUnsupported
		case "unknown_attributes":
			s.Tables[0].Indexes[0].Attributes = ""
		}
		for _, pair := range [][2]Schema{{declaredFixture(Schema{}), s}, {s, declaredFixture(Schema{})}} {
			p := DiffSchemas(pair[0], pair[1])
			if !p.ComparisonIncomplete || !CompareSchemaDrift(pair[1], pair[0]).Drifted {
				t.Fatal("whole table known change or unknown lost")
			}
		}
	}
}

func TestParentCoverageCannotBeOverridden(t *testing.T) {
	a, b := constraintFixture(), constraintFixture()
	a.Context.Coverage.ForeignKeys = CoverageUnsupported
	a.Tables[0].ForeignKeys = nil
	p := DiffSchemas(a, b)
	if !p.ComparisonIncomplete {
		t.Fatal("parent coverage lost")
	}
	for _, step := range p.Steps {
		if step.Type == AddForeignKey {
			t.Fatal("child overrode unsupported parent")
		}
	}
	a, b = constraintFixture(), constraintFixture()
	a.Tables[0].Checks[0].Inheritance = "no_inherit"
	if !hasStep(DiffSchemas(a, b).Steps, AlterCheck, "records") {
		t.Fatal("inheritance change lost")
	}
	for _, s := range []Schema{constraintFixture(), Schema{}, a} {
		if s.PublicView().Unknown != DiffSchemas(s, s).ComparisonIncomplete {
			t.Fatal("public/schema completeness disagree")
		}
	}
}
func TestCapabilitiesSeparateConcurrentRequests(t *testing.T) {
	c := &SchemaContext{Dialect: "postgres", ServerVersion: "16.10", observedVersion: "16.10", VersionSource: "catalog_select", Coverage: SchemaCoverage{Version: CoverageComplete}}
	caps := capabilitiesFor(c, []MigrationStep{{Type: AddIndex}, {Type: AddIndex, Concurrent: true}})
	if len(caps) != 2 || caps[0].Transactional.Support != "supported" || caps[1].Transactional.Support != "unsupported" {
		t.Fatal("operation condition conflated")
	}
	for _, version := range []string{"", "8.0.40", "8.4.6-vendor", "17.1", "secret_canary"} {
		c.ServerVersion = version
		c.observedVersion = version
		caps = capabilitiesFor(c, []MigrationStep{{Type: AddCheck}})
		if caps[0].Transactional.Support != "unknown" {
			t.Fatal("unknown version promoted")
		}
	}
}

func TestOldConsumerLosesExtendedPrecision(t *testing.T) {
	type oldTable struct {
		Name    string         `json:"name"`
		Columns []ColumnSchema `json:"columns,omitempty"`
		Indexes []IndexSchema  `json:"indexes,omitempty"`
	}
	type oldSchema struct {
		Tables []oldTable `json:"tables,omitempty"`
	}
	source := constraintFixture()
	wire, _ := json.Marshal(source)
	var old oldSchema
	if json.Unmarshal(wire, &old) != nil {
		t.Fatal("old consumer decode")
	}
	legacy, _ := json.Marshal(old)
	var next Schema
	if json.Unmarshal(legacy, &next) != nil {
		t.Fatal("legacy decode")
	}
	if !DiffSchemas(next, next).ComparisonIncomplete || next.Context != nil || len(next.Tables[0].ForeignKeys) != 0 {
		t.Fatal("old consumer precision promoted")
	}
	a, b := comparisonFixture(), comparisonFixture()
	a.Tables[0].Indexes[0].RawDefinition = "old catalog options"
	b.Tables[0].Indexes[0].RawDefinition = "new catalog options"
	p := DiffSchemas(a, b)
	if !p.ComparisonIncomplete || CompareSchemaDrift(b, a).Drifted {
		t.Fatal("unrepresented catalog difference ignored or invented")
	}
}

func TestKnownCollectionDifferenceRetainsUnknownDefinition(t *testing.T) {
	a, b := constraintFixture(), constraintFixture()
	a.Tables[0].ForeignKeys = nil
	a.Tables[0].Checks = nil
	a.Tables[0].Indexes = nil
	b.Tables[0].ForeignKeys[0].OnDelete = "future"
	b.Tables[0].Checks[0].Validation = "future"
	b.Tables[0].Indexes[0].DefinitionUnknown = true
	for _, tc := range []struct {
		a, b  Schema
		kinds []MigrationStepType
	}{{a, b, []MigrationStepType{AddIndex, AddForeignKey, AddCheck}}, {b, a, []MigrationStepType{DropIndex, DropForeignKey, DropCheck}}} {
		p := DiffSchemas(tc.a, tc.b)
		if !p.ComparisonIncomplete || EnsureExecutable(p) == nil {
			t.Fatal("definition unknown lost")
		}
		for _, kind := range tc.kinds {
			found := false
			for _, s := range p.Steps {
				if s.Type == kind {
					found = true
				}
			}
			if !found {
				t.Fatal("known collection change lost")
			}
		}
	}
}

func TestDecodedPlanCannotClaimLiveCapability(t *testing.T) {
	c := &SchemaContext{Dialect: "postgres", ServerVersion: "16.10", observedVersion: "16.10", VersionSource: "catalog_select", Coverage: SchemaCoverage{Version: CoverageComplete}}
	p := PlanSteps([]MigrationStep{{Type: AddCheck}})
	p.CurrentSchema = &Schema{Context: c}
	p.Capabilities = capabilitiesFor(c, p.Steps)
	v, _ := p.PublicView()
	if v.Capabilities[0].Source != "live_observation" {
		t.Fatal("reader observation missing")
	}
	wire, _ := p.ToJSON()
	var decoded MigrationPlan
	if json.Unmarshal(wire, &decoded) != nil {
		t.Fatal("decode")
	}
	v, _ = decoded.PublicView()
	if v.Capabilities[0].Source != "unverified" || v.Capabilities[0].Transactional.Support != "unknown" {
		t.Fatal("source labels became live facts")
	}
}

func TestDriftPublishesSameCapabilityAxes(t *testing.T) {
	a, b := constraintFixture(), constraintFixture()
	a.Context.ServerVersion = "16.10"
	a.Context.observedVersion = "16.10"
	a.Context.VersionSource = "catalog_select"
	a.Context.Coverage.Version = CoverageComplete
	b.Tables[0].Checks = nil
	p := DiffSchemas(a, b)
	r := CompareSchemaDrift(b, a)
	pv, _ := p.PublicView()
	dv := r.PublicView()
	if !reflect.DeepEqual(pv.Capabilities, dv.Capabilities) || len(dv.Capabilities) != 1 || dv.Capabilities[0].Transactional.Support != "supported" {
		t.Fatal("plan/drift capability divergence")
	}
	wire, _ := json.Marshal(r)
	var decoded DriftReport
	if json.Unmarshal(wire, &decoded) != nil {
		t.Fatal("decode")
	}
	if decoded.PublicView().Capabilities[0].Source != "unverified" {
		t.Fatal("drift source labels became evidence")
	}
}
