package db

import (
	"net/url"
	"os"

	"github.com/jmoiron/sqlx"
)

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
		return nil, err
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
		return nil, err
	}
	return d, nil
}
