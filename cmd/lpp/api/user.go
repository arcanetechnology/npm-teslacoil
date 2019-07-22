package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"gitlab.com/arcanecrypto/lpp/internal/users"
)

// GetUsers is a GET request that returns all the users in the database
func AllUsers(d *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, err := users.All(d)
		if err != nil {
			c.JSONP(500, gin.H{"error": err})
			return
		}
		c.JSONP(200, u)
	}
}

// GetUser is a GET request that returns users that match the one specified in the body
func GetUser(d *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSONP(404, gin.H{"error": "User id should be a integer"})
			return
		}
		u, err := users.GetByID(d, id)
		if err != nil {
			c.JSONP(
				http.StatusNotFound,
				gin.H{"error": "User not found"},
			)
			return
		}
		// Return the user when it is found and no errors where encountered
		c.JSONP(200, u)
	}
}

// CreateUser is a POST request and inserts all the users in the body into the database
func CreateUser(d *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var newUser users.UserNew

		if err := c.ShouldBindJSON(&newUser); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		u, err := users.Create(d, newUser)
		if err != nil {
			c.JSONP(http.StatusBadRequest, gin.H{"error": "Could not create user"})
			return
		}

		c.JSONP(200, u)
	}
}
