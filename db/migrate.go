package db

import (
	"fmt"
	"os"
	"path"
	"strings"
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

type migrationStatus struct {
	Dirty   bool
	Version uint
}

// MigrationStatus returns the migrations verison number and dirtyness
func (d *DB) MigrationStatus() (migrationStatus, error) {

	driver, err := postgres.WithInstance(d.DB.DB, &postgres.Config{})
	if err != nil {
		return migrationStatus{}, err
	}
	m, err := migrate.NewWithDatabaseInstance(
		d.MigrationsPath,
		"postgres",
		driver,
	)

	if err != nil {
		return migrationStatus{}, err
	}

	// Migrate all the way up ...
	version, dirty, err := m.Version()
	if err != nil {
		return migrationStatus{}, err
	}
	return migrationStatus{
		Dirty:   dirty,
		Version: version,
	}, nil
}

// MigrateUp Migrates everything up
func (d *DB) MigrateUp() error {
	log.WithField("migrationsPath", d.MigrationsPath).Info("Migrating up")
	driver, err := postgres.WithInstance(d.DB.DB, &postgres.Config{})
	if err != nil {
		log.WithError(err).Error("Could not get Postgres instance")
		return err
	}
	m, err := migrate.NewWithDatabaseInstance(
		d.MigrationsPath,
		"postgres",
		driver,
	)
	if err != nil {
		log.WithError(err).Error("Could not get migration instance")
		return err
	}

	// Migrate all the way up ...
	if err := m.Up(); err != nil {
		if err == migrate.ErrNoChange {
			log.Info("No migrations applied")
			return nil
		}
		log.WithError(err).Error("Could not migrate up")
		return fmt.Errorf("could not migrate up: %w", err)
	}

	log.Info("Succesfully migrated up")
	return nil
}

// MigrateDown migrates down
func (d *DB) MigrateDown(steps int) error {
	driver, err := postgres.WithInstance(d.DB.DB, &postgres.Config{})
	if err != nil {
		return err
	}
	m, err := migrate.NewWithDatabaseInstance(
		d.MigrationsPath,
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
func (d *DB) CreateMigration(migrationText string) error {
	migrationTime := time.Now().UTC().Format("20060102150405")

	parts := strings.SplitN(d.MigrationsPath, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("couldn't extract directory from migrations path: %s", d.MigrationsPath)
	}
	migrationsDir := parts[1]

	fileNameUp := path.Join(
		migrationsDir,
		migrationTime+"_"+strcase.ToSnake(migrationText)+".up.pgsql")
	if err := newMigrationFile(fileNameUp); err != nil {
		return err
	}

	fileNameDown := path.Join(
		migrationsDir,
		migrationTime+"_"+strcase.ToSnake(migrationText)+".down.pgsql")
	if err := newMigrationFile(fileNameDown); err != nil {
		return err
	}
	return nil
}
