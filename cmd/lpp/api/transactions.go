package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/lpp/internal/transactions"
)

// import (
// 	"net/http"
// 	"strconv"

// 	"github.com/gin-gonic/gin"
// 	"github.com/jinzhu/gorm"
// 	"gitlab.com/arcanecrypto/lpp/internal/transactions"
// )

// AllTransactions is a GET request that returns all the users in the database
func AllTransactions(d *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		t, err := transactions.All(d)
		if err != nil {
			c.JSONP(500, gin.H{"error": err.Error()})
			return
		}
		c.JSONP(200, t)
	}
}

// GetTransaction is a GET request that returns users that match the one specified in the body
func GetTransaction(d *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSONP(404, gin.H{"error": "Transactions id should be a integer"})
			return
		}
		t, err := transactions.GetByID(d, id)
		if err != nil {
			c.JSONP(
				http.StatusNotFound,
				gin.H{"error": errors.Wrap(err, "Transaction not found not found").Error()},
			)
			return
		}
		// Return the user when it is found and no errors where encountered
		c.JSONP(200, t)
	}
}

// CreateNewInvoice creates a new incove on behalf of a user
func CreateNewInvoice(d *sqlx.DB) gin.HandlerFunc {
	return func(c *gin.Context) {

		var newTransaction transactions.NewTransaction

		if err := c.ShouldBindJSON(&newTransaction); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		t, err := transactions.CreateInvoice(d, newTransaction)
		if err != nil {
			c.JSONP(http.StatusBadRequest, gin.H{"error": errors.Wrap(err, "Could not create new invoice").Error()})
			return
		}

		c.JSONP(200, t)
	}
}

// PayInvoice pays a valid invoice on behalf of a user
func PayInvoice(d *sqlx.DB) gin.HandlerFunc {
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
