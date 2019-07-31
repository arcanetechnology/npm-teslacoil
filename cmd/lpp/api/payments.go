package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/lpp/internal/payments"
)

// GetAllPayments is a GET request that returns all the users in the database
func GetAllPayments(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {
		t, err := payments.All(r.db)
		if err != nil {
			c.JSONP(500, gin.H{"error": err.Error()})
			return
		}
		c.JSONP(200, t)
	}
}

// GetPayment is a GET request that returns users that match the one specified in the body
func GetPayment(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSONP(404, gin.H{"error": "Payments id should be a integer"})
			return
		}
		t, err := payments.GetByID(r.db, id)
		if err != nil {
			c.JSONP(
				http.StatusNotFound,
				gin.H{"error": errors.Wrap(err, "Payment not found not found").Error()},
			)
			return
		}
		// Return the user when it is found and no errors where encountered
		c.JSONP(200, t)
	}
}

// CreateNewInvoice creates a new incove on behalf of a user
func CreateNewInvoice(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {

		var newPayment payments.NewPayment

		if err := c.ShouldBindJSON(&newPayment); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		t, err := payments.CreateInvoice(r.db, *r.lncli, newPayment)
		if err != nil {
			c.JSONP(http.StatusBadRequest, gin.H{"error": errors.Wrap(err, "Could not create new invoice").Error()})
			return
		}

		c.JSONP(200, t)
	}
}

// PayInvoice pays a valid invoice on behalf of a user
func PayInvoice(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var newPayment payments.NewPayment

		if err := c.ShouldBindJSON(&newPayment); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		t, err := payments.PayInvoice(r.db, *r.lncli, newPayment)
		if err != nil {
			c.JSONP(http.StatusBadRequest, gin.H{"error": errors.Wrap(err, "Could not pay invoice")})
			return
		}

		c.JSONP(200, t)
	}
}
