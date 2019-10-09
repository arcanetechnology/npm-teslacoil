package db

import (
	"fmt"
	"os"
	"path"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"

	// Necessary for migratiing
	_ "github.com/golang-migrate/migrate/v4/source/file"
	// Necessary for migratiing
	_ "github.com/golang-migrate/migrate/v4/source/github"
	"github.com/iancoleman/strcase"
	"github.com/pkg/errors"
)

// MigrationStatus prints the migrations verison number
func (d *DB) MigrationStatus(migrationsPath string) error {

	driver, err := postgres.WithInstance(d.DB.DB, &postgres.Config{})
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
func (d *DB) MigrateUp(migrationsPath string) error {
	driver, err := postgres.WithInstance(d.DB.DB, &postgres.Config{})
	if err != nil {
		log.WithError(err).Error("Could not get Postgres instance")
		return err
	}
	m, err := migrate.NewWithDatabaseInstance(
		migrationsPath,
		"postgres",
		driver,
	)
	if err != nil {
		log.WithError(err).Error("Could not get migration instance")
		return err
	}

	// Migrate all the way up ...
	err = m.Up()
	log.Info("Succesfully migrated up")
	return err
}

// MigrateDown migrates down
func (d *DB) MigrateDown(migrationsPath string, steps int) error {
	driver, err := postgres.WithInstance(d.DB.DB, &postgres.Config{})
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
	return m.Steps(-steps)
}

func newMigrationFile(filePath string) error {
	if _, err := os.Create(filePath); err != nil {
		return errors.Wrap(err, "Could not create new file")
	}
	return nil
}

// CreateMigration creates a new empty migration file with correct name
func CreateMigration(migrationsPath string, migrationText string) error {
	migrationTime := time.Now().UTC().Format("20060102150405")

	fileNameUp := path.Join(
		migrationsPath,
		migrationTime+"_"+strcase.ToSnake(migrationText)+".up.pgsql")
	if err := newMigrationFile(fileNameUp); err != nil {
		return err
	}

	fileNameDown := path.Join(
		migrationsPath,
		migrationTime+"_"+strcase.ToSnake(migrationText)+".down.pgsql")
	if err := newMigrationFile(fileNameDown); err != nil {
		return err
	}
	return nil
}
