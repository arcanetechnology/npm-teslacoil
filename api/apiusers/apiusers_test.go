package apiusers_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"gitlab.com/arcanecrypto/teslacoil/api"
	"gitlab.com/arcanecrypto/teslacoil/api/apierr"
	"gitlab.com/arcanecrypto/teslacoil/async"
	"gitlab.com/arcanecrypto/teslacoil/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/models/users"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/httptestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/lntestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/mock"
)

var (
	testDB              *db.DB
	h                   httptestutil.TestHarness
	mockLightningClient = lntestutil.GetLightningMockClient()
	mockBitcoindClient  = bitcoind.GetBitcoinMockClient()
	mockHttpPoster      = testutil.GetMockHttpPoster()
	mockSendGridClient  = mock.GetMockSendGridClient()
	conf                = api.Config{
		LogLevel: logrus.InfoLevel,
		Network:  chaincfg.RegressionNetParams,
	}
)

func init() {
	dbConf := testutil.GetDatabaseConfig("api_users")
	testDB = testutil.InitDatabase(dbConf)

	app, err := api.NewApp(testDB, mockLightningClient,
		mockSendGridClient, mockBitcoindClient,
		mockHttpPoster, conf)

	if err != nil {
		panic(err)
	}

	h = httptestutil.NewTestHarness(app.Router, testDB)
}

func TestCreateUser(t *testing.T) {
	t.Parallel()
	t.Run("creating a user must fail with a bad password", func(t *testing.T) {
		t.Parallel()

		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path: "/users", Method: "POST",
			Body: fmt.Sprintf(`{
				"password": "foobar",
				"email": %q
			}`, gofakeit.Email()),
		})
		_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})
	t.Run("Creating a user must fail without an email", func(t *testing.T) {
		t.Parallel()

		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path: "/users", Method: "POST",
			Body: fmt.Sprintf(`{
				"password": %q
			}`, gofakeit.Password(true, true, true, true, true, 32)),
		})
		_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("Creating a user must fail with an empty email", func(t *testing.T) {
		t.Parallel()

		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path: "/users", Method: "POST",
			Body: fmt.Sprintf(`{
				"password": %q,
				"email": ""
			}`, gofakeit.Password(true, true, true, true, true, 32)),
		})
		_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("creating an user with an invalid email should fail", func(t *testing.T) {
		t.Parallel()

		pass := gofakeit.Password(true, true, true, true, true, 32)
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/users",
			Method: "POST",
			Body: fmt.Sprintf(`{
				"password": %q,
				"email": "this-is-not@a-valid-mail"
			}`, pass),
		})
		_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("It should be possible to create a user without any names", func(t *testing.T) {
		t.Parallel()

		email := gofakeit.Email()
		pass := gofakeit.Password(true, true, true, true, true, 32)
		testutil.DescribeTest(t)
		body := httptestutil.RequestArgs{
			Path: "/users", Method: "POST",
			Body: fmt.Sprintf(`{
				"password": %q,
				"email": "%s"
			}`, pass, email),
		}
		req := httptestutil.GetRequest(t, body)
		jsonRes := h.AssertResponseOkWithJson(t, req)
		testutil.AssertEqual(t, jsonRes["email"], email)
		testutil.AssertEqual(t, jsonRes["firstName"], nil)
		testutil.AssertEqual(t, jsonRes["lastName"], nil)
	})

	t.Run("It should be possible to create a user with names", func(t *testing.T) {
		t.Parallel()

		email := gofakeit.Email()
		pass := gofakeit.Password(true, true, true, true, true, 32)
		firstName := gofakeit.FirstName()
		lastName := gofakeit.LastName()
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path: "/users", Method: "POST",
			Body: fmt.Sprintf(`{
				"password": %q,
				"email": "%s",
				"firstName": "%s", 
				"lastName": "%s"
			}`, pass, email, firstName, lastName),
		})
		jsonRes := h.AssertResponseOkWithJson(t, req)
		testutil.AssertEqual(t, jsonRes["email"], email)

		user := h.VerifyEmail(t, email)
		testutil.AssertEqual(t, *user.Firstname, firstName)
		testutil.AssertEqual(t, *user.Lastname, lastName)
	})
}

func TestPostUsersRoute(t *testing.T) {

	t.Run("create a user", func(t *testing.T) {

		// we need a new email sender here to record the amount of emails
		// sent correctly
		postUserEmailClient := mock.GetMockSendGridClient()
		app, err := api.NewApp(testDB, mockLightningClient,
			postUserEmailClient, mockBitcoindClient,
			mockHttpPoster, conf)
		require.NoError(t, err)

		harness := httptestutil.NewTestHarness(app.Router, testDB)
		preCreationEmails := postUserEmailClient.GetEmailVerificationMails()

		email := gofakeit.Email()
		pass := gofakeit.Password(true, true, true, true, true, 32)
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/users",
			Method: "POST",
			Body: fmt.Sprintf(`{
				"email": %q,
				"password": %q
			}`, email, pass),
		})
		jsonRes := harness.AssertResponseOkWithJson(t, req)
		require.False(t, t.Failed())

		var postCreation int
		// emails are sent in a go routine
		retry := func() error {
			postCreation = postUserEmailClient.GetEmailVerificationMails()
			if preCreationEmails+1 != postCreation {
				return fmt.Errorf("emails sent before creating user (%d) does not match ut with emails sent after creating user (%d)", preCreationEmails, postCreation)
			}
			return nil
		}

		require.NoError(t,
			async.RetryNoBackoff(10, time.Millisecond*50, retry),
			email)

		testutil.AssertEqual(t, jsonRes["firstName"], nil)
		testutil.AssertEqual(t, jsonRes["lastName"], nil)
		testutil.AssertEqual(t, jsonRes["email"], email)
		testutil.AssertEqual(t, jsonRes["balance"], 0)

		t.Run("not create the same user twice", func(t *testing.T) {
			otherReq := httptestutil.GetRequest(t, httptestutil.RequestArgs{
				Path:   "/users",
				Method: "POST",
				Body: fmt.Sprintf(`{
				"email": %q,
				"password": %q
			}`, email, pass),
			})
			_, err = harness.AssertResponseNotOk(t, otherReq)
			testutil.AssertEqual(t, apierr.ErrUserAlreadyExists, err)
			testutil.AssertEqual(t, postCreation, postUserEmailClient.GetEmailVerificationMails())
		})
	})

	t.Run("not create a user with no pass", func(t *testing.T) {
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/users",
			Method: "POST",
			Body: fmt.Sprintf(`{
				"email": %q
			}`, gofakeit.Email()),
		})
		_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("not create a user with no email", func(t *testing.T) {
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/users",
			Method: "POST",
			Body: fmt.Sprintf(`{
				"password": %q
			}`, gofakeit.Password(true, true, true, true, true, 32)),
		})
		_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})
}

func TestPutUserRoute(t *testing.T) {
	t.Parallel()

	email := gofakeit.Email()
	password := gofakeit.Password(true, true, true, true, true, 32)
	accessToken, _ := h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
		Email:    email,
		Password: password,
	})

	t.Run("updating with an invalid email should fail", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetAuthRequest(t, httptestutil.AuthRequestArgs{
			AccessToken: accessToken,
			Path:        "/user",
			Method:      "PUT",
			Body: `{
				"email": "bad-email.coming.through"
			}`,
		})
		_, _ = h.AssertResponseNotOkWithCode(t, req, http.StatusBadRequest)
	})

	t.Run("update email, first name and last name", func(t *testing.T) {
		t.Parallel()
		newFirst := "new-firstname"
		newLast := "new-lastname"
		newEmail := gofakeit.Email()

		// Update User endpoint
		updateUserReq := httptestutil.GetAuthRequest(t,
			httptestutil.AuthRequestArgs{
				AccessToken: accessToken,
				Path:        "/user", Method: "PUT",
				Body: fmt.Sprintf(`
			{
				"firstName": %q,
				"lastName": %q,
				"email": %q
			}`, newFirst, newLast, newEmail)})

		jsonRes := h.AssertResponseOkWithJson(t, updateUserReq)
		testutil.AssertEqual(t, jsonRes["firstName"], newFirst)
		testutil.AssertEqual(t, jsonRes["lastName"], newLast)
		testutil.AssertEqual(t, jsonRes["email"], newEmail)

		// Get User endpoint
		getUserReq := httptestutil.GetAuthRequest(t,
			httptestutil.AuthRequestArgs{
				AccessToken: accessToken,
				Method:      "GET", Path: "/user",
			})

		// Verify that update and get returns the same
		jsonRes = h.AssertResponseOkWithJson(t, getUserReq)
		testutil.AssertEqual(t, jsonRes["firstName"], newFirst)
		testutil.AssertEqual(t, jsonRes["lastName"], newLast)
		testutil.AssertEqual(t, jsonRes["email"], newEmail)
	})
}

func TestSendEmailVerificationEmail(t *testing.T) {
	t.Parallel()

	t.Run("reject a bad email request", func(t *testing.T) {
		t.Parallel()
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/user/verify_email",
			Method: "POST",
			Body: `{
			"email": "foobar"
			}`,
		})
		_, err := h.AssertResponseNotOk(t, req)
		testutil.AssertEqual(t, apierr.ErrRequestValidationFailed, err)
	})

	// we don't want to leak information about users, so we respond with 200
	t.Run("respond with 200 for a non-existant user", func(t *testing.T) {
		t.Parallel()
		email := gofakeit.Email()
		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/user/verify_email",
			Method: "POST",
			Body: fmt.Sprintf(`{
			"email": %q
			}`, email),
		})

		h.AssertResponseOk(t, req)
	})

	t.Run("Send out a password verification email for an existing user", func(t *testing.T) {
		t.Parallel()

		// we need a new email sender here to record the amount of emails
		// sent correctly
		emailClient := mock.GetMockSendGridClient()
		app, err := api.NewApp(testDB, mockLightningClient,
			emailClient, mockBitcoindClient,
			mockHttpPoster, conf)
		require.NoError(t, err)

		harness := httptestutil.NewTestHarness(app.Router, testDB)
		email := gofakeit.Email()
		password := gofakeit.Password(true, true, true, true, true, 32)

		emailsPreSignup := emailClient.GetEmailVerificationMails()
		_ = h.CreateUserNoVerifyEmail(t, users.CreateUserArgs{
			Email:    email,
			Password: password,
		})

		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/user/verify_email",
			Method: "POST",
			Body: fmt.Sprintf(`{
			"email": %q
			}`, email),
		})

		// we'll get sent one email when signing up
		await := func() bool {
			return emailClient.GetEmailVerificationMails() == emailsPreSignup+1
		}
		require.NoError(t, async.AwaitNoBackoff(10, time.Millisecond*50, await))

		emailsPreReq := emailClient.GetEmailVerificationMails()
		harness.AssertResponseOk(t, req)
		retry := func() error {
			postReq := emailClient.GetEmailVerificationMails()
			if emailsPreReq+1 != postReq {
				return fmt.Errorf("mismatch between emails pre equest (%d) and post request (%d)", emailsPreReq, postReq)
			}
			return nil
		}
		require.NoError(t, async.RetryNoBackoff(10, time.Millisecond*50, retry))
	})

	t.Run("not send out an email for already verified users", func(t *testing.T) {
		t.Parallel()

		// we need a new email sender here to record the amount of emails
		// sent correctly
		emailClient := mock.GetMockSendGridClient()
		app, err := api.NewApp(testDB, mockLightningClient,
			emailClient, mockBitcoindClient,
			mockHttpPoster, conf)
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		harness := httptestutil.NewTestHarness(app.Router, testDB)
		email := gofakeit.Email()
		password := gofakeit.Password(true, true, true, true, true, 32)
		_ = h.CreateUser(t, users.CreateUserArgs{
			Email:    email,
			Password: password,
		})

		req := httptestutil.GetRequest(t, httptestutil.RequestArgs{
			Path:   "/user/verify_email",
			Method: "POST",
			Body: fmt.Sprintf(`{
			"email": %q
			}`, email),
		})
		emailsPreReq := emailClient.GetEmailVerificationMails()
		harness.AssertResponseOk(t, req)
		if err = async.RetryNoBackoff(5, time.Millisecond*10, func() error {
			postReq := emailClient.GetEmailVerificationMails()
			if emailsPreReq+1 != postReq {
				return fmt.Errorf("mismatch between emails pre equest (%d) and post request (%d", emailsPreReq, postReq)
			}
			return nil
		}); err == nil {
			testutil.FatalMsg(t, "Sent out emails for already verified user!")
		}
	})

}
