package db

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"

	// Necessary for migratiing
	_ "github.com/golang-migrate/migrate/v4/source/file"
	// Necessary for migratiing
	_ "github.com/golang-migrate/migrate/v4/source/github"
	"github.com/iancoleman/strcase"
)

type MigrationStatus struct {
	Dirty   bool
	Version uint
}

func (d *DB) getMigrate() (migrate.Migrate, error) {
	driver, err := postgres.WithInstance(d.DB.DB, &postgres.Config{})
	if err != nil {
		return migrate.Migrate{}, err
	}
	m, err := migrate.NewWithDatabaseInstance(
		d.MigrationsPath,
		"postgres",
		driver,
	)
	if err != nil {
		return migrate.Migrate{}, err
	}

	return *m, nil
}

// MigrationStatus returns the migrations verison number and dirtyness
func (d *DB) MigrationStatus() (MigrationStatus, error) {
	m, err := d.getMigrate()
	if err != nil {
		log.WithError(err).Error("could not get migration instance")
		return MigrationStatus{}, err
	}

	version, dirty, err := m.Version()
	if err != nil {
		// ErrNilVersion indicates no migrations have been applied at all
		if errors.Is(err, migrate.ErrNilVersion) {
			return MigrationStatus{
				Dirty:   true,
				Version: 0,
			}, nil
		}
		return MigrationStatus{}, fmt.Errorf("could not get migration version: %w", err)
	}
	return MigrationStatus{
		Dirty:   dirty,
		Version: version,
	}, nil
}

// MigrateUp Migrates everything up
func (d *DB) MigrateUp() error {
	log.WithField("migrationsPath", d.MigrationsPath).Info("Migrating up")
	m, err := d.getMigrate()
	if err != nil {
		log.WithError(err).Error("could not get migration instance")
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
	m, err := d.getMigrate()
	if err != nil {
		log.WithError(err).Error("could not get migration instance")
		return err
	}

	// Migrate down x number of steps
	return m.Steps(-steps)
}

func newMigrationFile(filePath string) error {
	if _, err := os.Create(filePath); err != nil {
		return fmt.Errorf("could not create new file: %w", err)
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

// ForceVersion sets the migration version. It does not check any
// currently active version in database. It resets the dirty state to false.
// typically used if a migration failed, and you know which version the database
// is currently in
func (d *DB) ForceVersion(version int) error {
	m, err := d.getMigrate()
	if err != nil {
		log.WithError(err).Error("could not get migration instance")
		return err
	}

	err = m.Force(version)
	if err != nil {
		log.WithError(err).WithField("version", version).Error("could not set version")
		return err
	}

	return nil
}

// MigrateToVersion looks at the currently active migration version, then
// migrates either up or down to the specified version.
func (d *DB) MigrateToVersion(version uint) error {
	m, err := d.getMigrate()
	if err != nil {
		log.WithError(err).Error("could not get migration instance")
		return err
	}

	err = m.Migrate(version)
	if err != nil {
		log.WithError(err).WithField("version", version).Error("could not migrate to version")
		return err
	}

	return nil
}

type MigrationFile struct {
	Version     int
	Description string
}

func (d *DB) ListVersions() []MigrationFile {
	dir := d.MigrationsPath[5:]

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.WithError(err).WithField("migrationspath", dir).
			Error("could not open migrationspath directory")
	}

	var migFiles []MigrationFile
	for _, file := range files {
		versionDesc := strings.SplitN(file.Name(), "_", 2)
		version, err := strconv.ParseInt(versionDesc[0], 10, 64)
		if err != nil {
			log.WithError(err).WithField("version", versionDesc[0]).
				Fatal("could not parse version")
		}
		description := strings.Split(versionDesc[1], ".")[0]

		migFiles = append(migFiles, MigrationFile{
			Version:     int(version),
			Description: description,
		})
	}

	return migFiles
}
