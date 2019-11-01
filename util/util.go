/*
Package util contains functionality that's used across all other modules.
*/
package util

import (
	"log"
	"os"
	"strconv"
)

const defaultPostgresPort = 5432

// GetDatabasePort the `DATABASE_PORT` env var, falls back to 5432
func GetDatabasePort() int {

	if databasePortStr := os.Getenv("DATABASE_PORT"); databasePortStr != "" {
		databasePort, err := strconv.Atoi(databasePortStr)

		if err != nil {
			log.Fatalf("given database port (%s) is not a valid int", databasePortStr)

		}

		return databasePort

	}
	return defaultPostgresPort
}

// GetEnvAsBool gets the specified environment variable, and
// tries to parse it as a bool. If that doesn't work, it logs
// and quits the program.
func GetEnvAsBool(env string) bool {
	boolStr := os.Getenv(env)
	if len(boolStr) == 0 {
		log.Fatalf("Given environment variable (%s) is not set", env)
	}
	parsed, err := strconv.ParseBool(boolStr)

	if err != nil {
		log.Fatalf("Given environment variable (%s) was not a valid bool: %s", env, boolStr)
	}

	return parsed
}

// GetEnvAsInt gets the environment variable and parses it into an integer
// if the env variable can't be parsed, it logs the error and exits the program
func GetEnvAsInt(env string) int {
	intStr := os.Getenv(env)
	if len(intStr) == 0 {
		log.Fatalf("Given environment variable (%s) is not set", env)
	}
	parsed, err := strconv.ParseInt(intStr, 10, 64)

	if err != nil {
		log.Fatalf("Given environment variable ("+
			"%s) was not a valid bool: %s", env, intStr)
	}

	return int(parsed)
}

func GetEnvAsIntOrElse(env string, defaultValue int) int {
	envVar := os.Getenv(env)
	if len(envVar) == 0 {
		return defaultValue
	}

	return GetEnvAsInt(env)
}

// GetEnvOrElse returns the value of the given environment
// variable, or the provided default value if the env variable
// does not exist
func GetEnvOrElse(env string, defaultValue string) string {
	found := os.Getenv(env)
	if len(found) == 0 {
		return defaultValue
	}
	return found
}

// GetEnvOrFail returns the value of the given env variable,
// quitting the program if it doesn't exist. It should be used
// in cases where there's absolutely no recovery options, and
// the user should get told about this as soon as possible.
func GetEnvOrFail(env string) string {
	found := os.Getenv(env)
	if len(found) == 0 {
		log.Fatalf("%s is not set!", env)
	}
	return found
}
