package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"gitlab.com/arcanecrypto/lpp/internal/transactions"
)

// GetUsers is a GET request that returns all the users in the database
func AllTransactions(d *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		t, err := transactions.All(d)
		if err != nil {
			c.JSONP(500, gin.H{"error": err})
			return
		}
		c.JSONP(200, t)
	}
}

// GetTransaction is a GET request that returns users that match the one specified in the body
func GetTransaction(d *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			c.JSONP(404, gin.H{"error": "Transactions id should be a integer"})
			return
		}
		t, err := transactions.GetByID(d, id)
		if err != nil {
			c.JSONP(
				http.StatusNotFound,
				gin.H{"error": "User not found"},
			)
			return
		}
		// Return the user when it is found and no errors where encountered
		c.JSONP(200, t)
	}
}

// CreateNewInvoice creates a new incove on behalf of a user
func CreateNewInvoice(d *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {

		var newTransaction transactions.NewTransaction

		if err := c.ShouldBindJSON(&newTransaction); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		t, err := transactions.CreateInvoice(d, newTransaction)
		if err != nil {
			c.JSONP(http.StatusBadRequest, gin.H{"error": "Could not create new invoice"})
			return
		}

		c.JSONP(200, t)
	}
}

// PayInvoice pays a valid invoice on behalf of a user
func PayInvoice(d *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {

		var newTransaction transactions.NewTransaction

		if err := c.ShouldBindJSON(&newTransaction); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		t, err := transactions.PayInvoice(d, newTransaction)
		if err != nil {
			c.JSONP(http.StatusBadRequest, gin.H{"error": "Could not pay invoice"})
			return
		}

		c.JSONP(200, t)
	}
}
