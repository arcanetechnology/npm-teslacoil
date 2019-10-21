package db

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// DatabaseConfig has all the values we need to connect to a DB
type DatabaseConfig struct {
	// The user to use when connecting
	User     string
	Password string
	Host     string
	Port     int
	// The name of the DB to connect to
	Name string

	// MigrationsPath is where our migrations are located
	MigrationsPath string
}

// DB is our local DB struct
type DB struct {
	*sqlx.DB
	MigrationsPath string
}

// Open fetched the database credentials from environment variables
// and stars creates the gorm database object
func Open(conf DatabaseConfig) (*DB, error) {
	// Define SSL mode.
	sslMode := "disable" // require

	// Query parameters.
	q := make(url.Values)
	q.Set("sslmode", sslMode)
	q.Set("timezone", "utc")

	databaseHostWithPort := conf.Host + ":" + strconv.Itoa(conf.Port)
	databaseURL := url.URL{
		Scheme: "postgres",
		User: url.UserPassword(
			conf.User,
			conf.Password,
		),
		Host:     databaseHostWithPort,
		Path:     conf.Name,
		RawQuery: q.Encode(),
	}

	d, err := sqlx.Open("postgres", databaseURL.String())
	if err != nil {
		return nil, errors.Wrapf(err,
			"Cannot connect to database %s with user %s at %s",
			conf.Name,
			conf.User,
			databaseHostWithPort,
		)
	}

	log.WithFields(logrus.Fields{
		"host":     databaseHostWithPort,
		"user":     conf.User,
		"database": conf.Name,
	}).Info("Opened connection to DB")

	return &DB{
		DB:             d,
		MigrationsPath: conf.MigrationsPath,
	}, nil
}

// MigrateOrReset applies migrations to the DB. If already applied, drops
// the db first, then applies migrations
func (d *DB) MigrateOrReset() error {
	err := d.MigrateUp()

	if err != nil {
		log.WithError(err).Error("Error when migrating or resetting")
		if err.Error() == "no change" {
			log.Info("Resetting")
			return d.Reset()
		}
		log.Error(err)
		return errors.Wrapf(err,
			"Cannot connect to database %v",
			d,
		)
	}

	return nil
}

// Teardown drops the database, removing all data and schemas
func (d *DB) Teardown() error {
	err := d.Drop()
	if err != nil {
		return fmt.Errorf("cannot teardown DB: %w", err)
	}

	return nil
}

// Reset first drops the DB, then applies migrations
func (d *DB) Reset() error {
	if err := d.Teardown(); err != nil {
		return err
	}
	if err := d.MigrateOrReset(); err != nil {
		return err
	}

	return nil
}

// Drop drops the existing database
func (d *DB) Drop() error {
	driver, err := postgres.WithInstance(d.DB.DB, &postgres.Config{})
	if err != nil {
		return err
	}

	migrator, err := migrate.NewWithDatabaseInstance(
		d.MigrationsPath,
		"postgres",
		driver,
	)
	if err != nil {
		log.WithError(err).Error("Could not get migrator")
		return err
	}

	if err = migrator.Drop(); err != nil {
		return err
	}

	return nil
}

// InsertGetter can get and insert into a db
type InsertGetter interface {
	Getter
	Inserter
}

// Getter can get from a db
type Getter interface {
	Get(dest interface{}, query string, args ...interface{}) error
}

// Inserter can insert into a database
type Inserter interface {
	NamedQuery(query string, arg interface{}) (*sqlx.Rows, error)
}
