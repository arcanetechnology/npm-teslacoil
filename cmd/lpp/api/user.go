package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gitlab.com/arcanecrypto/teslacoil/internal/users"
)

//GetUserRequest is the expected type to find a user in the DB
type GetUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

//CreateUserRequest is the expected type to create a new user
type CreateUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password" binding:"required"`
}

//GetAllUsersResponse is the type returned by the api to the front-end
type GetAllUsersResponse struct {
	Users []users.User
}

//GetUserResponse is the type returned by the api to the front-end
type GetUserResponse struct {
	ID      uint   `db:"id"`
	Email   string `db:"email"`
	Balance int    `db:"balance"`
}

//CreateUserResponse is the type returned by the api to the front-end
type CreateUserResponse struct {
	ID      uint   `db:"id"`
	Email   string `db:"email"`
	Balance int    `db:"balance"`
}

// LoginResponse includes a jwt-token and the e-mail identifying the user
type LoginResponse struct {
	AccessToken string `json:"access_token"`
	Email       string `json:"email"`
	UserID      uint   `json:"user_id"`
	Balance     uint   `json:"balance"`
}

// RefreshTokenResponse is the response from /auth/refresh
type RefreshTokenResponse struct {
	AccessToken string `json:"access_token"`
}

// GetAllUsers is a GET request that returns all the users in the database
// TODO: Restrict this to only the admin user
func GetAllUsers(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {
		userResponse, err := users.All(r.db)
		if err != nil {
			log.Error(err)
			c.JSONP(http.StatusInternalServerError, gin.H{"error": err})
			return
		}
		c.JSONP(200, userResponse)
	}
}

// GetUser is a GET request that returns users that match the one specified in the body
// This endpoint is no longer needed, as Login takes care of the logic
/*
func GetUser(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Remove ID and add jwt later
		var req GetUserRequest

		if err := c.ShouldBindJSON(req); err != nil {
			log.Error(err)
			c.JSONP(http.StatusBadRequest, gin.H{
				"error": "request should only contain email {\"email\": \"email@domain.com\"}"})
			return
		}

		user, err := users.GetByCredentials(r.db, req.Email, req.Password)
		if err != nil {
			log.Error(err)
			c.JSONP(http.StatusNotFound, gin.H{"error": "User not found"})
		}

		res := GetUserResponse{
			ID:      user.ID,
			Email:   user.Email,
			Balance: user.Balance,
		}

		log.Info("GetUserResponse %v", res)

		// Return the user when it is found and no errors where encountered
		c.JSONP(200, res)
	}
}
*/

// CreateUser is a POST request and inserts all the users in the body into the database
func CreateUser(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateUserRequest

		log.Debugf("req: %v", req)

		if err := c.ShouldBindJSON(&req); err != nil {
			log.Error(err)
			c.JSONP(http.StatusBadRequest, gin.H{
				"error": "bad request, see documentation"})
			return
		}

		log.Info("creating user with credentials: ", req)

		// because the email column in users table has the unique tag, we don't
		// double check the email is unique
		u, err := users.Create(r.db, req.Email, req.Password)
		if err != nil {
			c.JSONP(http.StatusInternalServerError, gin.H{
				"error": "internal server error, please try again or contact support"})
			return
		}

		res := CreateUserResponse{
			ID:      u.ID,
			Email:   u.Email,
			Balance: u.Balance,
		}
		log.Info("successfully created user: ", res)

		c.JSONP(200, res)
	}
}

//Login logs in
func Login(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req GetUserRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			log.Error(err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "bad request, see documentation"})
			return
		}

		log.Info("logging in user: ", req)

		user, err := users.GetByCredentials(r.db, req.Email, req.Password)
		if err != nil {
			c.JSONP(http.StatusInternalServerError, gin.H{
				"error": "internal server error, please try again or contact support"})
			return
		}
		log.Info("found user: ", user)

		tokenString, err := createJWTToken(req.Email, user.ID)
		if err != nil {
			c.JSONP(http.StatusInternalServerError, gin.H{
				"error": "internal server error, please try again or contact support"})
			return
		}

		res := LoginResponse{
			UserID:      user.ID,
			Email:       user.Email,
			AccessToken: tokenString,
		}
		log.Info("LoginResponse: ", res)

		c.JSONP(200, res)
	}
}

// RefreshToken refreshes a jwt-token
func RefreshToken(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {
		// The JWT is already authenticated, but here we parse the JWT to
		// extract the email as it is required to create a new JWT.
		_, claims, err := parseBearerJWT(c.GetHeader("Authorization"))
		if err != nil {
			log.Error(err)
			c.JSONP(http.StatusBadRequest, gin.H{"error": "bad request, see documentation"})
		}

		tokenString, err := createJWTToken(claims.Email, claims.UserID)
		if err != nil {
			c.JSONP(http.StatusInternalServerError, gin.H{
				"error": "internal server error, please try again or contact support"})
		}

		res := &RefreshTokenResponse{
			AccessToken: tokenString,
		}

		log.Info("RefreshTokenResponse: ", res)

		c.JSONP(200, res)
	}
}
