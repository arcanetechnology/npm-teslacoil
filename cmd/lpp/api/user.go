package api

import (
	"net/http"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/lpp/internal/users"
)

//GetUserRequest is the expected type to find a user in the DB
type GetUserRequest struct {
	Email string `json:"email"`
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
	ID      uint64 `db:"id"`
	Email   string `db:"email"`
	Balance int    `db:"balance"`
}

//CreateUserResponse is the type returned by the api to the front-end
type CreateUserResponse struct {
	ID      uint64 `db:"id"`
	Email   string `db:"email"`
	Balance int    `db:"balance"`
}

// LoginResponse includes a jwt-token and the e-mail identifying the user
type LoginResponse struct {
	AccessToken string `json:"access_token"`
	Email       string `json:"email"`
}

// RefreshTokenResponse is the response from /auth/refresh
type RefreshTokenResponse struct {
	AccessToken string `json:"access_token"`
}

// JWTClaims is the common form for our jwts
type JWTClaims struct {
	Email string `json:"email"`
	jwt.StandardClaims
}

// GetAllUsers is a GET request that returns all the users in the database
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
func GetUser(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Remove ID and add jwt later
		var req GetUserRequest

		if err := c.ShouldBindJSON(req); err != nil {
			log.Error(err)
			c.JSONP(http.StatusBadRequest, gin.H{"error": "request should only contain email {\"email\": \"email@domain.com\"}"})
			return
		}

		user, err := users.GetByCredentials(r.db, req.Email)
		if err != nil {
			log.Error(err)
			c.JSONP(http.StatusNotFound, gin.H{"error": errors.Wrap(err, "User not found")})
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

// CreateUser is a POST request and inserts all the users in the body into the database
func CreateUser(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateUserRequest

		if err := c.ShouldBindJSON(req); err != nil {
			log.Error(err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		log.Info("creating user with credentials: %v", req)

		u, err := users.Create(r.db, req.Email, req.Password)
		if err != nil {
			log.Error(err)
			c.JSONP(200, gin.H{"error": err.Error()})
		}

		res := CreateUserResponse{
			ID:      u.ID,
			Email:   u.Email,
			Balance: u.Balance,
		}
		log.Info("created user %v", res)

		c.JSONP(200, res)
	}
}

//Login logs in
func Login(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req GetUserRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			log.Error(err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err})
			return
		}

		log.Info("logging in user %v", req)

		user, err := users.GetByCredentials(r.db, req.Email)
		if err != nil {
			log.Error(err)
			c.JSONP(http.StatusInternalServerError, gin.H{"error": err})
			return
		}
		log.Info("found user %v", user)

		tokenString, err := createJWTToken(req.Email)
		if err != nil {
			log.Error(err)
			c.JSONP(http.StatusInternalServerError, gin.H{"error": err})
			return
		}

		res := LoginResponse{
			Email:       user.Email,
			AccessToken: tokenString,
		}
		log.Info("LoginResponse %v", res)

		c.JSONP(200, res)
	}
}

// RefreshToken refreshes a jwt-token
func RefreshToken(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {
		// The JWT is already authenticated, but here we parse the JWT to
		// extract the email as it is required to create a new JWT.
		_, claims, err := parseToken(c.GetHeader("Authorization"))
		if err != nil {
			log.Error(err)
			c.JSONP(http.StatusBadRequest, gin.H{"error": err})
		}

		email, ok := claims["email"].(string)
		if !ok {
			err = errors.New("could not extract email from jwt-token")
			log.Error(err)
			c.JSONP(http.StatusInternalServerError, gin.H{"error": err})
			return
		}

		tokenString, err := createJWTToken(email)
		if err != nil {
			log.Error(err)
			c.JSONP(http.StatusInternalServerError, gin.H{"error": err})
		}

		res := &RefreshTokenResponse{
			AccessToken: tokenString,
		}

		log.Info("RefreshTokenResponse %v", res)

		c.JSONP(200, res)
	}
}
