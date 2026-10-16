package orm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
	"github.com/recoweft/goquent/orm/driver"
)

const commandTable = "goquent_commands_v1"

// CommandStore binds a fixed v1 ledger to a private snapshot of one owned DB,
// dialect, namespace and trusted application tenant. Its zero value is invalid.
// Construct a new store to adopt changed settings. It cannot be rebound.
type CommandStore struct {
	db                     *DB
	dialect, schema, table string
	scope                  []byte
	scopeHash              [32]byte
}

// NewCommandStore performs read-only ledger schema validation. It never installs
// or repairs a table. See docs/command-identity.md for the explicit deployment DDL.
func NewCommandStore(ctx context.Context, db *DB, namespace string) (*CommandStore, error) {
	if ctx == nil {
		return nil, ErrCommandInput
	}
	if db == nil || db.drv == nil || db.drv.DB == nil {
		return nil, ErrCommandOwnership
	}
	actual, ok := db.exec.(*sql.DB)
	if !ok || actual != db.drv.DB {
		return nil, ErrCommandOwnership
	}
	if !commandLabel.MatchString(namespace) {
		return nil, ErrCommandInput
	}
	dialect := ""
	switch db.Dialect().(type) {
	case driver.MySQLDialect:
		dialect = MySQL
	case driver.PostgresDialect:
		dialect = Postgres
	default:
		return nil, ErrCommandStore
	}
	if err := db.settings.Err(); err != nil {
		return nil, err
	}
	tenant, err := db.settings.ApplicationTenantContext()
	if err != nil {
		return nil, err
	}
	atom, _, err := commandScalar(tenant.Input().CurrentTenant)
	if err != nil || atom.Type == "null" || atom.Type == "bool" {
		return nil, ErrCommandInput
	}
	s := &CommandStore{db: db.Clone(), dialect: dialect}
	q := "SELECT DATABASE()"
	if dialect == Postgres {
		q = "SELECT current_schema()"
	}
	if err = s.db.exec.QueryRowContext(ctx, q).Scan(&s.schema); err != nil {
		return nil, err
	}
	if !commandIdent.MatchString(s.schema) {
		return nil, ErrCommandStore
	}
	s.table = s.qualified(commandTable)
	s.scope, err = json.Marshal(struct {
		Version                    int
		Dialect, Schema, Namespace string
		Tenant                     commandAtom
	}{1, dialect, s.schema, namespace, atom})
	if err != nil || len(s.scope) > CommandMaxBytes {
		return nil, ErrCommandInput
	}
	s.scopeHash = sha256.Sum256(append([]byte("goquent.command.scope.v1\x00"), s.scope...))
	if err = s.validateLedger(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *CommandStore) qualified(table string) string {
	quote := "`"
	if s.dialect == Postgres {
		quote = `"`
	}
	return quote + s.schema + quote + "." + quote + table + quote
}
func (s *CommandStore) sql(q string) string {
	if s.dialect != Postgres {
		return q
	}
	n := 0
	var b strings.Builder
	for _, c := range q {
		if c == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
		} else {
			b.WriteRune(c)
		}
	}
	return b.String()
}
func (s *CommandStore) valid() bool {
	return s != nil && s.db != nil && s.db.drv != nil && s.db.drv.DB != nil && s.db.exec == s.db.drv.DB && len(s.scope) > 0
}

// Run validates every step before looking up a replay or reserving the ID. The
// reservation, all business statements, ordered finite hooks and the fixed result
// commit together. Hooks require exactly one affected row each.
// No automatic retries are performed, including after an ambiguous commit.
func (s *CommandStore) Run(ctx context.Context, c Command) (CommandResult, error) {
	var zero CommandResult
	if !s.valid() {
		return zero, ErrCommandOwnership
	}
	if ctx == nil {
		return zero, ErrCommandInput
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	input, requests, err := snapshotCommand(c, s.schema)
	if err != nil {
		return zero, err
	}
	var plan compoundPlan
	for _, r := range requests {
		if err = plan.add(ctx, s.db, "command", 0, 1, r, nil); err != nil {
			return zero, err
		}
	}
	if err = s.validateLedger(ctx); err != nil {
		return zero, err
	}
	seen := map[string]bool{}
	for _, r := range requests {
		table := r.Options.TableParts[1]
		if !seen[table] {
			if err = s.validatePlainTable(ctx, table); err != nil {
				return zero, err
			}
			seen[table] = true
		}
	}
	fingerprint, err := commandFingerprint(input)
	if err != nil {
		return zero, err
	}
	existing, err := s.lookup(ctx, c.ID, input, fingerprint, len(requests))
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return zero, err
	}
	return s.apply(ctx, c.ID, input, fingerprint, &plan)
}

func (s *CommandStore) apply(ctx context.Context, id string, input []byte, fingerprint [32]byte, plan *compoundPlan) (CommandResult, error) {
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

func commandOne(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrCommandCorrupt
	}
	return nil
}
func (s *CommandStore) reservationDuplicate(err error) bool {
	if s.dialect == MySQL {
		var e *mysql.MySQLError
		return errors.As(err, &e) && e.Number == 1062
	}
	var e *pq.Error
	return errors.As(err, &e) && e.Code == "23505" && e.Constraint == commandTable+"_pkey" && e.Table == commandTable && e.Schema == s.schema
}

// Bounded server-side substrings avoid allocating an arbitrarily corrupted blob.
func (s *CommandStore) lookup(ctx context.Context, id string, input []byte, fp [32]byte, steps int) (CommandResult, error) {
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
	actual, decodeErr := commandFingerprint(stored)
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
