package users

import (
	"flag"
	"os"
	"path"
	"runtime"
	"testing"

	"github.com/jmoiron/sqlx"

	"github.com/brianvoe/gofakeit"
	"gitlab.com/arcanecrypto/lpp/internal/platform/db"
)

func createTestDatabase(testDB *sqlx.DB) error {
	_, filename, _, ok := runtime.Caller(1)
	if ok == false {
		return nil
	}
	migrationFiles := path.Join(path.Dir(filename), "../platform/migrations")
	err := db.DropDatabase(migrationFiles, testDB)
	if err != nil {
		return err
	}
	err = db.MigrateUp(migrationFiles, testDB)
	if err != nil {
		return err
	}
	return nil
}

var testDB *sqlx.DB

func TestMain(m *testing.M) {
	println("Configuring user test database")

	testDB, err := db.OpenTestDatabase()
	if err != nil {
		return
	}

	err = createTestDatabase(testDB)
	if err != nil {
		return
	}

	_, err = Create(testDB,
		"test_user@example.com",
		gofakeit.Password(true, true, true, true, true, 32),
	)

	if err != nil {
		return
	}
	flag.Parse()
	result := m.Run()
	println("All tests are done")
	os.Exit(result)
}

func TestCanGetUserByID(t *testing.T) {

	testDB, err := db.OpenTestDatabase()
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	user, err := GetByID(testDB, 1)
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	if user.Email != "test_user@example.com" {
		t.Log("User email stored incorrectly.")
		t.Fail()
	}
}
