package orm

import (
	sqldriver "database/sql/driver"
	"sync"

	"github.com/recoweft/goquent/orm/driver"
)

var (
	driversMu  sync.RWMutex
	drivers    = make(map[string]sqldriver.Driver)
	dialectsMu sync.RWMutex
	dialects   = make(map[string]driver.Dialect)
)

// RegisterDriver registers a database driver.
func RegisterDriver(name string, d sqldriver.Driver) {
	driversMu.Lock()
	defer driversMu.Unlock()
	drivers[name] = d
}

// RegisterDriverWithDialect registers a database driver along with its dialect.
func RegisterDriverWithDialect(name string, d sqldriver.Driver, dialect driver.Dialect) {
	RegisterDriver(name, d)
	RegisterDialect(name, dialect)
}

// RegisterDialect registers a SQL dialect for a driver name.
func RegisterDialect(name string, d driver.Dialect) {
	dialectsMu.Lock()
	defer dialectsMu.Unlock()
	dialects[name] = d
}

// GetDriver retrieves a registered driver.
func GetDriver(name string) (sqldriver.Driver, bool) {
	driversMu.RLock()
	defer driversMu.RUnlock()
	d, ok := drivers[name]
	return d, ok
}

func getDialect(name string) (driver.Dialect, bool) {
	dialectsMu.RLock()
	defer dialectsMu.RUnlock()
	d, ok := dialects[name]
	return d, ok
}
