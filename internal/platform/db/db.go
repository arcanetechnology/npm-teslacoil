package db

import (
	"database/sql"
	"net/url"
	"path"
	"runtime"
	"strconv"
	"strings"

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

// DatabaseConfig has all the values we need to connect to a
// DB
type DatabaseConfig struct {
	// The user to use when connecting
	User     string
	Password string
	Host     string
	Port     int
	// The name of the DB to connect to
	Name string
}

// OpenDatabase fetched the database credentials from environment variables
// and stars creates the gorm database object
func OpenDatabase(conf DatabaseConfig) (*sqlx.DB, error) {
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

	return d, nil
}

// CreateTestDatabase applies migrations to the DB. If already applied, drops
// the db first, then applies migrations
func CreateTestDatabase(testDB *sqlx.DB, conf DatabaseConfig) error {
	err := MigrateUp(path.Join("file://", MigrationsPath), testDB)

	if err != nil {
		if err.Error() == "no change" {
			return ResetDB(testDB, conf)
		}
		log.Error(err)
		return errors.Wrapf(err,
			"Cannot connect to database %v",
			testDB,
		)
	}

	return nil
}

// TeardownTestDB drops the database, removing all data and schemas
func TeardownTestDB(testDB *sqlx.DB, conf DatabaseConfig) error {
	err := DropDatabase(path.Join("file://", MigrationsPath), testDB)
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

// ResetDB first drops the DB, then applies migrations
func ResetDB(testDB *sqlx.DB, conf DatabaseConfig) error {
	if err := TeardownTestDB(testDB, conf); err != nil {
		return err
	}
	if err := CreateTestDatabase(testDB, conf); err != nil {
		return err
	}

	return nil
}

//ToNullString converts the argument s to a sql.NullString
func ToNullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: true}
}
