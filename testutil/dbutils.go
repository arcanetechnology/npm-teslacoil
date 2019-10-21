package testutil

import (
	"fmt"
	"path"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/teslacoil/db"
)

// GetDatabaseConfig returns a DB config suitable for testing purposes. The
// given argument is added to the name of the database
func GetDatabaseConfig(name string) db.DatabaseConfig {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatal("Could not find path to migrations files")
	}

	splitPath := strings.Split(filename, "testutil")
	basePath := splitPath[0]

	migrations := path.Join("file:", path.Clean(basePath), "db", "migrations")
	return db.DatabaseConfig{
		User:           "tlc_test",
		Password:       "password",
		Port:           5434, // we have Postgres running in a docker container exposed on 5434
		Host:           "localhost",
		Name:           "tlc_" + name,
		MigrationsPath: migrations,
	}
}

// CreateIfNotExists creates a new database from the given config if it does
// not exist.
func CreateIfNotExists(conf db.DatabaseConfig) error {
	rootConfig := db.DatabaseConfig{
		User:     "postgres",
		Password: "postgres",
		Host:     conf.Host,
		Port:     conf.Port,
		Name:     "postgres",
	}

	database, err := db.Open(rootConfig)
	defer func() {
		err = database.Close()
		if err != nil {
			panic(err)
		}
	}()

	if err != nil {
		return errors.Wrapf(err, "couldn't connect to root Postgres DB")
	}

	rows, err := database.Query("SELECT datname FROM pg_database WHERE datname=$1",
		conf.Name)

	if err != nil {
		return errors.Wrap(err, "couldn't query pg_database")
	}

	if err = rows.Err(); err != nil {
		return errors.Wrap(err, "rows.Err()")
	}

	// database does not exist
	if !rows.Next() {
		_, err = database.Exec(fmt.Sprintf("CREATE DATABASE %s", conf.Name))
		if err != nil {
			return errors.Wrap(err, "cannot create database")
		}

		if _, err = database.Exec(fmt.Sprintf(
			"GRANT ALL PRIVILEGES ON DATABASE %s TO %s",
			conf.Name,
			conf.User)); err != nil {
			return errors.Wrap(err, "cannot grant privileges to test user")
		}
	}

	return err

}

// InitDatabase initializes a DB for the given config such that tests can
// be run against it
func InitDatabase(config db.DatabaseConfig) *db.DB {
	log.Info("Opening, destroying and creating test DB")
	testDB, err := db.Open(config)

	if err != nil {
		log.Fatalf("could not open test DB with config %+v: %v", config, err)
	}

	if err = CreateIfNotExists(config); err != nil {
		log.Fatalf("could not create test DB with config %+v: %v", config, err)
	}

	if err = testDB.Teardown(); err != nil {
		log.Fatalf("could not tear down test DB: %v", err)
	}

	if err = testDB.MigrateOrReset(); err != nil {
		log.Fatalf("could not create test database: %v", err)
	}

	return testDB

}
