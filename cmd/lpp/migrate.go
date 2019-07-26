package main

import (
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/golang-migrate/migrate/v4/source/github"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// MigrationStatus prints the migrations verison number
func MigrationStatus(migrationsPath string, d *sqlx.DB) error {

	driver, err := postgres.WithInstance(d.DB, &postgres.Config{})
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
func MigrateUp(migrationsPaths string, d *sqlx.DB) {

	driver, err := postgres.WithInstance(d.DB, &postgres.Config{})
	m, err := migrate.NewWithDatabaseInstance(
		migrationsPaths,
		"postgres",
		driver,
	)

	if err != nil {
		log.Fatal(err)
	}

	// Migrate all the way up ...
	if err := m.Up(); err != nil {
		log.Fatal(err)
	}
}

// MigrateDown migrates down
func MigrateDown(migrationsPaths string, d *sqlx.DB, steps int) {

	driver, err := postgres.WithInstance(d.DB, &postgres.Config{})
	m, err := migrate.NewWithDatabaseInstance(
		migrationsPaths,
		"postgres",
		driver,
	)

	if err != nil {
		log.Fatal(err)
	}

	// Migrate down x number of steps
	if err := m.Steps(-steps); err != nil {
		log.Fatal(err)
	}
}
