package review

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/recoweft/goquent/orm/analysis"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/query"
)

const MaxReviewReceipts = 4096

// SourceReference is sensitive source API data. A test declares correspondence;
// neither this reference nor its digest proves that the source was executed.
type SourceReference struct {
	File    string `json:"file"`
	Offset  int    `json:"offset"`
	ID      string `json:"id,omitempty"`
	Version string `json:"version"`
	session *Session
	index   int
}

// OperationReview retains operations even when they produce no risk warnings.
type OperationReview struct {
	Suppressed     bool                    `json:"suppressed"`
	Source         SourceReference         `json:"source"`
	Location       *query.SourceLocation   `json:"location"`
	Detail         analysis.Detail         `json:"detail"`
	Correspondence analysis.Correspondence `json:"correspondence"`
	expected       query.ReviewReceipt
	operation      query.OperationType
}

// Session is valid only during Options.Evidence. It owns a bounded input snapshot.
// Methods are synchronous; callers must not mutate queries concurrently with review.
type Session struct {
	operationCount int
	manifestInput  []byte
	version        [32]byte
	versionSet     bool
	token          query.ReviewToken
	ctx            context.Context
	inputs         map[string][]byte
	operations     []OperationReview
	records        map[int][]query.ReviewReceipt
	ids            map[string]int
	opts           Options
	valid          bool
	reason         string
}

// References returns detached source references for test declarations.
func (s *Session) References() []SourceReference {
	out := make([]SourceReference, len(s.operations))
	for i := range out {
		out[i] = s.operations[i].Source
	}
	return out
}
func (s *Session) Token() query.ReviewToken { return s.token }

// Record associates a real planner receipt with one exact session operation.
// Duplicate registrations/IDs remain ambiguous, even when the receipts agree.
func (s *Session) Record(ref SourceReference, r query.ReviewReceipt) {
	if s == nil || !s.valid {
		return
	}
	if ref.session != s || ref.index < 0 || ref.index >= len(s.operations) {
		s.reason = "evidence_mismatch"
		return
	}
	op := &s.operations[ref.index]
	if ref.File != op.Source.File || ref.Offset != op.Source.Offset || ref.Version != op.Source.Version {
		op.Detail.Missing = append(op.Detail.Missing, "source_changed")
		op.Correspondence.Source = analysis.Mismatched
		return
	}
	if len(ref.ID) > 256 {
		s.reason = "budget_exceeded"
		return
	}
	if ref.ID != "" {
		if prev, ok := s.ids[ref.ID]; ok {
			s.operations[prev].Correspondence.Source = analysis.Mismatched
			op.Correspondence.Source = analysis.Mismatched
			s.operations[prev].Detail.Missing = append(s.operations[prev].Detail.Missing, "duplicate_identity")
			op.Detail.Missing = append(op.Detail.Missing, "duplicate_identity")
		}
		s.ids[ref.ID] = ref.index
	}
	if len(s.records[ref.index]) >= 2 {
		op.Correspondence.Source = analysis.Mismatched
		return
	}
	s.records[ref.index] = append(s.records[ref.index], r)
}

func newSession(opts Options, files []string) *Session {
	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}
	s := &Session{ctx: ctx, opts: opts, inputs: map[string][]byte{}, records: map[int][]query.ReviewReceipt{}, ids: map[string]int{}, valid: true, token: query.NewReviewToken(ctx)}
	s.manifestInput, _ = json.Marshal(opts.CurrentManifest)
	if len(s.manifestInput) > maxTypeInputBytes {
		s.reason = "budget_exceeded"
	}
	total := 0
	add := func(path string) {
		abs, err := filepath.Abs(path)
		if err != nil {
			s.reason = "source_changed"
			return
		}
		if _, ok := s.inputs[abs]; ok {
			return
		}
		st, err := os.Stat(abs)
		if err != nil || st.Size() > maxTypeFileBytes || len(s.inputs) >= maxTypeFiles || int64(total)+st.Size() > maxTypeInputBytes {
			s.reason = "budget_exceeded"
			return
		}
		b, err := os.ReadFile(abs)
		if err != nil || len(b) > maxTypeFileBytes {
			s.reason = "source_changed"
			return
		}
		total += len(b)
		s.inputs[abs] = b
	}
	for _, p := range []string{opts.ManifestPath, opts.ConfigPath} {
		if p != "" {
			add(p)
		}
	}
	for _, file := range files {
		if ctx.Err() != nil {
			s.reason = "canceled"
			break
		}
		add(file)
		root := moduleRoot(filepath.Dir(file))
		if root != "" {
			add(filepath.Join(root, "go.mod"))
			if _, err := os.Stat(filepath.Join(root, "go.sum")); err == nil {
				add(filepath.Join(root, "go.sum"))
			}
		}
	}
	return s
}
func (s *Session) unchanged() bool {
	if s.reason != "" || s.ctx.Err() != nil {
		return false
	}
	current, err := json.Marshal(s.opts.CurrentManifest)
	if err != nil || !bytes.Equal(current, s.manifestInput) {
		return false
	}
	for p, b := range s.inputs {
		st, err := os.Stat(p)
		if err != nil || st.Size() != int64(len(b)) {
			return false
		}
		now, err := os.ReadFile(p)
		if err != nil || sha256.Sum256(b) != sha256.Sum256(now) {
			return false
		}
	}
	return true
}
func (s *Session) add(path string, offset int, loc *query.SourceLocation, d analysis.Detail, expected query.ReviewReceipt) {
	s.operationCount++
	if len(s.operations) >= MaxReviewReceipts {
		s.reason = "budget_exceeded"
		return
	}
	abs, _ := filepath.Abs(path)
	_, ok := s.inputs[abs]
	if !ok {
		d.Missing = append(d.Missing, "source_changed")
	}
	root := moduleRoot(filepath.Dir(abs))
	rel, _ := filepath.Rel(root, abs)
	digest := s.inputVersion()
	ref := SourceReference{File: filepath.ToSlash(rel), Offset: offset, Version: hex.EncodeToString(digest[:]), session: s, index: len(s.operations)}
	s.operations = append(s.operations, OperationReview{Source: ref, Location: loc, Detail: d.Normalize(), Correspondence: analysis.Correspondence{}.Normalize(), expected: expected})
}
func (s *Session) finish(r *ReviewReport) {
	defer s.token.Close()
	defer func() { s.valid = false }()
	if s.opts.Evidence != nil && s.unchanged() {
		s.opts.Evidence(s)
	}
	unchanged := s.unchanged()
	for i := range s.operations {
		op := &s.operations[i]
		rs := s.records[i]
		switch {
		case !unchanged:
			op.Correspondence.Source = analysis.Mismatched
			reason := s.reason
			if reason == "" {
				reason = "source_changed"
			}
			if s.ctx.Err() != nil {
				reason = "canceled"
			}
			op.Detail.Missing = append(op.Detail.Missing, reason)
		case op.Correspondence.Source == analysis.Mismatched:
		// Preserve the original fixed refusal reason.
		case len(rs) > 1:
			op.Correspondence.Source = analysis.Mismatched
			op.Detail.Missing = append(op.Detail.Missing, "duplicate_identity")
		case len(rs) == 1:
			op.Correspondence = query.CompareReviewReceipts(s.token, op.expected, rs[0])
			if op.operation != "" {
				op.Correspondence.Operation = query.CompareReviewOperation(s.token, rs[0], op.operation)
			}
			if op.Correspondence.Source != analysis.Mismatched {
				op.Correspondence.Source = analysis.Matched
			}
			op.Detail.Reasons = append(op.Detail.Reasons, "planner_sample")
		default:
			op.Detail.Missing = append(op.Detail.Missing, "evidence_missing")
		}
		if op.Correspondence.Operation == analysis.Mismatched || op.Correspondence.Target == analysis.Mismatched || op.Correspondence.Structure == analysis.Mismatched || op.Correspondence.Arguments == analysis.Mismatched || op.Correspondence.Context == analysis.Mismatched || op.Correspondence.Source == analysis.Mismatched {
			op.Detail.Missing = append(op.Detail.Missing, "evidence_mismatch")
		}
		op.Correspondence = op.Correspondence.Normalize()
		op.Detail = op.Detail.Normalize()
	}
	for i := range s.operations {
		s.operations[i].expected = query.ReviewReceipt{}
		s.operations[i].Source.session = nil
	}
	r.Operations = s.operations
	r.OperationCount = s.operationCount

	if s.reason != "" {
		r.Findings = append(r.Findings, staticFinding(query.WarningStaticReviewUnsupported, query.RiskMedium, "review input coverage incomplete", "provide bounded current inputs", nil, query.AnalysisUnsupported))
	}
	// Finding gates use full internal analysis, never the bounded public view.
	apply := func(fs []Finding) {
		for i := range fs {
			f := &fs[i]
			for _, op := range r.Operations {
				if sameLocation(f.Location, op.Location) {
					f.Detail = op.Detail
					precision := query.AnalysisPrecision(op.Detail.Precision())
					if precisionRank(precision) > precisionRank(f.AnalysisPrecision) {
						f.AnalysisPrecision = precision
					}
					break
				}
			}
		}
	}
	apply(r.Findings)

	apply(r.SuppressedFindings)
	for i := range r.Operations {
		found, unsuppressed := false, false
		for _, fs := range [][]Finding{r.Findings, r.SuppressedFindings} {
			for _, f := range fs {
				if sameLocation(f.Location, r.Operations[i].Location) {
					found = true
					unsuppressed = unsuppressed || !f.Suppressed
				}
			}
		}
		r.Operations[i].Suppressed = found && !unsuppressed
	}
	r.privateOperations = cloneOperations(r.Operations)
}
func sameLocation(a, b *query.SourceLocation) bool {
	return a != nil && b != nil && a.File == b.File && a.Line == b.Line && a.Column == b.Column
}

func staticConstant(tf *typedFile, e ast.Expr) (any, bool) {
	if tf.info == nil {
		return nil, false
	}
	tv, ok := tf.info.Types[e]
	if !ok || tv.Value == nil {
		return nil, false
	}
	b, ok := types.Default(tv.Type).Underlying().(*types.Basic)
	if !ok {
		return nil, false
	}
	switch b.Kind() {
	case types.String:
		return constant.StringVal(tv.Value), true
	case types.Bool:
		return constant.BoolVal(tv.Value), true
	}
	n, ok := constant.Int64Val(tv.Value)
	if !ok {
		return nil, false
	}
	switch b.Kind() {
	case types.Int:
		return int(n), true
	case types.Int8:
		return int8(n), true
	case types.Int16:
		return int16(n), true
	case types.Int32:
		return int32(n), true
	case types.Int64:
		return n, true
	}
	return nil, false
}

// sampleSelect reconstructs a deliberately small generated SELECT domain. Unknown
// values are never filled from a test receipt. All other methods remain partial.
func (t *goTracker) sampleSelect(name string, calls []chainCall, partial bool) (analysis.Detail, query.ReviewReceipt) {
	d := analysis.Detail{SQL: analysis.Partial, Columns: analysis.Unknown, Bindings: analysis.Unknown, Policy: analysis.Unknown, Missing: []string{"runtime_policy", "database_specific"}}
	table, ok := tableFromChain(calls)
	if !ok {
		d.SQL = analysis.Unknown
		return d, query.ReviewReceipt{}
	}
	if _, exists := t.ctx.table(table); exists {
		d.Policy = analysis.Partial
		d.Reasons = append(d.Reasons, "declaration_only")
	}
	var dialect driver.Dialect = driver.MySQLDialect{}
	if t.ctx.session.opts.EvidenceDialect == "postgres" {
		dialect = driver.PostgresDialect{}
	}
	q := query.NewWithSettings(nil, table, dialect, t.ctx.session.opts.EvidenceSettings)
	supported := name == "Get" || name == "GetMaps" || name == "Plan"
	if !supported {
		d.Reasons = append(d.Reasons, "opaque_operation")
		return d, query.ReviewReceipt{}
	}
	bindings, columns := true, true
	for i := len(calls) - 1; i >= 1; i-- {
		c := calls[i]
		if c.Call == nil {
			supported = false
			continue
		}
		args := []any{}
		for _, e := range c.Call.Args {
			v, ok := staticConstant(t.tf, e)
			if !ok {
				args = nil
				break
			}
			args = append(args, v)
		}
		str := func(i int) (string, bool) {
			if i >= len(args) {
				return "", false
			}
			v, ok := args[i].(string)
			return v, ok
		}
		switch c.Method {
		case "Table":
		case "Select":
			names := []string{}
			for _, v := range args {
				s, ok := v.(string)
				if !ok {
					columns = false
				}
				names = append(names, s)
			}
			if len(args) != len(c.Call.Args) || c.Call.Ellipsis.IsValid() {
				columns = false
			}
			if columns {
				q.Select(names...)
			}
		case "Where", "OrWhere":
			col, ok := str(0)
			if !ok || len(args) != len(c.Call.Args) || len(args) < 2 || len(args) > 3 {
				bindings = false
				supported = false
				continue
			}
			if c.Method == "Where" {
				q.Where(col, args[1:]...)
			} else {
				q.OrWhere(col, args[1:]...)
			}
		case "WhereNull", "WhereNotNull":
			col, ok := str(0)
			if !ok || len(args) != 1 {
				supported = false
				continue
			}
			if c.Method == "WhereNull" {
				q.WhereNull(col)
			} else {
				q.WhereNotNull(col)
			}
		case "Limit", "Offset":
			if len(args) != 1 {
				supported = false
				bindings = false
				continue
			}
			n, ok := args[0].(int)
			if !ok {
				supported = false
				bindings = false
				continue
			}
			if c.Method == "Limit" {
				q.Limit(n)
			} else {
				q.Offset(n)
			}
		case "OrderBy":
			a, ok := str(0)
			b, ok2 := str(1)
			if !ok || !ok2 || len(args) != 2 {
				supported = false
			} else {
				q.OrderBy(a, b)
			}
		default:
			supported = false
			bindings = false
		}
	}
	if columns {
		d.Columns = analysis.Known
	}
	if bindings {
		d.Bindings = analysis.Known
	} else {
		d.Missing = append(d.Missing, "dynamic_value")
	}
	if partial {
		supported = false
		d.Columns = analysis.Partial
		d.Bindings = analysis.Unknown
		d.Missing = append(d.Missing, "static_unknown")
	}
	if !columns {
		supported = false
	}
	if !supported {
		d.Reasons = append(d.Reasons, "opaque_operation")
		return d, query.ReviewReceipt{}
	}
	d.SQL = analysis.Known
	// Application-test settings are declaration inputs, not static values from source.
	settings := t.ctx.session.opts.EvidenceSettings
	if len(settings.PolicySet().Policies()) > 0 || settings.ExecutionContext().Input().TenantPresent {
		d.SQL = analysis.Partial
		d.Bindings = analysis.Unknown
		d.Reasons = append(d.Reasons, "declaration_only")
		d.Missing = append(d.Missing, "dynamic_value")
	}
	if t.ctx.session.opts.EvidenceDialect != "mysql" && t.ctx.session.opts.EvidenceDialect != "postgres" {
		return d, query.ReviewReceipt{}
	}
	receipt, err := q.ReviewReceipt(t.ctx.session.ctx, t.ctx.session.token)
	if err != nil {
		d.Missing = append(d.Missing, "evidence_missing")
	}
	return d, receipt
}

func (s *Session) fallback(fs []Finding) {
	if len(s.operations) >= MaxReviewReceipts {
		return
	}
	// Loader/opaque/raw findings remain represented even without a tracked chain.
	seen := map[string]bool{}
	for _, op := range s.operations {
		if op.Location != nil {
			seen[locationKey(op.Location)] = true
		}
	}
	for _, f := range fs {
		if f.Location == nil || seen[locationKey(f.Location)] {
			continue
		}
		seen[locationKey(f.Location)] = true
		d := analysis.Detail{Reasons: []string{"static_unknown"}, Missing: []string{"runtime_policy", "evidence_missing"}}
		if ext := filepath.Ext(f.Location.File); ext == ".json" || ext == ".sql" {
			d.Reasons = []string{"unverified_input"}
		}
		if f.AnalysisPrecision == query.AnalysisUnsupported {
			d.SQL = analysis.Unsupported
			if len(f.Detail.Reasons) > 0 {
				d.Reasons = f.Detail.Reasons
			} else if filepath.Ext(f.Location.File) != ".json" {
				d.Reasons = []string{"opaque_operation"}
			}
		} else {
			d.SQL = analysis.Partial
		}
		s.add(f.Location.File, 0, f.Location, d, query.ReviewReceipt{})
	}
}
func locationKey(l *query.SourceLocation) string {
	return l.File + ":" + strconv.Itoa(l.Line) + ":" + strconv.Itoa(l.Column)
}

func cloneOperations(src []OperationReview) []OperationReview {
	out := append([]OperationReview(nil), src...)
	for i := range out {
		out[i].Detail.Reasons = append([]string(nil), src[i].Detail.Reasons...)
		out[i].Detail.Missing = append([]string(nil), src[i].Detail.Missing...)
		if src[i].Location != nil {
			loc := *src[i].Location
			out[i].Location = &loc
		}
	}
	return out
}

func (s *Session) inputVersion() [32]byte {
	if s.versionSet {
		return s.version
	}
	paths := make([]string, 0, len(s.inputs))
	for p := range s.inputs {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	h := sha256.New()
	for _, p := range paths {
		h.Write([]byte(p))
		h.Write([]byte{0})
		sum := sha256.Sum256(s.inputs[p])
		h.Write(sum[:])
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	s.version = out
	s.versionSet = true
	return out
}

func (*Session) MarshalJSON() ([]byte, error) { return nil, query.ErrReviewReceipt }
func (*Session) Format(s fmt.State, _ rune)   { _, _ = s.Write([]byte("<review session>")) }
