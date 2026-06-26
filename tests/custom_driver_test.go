package tests

import (
	"database/sql"
	"net"
	"os"
	"testing"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/recoweft/goquent/orm"
	"github.com/recoweft/goquent/orm/driver"
)

func setupCustomDB(t testing.TB, drvName string) *orm.DB {
	dsn := os.Getenv("TEST_DB_DSN")
	if dsn == "" {
		t.Skip("TEST_DB_DSN environment variable not set")
	}
	db, err := orm.OpenWithDriver(drvName, dsn)
	if err != nil {
		if _, ok := err.(*net.OpError); ok {
			t.Skip("mysql not available")
		}
		t.Fatalf("open: %v", err)
	}
	stdDB, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	_, err = stdDB.Exec(`CREATE TABLE IF NOT EXISTS users (
        id INT AUTO_INCREMENT PRIMARY KEY,
        name VARCHAR(64),
        age INT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    )`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	_, err = stdDB.Exec("TRUNCATE TABLE users")
	if err != nil {
		t.Fatalf("truncate table: %v", err)
	}
	_, err = stdDB.Exec("INSERT INTO users(name, age) VALUES ('cdrv', 1)")
	if err != nil {
		t.Fatalf("insert users: %v", err)
	}
	return db
}

func TestOpenWithRegisteredDriver(t *testing.T) {
	orm.RegisterDriverWithDialect("mysql-custom", &mysql.MySQLDriver{}, driver.MySQLDialect{})
	db := setupCustomDB(t, "mysql-custom")
	defer db.Close()

	var row map[string]any
	if err := db.Table("users").Where("name", "cdrv").FirstMap(&row); err != nil {
		t.Fatalf("select: %v", err)
	}
	if row["age"] != int64(1) {
		t.Errorf("expected age 1, got %v", row["age"])
	}
}
