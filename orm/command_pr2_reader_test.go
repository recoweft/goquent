package orm

// Frozen PR2 apply/lookup, canonical snapshot and fingerprint logic from
// 0f9359bb. This executable fixture shares unchanged scalar/planner helpers;
// it is not an independently built historical binary.
import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/recoweft/goquent/orm/internal/querybridge"
	"github.com/recoweft/goquent/orm/internal/writecheck"
	"github.com/recoweft/goquent/orm/internal/writeinput"
	"math"
)

type pr2ReaderFixture struct{ *CommandStore }

func (s *pr2ReaderFixture) apply(ctx context.Context, id string, input []byte, fingerprint [32]byte, plan *compoundPlan) (CommandResult, error) {
	var zero CommandResult
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return zero, err
	}
	// A panic before finalization must not leave a live transaction behind.
	defer tx.Rollback()
	rollback := func(cause error) (CommandResult, error) {
		if e := tx.Rollback(); e != nil {
			return zero, errors.Join(ErrCommandOutcomeUnknown, cause, e)
		}
		return zero, cause
	}
	if err = plan.rebind(ctx, tx.DB); err != nil {
		return rollback(err)
	}
	reservation := s.sql("INSERT INTO " + s.table + " (scope_hash, operation_id, format_version, canonical_scope, command_input, fingerprint, status, result) VALUES (?, ?, 1, ?, ?, ?, 0, ?)")
	res, err := tx.DB.exec.ExecContext(ctx, reservation, s.scopeHash[:], []byte(id), s.scope, input, fingerprint[:], []byte{})
	if err != nil {
		_, re := rollback(err)
		if errors.Is(re, ErrCommandOutcomeUnknown) {
			return zero, re
		}
		if !s.reservationDuplicate(err) {
			return zero, err
		}
		out, e := s.lookup(ctx, id, input, fingerprint, len(plan.steps))
		if errors.Is(e, sql.ErrNoRows) {
			return zero, ErrCommandOutcomeUnknown
		}
		return out, e
	}
	if err = commandOne(res); err != nil {
		return rollback(err)
	}
	summary := CommandSummary{Version: 1, RowsAffected: make([]int64, 0, len(plan.steps))}
	for _, step := range plan.steps {
		res, err = step.plan.Exec()
		if err != nil {
			return rollback(err)
		}
		n, e := res.RowsAffected()
		if e != nil {
			return rollback(e)
		}
		if n < 0 {
			return rollback(ErrCommandCorrupt)
		}
		summary.RowsAffected = append(summary.RowsAffected, n)
	}
	encoded, err := json.Marshal(summary)
	if err != nil || len(encoded) > CommandMaxBytes {
		return rollback(ErrCommandCorrupt)
	}
	res, err = tx.DB.exec.ExecContext(ctx, s.sql("UPDATE "+s.table+" SET result = ?, status = 1 WHERE scope_hash = ? AND operation_id = ? AND status = 0"), encoded, s.scopeHash[:], []byte(id))
	if err != nil {
		return rollback(err)
	}
	if err = commandOne(res); err != nil {
		return rollback(err)
	}
	if err = ctx.Err(); err != nil {
		return rollback(err)
	}
	if err = tx.Commit(); err != nil {
		return zero, errors.Join(ErrCommandOutcomeUnknown, err)
	}
	return CommandResult{Summary: summary, Applied: true}, nil
}

func (s *pr2ReaderFixture) lookup(ctx context.Context, id string, input []byte, fp [32]byte, steps int) (CommandResult, error) {
	var zero CommandResult
	var version, status int
	var scope, stored, hash, result []byte
	limit := fmt.Sprint(CommandMaxBytes + 1)
	q := "SELECT format_version, status, substring(canonical_scope,1," + limit + "), substring(command_input,1," + limit + "), substring(fingerprint,1,33), substring(result,1," + limit + ") FROM " + s.table + " WHERE scope_hash = ? AND operation_id = ?"
	err := s.db.exec.QueryRowContext(ctx, s.sql(q), s.scopeHash[:], []byte(id)).Scan(&version, &status, &scope, &stored, &hash, &result)
	if err != nil {
		return zero, err
	}
	if version != 1 || status != 1 || len(scope) > CommandMaxBytes || len(stored) > CommandMaxBytes || len(result) > CommandMaxBytes || len(hash) != 32 {
		return zero, ErrCommandCorrupt
	}
	actual, decodeErr := pr2Fingerprint(stored)
	if decodeErr != nil || !bytes.Equal(hash, actual[:]) {
		return zero, ErrCommandCorrupt
	}
	if !bytes.Equal(scope, s.scope) || !bytes.Equal(stored, input) || !bytes.Equal(hash, fp[:]) {
		return zero, ErrCommandIdentityConflict
	}
	var summary CommandSummary
	if err = json.Unmarshal(result, &summary); err != nil || summary.Version != 1 || len(summary.RowsAffected) != steps {
		return zero, ErrCommandCorrupt
	}
	for _, n := range summary.RowsAffected {
		if n < 0 {
			return zero, ErrCommandCorrupt
		}
	}
	// Exact writer-format roundtrip rejects duplicate/unknown fields and versions.
	canonical, _ := json.Marshal(summary)
	if !bytes.Equal(canonical, result) {
		return zero, ErrCommandCorrupt
	}
	return CommandResult{Summary: summary}, nil
}
func pr2Snapshot(c Command, schema string) ([]byte, []querybridge.Request, error) {
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

func pr2Fingerprint(input []byte) ([32]byte, error) {
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
	normalized, _, err = pr2Snapshot(c, "public")
	if err != nil || !bytes.Equal(input, normalized) {
		return zero, ErrCommandCorrupt
	}
	domain := "goquent.command.input.v1\x00"
	if header.Version == 2 {
		domain = "goquent.command.input.v2\x00"
	}
	return sha256.Sum256(append([]byte(domain), input...)), nil
}
