package operation

import (
	"encoding/json"
	"reflect"
	"strings"

	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/internal/querybridge"
	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/query"
)

func operationSettings(opts Options) Options {
	if opts.Settings == nil {
		s := query.SnapshotDefaultSettings()
		opts.Settings = &s
	} else {
		s := *opts.Settings
		opts.Settings = &s
	}
	return opts
}
func knownDialect(opts Options) string {
	declared := opts.Manifest.Dialect
	actual := ""
	if opts.Dialect == nil {
		if declared == "mysql" || declared == "postgres" {
			return declared
		}
		return ""
	}
	switch opts.Dialect.(type) {
	case driver.MySQLDialect:
		actual = "mysql"
	case driver.PostgresDialect:
		actual = "postgres"
	}
	if declared != "" && declared != actual {
		return ""
	}
	return actual
}
func checkManifestDeclarations(m *manifest.Manifest) error {
	if m.Version != manifest.Version {
		return ErrInvalidManifest
	}
	seen := map[string]bool{}
	for _, t := range m.Tables {
		k := normalizeName(t.Name)
		if k == "" || seen[k] {
			return ErrInvalidManifest
		}
		seen[k] = true
		columns := map[string]bool{}
		for _, c := range t.Columns {
			k := normalizeName(c.Name)
			if k == "" || columns[k] || c.TypeSource != "" && c.TypeSource != "sql" && c.TypeSource != "go" {
				return ErrInvalidManifest
			}
			columns[k] = true
		}
	}
	return nil
}
func typedValidate(spec OperationSpec, opts Options, r *validationResult) (failure error) {
	defer func() {
		if failure != nil {
			failure = &validationFailure{cause: failure, checks: append([]querybridge.OperationCheck(nil), r.typed.Checks...)}
		}
	}()

	dialect := knownDialect(opts)
	if spec.Limit != nil && (*spec.Limit < 0 || *spec.Limit > 10000) {
		return ErrInputLimit
	}
	for k := range opts.Values {
		if strings.EqualFold(strings.TrimSpace(k), "current_tenant") {
			return ErrReservedBinding
		}
	}
	policy, _ := opts.Settings.PolicySet().PolicyForTable(r.table.Name)
	tenant := ""
	addTenant := func(c string) bool {
		if c == "" {
			return false
		}
		if tenant != "" && tenant != c {
			return false
		}
		tenant = c
		return true
	}
	for _, c := range r.table.Columns {
		if c.TenantScope && !addTenant(c.Name) {
			return ErrInvalidManifest
		}
	}
	for _, p := range r.table.Policies {
		if p.Type == "tenant_scope" && !addTenant(p.Column) {
			return ErrInvalidManifest
		}
	}
	var current any
	if tenant != "" || policy.TenantColumn != "" {
		if tenant != policy.TenantColumn || !opts.Settings.IsStrict() {
			return ErrReservedBinding
		}
		ctx, err := opts.Settings.ApplicationTenantContext()
		if err != nil {
			return ErrReservedBinding
		}
		current = ctx.Input().CurrentTenant
	}
	r.tenant = tenant
	checked := map[string]bool{}
	checkColumn := func(field string, pos int, op string, v any, valuePresent, refPresent bool, provenance string) error {
		c, err := validateField(r.columns, field)
		if err != nil {
			return err
		}
		// Only the actual table qualifier is meaningful in a single-model operation.
		if field != c.Name && field != r.table.Name+"."+c.Name {
			return ErrUnknownField
		}
		t, err := parseColumnType(c, dialect)
		if err != nil {
			return err
		}
		check := querybridge.OperationCheck{DriverStatus: "builtin_domain_only_not_live", BindingStatus: "separate_private_gate", Position: pos, Field: c.Name, TypeSource: c.TypeSource, Type: c.Type, Checked: t.kind, Missing: t.missing, Provenance: provenance, ValuePresent: valuePresent, RefPresent: refPresent}
		defer func() { r.typed.Checks = append(r.typed.Checks, check) }()
		if op == "is_null" || op == "is_not_null" {
			if !c.NullableKnown || c.TypeSource != "sql" {
				check.Missing = "nullability_unknown"
			}
			check.Checked = "nullability"
		}
		if check.Missing != "" {
			if opts.Settings.IsStrict() {
				return ErrTypeUnverified
			}
			if !checked[check.Missing] {
				r.typed.Unknown = append(r.typed.Unknown, check.Missing)
				checked[check.Missing] = true
			}
		}
		if op != "" && op != "is_null" && op != "is_not_null" {
			if t.array {
				return ErrArrayBinding
			}
			if op == "in" {
				list := reflect.ValueOf(v)
				if !list.IsValid() || list.Kind() != reflect.Slice || list.Len() < 1 || list.Len() > 1000 {
					return ErrInvalidFilter
				}
				for i := 0; i < list.Len(); i++ {
					if err := validateScalar(c, t, dialect, op, list.Index(i).Interface()); err != nil {
						return err
					}
				}
			} else if err := validateScalar(c, t, dialect, op, v); err != nil {
				return err
			}
			// This preserves the existing private canonical domain. Decimal validation
			// alone must not turn a noncanonical argument into a bindable identity.
			inspectNumber := func(x any) bool { n, ok := x.(json.Number); return ok && !intLexeme.MatchString(string(n)) }
			unbound := inspectNumber(v)
			if op == "in" {
				list := reflect.ValueOf(v)
				for i := 0; i < list.Len(); i++ {
					unbound = unbound || inspectNumber(list.Index(i).Interface())
				}
			}
			driverUnsupported := func(x any) bool {
				if dialect != "postgres" {
					return false
				}
				switch n := x.(type) {
				case uint:
					return uint64(n) > uint64(1<<63-1)
				case uint64:
					return n > uint64(1<<63-1)
				}
				return false
			}
			unsupported := driverUnsupported(v)
			if op == "in" {
				list := reflect.ValueOf(v)
				for i := 0; i < list.Len(); i++ {
					unsupported = unsupported || driverUnsupported(list.Index(i).Interface())
				}
			}
			if unsupported {
				check.DriverStatus = "unsigned_domain_unsupported"
				check.Missing = "driver_unsigned_domain_unsupported"
				if opts.Settings.IsStrict() {
					return ErrTypeUnverified
				}
				r.typed.Unknown = append(r.typed.Unknown, check.Missing)
			}
			if unbound {
				check.BindingStatus = "numeric_domain_unsupported"
				check.Missing = "binding_numeric_domain_unsupported"
				if opts.Settings.IsStrict() {
					return ErrTypeUnverified
				}
				r.typed.Unknown = append(r.typed.Unknown, check.Missing)
			}
		}
		return nil
	}
	for _, f := range spec.Select {
		if err := checkColumn(f, -1, "", nil, false, false, ""); err != nil {
			return err
		}
	}
	for _, o := range spec.OrderBy {
		if err := checkColumn(o.Field, -1, "", nil, false, false, ""); err != nil {
			return err
		}
	}
	for i, f := range spec.Filters {
		op := normalizeFilterOp(f.Op)
		vp, rp := f.hasValue(), f.hasRef()
		if op == "is_null" || op == "is_not_null" {
			if vp || rp {
				r.typed.Checks = append(r.typed.Checks, querybridge.OperationCheck{Position: i, Field: f.Field, Missing: "arity", ValuePresent: vp, RefPresent: rp})
				return ErrInvalidFilter
			}
		} else {
			if vp == rp || rp && strings.TrimSpace(f.ValueRef) == "" {
				r.typed.Checks = append(r.typed.Checks, querybridge.OperationCheck{Position: i, Field: f.Field, Missing: "arity", ValuePresent: vp, RefPresent: rp})
				return ErrInvalidFilter
			}
		}
		v := f.Value
		provenance := "literal"
		reserved := strings.EqualFold(strings.TrimSpace(f.ValueRef), "current_tenant")
		if normalizeName(f.Field) == normalizeName(tenant) && tenant != "" {
			if !rp || f.ValueRef != "current_tenant" || op != "=" {
				return ErrReservedBinding
			}
		}
		if reserved {
			if f.ValueRef != "current_tenant" || tenant == "" || normalizeName(f.Field) != normalizeName(tenant) || op != "=" {
				return ErrReservedBinding
			}
			v = current
			provenance = "application"
		} else if rp {
			var ok bool
			v, ok = opts.Values[f.ValueRef]
			if !ok {
				return ErrValueRefMissing
			}
			provenance = "values"
		}
		if err := checkColumn(f.Field, i, op, v, vp, rp, provenance); err != nil {
			return err
		}
		f.Value = v
		f.ValueRef = ""
		f.refPresent = false
		f.ValuePresent = op != "is_null" && op != "is_not_null"
		if !(tenant != "" && normalizeName(f.Field) == normalizeName(tenant) && querybridge.AutomaticTenant(*opts.Settings)) {
			r.filters = append(r.filters, f)
		}
	}
	if tenant != "" && !specHasFilter(spec, tenant) {
		if !querybridge.AutomaticTenant(*opts.Settings) {
			return ErrRequiredFilterMissing
		}
		if err := checkColumn(tenant, -1, "=", current, false, true, "application_automatic"); err != nil {
			return err
		}
	}
	soft := tableSoftDeleteColumn(r.table)
	if policy.SoftDeleteColumn != "" {
		if soft != "" && soft != policy.SoftDeleteColumn {
			return ErrInvalidManifest
		}
		soft = policy.SoftDeleteColumn
	}
	if soft != "" && !specHasFilter(spec, soft) {
		if err := checkColumn(soft, -1, "is_null", nil, false, false, "implicit_soft_delete"); err != nil {
			return err
		}
	}
	return nil
}
