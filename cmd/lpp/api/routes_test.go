package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
)

var (
	databaseConfig = testutil.GetDatabaseConfig("routes")
	testDB         *db.DB
)

func TestMain(m *testing.M) {
	build.SetLogLevel(logrus.InfoLevel)
	testDB = testutil.InitDatabase(databaseConfig)

	result := m.Run()
	if err := testDB.Close(); err != nil {
		panic(err.Error())
	}
	os.Exit(result)
}

func TestPutUserRoute(t *testing.T) {
	testutil.DescribeTest(t)

	conf := Config{LogLevel: logrus.InfoLevel}
	app, err := NewApp(testDB, testutil.GetLightningMockClient(), conf)
	if err != nil {
		testutil.FatalMsgf(t, "Could not initialize app: %v", err)
	}

	createUserRes := httptest.NewRecorder()
	createUserRequest, _ := http.NewRequest(
		"POST", "/users",
		bytes.NewBuffer([]byte(`{ "email": "foobar", "password": "barfoo" }`)))
	app.Router.ServeHTTP(createUserRes, createUserRequest)

	loginUserRes := httptest.NewRecorder()
	loginUserReq := httptest.NewRequest(
		"POST", "/login",
		bytes.NewBuffer([]byte(
			`{
			"email":    "foobar",
			"password": "barfoo"
			}`,
		)))
	app.Router.ServeHTTP(loginUserRes, loginUserReq)
	if loginUserRes.Code != 200 {
		testutil.FatalMsgf(t, "Got failure code when logging in: %d", loginUserRes.Code)
	}

	marshalledLoginRes := LoginResponse{}
	_ = json.Unmarshal(loginUserRes.Body.Bytes(), &marshalledLoginRes)

	jsonBody := `{
		"firstName": "new-firstname",
		"lastName":  "new-lastname",
		"email":     "new-email"
	}`
	updateUserRes := httptest.NewRecorder()
	updateUserReq, _ := http.NewRequest("PUT", "/user",
		bytes.NewBuffer([]byte(jsonBody)))

	updateUserReq.Header.Set("Authorization", marshalledLoginRes.AccessToken)

	app.Router.ServeHTTP(updateUserRes, updateUserReq)

	marshalledUpdateRes := UserResponse{}
	_ = json.Unmarshal(updateUserRes.Body.Bytes(), &marshalledUpdateRes)
	if marshalledUpdateRes.Email != "new-email" ||
		marshalledUpdateRes.Firstname == nil ||
		*marshalledUpdateRes.Firstname != "new-firstname" ||
		marshalledUpdateRes.Lastname == nil ||
		*marshalledUpdateRes.Lastname != "new-lastname" {
		testutil.FatalMsgf(t,
			"PUT /user did not update user! Got: %+v",
			string(updateUserRes.Body.Bytes()))
	}

}
