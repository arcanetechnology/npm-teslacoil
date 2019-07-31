package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/lpp/internal/payments"
)

// Need to plan how this should be...
// I want to separate the DB calls and the API requests/responses, simply
// because it reads better. You can not mind the DB and only look at the API
// functions to see what is returned
// It also removes the link between the API and the DB. (You can't change the
// DB call without also changing the API)
// I am still of the opinion that more general SQL queries scale better than
// writing SQL for each query. You can scale better to special endpoints
// providing the user with important data

//GetAllInvoicesResponse is the type returned by the api to the front-end
type getAllInvoicesResponse struct {
	Invoices []payments.Payment
}
type getInvoiceResponse struct {
	PaymentRequest string
	Hash           string
	Direction      payments.Direction
}
type createInvoiceResponse struct {
	PaymentRequest string
}
type payInvoiceResponse struct {
	Status string
}

// GetAllInvoices is a GET request that returns all the users in the database
func GetAllInvoices(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {
		t, err := payments.GetAll(r.db)
		if err != nil {
			c.JSONP(500, gin.H{"error": err.Error()})
			return
		}
		c.JSONP(200, &getAllInvoicesResponse{Invoices: t})
	}
}

// GetInvoice is a GET request that returns users that match the one specified in the body
func GetInvoice(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			c.JSONP(404, gin.H{"error": "Invoices id should be a integer"})
			return
		}
		t, err := payments.GetByID(r.db, id)
		if err != nil {
			c.JSONP(
				http.StatusNotFound,
				gin.H{"error": errors.Wrap(err, "Invoice not found").Error()},
			)
			return
		}
		// Return the user when it is found and no errors where encountered
		c.JSONP(200, &getInvoiceResponse{
			PaymentRequest: t.PaymentRequest,
			Hash:           t.HashedPreImage,
			Direction:      t.Direction,
		})
	}
}

// CreateInvoice creates a new incove on behalf of a user
func CreateInvoice(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {

		var newInvoice payments.NewDeposit

		if err := c.ShouldBindJSON(&newInvoice); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		t, err := payments.CreateInvoice(r.db, *r.lncli, newInvoice)
		if err != nil {
			c.JSONP(http.StatusBadRequest, gin.H{"error": errors.Wrap(err, "Could not create new invoice").Error()})
			return
		}

		c.JSONP(200, &createInvoiceResponse{
			PaymentRequest: t.PaymentRequest,
		})
	}
}

// PayInvoice pays a valid invoice on behalf of a user
func PayInvoice(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var newPayment payments.NewWithdrawal

		if err := c.ShouldBindJSON(&newPayment); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		t, err := payments.PayInvoice(r.db, *r.lncli, newPayment)
		if err != nil {
			c.JSONP(http.StatusBadRequest, gin.H{"error": errors.Wrap(err, "Could not pay invoice")})
			return
		}

		c.JSONP(200, &payInvoiceResponse{
			Status: t.Status,
		})
	}
}
