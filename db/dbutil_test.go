package db_test

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"gitlab.com/arcanecrypto/teslacoil/db"

	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
)

var (
	databaseConfig = testutil.GetDatabaseConfig("db")
	testDB         = testutil.InitDatabase(databaseConfig)
)

func TestMain(m *testing.M) {
	build.SetLogLevels(logrus.InfoLevel)

	result := m.Run()

	if err := testDB.Close(); err != nil {
		panic(err.Error())
	}
	os.Exit(result)
}

func TestGetEverythingFromTable(t *testing.T) {
	t.Parallel()

	if _, err := testDB.Exec(
		"CREATE TABLE test_table (foobar VARCHAR(256), bazfoo INT NOT NULL)"); err != nil {
		testutil.FatalMsgf(t, "Could not create table: %+v", err)
	}

	rows, err := db.GetEverythingFromTable(testDB, "test_table")
	if err != nil {
		testutil.FatalMsg(t, err)
	}

	if len(rows) != 0 {
		testutil.FatalMsgf(t, "Rows had unexpected size: %d! Rows: %+v", len(rows), rows)
	}
	insertQuery := func(index int) string {
		return fmt.Sprintf("INSERT INTO test_table VALUES ('test_%d', %d)", index, index)
	}

	if _, err := testDB.Exec(insertQuery(0)); err != nil {
		testutil.FatalMsgf(t, "Could not insert row: %+v", err)
	}

	rows, err = db.GetEverythingFromTable(testDB, "test_table")
	if err != nil {
		testutil.FatalMsg(t, err)
	}
	if len(rows) != 1 {
		testutil.FatalMsgf(t, "Rows had unexpected size: %d", len(rows))
	}

	if _, err := testDB.Exec(insertQuery(1)); err != nil {
		testutil.FatalMsgf(t, "Could not insert row: %+v", err)
	}
	rows, err = db.GetEverythingFromTable(testDB, "test_table")
	if err != nil {
		testutil.FatalMsg(t, err)
	}
	if len(rows) != 2 {
		testutil.FatalMsgf(t, "Rows had unexpected size: %d", len(rows))
	}

	expected := [][]string{
		{"test_0", "0"},
		{"test_1", "1"},
	}
	if !reflect.DeepEqual(expected, rows) {
		testutil.FatalMsgf(t, "Expected: %+v, actual: %+v", expected, rows)
	}
}
