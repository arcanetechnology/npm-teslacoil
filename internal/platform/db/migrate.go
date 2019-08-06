package db

import (
	"fmt"
	"os"
	"path"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/golang-migrate/migrate/v4/source/github"
	"github.com/iancoleman/strcase"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// MigrationStatus prints the migrations verison number
func MigrationStatus(migrationsPath string, d *sqlx.DB) error {

	driver, err := postgres.WithInstance(d.DB, &postgres.Config{})
	if err != nil {
		return err
	}
	m, err := migrate.NewWithDatabaseInstance(
		migrationsPath,
		"postgres",
		driver,
	)

	if err != nil {
		return err
	}

	// Migrate all the way up ...
	version, dirty, err := m.Version()
	if err != nil {
		return err
	}
	fmt.Printf("Migration version: %d. Is dirty: %t\n", version, dirty)
	return nil
}

// MigrateUp Migrates everything up
func MigrateUp(migrationsPath string, d *sqlx.DB) error {

	driver, err := postgres.WithInstance(d.DB, &postgres.Config{})
	if err != nil {
		return err
	}
	m, err := migrate.NewWithDatabaseInstance(
		migrationsPath,
		"postgres",
		driver,
	)
	if err != nil {
		return err
	}

	// Migrate all the way up ...
<<<<<<< HEAD:internal/platform/db/migrate.go
	return m.Up()
=======
	if err := m.Up(); err != nil {
		return err
	}

	return nil
>>>>>>> master:cmd/lpp/migrate.go
}

// MigrateDown migrates down
func MigrateDown(migrationsPath string, d *sqlx.DB, steps int) error {
<<<<<<< HEAD:internal/platform/db/migrate.go

=======
>>>>>>> master:cmd/lpp/migrate.go
	driver, err := postgres.WithInstance(d.DB, &postgres.Config{})
	if err != nil {
		return err
	}
	m, err := migrate.NewWithDatabaseInstance(
		migrationsPath,
		"postgres",
		driver,
	)

	if err != nil {
		return err
	}

	// Migrate down x number of steps
<<<<<<< HEAD:internal/platform/db/migrate.go
	return m.Steps(-steps)
=======
	if err := m.Steps(-steps); err != nil {
		return err
	}

	return nil
>>>>>>> master:cmd/lpp/migrate.go
}

func newMigrationFile(filePath string) error {
	if _, err := os.Create(filePath); err != nil {
		return err
	}
	return nil
}

// CreateMigration creates a new empty migration file with correct name
func CreateMigration(migrationsPath string, migrationText string) error {
	migrationTime := time.Now().UTC().Format("20060102150405")

	fileNameUp := path.Join(migrationsPath, migrationTime+"_"+strcase.ToSnake(migrationText)+".up.pgsql")
	if err := newMigrationFile(fileNameUp); err != nil {
		return err
	}

	fileNameDown := path.Join(migrationsPath, migrationTime+"_"+strcase.ToSnake(migrationText)+".down.pgsql")
	if err := newMigrationFile(fileNameDown); err != nil {
		return err
	}
	return nil
}

// DropDatabase drops the existing database
func DropDatabase(migrationsPath string, d *sqlx.DB) error {
	driver, err := postgres.WithInstance(d.DB, &postgres.Config{})
	if err != nil {
		return err
	}
	migrator, err := migrate.NewWithDatabaseInstance(
		migrationsPath,
		"postgres",
		driver,
	)
	if err != nil {
		return err
	}

	if err = migrator.Drop(); err != nil {
		return err
	}

	return nil
}
