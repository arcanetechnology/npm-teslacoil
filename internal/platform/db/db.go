package db

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"runtime"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
)

// MigrationsPath is the migration path
var MigrationsPath string

func init() {
	// This is abstracted into a function because calling it directly inside
	// creates the wrong path
	setMigrationsPath()
}

func setMigrationsPath() {
	_, filename, _, ok := runtime.Caller(1)
	if ok == false {
		panic(errors.New("could not find path to migrations files"))
	}
	splitPath := strings.SplitAfter(filename, "teslacoil/")
	basePath := splitPath[0]

	MigrationsPath = path.Join("file://", path.Dir(basePath), "/internal/platform/db/migrations")
}

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
		Host:     os.Getenv("DATABASE_TEST_HOST"),
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
	err := MigrateUp(MigrationsPath, testDB)

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
	err := DropDatabase(MigrationsPath, testDB)
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
