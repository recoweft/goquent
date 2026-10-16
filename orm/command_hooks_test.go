package orm

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
)

func commandWithHooks() Command {
	c := commandTestInput()
	c.Hooks = []CommandHook{
		{Name: "audit", Table: "audit", Values: map[string]any{"id": int64(1), "score": int64(2)}},
		{Name: "outbox", Table: "outbox", Values: map[string]any{"id": int64(1), "score": int64(3)}},
	}
	return c
}

func TestCommandHookIdentityAndSnapshot(t *testing.T) {
	c := commandWithHooks()
	b, req, err := snapshotCommand(c, "public")
	if err != nil {
		t.Fatal(err)
	}
	fp, err := commandFingerprint(b)
	if err != nil || fp != sha256.Sum256(append([]byte("goquent.command.input.v3\x00"), b...)) {
		t.Fatal("v3 domain", err)
	}
	if len(req) != 3 || *req[1].Results.Expected != 1 || *req[2].Results.Expected != 1 {
		t.Fatal("hook count contract")
	}
	c.Hooks[0].Values["score"] = int64(99)
	c.Hooks[0].Name = "changed"
	if req[1].Rows[0]["score"] != int64(2) || bytes.Contains(b, []byte("changed")) {
		t.Fatal("mutable snapshot")
	}
	for _, mutate := range []func(*Command){
		func(c *Command) { c.Hooks = nil },
		func(c *Command) { c.Hooks[0].Name = "other" },
		func(c *Command) { c.Hooks[0].Table = "other" },
		func(c *Command) { c.Hooks[0].Values["score"] = int(2) },
		func(c *Command) { c.Hooks[0], c.Hooks[1] = c.Hooks[1], c.Hooks[0] },
		func(c *Command) { n := int64(1); c.Steps[0].ExpectAffected = &n },
	} {
		c := commandWithHooks()
		mutate(&c)
		other, _, e := snapshotCommand(c, "public")
		if e != nil || bytes.Equal(b, other) {
			t.Fatal("identity omitted", e)
		}
	}
	for _, version := range []bool{false, true} {
		c := commandTestInput()
		if version {
			n := int64(1)
			c.Steps[0].ExpectAffected = &n
		}
		old, _, e := pr2Snapshot(c, "public")
		if e != nil {
			t.Fatal(e)
		}
		c.Hooks = []CommandHook{}
		now, _, e := snapshotCommand(c, "public")
		if e != nil || !bytes.Equal(old, now) {
			t.Fatal("v1/v2 bytes changed", e)
		}
	}
	for _, bad := range [][]byte{
		append(append([]byte{}, b...), ' '),
		bytes.Replace(b, []byte(`"Version":3`), []byte(`"Version":3,"Version":3`), 1),
		bytes.Replace(b, []byte(`"Name":"audit"`), []byte(`"Name":"audit","extra":1`), 1),
		bytes.Replace(b, []byte(`"Name":"audit"`), []byte(`"Name":""`), 1),
		bytes.Replace(b, []byte(`"Version":3`), []byte(`"Version":4`), 1),
	} {
		if _, e := commandFingerprint(bad); !errors.Is(e, ErrCommandCorrupt) {
			t.Fatal("malformed accepted", e)
		}
	}
}

func TestCommandHookLimits(t *testing.T) {
	for name, mutate := range map[string]func(*Command){
		"empty-name": func(c *Command) { c.Hooks[0].Name = "" },
		"long-name":  func(c *Command) { c.Hooks[0].Name = strings.Repeat("a", 129) },
		"sql-name":   func(c *Command) { c.Hooks[0].Name = "x y" },
		"ledger":     func(c *Command) { c.Hooks[0].Table = commandTable },
		"schema":     func(c *Command) { c.Hooks[0].Table = "public.audit" },
		"empty":      func(c *Command) { c.Hooks[0].Values = nil },
		"callback":   func(c *Command) { c.Hooks[0].Values["score"] = func() {} },
		"struct":     func(c *Command) { c.Hooks[0].Values["score"] = struct{ X int }{1} },
		"hooks-only": func(c *Command) { c.Steps = nil },
		"statements": func(c *Command) {
			for len(c.Hooks) < 64 {
				c.Hooks = append(c.Hooks, c.Hooks[0])
			}
		},
		"columns": func(c *Command) {
			for i := 0; i < 257; i++ {
				c.Hooks[0].Values[fmt.Sprintf("c%d", i)] = i
			}
		},
		"shared-bytes": func(c *Command) {
			c.Steps[0].Values["score"] = strings.Repeat("a", CommandMaxBytes/2)
			c.Hooks[0].Values["score"] = strings.Repeat("a", CommandMaxBytes/2)
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := commandWithHooks()
			mutate(&c)
			if _, _, e := snapshotCommand(c, "public"); !errors.Is(e, ErrCommandInput) {
				t.Fatal("unsupported accepted", e)
			}
		})
	}
	c := commandWithHooks()
	for len(c.Hooks) < 63 {
		c.Hooks = append(c.Hooks, c.Hooks[0])
	}
	if _, _, e := snapshotCommand(c, "public"); e != nil {
		t.Fatal("64 statements rejected", e)
	}
}

func TestCommandHooksOldReadersRefuseV3(t *testing.T) {
	for _, reader := range []string{"pr1", "pr2"} {
		for _, race := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/race=%v", reader, race), func(t *testing.T) {
				s, m, p, legacy, lfp := commandMock(t)
				b, _, e := snapshotCommand(commandWithHooks(), "public")
				if e != nil {
					t.Fatal(e)
				}
				fp, e := commandFingerprint(b)
				if e != nil {
					t.Fatal(e)
				}
				if race {
					m.ExpectBegin()
					m.ExpectExec(`INSERT INTO "public"."goquent_commands_v1"`).WillReturnError(&pq.Error{Code: "23505", Constraint: commandTable + "_pkey", Table: commandTable, Schema: "public"})
					m.ExpectRollback()
				}
				m.ExpectQuery("SELECT format_version").WillReturnRows(sqlmock.NewRows([]string{"v", "status", "scope", "input", "hash", "result"}).AddRow(1, 1, s.scope, b, fp[:], []byte(`{"version":1,"rows_affected":[1,1,1]}`)))
				var r CommandResult
				if reader == "pr1" {
					old := &pr1ReaderFixture{s}
					if race {
						r, e = old.apply(t.Context(), commandTestID, legacy, lfp, p)
					} else {
						r, e = old.lookup(t.Context(), commandTestID, legacy, lfp, 1)
					}
				} else {
					old := &pr2ReaderFixture{s}
					if race {
						r, e = old.apply(t.Context(), commandTestID, legacy, lfp, p)
					} else {
						r, e = old.lookup(t.Context(), commandTestID, legacy, lfp, 1)
					}
				}
				if !errors.Is(e, ErrCommandCorrupt) || r.Applied {
					t.Fatal("old reader reused/reapplied v3", e)
				}
				if e = m.ExpectationsWereMet(); e != nil {
					t.Fatal(e)
				}
			})
		}
	}
}

func TestCommandHookFaults(t *testing.T) {
	for _, fault := range []string{"success", "audit", "outbox", "hook-zero", "hook-multiple", "hook-count-error", "save", "save-count", "rollback", "commit"} {
		t.Run(fault, func(t *testing.T) {
			s, m, _, _, _ := commandMock(t)
			b, req, e := snapshotCommand(commandWithHooks(), "public")
			if e != nil {
				t.Fatal(e)
			}
			fp, _ := commandFingerprint(b)
			p := &compoundPlan{}
			for _, r := range req {
				if e = p.add(t.Context(), s.db, "command", 0, 1, r, nil); e != nil {
					t.Fatal(e)
				}
			}
			fail := errors.New("hook failure")
			m.ExpectBegin()
			m.ExpectExec(`INSERT INTO "public"."goquent_commands_v1"`).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec(`INSERT INTO "public"."items"`).WillReturnResult(sqlmock.NewResult(0, 1))
			a := m.ExpectExec(`INSERT INTO "public"."audit"`)
			switch fault {
			case "audit", "rollback":
				a.WillReturnError(fail)
			case "hook-zero":
				a.WillReturnResult(sqlmock.NewResult(0, 0))
			case "hook-multiple":
				a.WillReturnResult(sqlmock.NewResult(0, 2))
			case "hook-count-error":
				a.WillReturnResult(sqlmock.NewErrorResult(fail))
			default:
				a.WillReturnResult(sqlmock.NewResult(0, 1))
				o := m.ExpectExec(`INSERT INTO "public"."outbox"`)
				if fault == "outbox" {
					o.WillReturnError(fail)
				} else {
					o.WillReturnResult(sqlmock.NewResult(0, 1))
					save := m.ExpectExec(`UPDATE "public"."goquent_commands_v1"`)
					if fault == "save" {
						save.WillReturnError(fail)
					} else if fault == "save-count" {
						save.WillReturnResult(sqlmock.NewResult(0, 0))
					} else {
						save.WillReturnResult(sqlmock.NewResult(0, 1))
					}
				}
			}
			if fault == "success" {
				m.ExpectCommit()
			} else if fault == "commit" {
				m.ExpectCommit().WillReturnError(fail)
			} else if fault == "rollback" {
				m.ExpectRollback().WillReturnError(fail)
			} else {
				m.ExpectRollback()
			}
			r, e := s.apply(t.Context(), commandTestID, b, fp, p)
			if fault == "success" {
				if e != nil || !r.Applied || len(r.Summary.RowsAffected) != 3 {
					t.Fatal(r, e)
				}
			} else if e == nil || r.Applied {
				t.Fatal("failure published", e)
			}
			if errors.Is(e, ErrCommandOutcomeUnknown) != (fault == "commit" || fault == "rollback") {
				t.Fatal("unknown classification", e)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

// This synthetic result models a late
// RowsAffected error. It is not a network COMMIT or power-loss experiment.
type hookFaultResult struct {
	late  bool
	reads int
}

func (r *hookFaultResult) LastInsertId() (int64, error) { return 0, nil }
func (r *hookFaultResult) RowsAffected() (int64, error) {
	r.reads++
	if r.late && r.reads > 1 {
		return 0, errors.New("late count")
	}
	return 1, nil
}

func TestCommandHookLateCount(t *testing.T) {
	s, m, _, _, _ := commandMock(t)
	c := commandWithHooks()
	b, req, e := snapshotCommand(c, "public")
	if e != nil {
		t.Fatal(e)
	}
	fp, _ := commandFingerprint(b)
	p := &compoundPlan{}
	for _, r := range req {
		if e = p.add(t.Context(), s.db, "command", 0, 1, r, nil); e != nil {
			t.Fatal(e)
		}
	}
	m.ExpectBegin()
	m.ExpectExec(`INSERT INTO "public"."goquent_commands_v1"`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`INSERT INTO "public"."items"`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`INSERT INTO "public"."audit"`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`INSERT INTO "public"."outbox"`).WillReturnResult(&hookFaultResult{late: true})
	m.ExpectRollback()
	if _, e = s.apply(t.Context(), c.ID, b, fp, p); e == nil {
		t.Fatal("late count accepted")
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

func TestCommandHookReplayVerification(t *testing.T) {
	for _, race := range []bool{false, true} {
		for _, kind := range []string{"same", "name", "scope", "short-summary", "hash", "unknown-input-field", "missing"} {
			t.Run(fmt.Sprintf("race=%v/%s", race, kind), func(t *testing.T) {
				s, m, _, _, _ := commandMock(t)
				c := commandWithHooks()
				b, req, e := snapshotCommand(c, "public")
				if e != nil {
					t.Fatal(e)
				}
				fp, _ := commandFingerprint(b)
				p := &compoundPlan{}
				for _, r := range req {
					if e = p.add(t.Context(), s.db, "command", 0, 1, r, nil); e != nil {
						t.Fatal(e)
					}
				}
				stored, scope, hash, result := b, s.scope, fp[:], []byte(`{"version":1,"rows_affected":[1,1,1]}`)
				switch kind {
				case "name":
					c.Hooks[0].Name = "other"
					stored, _, _ = snapshotCommand(c, "public")
					h, _ := commandFingerprint(stored)
					hash = h[:]
				case "scope":
					scope = []byte("other-tenant")
				case "short-summary":
					result = []byte(`{"version":1,"rows_affected":[1]}`)
				case "hash":
					hash = make([]byte, 32)
				case "unknown-input-field":
					stored = bytes.Replace(b, []byte(`"Name":"audit"`), []byte(`"Name":"audit","bad":true`), 1)
					h := sha256.Sum256(append([]byte("goquent.command.input.v3\x00"), stored...))
					hash = h[:]
				}
				if race {
					m.ExpectBegin()
					m.ExpectExec(`INSERT INTO "public"."goquent_commands_v1"`).WillReturnError(&pq.Error{Code: "23505", Constraint: commandTable + "_pkey", Table: commandTable, Schema: "public"})
					m.ExpectRollback()
				}
				rows := sqlmock.NewRows([]string{"v", "s", "scope", "input", "hash", "result"})
				if kind != "missing" {
					rows.AddRow(1, 1, scope, stored, hash, result)
				}
				m.ExpectQuery("SELECT format_version").WillReturnRows(rows)
				var r CommandResult
				if race {
					r, e = s.apply(t.Context(), commandTestID, b, fp, p)
				} else {
					r, e = s.lookup(t.Context(), commandTestID, b, fp, 3)
				}
				switch kind {
				case "same":
					if e != nil || r.Applied || len(r.Summary.RowsAffected) != 3 {
						t.Fatal("replay", r, e)
					}
				case "name", "scope":
					if !errors.Is(e, ErrCommandIdentityConflict) {
						t.Fatal("identity mismatch", e)
					}
				case "missing":
					if race && !errors.Is(e, ErrCommandOutcomeUnknown) {
						t.Fatal("race lost result", e)
					}
					if !race && e == nil {
						t.Fatal("missing returned success")
					}
				default:
					if !errors.Is(e, ErrCommandCorrupt) {
						t.Fatal("corrupt reused", e)
					}
				}
				if e = m.ExpectationsWereMet(); e != nil {
					t.Fatal(e)
				}
			})
		}
	}
}
