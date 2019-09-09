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
	os.Exit(result)
}

func TestParseBearerJWT(t *testing.T) {
	t.Parallel()
	testutil.DescribeTest(t)
	token := "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6InRlc3QxQGdtYWlsLmNvbSIsInVzZXJfaWQiOjEsImV4cCI6MTU2ODA0NjA3M30.hXVfKvnbsubvImxb32MQjyneXutDS9JHdSSAoPnULb4"

	_, _, err := parseBearerJWT(token)
	if err != nil {
		testutil.FatalMsgf(t, "Could not parse bearer token: %v", err)
	}
}

func TestPutUserRoute(t *testing.T) {
	testutil.DescribeTest(t)

	conf := Config{LightningConfig: testutil.GetLightingConfig()}
	app, err := NewApp(testDB, conf)
	if err != nil {
		testutil.FatalMsgf(t, "Could not initialize app: %v", err)
	}

	createUserRes := httptest.NewRecorder()

	createUserBody, _ := json.Marshal(
		map[string]string{"email": "foobar", "password": "barfoo"})
	createUserRequest, _ := http.NewRequest(
		"POST", "/users", bytes.NewBuffer(createUserBody))
	app.Router.ServeHTTP(createUserRes, createUserRequest)

	loginUserBody, _ := json.Marshal(
		map[string]string{
			"email":    "foobar",
			"password": "barfoo",
		},
	)
	loginUserRes := httptest.NewRecorder()
	loginUserReq := httptest.NewRequest(
		"POST", "/login", bytes.NewBuffer(loginUserBody))
	app.Router.ServeHTTP(loginUserRes, loginUserReq)
	marshalledLoginRes := LoginResponse{}
	_ = json.Unmarshal(loginUserRes.Body.Bytes(), &marshalledLoginRes)

	updateUserBody, _ := json.Marshal(
		map[string]string{"email": "new-email"},
	)
	updateUserRes := httptest.NewRecorder()
	updateUserReq, _ := http.NewRequest("PUT", "/user", bytes.NewBuffer(updateUserBody))
	updateUserReq.Header.Set("Authorization", marshalledLoginRes.AccessToken)

	app.Router.ServeHTTP(updateUserRes, updateUserReq)

	marshalledUpdateRes := UserResponse{}
	_ = json.Unmarshal(updateUserRes.Body.Bytes(), &marshalledUpdateRes)
	if marshalledUpdateRes.Email != "new-email" {
		testutil.FatalMsgf(t,
			"PUT /user did not update email! Expected: new-email, got: %v",
			marshalledLoginRes.Email)
	}

}
