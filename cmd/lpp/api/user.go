package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/lpp/internal/users"
)

//GetUserRequest is the expected type to find a user in the DB
type GetUserRequest struct {
	// Email    string `json:"email"`
	// Password string `json:"password" binding:"required"`
	ID uint64
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

// GetAllUsers is a GET request that returns all the users in the database
func GetAllUsers(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, err := users.All(r.db)
		if err != nil {
			c.JSONP(http.StatusInternalServerError, gin.H{"error": err})
			return
		}
		c.JSONP(200, GetAllUsersResponse{
			Users: u,
		})
	}
}

// GetUser is a GET request that returns users that match the one specified in the body
func GetUser(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Remove ID and add jwt later
		var req GetUserRequest
		req.ID = 1

		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSONP(404, gin.H{"error": "User id should be a integer"})
			return
		}

		user, err := users.GetByID(r.db, uint(id))
		if err != nil {
			c.JSONP(http.StatusNotFound, gin.H{"error": errors.Wrap(err, "User not found")})
		}

		// Return the user when it is found and no errors where encountered
		c.JSONP(200, GetUserResponse{
			ID:      user.ID,
			Email:   user.Email,
			Balance: user.Balance,
		})
	}
}

// CreateUser is a POST request and inserts all the users in the body into the database
func CreateUser(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateUserRequest

		if err := c.ShouldBindJSON(req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		u, err := users.Create(r.db, req.Email, req.Password)
		if err != nil {
			c.JSONP(200, gin.H{"error": err.Error()})
		}

		c.JSONP(200, CreateUserResponse{
			ID:      u.ID,
			Email:   u.Email,
			Balance: u.Balance,
		})
	}
}
