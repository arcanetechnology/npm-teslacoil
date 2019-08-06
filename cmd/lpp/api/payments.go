package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/lpp/internal/payments"
)

//GetAllInvoicesResponse is the type returned by the api to the front-end
type GetAllInvoicesResponse struct {
	Invoices []payments.Payment
}

// GetInvoiceResponse is the response for the /invoice/:id endpoint
type GetInvoiceResponse struct {
	ID             uint64
	UserID         uint64
	PaymentRequest string
	Preimage       string
	Hash           string
	CallbackURL    string
	Status         payments.Status
	Description    string
	Direction      payments.Direction
	AmountSat      int64
	AmountMSat     int64
	SettledAt      string
}

// CreateInvoiceResponse is the request for the /invoice/create endpoint
type CreateInvoiceResponse struct {
	ID             uint64
	UserID         uint64          `json:""`
	PaymentRequest string          `json:""`
	HashedPreimage string          `json:""`
	CallbackURL    string          `json:""`
	Status         payments.Status `json:""`
	Description    string          `json:""`
	AmountSat      int64           `json:""`
	AmountMSat     int64
}

// PayInvoiceResponse is the response for the /invoice/pay endpoint
type PayInvoiceResponse struct {
	ID             uint64
	UserID         uint64
	PaymentRequest string
	Preimage       string
	Hash           string
	CallbackURL    string
	Status         payments.Status
	Description    string
	Direction      payments.Direction
	AmountSat      int64
	AmountMSat     int64
	SettledAt      string
}

// GetAllInvoices is a GET request that returns all the users in the database
func GetAllInvoices(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {
		t, err := payments.GetAll(r.db)
		if err != nil {
			c.JSONP(500, gin.H{"error": err.Error()})
			return
		}
		c.JSONP(200, &GetAllInvoicesResponse{Invoices: t})
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
		c.JSONP(200, &GetInvoiceResponse{
			ID:             t.ID,
			UserID:         t.UserID,
			PaymentRequest: t.PaymentRequest,
			Preimage:       t.Preimage,
			Hash:           t.HashedPreimage,
			CallbackURL:    *t.CallbackURL,
			Status:         t.Status,
			Description:    t.Description,
			AmountSat:      t.AmountSat,
			AmountMSat:     t.AmountMSat,
			SettledAt:      t.SettledAt.String(),
		})
	}
}

// CreateInvoice creates a new incove on behalf of a user
func CreateInvoice(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var newInvoice payments.CreateInvoiceData

		if err := c.ShouldBindJSON(&newInvoice); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		fmt.Printf("new invoice %v\n", newInvoice)

		t, err := payments.CreateInvoice(r.db, *r.lncli, newInvoice)
		if err != nil {
			c.JSONP(http.StatusBadRequest, gin.H{"error": errors.Wrap(err,
				"Could not create new invoice").Error()})
			return
		}

		// Return as much info as possible
		c.JSONP(200, &CreateInvoiceResponse{
			ID:             t.ID,
			UserID:         t.UserID,
			PaymentRequest: t.PaymentRequest,
			HashedPreimage: t.HashedPreimage,
			CallbackURL:    *t.CallbackURL,
			Status:         t.Status,
			Description:    t.Description,
			AmountSat:      t.AmountSat,
			AmountMSat:     t.AmountMSat,
		})
	}
}

// PayInvoice pays a valid invoice on behalf of a user
func PayInvoice(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var newPayment payments.PayInvoiceData

		if err := c.ShouldBindJSON(&newPayment); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		t, err := payments.PayInvoice(r.db, *r.lncli, newPayment)
		if err != nil {
			c.JSONP(http.StatusBadRequest, gin.H{"error": errors.Wrap(err, "Could not pay invoice")})
			return
		}

		// Return as much info as possible
		c.JSONP(200, &PayInvoiceResponse{
			ID:             t.Payment.ID,
			UserID:         t.UserID,
			PaymentRequest: t.PaymentRequest,
			Preimage:       t.Preimage,
			Hash:           t.HashedPreimage,
			Status:         t.Status,
			Description:    t.Description,
			AmountSat:      t.AmountSat,
			AmountMSat:     t.AmountMSat,
			SettledAt:      t.SettledAt.String(),
		})
	}
}
