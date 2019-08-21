package db

import (
	"fmt"
	"net/url"
	"os"
	"path"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

var migrationsPath = path.Join("file://",
	os.Getenv("GOPATH"),
	"/src/gitlab.com/arcanecrypto/teslacoil/internal/platform/migrations")

// OpenDatabase fetched the database credentials from environment variables
// and stars creates the gorm database object
func OpenDatabase() (*sqlx.DB, error) {

	// Define SSL mode.
	sslMode := "disable" // require

	// Query parameters.
	q := make(url.Values)
	q.Set("sslmode", sslMode)
	q.Set("timezone", "utc")

	databaseURL := url.URL{
		Scheme: "postgres",
		User: url.UserPassword(
			os.Getenv("DATABASE_USER"),
			os.Getenv("DATABASE_PASSWORD")),
		Host:     "localhost",
		Path:     os.Getenv("DATABASE_NAME"),
		RawQuery: q.Encode(),
	}

	d, err := sqlx.Open("postgres", databaseURL.String())
	if err != nil {
		return nil, errors.Wrapf(err,
			"Cannot connect to database %s with user %s",
			os.Getenv("DATABASE_NAME"),
			os.Getenv("DATABASE_USER"),
		)
	}

	log.Debugf("opened connection to db")

	return d, nil
}

// OpenTestDatabase Fetches the database credentials from env vars and opens a
// connection to the test db
func OpenTestDatabase() (*sqlx.DB, error) {

	// Define SSL mode.
	sslMode := "disable" // require

	// Query parameters.
	q := make(url.Values)
	q.Set("sslmode", sslMode)
	q.Set("timezone", "utc")

	databaseURL := url.URL{
		Scheme: "postgres",
		User: url.UserPassword(
			os.Getenv("DATABASE_TEST_USER"),
			os.Getenv("DATABASE_TEST_PASSWORD")),
		Host:     "localhost",
		Path:     os.Getenv("DATABASE_TEST_NAME"),
		RawQuery: q.Encode(),
	}

	d, err := sqlx.Open("postgres", databaseURL.String())
	if err != nil {
		return nil, errors.Wrapf(err,
			"Cannot connect to database %s with user %s",
			os.Getenv("DATABASE_TEST_NAME"),
			os.Getenv("DATABASE_TEST_USER"),
		)
	}

	return d, nil
}

// CreateTestDatabase applies migrations to the DB. If already applied, drops
// the db first, then applies migrations
func CreateTestDatabase(testDB *sqlx.DB) error {
	err := MigrateUp(migrationsPath, testDB)

	if err != nil {
		if err.Error() == "no change" {
			return ResetDB(testDB)
		}
		fmt.Println(err)
		return errors.Wrapf(err,
			"Cannot connect to database %s with user %s",
			os.Getenv("DATABASE_TEST_NAME"),
			os.Getenv("DATABASE_TEST_USER"),
		)
	}

	return nil
}

// TeardownTestDB drops the database, removing all data and schemas
func TeardownTestDB(testDB *sqlx.DB) error {
	err := DropDatabase(migrationsPath, testDB)
	if err != nil {
		return errors.Wrapf(err,
			"teardownTestDB cannot connect to database %s with user %s",
			os.Getenv("DATABASE_TEST_NAME"),
			os.Getenv("DATABASE_TEST_USER"),
		)
	}

	return nil
}

// ResetDB first drops the DB, then applies migrations
func ResetDB(testDB *sqlx.DB) error {
	if err := TeardownTestDB(testDB); err != nil {
		return err
	}
	if err := CreateTestDatabase(testDB); err != nil {
		return err
	}

	return nil
}
