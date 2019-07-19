package db

import (
	"fmt"
	"os"

	"github.com/jinzhu/gorm"
)

// DB is used explicitly to be able to extend functions on a gorm.DB instance
type DB struct {
	*gorm.DB
}

// OpenDatabase fetched the database credentials from environment variables
// and stars creates the gorm database object
func OpenDatabase() *gorm.DB {
	databaseURI := fmt.Sprintf(
		"user=%s password=%s dbname=%s sslmode=disable",
		os.Getenv("DATABASE_USER"),
		os.Getenv("DATABASE_PASSWORD"),
		os.Getenv("DATABASE_NAME"),
	)

	d, err := gorm.Open("postgres", databaseURI)
	if err != nil {
		panic(err.Error())
	}
	return d
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
