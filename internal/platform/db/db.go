package db

import (
	"net/url"
	"path"
	"runtime"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

var (
	// MigrationsPath is the migration path
	MigrationsPath string
)

func init() {
	// This is abstracted into a function because calling it directly inside
	// creates the wrong path
	setMigrationsPath()
}

func setMigrationsPath() {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("Could not find path to migrations files")
	}

	splitPath := strings.SplitAfter(filename, "db")
	basePath := splitPath[0]

	MigrationsPath = path.Join(path.Clean(basePath), "migrations")

}

// DatabaseConfig has all the values we need to connect to a DB
type DatabaseConfig struct {
	// The user to use when connecting
	User     string
	Password string
	Host     string
	Port     int
	// The name of the DB to connect to
	Name string
}

// DB is our local DB struct
type DB struct {
	*sqlx.DB
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

	log.Infof("opened connection to DB at %s", databaseHostWithPort)

	return &DB{d}, nil
}

// MigrateOrReset applies migrations to the DB. If already applied, drops
// the db first, then applies migrations
func (d *DB) MigrateOrReset(conf DatabaseConfig) error {
	err := d.MigrateUp(path.Join("file://", MigrationsPath))

	if err != nil {
		log.WithError(err).Error("Error when migrating or resetting")
		if err.Error() == "no change" {
			log.Info("Resetting")
			return d.Reset(conf)
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
func (d *DB) Teardown(conf DatabaseConfig) error {
	err := d.Drop(path.Join("file://", MigrationsPath))
	if err != nil {
		return errors.Wrapf(err,
			"teardownTestDB cannot connect to database %s with user %s at %s",
			conf.Name,
			conf.User,
			conf.Host,
		)
	}

	return nil
}

// Reset first drops the DB, then applies migrations
func (d *DB) Reset(conf DatabaseConfig) error {
	if err := d.Teardown(conf); err != nil {
		return err
	}
	if err := d.MigrateOrReset(conf); err != nil {
		return err
	}

	return nil
}

// Drop drops the existing database
func (d *DB) Drop(migrationsPath string) error {
	driver, err := postgres.WithInstance(d.DB.DB, &postgres.Config{})
	if err != nil {
		return err
	}

	migrator, err := migrate.NewWithDatabaseInstance(
		migrationsPath,
		"postgres",
		driver,
	)
	if err != nil {
		log.Error(err)
		return err
	}

	if err = migrator.Drop(); err != nil {
		return err
	}

	return nil
}
