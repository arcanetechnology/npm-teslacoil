package httptestutil

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/arcanecrypto/teslacoil/db"

	"gitlab.com/arcanecrypto/teslacoil/api/apierr"
	"gitlab.com/arcanecrypto/teslacoil/api/httptypes"
	"gitlab.com/arcanecrypto/teslacoil/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/models/users"

	"github.com/lightningnetwork/lnd/lnrpc"
	"gitlab.com/arcanecrypto/teslacoil/api/auth"
)

// Server is something that can serve HTTP requests
type Server interface {
	ServeHTTP(response http.ResponseWriter, request *http.Request)
}

// TestHarness is a structure that allows us to execute tests that need
// HTTP serving capabilities, as well as other potential external services.
// At the moment this only includes HTTP serving, but in the future this
// is likely to expand.
type TestHarness struct {
	server   Server
	database *db.DB
}

func NewTestHarness(server Server, database *db.DB) TestHarness {
	randomKey, err := rsa.GenerateKey(cryptorand.Reader, 4096)
	if err != nil {
		panic(fmt.Errorf("could not generate RSA key: %w", err))
	}

	auth.SetJwtPrivateKey(randomKey)
	return TestHarness{server: server, database: database}
}

func (harness *TestHarness) CreateUserNoVerifyEmail(t *testing.T, args users.CreateUserArgs) map[string]interface{} {
	t.Helper()

	require.NotEmpty(t, args.Password, "You forgot to set the password!")
	require.NotEmpty(t, args.Email, "You forgot to set the email!")

	var firstName string
	var lastName string
	if args.FirstName != nil {
		firstName = fmt.Sprintf("%q", *args.FirstName)
	} else {
		firstName = "null"
	}

	if args.LastName != nil {
		lastName = fmt.Sprintf("%q", *args.LastName)
	} else {
		lastName = "null"
	}

	createUserRequest := GetRequest(t, RequestArgs{
		Path:   "/users",
		Method: "POST",
		Body: fmt.Sprintf(`{
			"email": %q,
			"password": %q,
			"firstName": %s,
			"lastName": %s
		}`, args.Email, args.Password, firstName, lastName),
	})

	return harness.AssertResponseOkWithJson(t, createUserRequest)
}

func (harness *TestHarness) VerifyEmail(t *testing.T, email string) users.User {
	t.Helper()
	token, err := users.GetEmailVerificationToken(harness.database, email)
	require.NoError(t, err)

	verifyEmailRequest := GetRequest(t, RequestArgs{
		Path:   "/users/verify_email",
		Method: "PUT",
		Body: fmt.Sprintf(`{
			"token": %q
		}`, token),
	})

	harness.AssertResponseOk(t, verifyEmailRequest)

	verified, err := users.GetByEmail(harness.database, email)
	require.NoError(t, err)

	assert.True(t, verified.HasVerifiedEmail, "User hasn't verified email!")
	return verified
}

func (harness *TestHarness) CreateUser(t *testing.T, args users.CreateUserArgs) users.User {
	t.Helper()

	createUserResponse := harness.CreateUserNoVerifyEmail(t, args)
	email, ok := createUserResponse["email"].(string)
	assert.True(t, ok, createUserResponse)

	return harness.VerifyEmail(t, email)

}

type AuthRequestArgs struct {
	AccessToken string
	Path        string
	Method      string
	Body        string
}

// GetAuthRequest returns a HTTP request that carries a proper auth header
// and an optional JSON body
func GetAuthRequest(t *testing.T, args AuthRequestArgs) *http.Request {
	t.Helper()
	require.NotEmpty(t, args.AccessToken, "You forgot to set AccessToken")

	req := GetRequest(t, RequestArgs{Path: args.Path,
		Method: args.Method, Body: args.Body})
	req.Header.Set("Authorization", args.AccessToken)
	return req
}

type RequestArgs struct {
	Path   string
	Method string
	Body   string
}

// GetRequest returns a HTTP request with an optional JSON body
func GetRequest(t *testing.T, args RequestArgs) *http.Request {
	t.Helper()
	require.NotEmpty(t, args.Path, "You forgot to set Path")
	require.NotEmpty(t, args.Method, "You forgot to set Method")

	var body *bytes.Buffer
	var js interface{}
	if args.Body == "" {
		body = &bytes.Buffer{}
		// we have valid JSON
	} else if json.Unmarshal([]byte(args.Body), &js) == nil {
		// marshal again, to remove uneccesary whitespace
		marshalled, err := json.Marshal(js)
		require.NoError(t, err)
		body = bytes.NewBuffer(marshalled)
	} else {
		assert.FailNow(t, fmt.Sprintf("Body was not valid JSON: %s", args.Body))
	}

	res, err := http.NewRequest(args.Method, args.Path, body)
	require.NoError(t, err)
	return res
}

// Word that starts with ERR_ and only contains A-Z, _ or digits
var uppercaseAndUnderScoreRegex = regexp.MustCompile("^ERR_([A-Z]|_|[0-9])+$")

func assertErrorIsOk(t *testing.T, response *httptest.ResponseRecorder) (*httptest.ResponseRecorder, httptypes.StandardErrorResponse) {

	body, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)

	var parsed httptypes.StandardErrorResponse
	require.NoError(t, json.Unmarshal(body, &parsed))

	assert.NotEmpty(t, parsed.ErrorField.Message, string(body))
	assert.NotEmpty(t, parsed.ErrorField.Code, string(body))
	assert.Regexp(t, uppercaseAndUnderScoreRegex, parsed.ErrorField.Code)

	assert.False(t, stderrors.Is(parsed, apierr.ErrUnknownError), "Error was ErrUnknownError! We should always make sure we're setting a sensible error")

	assert.NotNil(t, parsed.ErrorField.Fields, "Fields was nil! Expected at least empty list")
	for _, field := range parsed.ErrorField.Fields {
		assert.NotEqual(t, field.Code, apierr.UnknownValidationTag, "Encountered unknown validation tag! We should make sure all valiation tags get a nice error message.")
	}
	return response, parsed
}

// Asserts that the given request fails, and that it conforms to our
// expected error format.
func (harness *TestHarness) AssertResponseNotOk(t *testing.T, request *http.Request) (*httptest.ResponseRecorder, httptypes.StandardErrorResponse) {
	t.Helper()
	response := httptest.NewRecorder()
	harness.server.ServeHTTP(response, request)
	if response.Code < 300 {
		assert.Fail(t, "", "Got success code (%d) on path %s", response.Code, extractMethodAndPath(request))
	}

	return assertErrorIsOk(t, response)

}

// AssertResponseNotOkWithCode checks that the given request results in the
// given HTTP status code. It returns the response to the request.
func (harness *TestHarness) AssertResponseNotOkWithCode(t *testing.T, request *http.Request, code int) (*httptest.ResponseRecorder, httptypes.StandardErrorResponse) {
	require.Truef(t, code >= 100 && code <= 500, "Given code (%d) is not a valid HTTP code", code)
	t.Helper()

	reqBody, err := ioutil.ReadAll(request.Body)
	require.NoError(t, err)

	request.Body = ioutil.NopCloser(bytes.NewReader(reqBody))

	response, error := harness.AssertResponseNotOk(t, request)
	resBody := response.Body.String()
	if resBody == "" {
		resBody = "empty body"
	}
	require.Equalf(t, code, response.Code, "%s %s: Request: %s. Response: %s", request.Method, request.URL.Path, reqBody, resBody)
	return response, error
}

func (harness *TestHarness) AssertResponseOkWithBody(t *testing.T, request *http.Request) bytes.Buffer {
	t.Helper()
	response := harness.AssertResponseOk(t, request)

	assert.NotEmpty(t, response.Body, "Body was empty!")

	return *response.Body
}

// First performs `assertResponseOk`, then asserts that the body of the response
// can be parsed as JSON, and then returns the parsed JSON
func (harness *TestHarness) AssertResponseOkWithJson(t *testing.T, request *http.Request) map[string]interface{} {
	t.Helper()
	var destination map[string]interface{}
	harness.AssertResponseOKWithStruct(t, request, &destination)
	return destination
}

// First performs `assertResponseOk`, then asserts that the body of the response
// can be parsed as a JSON list, and then returns the parsed JSON list
func (harness *TestHarness) AssertResponseOkWithJsonList(t *testing.T, request *http.Request) []map[string]interface{} {
	t.Helper()

	var destination []map[string]interface{}
	harness.AssertResponseOKWithStruct(t, request, &destination)
	assert.NotNil(t, destination, "Did not receive JSON list, but null")

	return destination

}

func extractMethodAndPath(req *http.Request) string {
	return req.Method + " " + req.URL.Path
}

// Performs the given request against the API. Asserts that the
// response completed successfully. Returns the response from the API
func (harness *TestHarness) AssertResponseOk(t *testing.T, request *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	var bodyBytes []byte
	var err error
	if request.Body != nil {
		// read the body bytes for potential error messages later
		bodyBytes, err = ioutil.ReadAll(request.Body)
		require.NoError(t, err)

		request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	response := httptest.NewRecorder()
	harness.server.ServeHTTP(response, request)

	if response.Code >= 300 {
		methodAndPath := extractMethodAndPath(request)
		body := response.Body.String()
		assert.Failf(t, "Got failure response", "code: %d, path %s: %s", response.Code, methodAndPath, body)
		_, _ = assertErrorIsOk(t, response)
	}

	// restore the request body so it can be served again
	request.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

	return response
}

// AssertResponseOKWithStruct attempts to unmarshal the body into the
// struct passed as an argument. third argument MUST be a pointer to a
// struct
func (harness *TestHarness) AssertResponseOKWithStruct(t *testing.T, request *http.Request, s interface{}) {
	t.Helper()

	response := harness.AssertResponseOkWithBody(t, request)

	assert.NoError(t, json.Unmarshal(response.Bytes(), s))
}

func (harness *TestHarness) AuthenticaticateUser(t *testing.T, args users.CreateUserArgs) (string, int) {

	loginUserReq := GetRequest(t, RequestArgs{
		Path:   "/login",
		Method: "POST",
		Body: fmt.Sprintf(`{
			"email": %q,
			"password": %q
		}`, args.Email, args.Password),
	})

	fail := func(json map[string]interface{}, key, methodAndPath string) {
		assert.Fail(t, "Returned JSON (%+v) did not have string property '%s'. Path: %s",
			json, key, methodAndPath)
	}

	jsonRes := harness.AssertResponseOkWithJson(t, loginUserReq)

	tokenPath := "accessToken"
	maybeNilToken, ok := jsonRes["accessToken"]
	if !ok {
		fail(jsonRes, tokenPath, extractMethodAndPath(loginUserReq))
	}

	var token string
	switch untypedToken := maybeNilToken.(type) {
	case string:
		token = untypedToken
	default:
		fail(jsonRes, tokenPath, extractMethodAndPath(loginUserReq))
	}

	userID, ok := jsonRes["userId"]
	if !ok {
		fail(jsonRes, "userId", extractMethodAndPath(loginUserReq))
	}

	var idFloat float64
	switch untypedId := userID.(type) {
	case float64:
		idFloat = untypedId
	default:
		fail(jsonRes, "userId", extractMethodAndPath(loginUserReq))
	}
	id := int(idFloat)

	// we want to alternate between authenticating users with an API key or
	// a JWT. We flip a coin here, and if the coin flip succeeds we create an
	// API key and return that.
	coinFlip := rand.Float32()
	if coinFlip < 0.5 {
		return token, id
	}

	apiKeyRequest := GetAuthRequest(t, AuthRequestArgs{
		Path:        "/apikey",
		Method:      "POST",
		AccessToken: token,
		Body: `{
			"readWallet": true,
			"createInvoice": true,
			"sendTransaction": true,
			"editAccount": true
		}`,
	})
	apiKeyJson := harness.AssertResponseOkWithJson(t, apiKeyRequest)
	apiKeyPath := "key"
	maybeNilKey, ok := apiKeyJson[apiKeyPath]
	if !ok {
		fail(apiKeyJson, apiKeyPath, extractMethodAndPath(apiKeyRequest))
	}

	switch untypedKey := maybeNilKey.(type) {
	case string:
		return untypedKey, id
	default:
		fail(apiKeyJson, apiKeyPath, extractMethodAndPath(apiKeyRequest))
		panic("wont reach this")
	}
}

// Creates and and authenticates a user with the given email and password.
// We either log in (and return an access token), or create an API key (and
// return that). They should be equivalent (until scopes are implemented, so
// this should not matter and might uncover some edge cases.
func (harness *TestHarness) CreateAndAuthenticateUser(t *testing.T, args users.CreateUserArgs) (string, int) {
	_ = harness.CreateUser(t, args)

	return harness.AuthenticaticateUser(t, args)

}

func (harness *TestHarness) GiveUserBalance(t *testing.T, lncli lnrpc.LightningClient,
	bitcoin bitcoind.TeslacoilBitcoind, accessToken string, amount int64) {

	getDepositAddr := GetAuthRequest(t, AuthRequestArgs{
		AccessToken: accessToken,
		Path:        "/deposit",
		Method:      "POST",
		Body: fmt.Sprintf(`{
			"forceNewAddress": %t
		}`, false),
	})

	type res struct {
		Address string `json:"address"`
	}
	var r res

	harness.AssertResponseOKWithStruct(t, getDepositAddr, &r)

	_, err := lncli.SendCoins(context.Background(), &lnrpc.SendCoinsRequest{
		Addr:   r.Address,
		Amount: amount,
	})
	require.NoError(t, err)

	// confirm it
	_, err = bitcoin.Btcctl().Generate(7)
	require.NoError(t, err)
}
