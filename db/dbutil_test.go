package db_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	_, err := testDB.Exec(
		"CREATE TABLE test_table (foobar VARCHAR(256), bazfoo INT NOT NULL)")
	require.NoError(t, err)

	rows, err := db.GetEverythingFromTable(testDB, "test_table")
	require.NoError(t, err)

	assert.Empty(t, rows, 0)

	insertQuery := func(index int) string {
		return fmt.Sprintf("INSERT INTO test_table VALUES ('test_%d', %d)", index, index)
	}

	_, err = testDB.Exec(insertQuery(0))
	require.NoError(t, err)

	rows, err = db.GetEverythingFromTable(testDB, "test_table")
	require.NoError(t, err)
	assert.Len(t, rows, 1)

	_, err = testDB.Exec(insertQuery(1))
	require.NoError(t, err)

	rows, err = db.GetEverythingFromTable(testDB, "test_table")
	require.NoError(t, err)

	assert.Len(t, rows, 2)

	expected := [][]string{
		{"test_0", "0"},
		{"test_1", "1"},
	}
	assert.Equal(t, expected, rows)
}
