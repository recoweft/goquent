package orm

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"github.com/lib/pq"
	"github.com/recoweft/goquent/orm/driver"
)

func commandMock(t *testing.T) (*CommandStore, sqlmock.Sqlmock, *compoundPlan, []byte, [32]byte) {
	t.Helper()
	std, m, e := sqlmock.New()
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { std.Close() })
	s := &CommandStore{db: NewDB(std, driver.PostgresDialect{}), dialect: Postgres, schema: "public", table: `"public"."goquent_commands_v1"`, scope: []byte("scope")}
	s.scopeHash = sha256.Sum256(s.scope)
	b, r, e := snapshotCommand(commandTestInput(), s.schema)
	if e != nil {
		t.Fatal(e)
	}
	p := &compoundPlan{}
	if e = p.add(t.Context(), s.db, "command", 0, 1, r[0], nil); e != nil {
		t.Fatal(e)
	}
	return s, m, p, b, sha256.Sum256(append([]byte("goquent.command.input.v1\x00"), b...))
}
func TestCommandTransactionFailures(t *testing.T) {
	for _, kind := range []string{"reservation-count", "business", "business-count-error", "save", "save-count", "rollback", "commit"} {
		t.Run(kind, func(t *testing.T) {
			s, m, p, b, fp := commandMock(t)
			failure := errors.New("PRIVATE_COMMAND_CANARY")
			m.ExpectBegin()
			reservation := m.ExpectExec(`INSERT INTO "public"."goquent_commands_v1"`).WithArgs(s.scopeHash[:], []byte(commandTestID), s.scope, b, fp[:], []byte{})
			if kind == "reservation-count" {
				reservation.WillReturnResult(sqlmock.NewResult(0, 0))
			} else {
				reservation.WillReturnResult(sqlmock.NewResult(0, 1))
				business := m.ExpectExec(`INSERT INTO "public"."items"`)
				if kind == "business" || kind == "rollback" {
					business.WillReturnError(failure)
				} else if kind == "business-count-error" {
					business.WillReturnResult(sqlmock.NewErrorResult(failure))
				} else {
					business.WillReturnResult(sqlmock.NewResult(0, 1))
					save := m.ExpectExec(`UPDATE "public"."goquent_commands_v1"`)
					if kind == "save" {
						save.WillReturnError(failure)
					} else if kind == "save-count" {
						save.WillReturnResult(sqlmock.NewResult(0, 2))
					} else {
						save.WillReturnResult(sqlmock.NewResult(0, 1))
					}
				}
			}
			if kind == "commit" {
				m.ExpectCommit().WillReturnError(failure)
			} else if kind == "rollback" {
				m.ExpectRollback().WillReturnError(failure)
			} else {
				m.ExpectRollback()
			}
			r, e := s.apply(t.Context(), commandTestID, b, fp, p)
			if e == nil || r.Applied || r.Summary.Version != 0 {
				t.Fatal("failure published result")
			}
			if (kind == "commit" || kind == "rollback") != errors.Is(e, ErrCommandOutcomeUnknown) {
				t.Fatal("unknown classification")
			}
			if kind == "business" && !errors.Is(e, failure) {
				t.Fatal("business error identity lost")
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestCommandRaceLookupVerification(t *testing.T) {
	for _, kind := range []string{"same", "different-input", "different-scope", "corrupt", "missing", "unknown-version", "incomplete", "oversized", "unknown-result-field"} {
		t.Run(kind, func(t *testing.T) {
			s, m, p, b, fp := commandMock(t)
			m.ExpectBegin()
			m.ExpectExec(`INSERT INTO "public"."goquent_commands_v1"`).WillReturnError(&pq.Error{Code: "23505", Constraint: commandTable + "_pkey", Table: commandTable, Schema: "public"})
			m.ExpectRollback()
			stored, scope, hash, result := b, s.scope, fp[:], []byte(`{"version":1,"rows_affected":[1]}`)
			version, status := 1, 1
			switch kind {
			case "different-input":
				other := commandTestInput()
				other.Kind = "different"
				stored, _, _ = snapshotCommand(other, "public")
				h := sha256.Sum256(append([]byte("goquent.command.input.v1\x00"), stored...))
				hash = h[:]
			case "different-scope":
				scope = []byte("other")
			case "corrupt":
				hash = make([]byte, 32)
			case "unknown-version":
				version = 2
			case "incomplete":
				status = 0
			case "oversized":
				result = bytes.Repeat([]byte("x"), CommandMaxBytes+1)
			case "unknown-result-field":
				result = []byte(`{"version":1,"rows_affected":[1],"secret":1}`)
			}
			rows := sqlmock.NewRows([]string{"v", "s", "scope", "input", "hash", "result"})
			if kind != "missing" {
				rows.AddRow(version, status, scope, stored, hash, result)
			}
			m.ExpectQuery(regexp.QuoteMeta("SELECT format_version, status,")).WithArgs(s.scopeHash[:], []byte(commandTestID)).WillReturnRows(rows)
			r, e := s.apply(t.Context(), commandTestID, b, fp, p)
			switch kind {
			case "same":
				if e != nil || r.Applied || r.Summary.RowsAffected[0] != 1 {
					t.Fatal("replay failed")
				}
			case "different-input", "different-scope":
				if !errors.Is(e, ErrCommandIdentityConflict) {
					t.Fatal("identity mismatch accepted")
				}
			case "missing":
				if !errors.Is(e, ErrCommandOutcomeUnknown) {
					t.Fatal("missing race retried")
				}
			default:
				if !errors.Is(e, ErrCommandCorrupt) {
					t.Fatal("corruption accepted")
				}
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestCommandDuplicateDomain(t *testing.T) {
	pg := &CommandStore{dialect: Postgres, schema: "public"}
	my := &CommandStore{dialect: MySQL}
	if pg.reservationDuplicate(ErrConflict) || my.reservationDuplicate(ErrConflict) || pg.reservationDuplicate(&pq.Error{Code: "23505", Constraint: "business_pkey", Table: "business", Schema: "public"}) || my.reservationDuplicate(&mysql.MySQLError{Number: 1213}) {
		t.Fatal("business conflict classified as operation duplicate")
	}
}
