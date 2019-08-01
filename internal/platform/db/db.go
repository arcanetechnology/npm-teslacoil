package db

import (
	"fmt"
	"net/url"
	"os"

	"github.com/jinzhu/gorm"
	"github.com/jmoiron/sqlx"
)

// DB is used explicitly to be able to extend functions on a gorm.DB instance
type DB struct {
	*gorm.DB
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
		return nil, err
	}
	return d, nil
}

// OpenTestDatabase Fetches the database credentials from env vars and opens a
// connection to the test db
func OpenTestDatabase() *gorm.DB {
	databaseURI := fmt.Sprintf(
		"user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DATABASE_TEST_USER"),
		os.Getenv("DATABASE_TEST_PASSWORD"),
		os.Getenv("DATABASE_TEST_NAME"),
	)

	d, err := gorm.Open("postgres", databaseURI)
	if err != nil {
		panic(err.Error())
	}
	return d
}
