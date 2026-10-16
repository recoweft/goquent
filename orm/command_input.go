package orm

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/recoweft/goquent/orm/internal/writecheck"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"unicode/utf8"

	"github.com/recoweft/goquent/orm/internal/querybridge"
	"github.com/recoweft/goquent/orm/internal/writeinput"
)

// Command limits are independent of the read-only OperationSpec contract.
const (
	CommandMaxBytes   = 1 << 20
	CommandMaxSteps   = 64
	CommandMaxColumns = 256
	commandMaxNodes   = 16384
)

var (
	ErrCommandInput            = errors.New("goquent: unsupported command input")
	ErrCommandIdentityConflict = errors.New("goquent: command identity conflict")
	ErrCommandStore            = errors.New("goquent: invalid command store")
	ErrCommandCorrupt          = errors.New("goquent: corrupt or unsupported command record")
	ErrCommandOutcomeUnknown   = errors.New("goquent: command outcome unknown; reconcile with the same ID and input")
	ErrCommandOwnership        = errors.New("goquent: command requires an owned database")
)

// CommandStep is one literal INSERT or equality-key UPDATE. Values and Keys
// accept only the scalar domain documented in docs/command-identity.md.
// Keys are mandatory for updates and forbidden for inserts. No SQL is accepted.
// CommandVersion is a finite UPDATE comparison and increment.
type CommandVersion struct {
	Column   string
	Expected int64
}

type CommandStep struct {
	ExpectAffected *int64
	Version        *CommandVersion
	Operation      string
	Table          string
	Values         map[string]any
	Keys           map[string]any
}

// Command identifies an ordered, bounded write request. Keep this ID and input
// durably before calling Run; never replace the ID after an ambiguous response.
type Command struct {
	ID    string
	Kind  string
	Steps []CommandStep
}

// CommandSummary contains actual driver row counts, not hydrated rows or IDs.
type CommandSummary struct {
	Version      int     `json:"version"`
	RowsAffected []int64 `json:"rows_affected"`
}

// CommandResult returns the persisted summary. Applied is false for a replay.
// This source result is not a redacted public display artifact.
type CommandResult struct {
	Summary CommandSummary
	Applied bool
}

var commandIdent = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
var commandLabel = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)
var commandUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
var commandInteger = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)

type commandAtom struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}
type commandField struct {
	Name string      `json:"name"`
	Atom commandAtom `json:"atom"`
}
type commandCanonicalStep struct {
	Operation    string
	Table        string
	Values, Keys []commandField
}
type commandInput struct {
	Version int
	Kind    string
	Steps   []commandCanonicalStep
}

// commandScalar never calls user methods. The retained value is immutable.
func commandScalar(v any) (commandAtom, any, error) {
	a := commandAtom{}
	switch x := v.(type) {
	case nil:
		a.Type = "null"
	case string:
		if len(x) > CommandMaxBytes || !utf8.ValidString(x) {
			return a, nil, ErrCommandInput
		}
		a.Type = "string"
		a.Value = x
	case bool:
		a.Type = "bool"
		a.Value = strconv.FormatBool(x)
	case int, int8, int16, int32, int64:
		a.Type = reflect.TypeOf(v).String()
		a.Value = strconv.FormatInt(reflect.ValueOf(v).Int(), 10)
	case uint, uint8, uint16, uint32, uint64:
		n := reflect.ValueOf(v).Uint()
		if n > math.MaxInt64 {
			return a, nil, ErrCommandInput
		}
		a.Type = reflect.TypeOf(v).String()
		a.Value = strconv.FormatUint(n, 10)
	case json.Number:
		if len(x) > 20 || !commandInteger.MatchString(string(x)) {
			return a, nil, ErrCommandInput
		}
		if _, err := strconv.ParseInt(string(x), 10, 64); err != nil {
			return a, nil, ErrCommandInput
		}
		a.Type = "jsonint"
		a.Value = string(x)
	default:
		return a, nil, ErrCommandInput
	}
	return a, v, nil
}

func commandFields(in map[string]any, budget *int, nodes *int) ([]commandField, map[string]any, error) {
	if len(in) > CommandMaxColumns {
		return nil, nil, ErrCommandInput
	}
	names := make([]string, 0, len(in))
	for k := range in {
		if !commandIdent.MatchString(k) {
			return nil, nil, ErrCommandInput
		}
		names = append(names, k)
	}
	sort.Strings(names)
	fields := make([]commandField, 0, len(in))
	values := make(map[string]any, len(in))
	for _, k := range names {
		a, v, err := commandScalar(in[k])
		if err != nil {
			return nil, nil, err
		}
		*budget += len(k) + len(a.Value) + len(a.Type)
		*nodes++
		if *budget > CommandMaxBytes || *nodes > commandMaxNodes {
			return nil, nil, ErrCommandInput
		}
		fields = append(fields, commandField{k, a})
		values[k] = v
	}
	return fields, values, nil
}

func snapshotCommand(c Command, schema string) ([]byte, []querybridge.Request, error) {
	if !commandUUID.MatchString(c.ID) || !commandLabel.MatchString(c.Kind) || len(c.Steps) == 0 || len(c.Steps) > CommandMaxSteps {
		return nil, nil, ErrCommandInput
	}
	canonical := commandInput{Version: 1, Kind: c.Kind}
	v2 := commandInputV2{Version: 2, Kind: c.Kind}
	extended := false
	requests := make([]querybridge.Request, 0, len(c.Steps))
	budget, nodes := len(c.Kind), 0
	for _, s := range c.Steps {
		if !commandIdent.MatchString(s.Table) || s.Table == commandTable || len(s.Values) == 0 || (s.Operation != "insert" && s.Operation != "update") || (s.Operation == "insert" && len(s.Keys) != 0) || (s.Operation == "update" && len(s.Keys) == 0) {
			return nil, nil, ErrCommandInput
		}
		vf, values, err := commandFields(s.Values, &budget, &nodes)
		if err != nil {
			return nil, nil, err
		}
		kf, keys, err := commandFields(s.Keys, &budget, &nodes)
		if err != nil {
			return nil, nil, err
		}
		result := writecheck.Options{Expected: s.ExpectAffected}.Clone()
		if s.Version != nil {
			result.Version = &writecheck.Version{Column: s.Version.Column, Expected: s.Version.Expected}
		}
		if v := result.Version; v != nil {
			_, inValues := values[v.Column]
			_, inKeys := keys[v.Column]
			if !commandIdent.MatchString(v.Column) || v.Expected == math.MaxInt64 || inValues || inKeys {
				return nil, nil, ErrCommandInput
			}
		}
		if result.Validate() != nil || (result.Version != nil && s.Operation != "update") {
			return nil, nil, ErrCommandInput
		}
		extended = extended || result.Expected != nil || result.Version != nil
		r := querybridge.Request{Results: result, Operation: s.Operation, Table: schema + "." + s.Table, Rows: []map[string]any{values}, Options: writeinput.Options{Literal: true, TableParts: []string{schema, s.Table}}}
		for _, v := range vf {
			r.Options.Columns = append(r.Options.Columns, v.Name)
		}
		for _, k := range kf {
			if keys[k.Name] == nil {
				return nil, nil, ErrCommandInput
			}
			r.WhereColumns = append(r.WhereColumns, k.Name)
			r.WhereValues = append(r.WhereValues, keys[k.Name])
		}
		canonical.Steps = append(canonical.Steps, commandCanonicalStep{s.Operation, s.Table, vf, kf})
		v2.Steps = append(v2.Steps, commandCanonicalStepV2{s.Operation, s.Table, vf, kf, result.Expected, result.Version})
		requests = append(requests, r)
	}
	var encoded any = canonical
	if extended {
		encoded = v2
	}
	b, err := json.Marshal(encoded)
	if err != nil || len(b) > CommandMaxBytes {
		return nil, nil, ErrCommandInput
	}
	return b, requests, nil
}

// Input versions do not change the ledger envelope, scope, or result version.
type commandCanonicalStepV2 struct {
	Operation      string
	Table          string
	Values, Keys   []commandField
	ExpectAffected *int64
	Version        *writecheck.Version
}
type commandInputV2 struct {
	Version int
	Kind    string
	Steps   []commandCanonicalStepV2
}

func commandFingerprint(input []byte) ([32]byte, error) {
	var zero [32]byte
	if len(input) > CommandMaxBytes {
		return zero, ErrCommandCorrupt
	}
	var header struct{ Version int }
	if json.Unmarshal(input, &header) != nil {
		return zero, ErrCommandCorrupt
	}
	var normalized []byte
	var err error
	c := Command{ID: "00000000-0000-4000-8000-000000000001"}
	switch header.Version {
	case 1:
		var v commandInput
		if json.Unmarshal(input, &v) != nil {
			return zero, ErrCommandCorrupt
		}
		c.Kind = v.Kind
		for _, s := range v.Steps {
			values, e := commandDecodeFields(s.Values)
			if e != nil {
				return zero, e
			}
			keys, e := commandDecodeFields(s.Keys)
			if e != nil {
				return zero, e
			}
			c.Steps = append(c.Steps, CommandStep{Operation: s.Operation, Table: s.Table, Values: values, Keys: keys})
		}
	case 2:
		var v commandInputV2
		if json.Unmarshal(input, &v) != nil {
			return zero, ErrCommandCorrupt
		}
		c.Kind = v.Kind
		for _, s := range v.Steps {
			values, e := commandDecodeFields(s.Values)
			if e != nil {
				return zero, e
			}
			keys, e := commandDecodeFields(s.Keys)
			if e != nil {
				return zero, e
			}
			step := CommandStep{Operation: s.Operation, Table: s.Table, Values: values, Keys: keys, ExpectAffected: s.ExpectAffected}
			if s.Version != nil {
				step.Version = &CommandVersion{Column: s.Version.Column, Expected: s.Version.Expected}
			}
			c.Steps = append(c.Steps, step)
		}
	default:
		return zero, ErrCommandCorrupt
	}
	normalized, _, err = snapshotCommand(c, "public")
	if err != nil || !bytes.Equal(input, normalized) {
		return zero, ErrCommandCorrupt
	}
	domain := "goquent.command.input.v1\x00"
	if header.Version == 2 {
		domain = "goquent.command.input.v2\x00"
	}
	return sha256.Sum256(append([]byte(domain), input...)), nil
}
func commandDecodeFields(fields []commandField) (map[string]any, error) {
	out := make(map[string]any, len(fields))
	for _, f := range fields {
		var value any
		a := f.Atom
		switch a.Type {
		case "null":
			value = nil
		case "string":
			value = a.Value
		case "bool":
			n, e := strconv.ParseBool(a.Value)
			if e != nil {
				return nil, ErrCommandCorrupt
			}
			value = n
		case "jsonint":
			value = json.Number(a.Value)
		default:
			types := map[string]reflect.Type{"int": reflect.TypeOf(int(0)), "int8": reflect.TypeOf(int8(0)), "int16": reflect.TypeOf(int16(0)), "int32": reflect.TypeOf(int32(0)), "int64": reflect.TypeOf(int64(0)), "uint": reflect.TypeOf(uint(0)), "uint8": reflect.TypeOf(uint8(0)), "uint16": reflect.TypeOf(uint16(0)), "uint32": reflect.TypeOf(uint32(0)), "uint64": reflect.TypeOf(uint64(0))}
			typ, ok := types[a.Type]
			if !ok {
				return nil, ErrCommandCorrupt
			}
			v := reflect.New(typ).Elem()
			if a.Type[0] == 'u' {
				n, e := strconv.ParseUint(a.Value, 10, typ.Bits())
				if e != nil {
					return nil, ErrCommandCorrupt
				}
				v.SetUint(n)
			} else {
				n, e := strconv.ParseInt(a.Value, 10, typ.Bits())
				if e != nil {
					return nil, ErrCommandCorrupt
				}
				v.SetInt(n)
			}
			value = v.Interface()
		}
		atom, _, e := commandScalar(value)
		if e != nil || atom != a {
			return nil, ErrCommandCorrupt
		}
		if _, exists := out[f.Name]; exists {
			return nil, ErrCommandCorrupt
		}
		out[f.Name] = value
	}
	return out, nil
}
