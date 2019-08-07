package users

import (
	"flag"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/jmoiron/sqlx"
	"gitlab.com/arcanecrypto/lpp"

	"github.com/brianvoe/gofakeit"
	"gitlab.com/arcanecrypto/lpp/internal/platform/db"
)

var migrationsPath = path.Join(
	os.Getenv("GOPATH"),
	"/src/gitlab.com/arcanecrypto/lpp/internal/platform/migrations")

func createTestDatabase(testDB *sqlx.DB) error {
	err := db.DropDatabase(migrationsPath, testDB)
	if err != nil {
		return err
	}
	err = db.MigrateUp(migrationsPath, testDB)
	if err != nil {
		return err
	}
	return nil
}

var testDB *sqlx.DB

func TestMain(m *testing.M) {
	println("Configuring user test database")
	lpp.InitLogRotator("/home/bo/gocode/src/gitlab.com/arcanecrypto/lpp/logs/lpp.log", 10, 3)

	testDB, err := db.OpenTestDatabase()
	if err != nil {
		return
	}
	fmt.Println(testDB.Ping())

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

func TestCanGetUserByEmail(t *testing.T) {

	testDB, err := db.OpenTestDatabase()
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	user, err := GetByEmail(testDB, "test_user@example.com")
	if err != nil {
		t.Log(err)
		t.Fail()
	}

	t.Error(user)
	if user.Email != "test_user@example.com" {
		t.Log("User email stored incorrectly.")
		t.Fail()
	}
}
