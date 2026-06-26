package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/recoweft/goquent/orm"
	ormdriver "github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/migration"
	"github.com/recoweft/goquent/orm/review"
)

func runMigrate(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printMigrateUsage(stderr)
		return 2
	}

	switch args[0] {
	case "plan":
		return runMigrateCommand("plan", args[1:], stdout, stderr)
	case "dry-run":
		return runMigrateCommand("dry-run", args[1:], stdout, stderr)
	case "apply":
		return runMigrateCommand("apply", args[1:], stdout, stderr)
	case "status":
		return runMigrateStatus(args[1:], stdout, stderr)
	case "schema":
		return runMigrateSchema(args[1:], stdout, stderr)
	case "drift":
		return runMigrateDrift(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		printMigrateUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown migrate command %q\n", args[0])
		printMigrateUsage(stderr)
		return 2
	}
}

func runMigrateStatus(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("goquent migrate status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "pretty", "output format: pretty, json")
	driverName := fs.String("driver", "", "database driver: mysql or postgres")
	dsn := fs.String("dsn", "", "database DSN")
	table := fs.String("table", "schema_migrations", "migration status table")
	versionColumn := fs.String("version-column", "version", "migration version column")
	dirtyColumn := fs.String("dirty-column", "", "optional dirty-state column")
	appliedAtColumn := fs.String("applied-at-column", "", "optional applied-at timestamp column")
	var desired stringListFlag
	var desiredFiles stringListFlag
	fs.Var(&desired, "desired", "desired migration version; may be repeated")
	fs.Var(&desiredFiles, "desired-file", "file containing desired migration versions, one per line; may be repeated")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: goquent migrate status --driver mysql --dsn <dsn> [flags]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	outputFormat := strings.ToLower(strings.TrimSpace(*format))
	switch outputFormat {
	case "", "pretty", "text", "json":
	default:
		fmt.Fprintf(stderr, "unknown migrate status format %q\n", *format)
		return 2
	}
	if strings.TrimSpace(*driverName) == "" || strings.TrimSpace(*dsn) == "" {
		fmt.Fprintln(stderr, "goquent migrate status requires --driver and --dsn")
		return 2
	}
	dialect, err := migrationStatusDialect(*driverName)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	versions, err := readDesiredVersions(desired, desiredFiles)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	db, err := orm.OpenWithDriver(*driverName, *dsn)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	defer db.Close()

	status, err := migration.ReadStatus(
		context.Background(),
		db.SQLDB(),
		dialect,
		versions,
		migration.WithStatusTable(*table),
		migration.WithStatusVersionColumn(*versionColumn),
		migration.WithStatusDirtyColumn(*dirtyColumn),
		migration.WithStatusAppliedAtColumn(*appliedAtColumn),
	)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if err := writeMigrationStatusOutput(stdout, outputFormat, status); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if !status.Exists || status.Dirty || status.Unknown || len(status.Pending) > 0 {
		return 1
	}
	return 0
}

func runMigrateSchema(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("goquent migrate schema", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "json", "output format: json, pretty")
	driverName := fs.String("driver", "", "database driver: mysql or postgres")
	dsn := fs.String("dsn", "", "database DSN")
	schemaName := fs.String("schema", "", "database schema/catalog to inspect")
	var tables stringListFlag
	fs.Var(&tables, "table", "table name to include; may be repeated")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: goquent migrate schema --driver mysql --dsn <dsn> [flags]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	outputFormat := strings.ToLower(strings.TrimSpace(*format))
	switch outputFormat {
	case "", "pretty", "text", "json":
	default:
		fmt.Fprintf(stderr, "unknown migrate schema format %q\n", *format)
		return 2
	}
	if strings.TrimSpace(*driverName) == "" || strings.TrimSpace(*dsn) == "" {
		fmt.Fprintln(stderr, "goquent migrate schema requires --driver and --dsn")
		return 2
	}
	dialect, err := migrationDialect(*driverName)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	db, err := orm.OpenWithDriver(*driverName, *dsn)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	defer db.Close()

	schema, err := migration.ReadSchema(
		context.Background(),
		db.SQLDB(),
		dialect,
		migration.WithSchemaReadSchema(*schemaName),
		migration.WithSchemaReadTables(tables...),
	)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if err := writeMigrationSchemaOutput(stdout, outputFormat, schema); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}

func runMigrateDrift(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("goquent migrate drift", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "pretty", "output format: pretty, json")
	desiredSchemaPath := fs.String("desired-schema", "", "desired schema JSON path")
	databaseSchemaPath := fs.String("database-schema", "", "current database schema JSON path")
	driverName := fs.String("driver", "", "database driver for live database schema export: mysql or postgres")
	dsn := fs.String("dsn", "", "database DSN for live database schema export")
	schemaName := fs.String("schema", "", "database schema/catalog to inspect")
	var tables stringListFlag
	fs.Var(&tables, "table", "table name to include in live schema export; may be repeated")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: goquent migrate drift --desired-schema desired.json (--database-schema current.json | --driver mysql --dsn <dsn>) [flags]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	outputFormat := strings.ToLower(strings.TrimSpace(*format))
	switch outputFormat {
	case "", "pretty", "text", "json":
	default:
		fmt.Fprintf(stderr, "unknown migrate drift format %q\n", *format)
		return 2
	}
	if strings.TrimSpace(*desiredSchemaPath) == "" {
		fmt.Fprintln(stderr, "goquent migrate drift requires --desired-schema")
		return 2
	}
	desired, err := loadSchema(*desiredSchemaPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	current, err := loadDriftCurrentSchema(*databaseSchemaPath, *driverName, *dsn, *schemaName, tables)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	report := migration.CompareSchemaDrift(*desired, *current)
	if err := writeMigrationDriftOutput(stdout, outputFormat, report); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if report.Drifted {
		return 1
	}
	return 0
}

func loadDriftCurrentSchema(databaseSchemaPath, driverName, dsn, schemaName string, tables []string) (*migration.Schema, error) {
	if strings.TrimSpace(databaseSchemaPath) != "" {
		return loadSchema(databaseSchemaPath)
	}
	if strings.TrimSpace(driverName) == "" || strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("goquent migrate drift requires --database-schema or --driver and --dsn")
	}
	dialect, err := migrationDialect(driverName)
	if err != nil {
		return nil, err
	}
	db, err := orm.OpenWithDriver(driverName, dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	schema, err := migration.ReadSchema(
		context.Background(),
		db.SQLDB(),
		dialect,
		migration.WithSchemaReadSchema(schemaName),
		migration.WithSchemaReadTables(tables...),
	)
	if err != nil {
		return nil, err
	}
	return &schema, nil
}

func runMigrateCommand(mode string, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("goquent migrate "+mode, flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "pretty", "output format: pretty, json")
	failOn := fs.String("fail-on", "", "optional risk threshold that returns exit code 1")
	approve := fs.String("approve", "", "approval reason for applying risky migrations")
	reviewMode := fs.String("review-mode", "", "optional migration review mode: backfill")
	driverName := fs.String("driver", "", "database driver for apply: mysql or postgres")
	dsn := fs.String("dsn", "", "database DSN for apply")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: goquent migrate %s [flags] <migration.sql ...>\n", mode)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	outputFormat := strings.ToLower(strings.TrimSpace(*format))
	switch outputFormat {
	case "", "pretty", "text", "json":
	default:
		fmt.Fprintf(stderr, "unknown migrate format %q\n", *format)
		return 2
	}

	var thresholdSet bool
	var threshold orm.RiskLevel
	if strings.TrimSpace(*failOn) != "" {
		parsed, err := review.ParseRiskLevel(*failOn)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		threshold = parsed
		thresholdSet = true
	}

	sqlText, err := readMigrationSQL(fs.Args())
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	migrator := migration.New(sqlText)
	if strings.TrimSpace(*approve) != "" {
		migrator.RequireApproval(*approve)
	}
	if strings.TrimSpace(*reviewMode) != "" {
		migrator.ReviewMode(migration.ReviewMode(*reviewMode))
	}

	ctx := context.Background()
	plan, err := migrator.Plan(ctx)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	if mode == "dry-run" || mode == "apply" {
		if err := migration.EnsureExecutable(plan); err != nil {
			_ = writeMigrationOutput(stdout, outputFormat, plan)
			fmt.Fprintln(stderr, err)
			return 1
		}
	}

	if mode == "apply" {
		if strings.TrimSpace(*driverName) == "" || strings.TrimSpace(*dsn) == "" {
			_ = writeMigrationOutput(stdout, outputFormat, plan)
			fmt.Fprintln(stderr, "goquent migrate apply requires --driver and --dsn")
			return 2
		}
		db, err := orm.OpenWithDriver(*driverName, *dsn)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		defer db.Close()
		plan, err = migrator.Apply(ctx, db)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
	}

	if err := writeMigrationOutput(stdout, outputFormat, plan); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if thresholdSet && compareMigrationRisk(plan.RiskLevel, threshold) >= 0 {
		return 1
	}
	return 0
}

func readDesiredVersions(inline []string, files []string) ([]string, error) {
	out := make([]string, 0, len(inline))
	out = append(out, inline...)
	for _, path := range files {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			out = append(out, line)
		}
	}
	return out, nil
}

func migrationStatusDialect(driverName string) (ormdriver.Dialect, error) {
	return migrationDialect(driverName)
}

func migrationDialect(driverName string) (ormdriver.Dialect, error) {
	switch strings.ToLower(strings.TrimSpace(driverName)) {
	case orm.MySQL:
		return ormdriver.MySQLDialect{}, nil
	case orm.Postgres, "postgresql", "pgx":
		return ormdriver.PostgresDialect{}, nil
	default:
		return nil, fmt.Errorf("goquent migrate unsupported driver %q", driverName)
	}
}

func readMigrationSQL(paths []string) (string, error) {
	if len(paths) == 0 {
		return "", fmt.Errorf("goquent migrate requires at least one SQL file")
	}
	var b strings.Builder
	for _, path := range paths {
		var data []byte
		var err error
		if path == "-" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(path)
		}
		if err != nil {
			return "", err
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.Write(data)
	}
	return b.String(), nil
}

func writeMigrationStatusOutput(w io.Writer, format string, status migration.Status) error {
	switch format {
	case "", "pretty", "text":
		return migration.WriteStatusPretty(w, status)
	case "json":
		return migration.WriteStatusJSON(w, status)
	default:
		return fmt.Errorf("unknown migrate status format %q", format)
	}
}

func writeMigrationSchemaOutput(w io.Writer, format string, schema migration.Schema) error {
	switch format {
	case "", "pretty", "text":
		return migration.WriteSchemaPretty(w, schema)
	case "json":
		return migration.WriteSchemaJSON(w, schema)
	default:
		return fmt.Errorf("unknown migrate schema format %q", format)
	}
}

func writeMigrationDriftOutput(w io.Writer, format string, report migration.DriftReport) error {
	switch format {
	case "", "pretty", "text":
		return migration.WriteDriftPretty(w, report)
	case "json":
		return migration.WriteDriftJSON(w, report)
	default:
		return fmt.Errorf("unknown migrate drift format %q", format)
	}
}

func writeMigrationOutput(w io.Writer, format string, plan *migration.MigrationPlan) error {
	switch format {
	case "", "pretty", "text":
		return migration.WritePretty(w, plan)
	case "json":
		return migration.WriteJSON(w, plan)
	default:
		return fmt.Errorf("unknown migrate format %q", format)
	}
}

func compareMigrationRisk(a, b orm.RiskLevel) int {
	return migrationRiskRank(a) - migrationRiskRank(b)
}

func migrationRiskRank(level orm.RiskLevel) int {
	switch level {
	case orm.RiskLow, "":
		return 0
	case orm.RiskMedium:
		return 1
	case orm.RiskHigh:
		return 2
	case orm.RiskDestructive:
		return 3
	case orm.RiskBlocked:
		return 4
	default:
		return 0
	}
}

func printMigrateUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: goquent migrate <command>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  plan      print a migration plan")
	fmt.Fprintln(w, "  status    read migration table status")
	fmt.Fprintln(w, "  schema    export live database schema as migration.Schema JSON")
	fmt.Fprintln(w, "  drift     compare desired and database schema JSON")
	fmt.Fprintln(w, "  dry-run   validate whether a migration can be applied")
	fmt.Fprintln(w, "  apply     apply migration SQL after approval checks")
}
