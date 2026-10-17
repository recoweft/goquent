package migration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"sort"
	"time"
)

// Preflight is a sensitive, detached source artifact, not an execution recipe.
// Only a live Collector can authenticate its own observations while still open.
type Preflight struct {
	Version    int                  `json:"version"`
	Plan       PreflightPlan        `json:"plan"`
	Collection *PreflightCollection `json:"collection,omitempty"`
	receipt    *preflightReceipt
}

// PreflightPlan retains the finite inputs used to derive every requirement.
type PreflightPlan struct {
	ComparisonIncomplete bool                   `json:"comparison_incomplete"`
	MigrationBlocked     bool                   `json:"migration_blocked"`
	ID                   string                 `json:"id"`
	Current              Schema                 `json:"current"`
	Desired              Schema                 `json:"desired"`
	References           []Schema               `json:"references,omitempty"`
	SQL                  string                 `json:"sql,omitempty"`
	Statements           []MigrationStatement   `json:"statements,omitempty"`
	Steps                []PreflightStep        `json:"steps"`
	Requirements         []PreflightRequirement `json:"requirements"`
}

// PhaseContract is a declaration, never evidence that an application rolled out.
// Dependencies are zero-based earlier step ordinals. Missing compatibility is unknown.
type PhaseContract struct {
	Phase      string `json:"phase"`
	DependsOn  []int  `json:"depends_on,omitempty"`
	OldReaders string `json:"old_readers,omitempty"`
	OldWriters string `json:"old_writers,omitempty"`
}

// PreflightStep excludes approval and free-form display metadata from identity.
type PreflightStep struct {
	ID         string        `json:"id"`
	Definition MigrationStep `json:"definition"`
	Phase      PhaseContract `json:"phase"`
}

type PreflightRequirement struct {
	ID                string         `json:"id"`
	StepID            string         `json:"step_id"`
	Kind              string         `json:"kind"`
	Target            ObjectIdentity `json:"target"`
	Columns           []string       `json:"columns,omitempty"`
	Reference         ObjectIdentity `json:"reference"`
	ReferencedColumns []string       `json:"referenced_columns,omitempty"`
	Semantics         string         `json:"semantics"`
	Status            string         `json:"status"`
	Code              string         `json:"code"`
}

// PreflightResult contains aggregate counts only; no row values are retained.
// PreflightTargetObservation records the finite per-target read coverage separately
// from the original (possibly partial) catalog inventory.
type PreflightTargetObservation struct {
	Target   ObjectIdentity `json:"target"`
	Columns  []string       `json:"columns"`
	Types    []string       `json:"types,omitempty"`
	Coverage string         `json:"coverage"`
}

type PreflightResult struct {
	Targets       []PreflightTargetObservation `json:"targets,omitempty"`
	RequirementID string                       `json:"requirement_id"`
	Count         *int64                       `json:"count,omitempty"`
	Completed     bool                         `json:"completed"`
	Status        string                       `json:"status"`
	Code          string                       `json:"code"`
}

type PreflightCollection struct {
	PlanID            string            `json:"plan_id"`
	SchemaFingerprint string            `json:"schema_fingerprint"`
	FingerprintDomain string            `json:"fingerprint_domain"`
	StartedAt         time.Time         `json:"started_at"`
	FinishedAt        time.Time         `json:"finished_at"`
	Method            string            `json:"method"`
	IdentityBasis     string            `json:"identity_basis"`
	Scope             string            `json:"scope"`
	Completed         bool              `json:"completed"`
	Code              string            `json:"code"`
	Results           []PreflightResult `json:"results"`
	Digest            string            `json:"digest"`
}

const PreflightMaxAge = 5 * time.Minute

var ErrPreflightInput = errors.New("goquent: invalid preflight input")
var ErrPreflightRead = errors.New("goquent: preflight collection failed")

// PreflightOptions requires an explicit clock. MaxAge must be positive and no
// greater than five minutes. The clock is caller configuration, not a trust root.
type PreflightOptions struct {
	Clock  func() time.Time
	MaxAge time.Duration
}

// PlanPreflight is disconnected. Current/desired must be supplied explicitly;
// references are explicit additional read scopes, including cross-schema targets.
func PlanPreflight(p *MigrationPlan, phases []PhaseContract, references []Schema) (*Preflight, error) {
	if p == nil || p.CurrentSchema == nil || p.DesiredSchema == nil || len(p.Steps) == 0 || len(p.Steps) > 64 || (len(phases) != 0 && len(phases) != len(p.Steps)) {
		return nil, ErrPreflightInput
	}
	out := &Preflight{Version: 1, Plan: PreflightPlan{ComparisonIncomplete: p.ComparisonIncomplete, MigrationBlocked: p.Blocked, Current: *copySchema(*p.CurrentSchema), Desired: *copySchema(*p.DesiredSchema), SQL: p.SQL, Statements: slices.Clone(p.Statements)}}
	for _, s := range references {
		out.Plan.References = append(out.Plan.References, *copySchema(s))
	}
	for i, s := range p.Steps {
		// Strip diagnostic and approval material before JSON detachment. These fields
		// are not requirements and must never smuggle custom values into canonical data.
		s.Warnings = nil
		s.Preflight = nil
		s.RiskLevel = ""
		s.AnalysisPrecision = ""
		s.Line = 0
		b, e := json.Marshal(s)
		if e != nil {
			return nil, ErrPreflightInput
		}
		var detached MigrationStep
		if json.Unmarshal(b, &detached) != nil {
			return nil, ErrPreflightInput
		}
		ph := PhaseContract{Phase: "unknown"}
		if len(phases) > 0 {
			ph = phases[i]
			ph.DependsOn = slices.Clone(ph.DependsOn)
		}
		out.Plan.Steps = append(out.Plan.Steps, PreflightStep{Definition: detached, Phase: ph})
	}
	if err := buildPreflight(&out.Plan); err != nil {
		return nil, err
	}
	return out, nil
}

func digestPreflight(domain string, v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(append([]byte(domain+"\x00"), b...))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// Only unordered named collections are sorted. Column and key order stays exact.
func canonicalSchema(s *Schema) error {
	if !contextKnown(s.Context) {
		return ErrPreflightInput
	}
	s.Context.observedVersion = ""
	sort.Strings(s.Context.Tables)
	seen := map[ObjectIdentity]bool{}
	for i := range s.Tables {
		t := &s.Tables[i]
		if !tableIdentityValid(*t, s.Context) || seen[t.Identity] {
			return ErrPreflightInput
		}
		seen[t.Identity] = true
		names := map[string]bool{}
		for _, c := range t.Columns {
			if !simpleSchemaName.MatchString(c.Name) || names[c.Name] {
				return ErrPreflightInput
			}
			names[c.Name] = true
		}
		names = map[string]bool{}
		for _, x := range t.Indexes {
			if x.Name == "" || names[x.Name] {
				return ErrPreflightInput
			}
			names[x.Name] = true
		}
		names = map[string]bool{}
		for _, x := range t.ForeignKeys {
			if x.Name == "" || names[x.Name] {
				return ErrPreflightInput
			}
			names[x.Name] = true
		}
		for _, x := range t.Checks {
			if x.Name == "" || names[x.Name] {
				return ErrPreflightInput
			}
			names[x.Name] = true
		}
		sort.Slice(t.Indexes, func(a, b int) bool { return t.Indexes[a].Name < t.Indexes[b].Name })
		sort.Slice(t.ForeignKeys, func(a, b int) bool { return t.ForeignKeys[a].Name < t.ForeignKeys[b].Name })
		sort.Slice(t.Checks, func(a, b int) bool { return t.Checks[a].Name < t.Checks[b].Name })
	}
	sort.Slice(s.Tables, func(a, b int) bool { return s.Tables[a].Name < s.Tables[b].Name })
	return nil
}
func buildPreflight(p *PreflightPlan) error {
	if len(p.Steps) == 0 || len(p.Steps) > 64 || len(p.References) > 64 {
		return ErrPreflightInput
	}
	if canonicalSchema(&p.Current) != nil || canonicalSchema(&p.Desired) != nil {
		return ErrPreflightInput
	}
	if !sameScope(p.Current.Context, p.Desired.Context) {
		return ErrPreflightInput
	}
	scopes := map[string]bool{p.Current.Context.Catalog + "/" + p.Current.Context.Schema: true}
	for i := range p.References {
		s := &p.References[i]
		if canonicalSchema(s) != nil || s.Context.Dialect != p.Current.Context.Dialect {
			return ErrPreflightInput
		}
		key := s.Context.Catalog + "/" + s.Context.Schema
		if scopes[key] {
			return ErrPreflightInput
		}
		scopes[key] = true
	}
	sort.Slice(p.References, func(i, j int) bool {
		return p.References[i].Context.Catalog+"/"+p.References[i].Context.Schema < p.References[j].Context.Catalog+"/"+p.References[j].Context.Schema
	})
	p.ID = ""
	p.Requirements = []PreflightRequirement{}
	last := 0
	for i := range p.Steps {
		s := &p.Steps[i]
		s.ID = ""
		seen := map[int]bool{}
		for _, d := range s.Phase.DependsOn {
			if d < 0 || d >= i || seen[d] {
				return ErrPreflightInput
			}
			seen[d] = true
		}
		rank := phaseRank(s.Phase.Phase)
		if rank > 0 && rank < last {
			return ErrPreflightInput
		}
		if rank > 0 {
			last = rank
		}
	}
	p.ID = digestPreflight("goquent.preflight.plan.v1", p)
	for i := range p.Steps {
		s := &p.Steps[i]
		s.ID = digestPreflight("goquent.preflight.step.v1", struct {
			Plan    string
			Ordinal int
			Step    PreflightStep
		}{p.ID, i, *s})
		req := requirementsFor(*p, i)
		for j := range req {
			req[j].StepID = s.ID
			req[j].ID = digestPreflight("goquent.preflight.requirement.v1", req[j])
			p.Requirements = append(p.Requirements, req[j])
		}
	}
	if len(p.Requirements) > 64 {
		return ErrPreflightInput
	}
	return nil
}
func phaseRank(s string) int {
	switch s {
	case "expand":
		return 1
	case "backfill":
		return 2
	case "contract":
		return 3
	}
	return 0
}
func tableFor(s Schema, name string) (TableSchema, bool) {
	for _, t := range s.Tables {
		if t.Name == name {
			return t, true
		}
	}
	return TableSchema{}, false
}
func columnsOK(cols []string, t TableSchema) bool {
	return len(cols) > 0 && len(cols) <= 16 && knownColumns(cols, t)
}
func requirementsFor(p PreflightPlan, i int) []PreflightRequirement {
	s := p.Steps[i]
	d := s.Definition
	t, found := tableFor(p.Current, d.Table)
	desired, hasDesired := tableFor(p.Desired, d.Table)
	base := PreflightRequirement{Kind: "unknown", Target: t.Identity, Semantics: "unknown", Status: "unknown", Code: "PREFLIGHT_DEFINITION"}
	var out []PreflightRequirement
	addUnknown := func(code string) { r := base; r.Code = code; out = append(out, r) }
	if p.ComparisonIncomplete {
		addUnknown("PREFLIGHT_DEFINITION")
	}
	if phaseRank(s.Phase.Phase) == 0 {
		addUnknown("PREFLIGHT_PHASE")
	}
	if s.Phase.OldReaders != "compatible" || s.Phase.OldWriters != "compatible" {
		addUnknown("PREFLIGHT_COMPATIBILITY")
	}
	if i > 0 && s.Phase.Phase == "contract" && len(s.Phase.DependsOn) == 0 {
		addUnknown("PREFLIGHT_DEPENDENCY")
	}
	if d.SQL != "" || p.SQL != "" || len(p.Statements) != 0 {
		addUnknown("PREFLIGHT_DEFINITION")
	}
	if d.CurrentTable != nil && digestPreflight("table", *d.CurrentTable) != digestPreflight("table", t) {
		addUnknown("PREFLIGHT_DEFINITION")
	}
	if d.DesiredTable != nil && digestPreflight("table", *d.DesiredTable) != digestPreflight("table", desired) {
		addUnknown("PREFLIGHT_DEFINITION")
	}
	if d.ComparisonUnknown {
		addUnknown("PREFLIGHT_DEFINITION")
		return out
	}
	r := base
	switch d.Type {
	case AlterNullability:
		if d.Nullable == nil {
			addUnknown("PREFLIGHT_DEFINITION")
			break
		}
		if *d.Nullable {
			break
		}
		r.Kind = "null_count"
		r.Columns = []string{d.Column}
		r.Semantics = "null_rows"
		if s.Phase.Phase != "contract" {
			addUnknown("PREFLIGHT_PHASE")
		}
		desiredColumnKnown := false
		for _, col := range desired.Columns {
			if col.Name == d.Column && col.NullableKnown && !col.Nullable {
				desiredColumnKnown = true
			}
		}
		if found && hasDesired && desiredColumnKnown && columnsOK(r.Columns, t) {
			r.Status = "pending"
			r.Code = "PREFLIGHT_MISSING"
		}
		out = append(out, r)
	case AddIndex, AlterIndex:
		var idx *IndexSchema
		if hasDesired {
			for j := range desired.Indexes {
				if desired.Indexes[j].Name == d.Index {
					idx = &desired.Indexes[j]
				}
			}
		}
		if idx == nil {
			addUnknown("PREFLIGHT_DEFINITION")
			break
		}
		if d.DesiredIndex != nil && digestPreflight("index", *d.DesiredIndex) != digestPreflight("index", *idx) {
			addUnknown("PREFLIGHT_DEFINITION")
		}
		if !idx.Unique {
			break
		}
		r.Kind = "duplicate_count"
		r.Semantics = "duplicate_groups_nonnull"
		r.Status = "unsupported"
		r.Code = "PREFLIGHT_UNSUPPORTED"
		r.Columns = slices.Clone(idx.Columns)
		if len(idx.Keys) > 0 {
			r.Columns = nil
			for _, k := range idx.Keys {
				r.Columns = append(r.Columns, k.Column)
			}
		}
		supported := validExtendedIndex(*idx, desired) && idx.Predicate.State == "none" && len(idx.Include) == 0 && idx.RawDefinition == ""
		for _, k := range idx.Keys {
			supported = supported && k.Expression == "" && k.Prefix == 0
		}
		if found && supported && columnsOK(r.Columns, t) {
			r.Status = "pending"
			r.Code = "PREFLIGHT_MISSING"
		}
		out = append(out, r)
	case AddForeignKey, AlterForeignKey:
		var fk *ForeignKeySchema
		if hasDesired {
			for j := range desired.ForeignKeys {
				if desired.ForeignKeys[j].Name == d.Constraint {
					fk = &desired.ForeignKeys[j]
				}
			}
		}
		if fk == nil {
			addUnknown("PREFLIGHT_DEFINITION")
			break
		}
		r.Kind = "orphan_count"
		r.Semantics = "match_simple_nonnull_rows"
		r.Columns = slices.Clone(fk.Columns)
		r.Reference = fk.Reference
		r.ReferencedColumns = slices.Clone(fk.ReferencedColumns)
		r.Status = "unsupported"
		r.Code = "PREFLIGHT_UNSUPPORTED"
		if found && validFK(*fk, desired) && fk.Match == "simple" && columnsOK(r.Columns, t) && len(r.ReferencedColumns) <= 16 {
			r.Status = "pending"
			r.Code = "PREFLIGHT_MISSING"
		}
		out = append(out, r)
	case DropTable, DropColumn, RenameColumn, AlterColumnType:
		addUnknown("PREFLIGHT_EXTERNAL_WORK")
	case AddTable, AddColumn:
		// New objects cannot be measured before they exist; do not infer a backfill.
		addUnknown("PREFLIGHT_EXTERNAL_WORK")
	default:
		r.Status = "unsupported"
		r.Code = "PREFLIGHT_UNSUPPORTED"
		out = append(out, r)
	}
	if s.Phase.Phase == "backfill" {
		addUnknown("PREFLIGHT_EXTERNAL_WORK")
	}
	return out
}

// ValidatePreflightSource checks recomputed bindings, not authenticity. It never
// reconstitutes a receipt and accepts no caller-supplied requirements as authority.
func ValidatePreflightSource(p *Preflight) error {
	if p == nil || p.Version != 1 {
		return ErrPreflightInput
	}
	b, e := json.Marshal(p.Plan)
	if e != nil {
		return ErrPreflightInput
	}
	var expected PreflightPlan
	if json.Unmarshal(b, &expected) != nil || buildPreflight(&expected) != nil {
		return ErrPreflightInput
	}
	if digestPreflight("compare", expected) != digestPreflight("compare", p.Plan) {
		return ErrPreflightInput
	}
	if c := p.Collection; c != nil {
		if c.Digest == "" || c.PlanID != p.Plan.ID || c.FingerprintDomain != "goquent.preflight.schema.v1" || c.SchemaFingerprint != schemaFingerprint(p.Plan) || c.Digest != collectionDigest(*c) {
			return ErrPreflightInput
		}
		if len(c.Results) > len(p.Plan.Requirements) {
			return ErrPreflightInput
		}
		seen := map[string]bool{}
		for i, r := range c.Results {
			if r.RequirementID != p.Plan.Requirements[i].ID || seen[r.RequirementID] || r.Count != nil && *r.Count < 0 || r.Completed && r.Count == nil {
				return ErrPreflightInput
			}
			seen[r.RequirementID] = true
		}
	}
	return nil
}
func schemaFingerprint(p PreflightPlan) string {
	return digestPreflight("goquent.preflight.schema.v1", struct {
		Current    Schema
		References []Schema
	}{p.Current, p.References})
}
func collectionDigest(c PreflightCollection) string {
	c.Digest = ""
	return digestPreflight("goquent.preflight.collection.v1", c)
}
