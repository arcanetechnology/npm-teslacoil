package errhandling

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/internal/httptypes"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
)

type Request struct {
	Foo int    `form:"foo" json:"foo" binding:"required"`
	Bar string `form:"bar" json:"bar" binding:"required"`
}

var (
	middleware = GetMiddleware(build.Log)
	router     = setupRouter(middleware)
	emptyBody  = bytes.NewBuffer([]byte(""))

	publicError = errors.New("this is a public error")
	metaMessage = "META_MESSAGE"
)

func setupRouter(middleware gin.HandlerFunc) *gin.Engine {
	r := gin.Default()
	r.Use(middleware)
	r.GET("/query", func(c *gin.Context) {
		var req Request
		if c.BindQuery(&req) != nil {
			return
		}
		c.Status(200)
	})
	r.GET("/json", func(c *gin.Context) {
		var req Request
		if c.BindJSON(&req) != nil {
			return
		}
		c.Status(200)
	})
	r.GET("/private", func(c *gin.Context) {
		_ = c.Error(errors.New("this is a private error"))
	})
	r.GET("/public", func(c *gin.Context) {
		_ = c.Error(publicError).SetType(gin.ErrorTypePublic)
	})
	r.GET("/publicWithMeta", func(c *gin.Context) {
		_ = c.Error(publicError).SetType(gin.ErrorTypePublic).SetMeta(metaMessage)
	})
	r.GET("/withCode", func(c *gin.Context) {
		_ = c.AbortWithError(http.StatusUnauthorized, errors.New("with a code"))
	})
	return r
}

func assertErrorResponseOk(t *testing.T, w *httptest.ResponseRecorder, expectedFieldErrors int) httptypes.StandardError {
	bodyBytes, err := ioutil.ReadAll(w.Body)
	if err != nil {
		testutil.FatalMsg(t, err)
	}
	var res httptypes.StandardResponse
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		testutil.FatalMsg(t, err)
	}
	testutil.AssertMsg(t, !(res.Error == nil && res.Result == nil), "Both error and result was nil!")
	testutil.AssertMsg(t, !(res.Error != nil && res.Result != nil), "Both error and result was not nil!")
	testutil.AssertMsg(t, res.Error != nil, "error was nil!")
	testutil.AssertMsg(t, res.Error.Fields != nil, "Fields was nil!")
	testutil.AssertMsgf(t, len(res.Error.Fields) == expectedFieldErrors, "Unexpected number of errors: %d", len(res.Error.Fields))
	return *res.Error
}

func TestJsonValidation(t *testing.T) {
	t.Parallel()
	t.Run("reject bad JSON body request", func(t *testing.T) {
		t.Run("empty body", func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/json", emptyBody)
			router.ServeHTTP(w, req)
			testutil.AssertEqual(t, w.Code, http.StatusBadRequest)
			err := assertErrorResponseOk(t, w, 0)
			testutil.AssertMsg(t, err.Message != "", "Error message was empty")
			testutil.AssertEqual(t, err.Code, ErrInvalidJson)
		})
		t.Run("Invalid JSON", func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/json",
				bytes.NewBuffer([]byte(`{[{"foo": 2 }]`))) // missing }
			router.ServeHTTP(w, req)
			testutil.AssertEqual(t, w.Code, http.StatusBadRequest)
			err := assertErrorResponseOk(t, w, 0)
			testutil.AssertMsg(t, err.Message != "", "Error message was empty")
			testutil.AssertEqual(t, err.Code, ErrInvalidJson)
		})

		t.Run("no parameters", func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/json", bytes.NewBuffer([]byte(`{}`)))
			router.ServeHTTP(w, req)
			testutil.AssertEqual(t, w.Code, http.StatusBadRequest)
			err := assertErrorResponseOk(t, w, 2)
			barOkErrorCheck := false
			fooOkErrorCheck := false
			for _, field := range err.Fields {
				if field.Field == "bar" && field.Message == "bar is required" && field.Code == "required" {
					barOkErrorCheck = true
				}
				if field.Field == "foo" && field.Message == "foo is required" && field.Code == "required" {
					fooOkErrorCheck = true
				}
			}
			testutil.AssertMsg(t, barOkErrorCheck, `"bar" did not have a meaningful message!`)
			testutil.AssertMsg(t, fooOkErrorCheck, `"foo" did not have a meaningful message!`)
		})

		t.Run("just foo", func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/json", bytes.NewBuffer([]byte(`
			{
				"foo": 1
			}
			`)))
			router.ServeHTTP(w, req)
			testutil.AssertEqual(t, w.Code, http.StatusBadRequest)

			err := assertErrorResponseOk(t, w, 1)
			barOkErrorCheck := false
			field := err.Fields[0]
			if field.Field == "bar" && field.Message == "bar is required" && field.Code == "required" {
				barOkErrorCheck = true
			}
			testutil.AssertMsg(t, barOkErrorCheck, `"bar" did not have a meaningful message!`)
		})
		t.Run("just bar", func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/json", bytes.NewBuffer([]byte(`
			{
				"bar": "bazz"
			}
			`)))
			router.ServeHTTP(w, req)
			testutil.AssertEqual(t, w.Code, http.StatusBadRequest)
			err := assertErrorResponseOk(t, w, 1)
			fooOkErrorCheck := false
			field := err.Fields[0]
			if field.Field == "foo" && field.Message == "foo is required" && field.Code == "required" {
				fooOkErrorCheck = true
			}
			testutil.AssertMsg(t, fooOkErrorCheck, `"foo" did not have a meaningful message!`)
		})
	})

	t.Run("accept good JSON request", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET",
			"/json",
			bytes.NewBuffer([]byte(`
			{
				"foo": 1238,
				"bar": "bazzzzz"
			}
			`)))
		router.ServeHTTP(w, req)
		testutil.AssertEqual(t, w.Code, http.StatusOK)
	})
}

func TestQueryValidation(t *testing.T) {
	t.Parallel()
	t.Run("reject bad query parameter request", func(t *testing.T) {
		t.Run("no parameters", func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/query", emptyBody)
			router.ServeHTTP(w, req)
			testutil.AssertEqual(t, w.Code, http.StatusBadRequest)
			err := assertErrorResponseOk(t, w, 2)
			barOkErrorCheck := false
			fooOkErrorCheck := false
			for _, field := range err.Fields {
				if field.Field == "bar" && field.Message == "bar is required" && field.Code == "required" {
					barOkErrorCheck = true
				}
				if field.Field == "foo" && field.Message == "foo is required" && field.Code == "required" {
					fooOkErrorCheck = true
				}
			}
			testutil.AssertMsg(t, barOkErrorCheck, `"bar" did not have a meaningful message!`)
			testutil.AssertMsg(t, fooOkErrorCheck, `"foo" did not have a meaningful message!`)
		})

		t.Run("just foo", func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/query?foo=12", emptyBody)
			router.ServeHTTP(w, req)
			testutil.AssertEqual(t, w.Code, http.StatusBadRequest)

			err := assertErrorResponseOk(t, w, 1)
			barOkErrorCheck := false
			field := err.Fields[0]
			if field.Field == "bar" && field.Message == "bar is required" && field.Code == "required" {
				barOkErrorCheck = true
			}
			testutil.AssertMsg(t, barOkErrorCheck, `"bar" did not have a meaningful message!`)
		})
		t.Run("just bar", func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/query?bar=baz", emptyBody)
			router.ServeHTTP(w, req)
			testutil.AssertEqual(t, w.Code, http.StatusBadRequest)
			err := assertErrorResponseOk(t, w, 1)
			fooOkErrorCheck := false
			field := err.Fields[0]
			if field.Field == "foo" && field.Message == "foo is required" && field.Code == "required" {
				fooOkErrorCheck = true
			}
			testutil.AssertMsg(t, fooOkErrorCheck, `"foo" did not have a meaningful message!`)
		})
	})

	t.Run("accept good query parameter request", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET",
			"/query?foo=1&bar=bar",
			emptyBody)
		router.ServeHTTP(w, req)
		testutil.AssertEqual(t, w.Code, http.StatusOK)
	})
}

// When a request errors with a code we expect that code to be set, instead of
// the default code (500)
func TestErrorWithCode(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/withCode", emptyBody)
	router.ServeHTTP(w, req)
	testutil.AssertNotEqual(t, w.Code, http.StatusInternalServerError)
}

// When a request errors with a public error we expect that error message to
// be sent
func TestPublicError(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/public", emptyBody)
	router.ServeHTTP(w, req)
	testutil.AssertEqual(t, w.Code, http.StatusInternalServerError)

	err := assertErrorResponseOk(t, w, 0)
	testutil.AssertEqual(t, err.Message, publicError.Error())
	testutil.AssertEqual(t, err.Code, ErrUnknownError)
}

// When a request errors with a public error we expect that error message to
// be sent. Also, when a meta message is attached we expect that meta message
// to be the error code.
func TestPublicErrorWithMeta(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/publicWithMeta", emptyBody)
	router.ServeHTTP(w, req)
	testutil.AssertEqual(t, w.Code, http.StatusInternalServerError)

	err := assertErrorResponseOk(t, w, 0)
	testutil.AssertEqual(t, err.Message, publicError.Error())
	testutil.AssertEqual(t, err.Code, metaMessage)
}
