package apierr

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/arcanecrypto/teslacoil/api/httptypes"
	"gitlab.com/arcanecrypto/teslacoil/build"
)

type Request struct {
	Foo int    `form:"foo" json:"foo" binding:"required"`
	Bar string `form:"bar" json:"bar" binding:"required"`
}

var (
	middleware = GetMiddleware(build.AddSubLogger("API_ERR_TEST"))
	router     = setupRouter(middleware)
	emptyBody  = bytes.NewBuffer([]byte(""))

	publicError = apiError{
		err:  errors.New("this is a public error"),
		code: "ERR_PUBLIC",
	}
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
	r.POST("/body", func(c *gin.Context) {
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
		Public(c, http.StatusInternalServerError, publicError)
	})
	r.GET("/withCode", func(c *gin.Context) {
		_ = c.AbortWithError(http.StatusUnauthorized, errors.New("with a code"))
	})
	return r
}

func assertErrorResponseOk(t *testing.T, w *httptest.ResponseRecorder, expectedFieldErrors int) httptypes.StandardErrorResponse {
	bodyBytes, err := ioutil.ReadAll(w.Body)
	require.NoError(t, err)

	var res httptypes.StandardErrorResponse

	require.NoError(t, json.Unmarshal(bodyBytes, &res))

	require.NotNil(t, res.ErrorField.Fields)
	assert.Len(t, res.ErrorField.Fields, expectedFieldErrors)
	return res
}

func TestJsonValidation(t *testing.T) {
	t.Parallel()
	t.Run("reject bad JSON body request", func(t *testing.T) {
		t.Run("Invalid JSON", func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/body",
				bytes.NewBuffer([]byte(`{[{"foo": 2 }]`))) // missing }
			router.ServeHTTP(w, req)
			assert.Equal(t, w.Code, http.StatusBadRequest)

			err := assertErrorResponseOk(t, w, 0)
			assert.NotEqual(t, err.ErrorField.Message, "", "Error message was empty")
			assert.Equal(t, err.ErrorField.Code, errInvalidJson.code)
		})

		t.Run("no parameters", func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/body", bytes.NewBuffer([]byte(`{}`)))
			router.ServeHTTP(w, req)
			assert.Equal(t, w.Code, http.StatusBadRequest)

			err := assertErrorResponseOk(t, w, 2)
			barOkErrorCheck := false
			fooOkErrorCheck := false
			for _, field := range err.ErrorField.Fields {
				if field.Field == "bar" && field.Message == `"bar" is required` && field.Code == "required" {
					barOkErrorCheck = true
				}
				if field.Field == "foo" && field.Message == `"foo" is required` && field.Code == "required" {
					fooOkErrorCheck = true
				}
			}
			assert.True(t, barOkErrorCheck, `"bar" did not have a meaningful message!`)
			assert.True(t, fooOkErrorCheck, `"foo" did not have a meaningful message!`)
		})

		t.Run("just foo", func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/body", bytes.NewBuffer([]byte(`
			{
				"foo": 1
			}
			`)))
			router.ServeHTTP(w, req)
			assert.Equal(t, w.Code, http.StatusBadRequest)

			err := assertErrorResponseOk(t, w, 1)
			barOkErrorCheck := false
			assert.True(t, len(err.ErrorField.Fields) > 0)
			field := err.ErrorField.Fields[0]
			if field.Field == "bar" && field.Message == `"bar" is required` && field.Code == "required" {
				barOkErrorCheck = true
			}
			assert.True(t, barOkErrorCheck, `"bar" did not have a meaningful message!`)
		})
		t.Run("just bar", func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/body", bytes.NewBuffer([]byte(`
			{
				"bar": "bazz"
			}
			`)))
			router.ServeHTTP(w, req)
			assert.Equal(t, w.Code, http.StatusBadRequest)

			err := assertErrorResponseOk(t, w, 1)
			fooOkErrorCheck := false
			field := err.ErrorField.Fields[0]
			if field.Field == "foo" && field.Message == `"foo" is required` && field.Code == "required" {
				fooOkErrorCheck = true
			}
			assert.True(t, fooOkErrorCheck, `"foo" did not have a meaningful message!`)
		})
	})

	t.Run("accept good JSON request", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST",
			"/body",
			bytes.NewBuffer([]byte(`
			{
				"foo": 1238,
				"bar": "bazzzzz"
			}
			`)))
		router.ServeHTTP(w, req)
		assert.Equal(t, w.Code, http.StatusOK)
	})
}

func TestQueryValidation(t *testing.T) {
	t.Parallel()
	t.Run("reject bad query parameter request", func(t *testing.T) {
		t.Run("no parameters", func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/query", emptyBody)
			router.ServeHTTP(w, req)
			assert.Equal(t, w.Code, http.StatusBadRequest)

			err := assertErrorResponseOk(t, w, 2)
			barOkErrorCheck := false
			fooOkErrorCheck := false
			for _, field := range err.ErrorField.Fields {
				if field.Field == "bar" && field.Message == `"bar" is required` && field.Code == "required" {
					barOkErrorCheck = true
				}
				if field.Field == "foo" && field.Message == `"foo" is required` && field.Code == "required" {
					fooOkErrorCheck = true
				}
			}
			assert.True(t, barOkErrorCheck, `"bar" did not have a meaningful message!`)
			assert.True(t, fooOkErrorCheck, `"foo" did not have a meaningful message!`)
		})

		t.Run("just foo", func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/query?foo=12", emptyBody)
			router.ServeHTTP(w, req)
			assert.Equal(t, w.Code, http.StatusBadRequest)

			err := assertErrorResponseOk(t, w, 1)
			barOkErrorCheck := false
			field := err.ErrorField.Fields[0]
			if field.Field == "bar" && field.Message == `"bar" is required` && field.Code == "required" {
				barOkErrorCheck = true
			}
			assert.True(t, barOkErrorCheck, `"bar" did not have a meaningful message!`)
		})
		t.Run("just bar", func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/query?bar=baz", emptyBody)
			router.ServeHTTP(w, req)
			assert.Equal(t, w.Code, http.StatusBadRequest)

			err := assertErrorResponseOk(t, w, 1)
			fooOkErrorCheck := false
			field := err.ErrorField.Fields[0]
			if field.Field == "foo" && field.Message == `"foo" is required` && field.Code == "required" {
				fooOkErrorCheck = true
			}
			assert.True(t, fooOkErrorCheck, `"foo" did not have a meaningful message!`)
		})
	})

	t.Run("accept good query parameter request", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET",
			"/query?foo=1&bar=bar",
			emptyBody)
		router.ServeHTTP(w, req)
		assert.Equal(t, w.Code, http.StatusOK)
	})
}

// When a request errors with a code we expect that code to be set, instead of
// the default code (500)
func TestErrorWithCode(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/withCode", emptyBody)
	router.ServeHTTP(w, req)
	assert.NotEqual(t, w.Code, http.StatusInternalServerError)
}

// When a request errors with a public error we expect that error message to
// be sent
func TestPublicError(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/public", emptyBody)
	router.ServeHTTP(w, req)
	assert.Equal(t, w.Code, http.StatusInternalServerError)

	err := assertErrorResponseOk(t, w, 0)
	assert.Equal(t, err.ErrorField.Code, publicError.code)
}

func TestBodyRequired(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/body", emptyBody)
	router.ServeHTTP(w, req)
	assert.Equal(t, w.Code, http.StatusBadRequest)
	err := assertErrorResponseOk(t, w, 0)
	assert.True(t, errors.Is(err, errBodyRequired), err)
}
