package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gitlab.com/arcanecrypto/teslacoil/cmd/lpp/api/auth"
)

// getUserIdOrReject retrieves the user ID associated with this request. This
// user ID should be set by the authentication middleware. This means that this
// method can safely be called by all endpoints that use the authentication
// middleware.
func getUserIdOrReject(c *gin.Context) (int, bool) {
	id, exists := c.Get(auth.UserIdVariable)
	if !exists {
		msg := "User ID is not set in request! This is a serious error, and means our authentication middleware did not set the correct variable."
		_ = c.AbortWithError(http.StatusInternalServerError, errors.New(msg))
		return -1, false
	}
	idInt, ok := id.(int)
	if !ok {
		msg := "User ID was not an int! This means our authentication middleware did something bad."
		_ = c.AbortWithError(http.StatusInternalServerError, errors.New(msg))
		return -1, false
	}

	return idInt, true
}
