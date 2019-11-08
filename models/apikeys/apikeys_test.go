package apikeys_test

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/models/apikeys"
	"gitlab.com/arcanecrypto/teslacoil/models/users"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/txtest"
	"gitlab.com/arcanecrypto/teslacoil/testutil/userstestutil"
)

var (
	databaseConfig = testutil.GetDatabaseConfig("api_keys")
	testDB         *db.DB
)

func TestMain(m *testing.M) {
	gofakeit.Seed(0)
	build.SetLogLevels(logrus.ErrorLevel)

	testDB = testutil.InitDatabase(databaseConfig)

	result := m.Run()

	os.Exit(result)
}

func TestNew(t *testing.T) {
	t.Parallel()
	user := userstestutil.CreateUserOrFail(t, testDB)
	t.Run("creating an api key should work", func(t *testing.T) {
		t.Parallel()
		desc := gofakeit.Sentence(10)
		rawKey, key, err := apikeys.New(testDB, user.ID, apikeys.AllPermissions, desc)
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		found, err := apikeys.Get(testDB, rawKey)
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		assert := assert.New(t)
		assert.Equal(key, found)
		assert.True(strings.HasSuffix(rawKey.String(), key.LastLetters))
		assert.True(found.Permissions.EditAccount)
		assert.True(found.Permissions.SendTransaction)
		assert.True(found.Permissions.ReadWallet)
		assert.True(found.Permissions.CreateInvoice)
		assert.Equal(desc, found.Description)
	})

	t.Run("creating an API key with no permissions should fail", func(t *testing.T) {
		t.Parallel()
		_, _, err := apikeys.New(testDB, user.ID, apikeys.Permissions{}, "")
		assert.Error(t, err)
	})

	t.Run("creating an API key with permissions should work", func(t *testing.T) {
		t.Parallel()
		perm := apikeys.RandomPermissionSet()
		_, key, err := apikeys.New(testDB, user.ID, perm, "")
		require.NoError(t, err)
		assert.Equal(t, key.Permissions, perm)
	})

	t.Run("creating an api key with no related user should not work", func(t *testing.T) {
		t.Parallel()
		_, _, err := apikeys.New(testDB, 99999999, apikeys.AllPermissions, "")
		if err == nil {
			testutil.FatalMsg(t, "Created an API key with no corresponding user")
		}
	})

	t.Run("getting an non-existing key should not work", func(t *testing.T) {
		t.Parallel()

		_, err := apikeys.Get(testDB, uuid.Must(uuid.FromString(gofakeit.UUID())))
		if err == nil {
			testutil.FatalMsg(t, "Was able to find non existant key!")
		}
	})
}

func TestGetByUserId(t *testing.T) {
	t.Parallel()
	t.Run("find created API keys", func(t *testing.T) {
		t.Parallel()
		user, err := users.Create(testDB, users.CreateUserArgs{
			Email:    gofakeit.Email(),
			Password: gofakeit.Password(true, true, true, true, true, 32),
		})
		require.NoError(t, err)

		zero, err := apikeys.GetByUserId(testDB, user.ID)
		require.NoError(t, err)
		assert.Len(t, zero, 0)

		perm := apikeys.RandomPermissionSet()
		_, _, err = apikeys.New(testDB, user.ID, perm, "")
		require.NoError(t, err)

		one, err := apikeys.GetByUserId(testDB, user.ID)
		require.NoError(t, err)
		assert.Len(t, one, 1)
		assert.Equal(t, perm, one[0].Permissions)

		two, err := apikeys.GetByUserId(testDB, user.ID)
		require.NoError(t, err)
		assert.Len(t, two, 1)
		assert.Equal(t, perm, two[0].Permissions)
	})

	t.Run("Get empty list of keys for new user", func(t *testing.T) {
		t.Parallel()
		otherUser, err := users.Create(testDB, users.CreateUserArgs{
			Email:    gofakeit.Email(),
			Password: gofakeit.Password(true, true, true, true, true, 32),
		})
		require.NoError(t, err)

		keys, err := apikeys.GetByUserId(testDB, otherUser.ID)
		require.NoError(t, err)
		assert.Len(t, keys, 0)
	})
}

func TestDelete(t *testing.T) {
	t.Parallel()
	user := userstestutil.CreateUserOrFail(t, testDB)

	t.Run("delete a key", func(t *testing.T) {
		t.Parallel()
		_, key, err := apikeys.New(testDB, user.ID, apikeys.AllPermissions, "desc")
		require.NoError(t, err)

		deleted, err := apikeys.Delete(testDB, user.ID, key.HashedKey)
		require.NoError(t, err)

		require.NotNil(t, deleted.DeletedAt)
		assert.WithinDuration(t, time.Now(), *deleted.DeletedAt, time.Second)
		key.DeletedAt = deleted.DeletedAt
		assert.Equal(t, key, deleted)

		_, err = apikeys.GetByHash(testDB, user.ID, key.HashedKey)
		require.Error(t, err)
	})

	t.Run("not delete a non existant key", func(t *testing.T) {
		t.Parallel()
		_, err := apikeys.Delete(testDB, user.ID, txtest.MockPreimage())
		require.Error(t, err)
	})

	t.Run("not delete a key belonging to another user", func(t *testing.T) {
		t.Parallel()
		otherUser := userstestutil.CreateUserOrFail(t, testDB)
		_, key, err := apikeys.New(testDB, user.ID, apikeys.AllPermissions, "desc")
		require.NoError(t, err)

		_, err = apikeys.Delete(testDB, otherUser.ID, key.HashedKey)
		assert.Error(t, err)

		found, err := apikeys.GetByHash(testDB, user.ID, key.HashedKey)
		require.NoError(t, err)
		assert.Equal(t, key, found)

	})
}

func TestGetByHash(t *testing.T) {
	t.Parallel()

	t.Run("get an existing key", func(t *testing.T) {
		t.Parallel()
		user := userstestutil.CreateUserOrFail(t, testDB)
		_, key, err := apikeys.New(testDB, user.ID, apikeys.AllPermissions, "")
		require.NoError(t, err)

		found, err := apikeys.GetByHash(testDB, user.ID, key.HashedKey)
		require.NoError(t, err)

		assert.Equal(t, key, found)
	})

	t.Run("not get another users key", func(t *testing.T) {
		t.Parallel()
		user := userstestutil.CreateUserOrFail(t, testDB)
		otherUser := userstestutil.CreateUserOrFail(t, testDB)
		_, key, err := apikeys.New(testDB, user.ID, apikeys.AllPermissions, "")
		require.NoError(t, err)

		_, err = apikeys.GetByHash(testDB, otherUser.ID, key.HashedKey)
		assert.True(t, errors.Is(err, sql.ErrNoRows), err)

	})

	t.Run("not find a non-existant key", func(t *testing.T) {
		u := uuid.NewV4()
		hasher := sha256.New()
		// according to godoc, this operation never fails
		_, _ = hasher.Write(u.Bytes())
		hashedKey := hasher.Sum(nil)

		user := userstestutil.CreateUserOrFail(t, testDB)

		_, err := apikeys.GetByHash(testDB, user.ID, hashedKey)
		assert.Error(t, err)
	})

}
