package orm

import (
	"bytes"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"testing"
)

func TestCommandV2PresenceAndSnapshot(t *testing.T) {
	c := commandTestInput()
	v1, _, e := snapshotCommand(c, "public")
	if e != nil {
		t.Fatal(e)
	}
	golden := `{"Version":1,"Kind":"set-score","Steps":[{"Operation":"insert","Table":"items","Values":[{"name":"id","atom":{"type":"int64","value":"1"}},{"name":"score","atom":{"type":"int64","value":"2"}}],"Keys":[]}]}`
	if string(v1) != golden {
		t.Fatal("v1 bytes changed")
	}
	if _, e = commandFingerprint(v1); e != nil {
		t.Fatal(e)
	}
	zero := int64(0)
	c.Steps[0].ExpectAffected = &zero
	v2, requests, e := snapshotCommand(c, "public")
	if e != nil {
		t.Fatal(e)
	}
	zero = 1
	if *requests[0].Results.Expected != 0 {
		t.Fatal("mutable expected retained")
	}
	v2one, _, e := snapshotCommand(c, "public")
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Equal(v1, v2) || bytes.Equal(v2, v2one) {
		t.Fatal("presence collapsed")
	}
	for _, b := range [][]byte{v1, v2, v2one} {
		if _, e = commandFingerprint(b); e != nil {
			t.Fatal(e)
		}
	}
	for _, bad := range [][]byte{bytes.Replace(v2, []byte(`"Version":2`), []byte(`"Version":3`), 1), bytes.Replace(v2, []byte(`"Version":2`), []byte(`"Version":2,"Version":2`), 1), append(v2, ' '), bytes.Replace(v2, []byte(`"int64"`), []byte(`"unknown"`), 1)} {
		if _, e = commandFingerprint(bad); !errors.Is(e, ErrCommandCorrupt) {
			t.Fatal("malformed accepted")
		}
	}
	c.Steps[0].Operation = "update"
	c.Steps[0].Keys = map[string]any{"id": int64(1)}
	c.Steps[0].Version = &CommandVersion{Column: "version", Expected: 4}
	encoded, requests, e := snapshotCommand(c, "public")
	if e != nil {
		t.Fatal(e)
	}
	c.Steps[0].Version.Expected = 5
	if requests[0].Results.Version.Expected != 4 {
		t.Fatal("mutable version retained")
	}
	if _, e = commandFingerprint(encoded); e != nil {
		t.Fatal(e)
	}
}

func TestPR1ReaderV1Compatibility(t *testing.T) {
	for _, race := range []bool{false, true} {
		s, m, plan, input, fp := commandMock(t)
		old := &pr1ReaderFixture{s}
		if race {
			m.ExpectBegin()
			m.ExpectExec(`INSERT INTO "public"."goquent_commands_v1"`).WillReturnError(&pq.Error{Code: "23505", Constraint: commandTable + "_pkey", Table: commandTable, Schema: "public"})
			m.ExpectRollback()
		}
		m.ExpectQuery("SELECT format_version").WillReturnRows(sqlmock.NewRows([]string{"version", "status", "scope", "input", "hash", "result"}).AddRow(1, 1, s.scope, input, fp[:], []byte(`{"version":1,"rows_affected":[1]}`)))
		var result CommandResult
		var err error
		if race {
			result, err = old.apply(t.Context(), commandTestID, input, fp, plan)
		} else {
			result, err = old.lookup(t.Context(), commandTestID, input, fp, 1)
		}
		if err != nil || result.Applied || result.Summary.RowsAffected[0] != 1 {
			t.Fatal("old v1 replay", err)
		}
		if err = m.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}
