package orm

// Frozen PR1 apply/lookup implementation from main 68f0d7b. This is an
// executable old-reader fixture, not a claim about an independently built binary.
import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

type pr1ReaderFixture struct{ *CommandStore }

func (s *pr1ReaderFixture) apply(ctx context.Context, id string, input []byte, fingerprint [32]byte, plan *compoundPlan) (CommandResult, error) {
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

func (s *pr1ReaderFixture) lookup(ctx context.Context, id string, input []byte, fp [32]byte, steps int) (CommandResult, error) {
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
	actual := sha256.Sum256(append([]byte("goquent.command.input.v1\x00"), stored...))
	if !bytes.Equal(hash, actual[:]) {
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
