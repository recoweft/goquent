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
func typedValidate(spec OperationSpec, opts Options, r *validationResult, d *diagnosticRecorder) (failure error) {
	defer func() {
		if failure != nil {
			failure = &validationFailure{cause: failure, checks: append([]querybridge.OperationCheck(nil), r.typed.Checks...)}
		}
	}()

	d.at("spec", "limit", "", -1, "")
	d.expect("INPUT_LIMIT")
	if spec.Limit != nil && (*spec.Limit < 0 || *spec.Limit > 10000) {
		return ErrInputLimit
	}
	d.at("values", "root", "", -1, "")
	d.expect("RESERVED_BINDING")
	for k := range opts.Values {
		if strings.EqualFold(strings.TrimSpace(k), "current_tenant") {
			return ErrReservedBinding
		}
	}
	d.at("manifest", "root", "", -1, "")
	d.expect("MANIFEST_INVALID")
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
	d.at("settings", "implicit", "", -1, tenant)
	d.target(r.table.Name, tenant)
	d.expect("RESERVED_BINDING")
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
	checkColumn := columnChecker(opts, r, d)
	for i, f := range spec.Select {
		d.at("spec", "select", "", i, f)
		if err := checkColumn(f, -1, "", nil, false, false, ""); err != nil {
			return err
		}
	}
	for i, o := range spec.OrderBy {
		d.at("spec", "order_by", "field", i, o.Field)
		if err := checkColumn(o.Field, -1, "", nil, false, false, ""); err != nil {
			return err
		}
	}
	for i, f := range spec.Filters {
		d.at("spec", "filters", "value", i, f.Field)
		d.target(r.table.Name, f.Field)
		d.expect("ARITY_INVALID")
		op := normalizeFilterOp(f.Op)
		vp, rp := f.hasValue(), f.hasRef()
		d.current.ValuePresent = vp
		d.current.RefPresent = rp
		d.current.Missing = "arity"
		if rp {
			d.current.Member = "value_ref"
		}
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
		d.current.Missing = ""
		d.expect("RESERVED_BINDING")
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
			d.expect("VALUE_REF_MISSING")
			d.current.Missing = "reference_unresolved"
			var ok bool
			v, ok = opts.Values[f.ValueRef]
			if !ok {
				return ErrValueRefMissing
			}
			provenance = "values"
			d.current.Missing = ""
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
		d.at("spec", "implicit", "field", -1, tenant)
		d.target(r.table.Name, tenant)
		d.expect("REQUIRED_FILTER_MISSING")
		if !querybridge.AutomaticTenant(*opts.Settings) {
			return ErrRequiredFilterMissing
		}
		if err := checkColumn(tenant, -1, "=", current, false, true, "application_automatic"); err != nil {
			return err
		}
	}
	d.at("manifest", "implicit", "field", -1, "")
	d.expect("MANIFEST_INVALID")
	soft := tableSoftDeleteColumn(r.table)
	if policy.SoftDeleteColumn != "" {
		if soft != "" && soft != policy.SoftDeleteColumn {
			return ErrInvalidManifest
		}
		soft = policy.SoftDeleteColumn
	}
	if soft != "" && !specHasFilter(spec, soft) {
		d.at("spec", "implicit", "field", -1, soft)
		if err := checkColumn(soft, -1, "is_null", nil, false, false, "implicit_soft_delete"); err != nil {
			return err
		}
	}
	return nil
}

// columnChecker is shared by predicates, declarations and update assignments.
func columnChecker(opts Options, r *validationResult, d *diagnosticRecorder) func(string, int, string, any, bool, bool, string) error {
	dialect := knownDialect(opts)
	checked := map[string]bool{}
	return func(field string, pos int, op string, v any, valuePresent, refPresent bool, provenance string) error {
		d.typedReached = true
		d.target(r.table.Name, field)
		c, err := diagnosticField(r.columns, field, d)
		if err != nil {
			return err
		}
		// Only the actual table qualifier is meaningful in a single-model operation.
		if field != c.Name && field != r.table.Name+"."+c.Name {
			return ErrUnknownField
		}
		d.expect("MANIFEST_INVALID")
		d.current.Declaration = c.Type
		d.current.TypeSource = c.TypeSource
		d.current.NullableKnown = c.NullableKnown
		d.current.Nullable = c.Nullable
		d.current.ValuePresent = valuePresent
		d.current.RefPresent = refPresent
		t, err := parseColumnType(c, dialect)
		if err != nil {
			return err
		}
		d.current.Bits = t.bits
		d.current.Unsigned = t.unsigned
		d.current.Precision = t.precision
		d.current.Scale = t.scale
		d.current.Length = t.length
		d.current.Fraction = t.fraction
		d.current.Evidence = "supplied_declaration_not_live"
		check := querybridge.OperationCheck{DriverStatus: "builtin_domain_only_not_live", BindingStatus: "separate_private_gate", Position: pos, Field: c.Name, TypeSource: c.TypeSource, Type: c.Type, Checked: t.kind, Missing: t.missing, Provenance: provenance, ValuePresent: valuePresent, RefPresent: refPresent}
		defer func() { r.typed.Checks = append(r.typed.Checks, check) }()
		if op == "is_null" || op == "is_not_null" {
			if !c.NullableKnown || c.TypeSource != "sql" {
				check.Missing = "nullability_unknown"
			}
			check.Checked = "nullability"
		}
		if check.Missing != "" {
			d.partial = true
			d.current.Missing = check.Missing
			d.expect("TYPE_UNVERIFIED")
			d.add("OPERATION_TYPE_UNVERIFIED", "unverified", "", check.Missing, "supply_supported_explicit_declarations")
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
				d.expect("ARRAY_UNSUPPORTED")
				d.current.Missing = "array_binding_unsupported"
				return ErrArrayBinding
			}
			if op == "in" {
				d.expect("FILTER_INVALID")
				d.current.Missing = "in_requires_1_to_1000_scalars"
				list := reflect.ValueOf(v)
				if !list.IsValid() || list.Kind() != reflect.Slice || list.Len() < 1 || list.Len() > 1000 {
					return ErrInvalidFilter
				}
				d.expect("TYPE_MISMATCH")
				d.current.Missing = "value_does_not_satisfy_declaration"
				for i := 0; i < list.Len(); i++ {
					d.current.ElementKnown = true
					d.current.Element = i
					if err := validateScalar(c, t, dialect, op, list.Index(i).Interface()); err != nil {
						return err
					}
				}
				d.current.ElementKnown = false
				d.current.Element = 0
			} else {
				d.expect("TYPE_MISMATCH")
				d.current.Missing = "value_does_not_satisfy_declaration"
				if err := validateScalar(c, t, dialect, op, v); err != nil {
					return err
				}
			}
			d.current.Missing = ""
			if t.kind != "" {
				d.add("OPERATION_CHECK_TYPE", "checked", "scalar_type_and_declared_constraints", "", "")
			}
			if len(c.EnumValues) > 0 {
				d.add("OPERATION_CHECK_ENUM", "checked", "exact_declared_enum_membership", "", "")
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
				d.partial = true
				d.expect("TYPE_UNVERIFIED")
				d.current.Missing = "driver_unsigned_domain_unsupported"
				d.add("OPERATION_UNVERIFIED_DRIVER", "unverified", "", "driver_unsigned_domain_unsupported", "use_supported_driver_domain")
				check.DriverStatus = "unsigned_domain_unsupported"
				check.Missing = "driver_unsigned_domain_unsupported"
				if opts.Settings.IsStrict() {
					return ErrTypeUnverified
				}
				r.typed.Unknown = append(r.typed.Unknown, check.Missing)
			}
			if unbound {
				d.partial = true
				d.expect("TYPE_UNVERIFIED")
				d.current.Missing = "binding_numeric_domain_unsupported"
				d.add("OPERATION_UNVERIFIED_BINDING", "unverified", "", "binding_numeric_domain_unsupported", "use_supported_private_binding_domain")
				check.BindingStatus = "numeric_domain_unsupported"
				check.Missing = "binding_numeric_domain_unsupported"
				if opts.Settings.IsStrict() {
					return ErrTypeUnverified
				}
				r.typed.Unknown = append(r.typed.Unknown, check.Missing)
			}
			if dialect == "" {
				d.partial = true
				d.add("OPERATION_UNVERIFIED_DRIVER", "unverified", "", "dialect_unknown", "supply_supported_dialect")
			} else if !unbound && !unsupported {
				d.add("OPERATION_CHECK_BINDING", "checked", "existing_numeric_and_builtin_driver_subset_only", "", "")
			}
		} else if op == "" && t.kind != "" {
			d.add("OPERATION_CHECK_TYPE", "checked", "declaration_grammar_only", "", "")
		} else if (op == "is_null" || op == "is_not_null") && c.NullableKnown && c.TypeSource == "sql" {
			d.add("OPERATION_CHECK_NULLABILITY", "checked", "supplied_sql_nullability_presence", "", "")
		}
		return nil
	}
}
