package main

import (
	"context"
	"log"

	"github.com/recoweft/goquent/orm"
	"github.com/recoweft/goquent/orm/query"
)

type User struct {
	ID     int64  `db:"id,pk"`
	Name   string `db:"name"`
	Age    int    `db:"age"`
	Active bool   `db:"active"`
}

func (User) TableName() string { return "users" }

func selectUsers() orm.Scope {
	return func(q *query.Query) *query.Query {
		return q.Select("id", "name", "age", "active").OrderBy("id", "asc")
	}
}

func adultsOnly() orm.Scope {
	return func(q *query.Query) *query.Query {
		return q.Where("age", ">", 20)
	}
}

func inactiveOnly() orm.Scope {
	return func(q *query.Query) *query.Query {
		return q.Where("active", false)
	}
}

func main() {
	ctx := context.Background()

	db, err := orm.OpenWithDriver(orm.MySQL, "root:password@tcp(localhost:3306)/testdb?parseTime=true")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Generic insert from a struct value.
	if _, err := orm.Insert(ctx, db, User{Name: "sam", Age: 18, Active: false}); err != nil {
		log.Fatal(err)
	}

	// Generic read from raw SQL.
	user, err := orm.SelectOne[User](ctx, db, "SELECT id, name, age, active FROM users WHERE id = ?", 1)
	if err != nil {
		log.Fatal(err)
	}

	// Generic read from a scoped query-builder query.
	inactiveAdults := orm.ComposeScopes(selectUsers(), adultsOnly(), inactiveOnly())
	users, err := orm.SelectAllBy[User](ctx, db, db.Model(&User{}), inactiveAdults)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("loaded %d inactive adult users; seed user=%+v", len(users), user)

	// Generic primary-key update for a single row.
	if _, err := orm.Update(ctx, db, User{ID: user.ID, Active: true}, orm.Columns("active"), orm.WherePK()); err != nil {
		log.Fatal(err)
	}

	// Scoped update for more complex predicates.
	if err := db.TransactionContext(ctx, func(tx orm.Tx) error {
		_, err := orm.UpdateBy(ctx, tx.Table("users"), map[string]any{"active": true}, inactiveAdults)
		return err
	}); err != nil {
		log.Fatal(err)
	}

	// Scoped delete for cases that do not fit generic WherePK writes.
	if _, err := orm.DeleteBy(ctx, db.Table("users"), func(q *query.Query) *query.Query {
		return q.Where("age", "<", 13)
	}); err != nil {
		log.Fatal(err)
	}
}
