package orm

import (
	"context"
	"github.com/recoweft/goquent/orm/internal/observe"
	"strings"
)

// Metadata checks are explicit reads, not public plan authorization. Administrators
// must prevent concurrent DDL, triggers and out-of-band ledger edits during use.
func (s *CommandStore) validatePlainTable(ctx context.Context, table string) error {
	if s.dialect == MySQL {
		var kind, engine string
		err := observe.InternalRow(ctx, s.db.exec, "SELECT TABLE_TYPE, COALESCE(ENGINE,'') FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?", s.schema, table).Scan(&kind, &engine)
		if err != nil {
			return err
		}
		if kind != "BASE TABLE" || engine != "InnoDB" {
			return ErrCommandStore
		}
	} else {
		var kind, persistence string
		var rls, partition bool
		err := observe.InternalRow(ctx, s.db.exec, "SELECT c.relkind::text, c.relpersistence::text, c.relrowsecurity, c.relispartition FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=$1 AND c.relname=$2", s.schema, table).Scan(&kind, &persistence, &rls, &partition)
		if err != nil {
			return err
		}
		if kind != "r" || persistence != "p" || rls || partition {
			return ErrCommandStore
		}
		var inherited int
		if err = observe.InternalRow(ctx, s.db.exec, "SELECT count(*) FROM pg_catalog.pg_inherits i JOIN pg_catalog.pg_class c ON c.oid=i.inhparent OR c.oid=i.inhrelid JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=$1 AND c.relname=$2", s.schema, table).Scan(&inherited); err != nil {
			return err
		}
		if inherited != 0 {
			return ErrCommandStore
		}
	}
	var triggers int
	q := "SELECT count(*) FROM information_schema.TRIGGERS WHERE EVENT_OBJECT_SCHEMA = ? AND EVENT_OBJECT_TABLE = ?"
	if s.dialect == Postgres {
		q = "SELECT count(*) FROM pg_catalog.pg_trigger t JOIN pg_catalog.pg_class c ON c.oid=t.tgrelid JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=? AND c.relname=? AND NOT t.tgisinternal"
	}
	if err := observe.InternalRow(ctx, s.db.exec, s.sql(q), s.schema, table).Scan(&triggers); err != nil {
		return err
	}
	if triggers != 0 {
		return ErrCommandStore
	}
	// Rules can rewrite a base table write into a different target.
	if s.dialect == Postgres {
		var rules int
		if err := observe.InternalRow(ctx, s.db.exec, "SELECT count(*) FROM pg_catalog.pg_rules WHERE schemaname=$1 AND tablename=$2", s.schema, table).Scan(&rules); err != nil {
			return err
		}
		if rules != 0 {
			return ErrCommandStore
		}
	}
	return nil
}

func (s *CommandStore) validateLedger(ctx context.Context) error {
	if err := s.validatePlainTable(ctx, commandTable); err != nil {
		return err
	}
	expected := map[string]string{"scope_hash": "binary(32)", "operation_id": "binary(36)", "format_version": "int", "canonical_scope": "mediumblob", "command_input": "mediumblob", "fingerprint": "binary(32)", "status": "int", "result": "mediumblob"}
	typeColumn := "COLUMN_TYPE"
	extra := "EXTRA"
	if s.dialect == Postgres {
		typeColumn = "DATA_TYPE"
		extra = "is_generated"
		for k := range expected {
			expected[k] = "bytea"
		}
		expected["format_version"] = "integer"
		expected["status"] = "integer"
	}
	q := "SELECT COLUMN_NAME, " + typeColumn + ", IS_NULLABLE, " + extra + " FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?"
	rows, err := observe.InternalQuery(ctx, s.db.exec, s.sql(q), s.schema, commandTable)
	if err != nil {
		return err
	}
	count := 0
	good := true
	for rows.Next() {
		var name, typ, nullable, generated string
		if err = rows.Scan(&name, &typ, &nullable, &generated); err != nil {
			rows.Close()
			return err
		}
		count++
		want, ok := expected[name]
		if !ok || strings.ToLower(typ) != want || nullable != "NO" || (s.dialect == MySQL && generated != "") || (s.dialect == Postgres && generated != "NEVER") {
			good = false
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if !good || count != len(expected) {
		return ErrCommandStore
	}
	// Exactly one unique index, with the full ordered scope/ID key. No secondary
	// uniqueness is allowed to masquerade as reservation contention.
	if s.dialect == MySQL {
		rows, err = observe.InternalQuery(ctx, s.db.exec, "SELECT INDEX_NAME, SEQ_IN_INDEX, COLUMN_NAME, COALESCE(SUB_PART,0) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA=? AND TABLE_NAME=? AND NON_UNIQUE=0 ORDER BY INDEX_NAME,SEQ_IN_INDEX", s.schema, commandTable)
		if err != nil {
			return err
		}
		count = 0
		good = true
		for rows.Next() {
			var name, col string
			var seq, prefix int
			if err = rows.Scan(&name, &seq, &col, &prefix); err != nil {
				rows.Close()
				return err
			}
			count++
			want := "scope_hash"
			if count == 2 {
				want = "operation_id"
			}
			if name != "PRIMARY" || seq != count || count > 2 || col != want || prefix != 0 {
				good = false
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if !good || count != 2 {
			return ErrCommandStore
		}
	} else {
		q := `SELECT i.relname, x.indisprimary, x.indisvalid, x.indimmediate, x.indpred IS NULL, x.indexprs IS NULL, x.indnkeyatts, x.indnatts, pg_get_indexdef(x.indexrelid,1,true), pg_get_indexdef(x.indexrelid,2,true) FROM pg_catalog.pg_index x JOIN pg_catalog.pg_class t ON t.oid=x.indrelid JOIN pg_catalog.pg_namespace n ON n.oid=t.relnamespace JOIN pg_catalog.pg_class i ON i.oid=x.indexrelid WHERE n.nspname=$1 AND t.relname=$2 AND x.indisunique`
		rows, err = observe.InternalQuery(ctx, s.db.exec, q, s.schema, commandTable)
		if err != nil {
			return err
		}
		count = 0
		good = true
		for rows.Next() {
			var name, a, b string
			var primary, valid, immediate, plain, expression bool
			var keys, attrs int
			if err = rows.Scan(&name, &primary, &valid, &immediate, &plain, &expression, &keys, &attrs, &a, &b); err != nil {
				rows.Close()
				return err
			}
			count++
			if name != commandTable+"_pkey" || !primary || !valid || !immediate || !plain || !expression || keys != 2 || attrs != 2 || a != "scope_hash" || b != "operation_id" {
				good = false
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if count != 1 || !good {
			return ErrCommandStore
		}
	}
	var foreignKeys int
	if err = observe.InternalRow(ctx, s.db.exec, s.sql("SELECT count(*) FROM information_schema.TABLE_CONSTRAINTS WHERE TABLE_SCHEMA=? AND TABLE_NAME=? AND CONSTRAINT_TYPE='FOREIGN KEY'"), s.schema, commandTable).Scan(&foreignKeys); err != nil {
		return err
	}
	if foreignKeys != 0 {
		return ErrCommandStore
	}
	return nil
}
