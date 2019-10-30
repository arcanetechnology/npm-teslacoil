package testutil

import (
	"fmt"

	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/util"
)

// GetDatabaseConfig returns a DB config suitable for testing purposes. The
// given argument is added to the name of the database
func GetDatabaseConfig(name string) db.DatabaseConfig {
	return db.DatabaseConfig{
		User:     "tlc_test",
		Password: "password",
		Port:     util.GetDatabasePort(),
		Host:     util.GetEnvOrElse("DATABASE_HOST", "localhost"),
		Name:     "tlc_" + name,
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
		log.Fatalf("Could not open test database: %+v\n", err)
	}

	if err = CreateIfNotExists(config); err != nil {
		log.Fatalf("Could not create test DB: %v", err)
	}

	if err = testDB.Teardown(config); err != nil {
		log.Fatalf("Could not tear down test DB: %v", err)
	}

	if err = testDB.MigrateOrReset(config); err != nil {
		log.Fatalf("Could not create test database: %v", err)
	}

	return testDB

}
