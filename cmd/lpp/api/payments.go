package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gitlab.com/arcanecrypto/teslacoil/internal/payments"
)

//GetAllInvoicesResponse is the type returned by the api to the front-end
type GetAllInvoicesResponse struct {
	Invoices []payments.Payment
}

// GetInvoiceResponse is the response for the /invoice/:id endpoint
type GetInvoiceResponse struct {
	ID             uint               `json:"id"`
	UserID         uint               `json:"userId"`
	PaymentRequest string             `json:"paymentRequest"`
	Preimage       string             `json:"preimage"`
	Hash           string             `json:"hash"`
	CallbackURL    *string            `json:"callbackUrl"`
	Status         payments.Status    `json:"status"`
	Memo           string             `json:"memo"`
	Direction      payments.Direction `json:"direction"`
	AmountSat      int64              `json:"amountSat"`
	AmountMSat     int64              `json:"amountMSat"`
	SettledAt      string             `json:"settledAt"`
}

// CreateInvoiceResponse is the request for the /invoice/create endpoint
type CreateInvoiceResponse struct {
	ID             uint            `json:"id"`
	UserID         uint            `json:"userId"`
	PaymentRequest string          `json:"paymentRequest"`
	HashedPreimage string          `json:"hashedPreimage"`
	CallbackURL    *string         `json:"callbackUrl"`
	Status         payments.Status `json:"status"`
	Memo           string          `json:"memo"`
	AmountSat      int64           `json:"amountSat"`
	AmountMSat     int64           `json:"amountMSat"`
}

// PayInvoiceResponse is the response for the /invoice/pay endpoint
type PayInvoiceResponse struct {
	ID             uint               `json:"id"`
	UserID         uint               `json:"userId"`
	PaymentRequest string             `json:"paymentRequest"`
	Preimage       string             `json:"preimage"`
	Hash           string             `json:"hash"`
	CallbackURL    *string            `json:"callbackUrl"`
	Status         payments.Status    `json:"status"`
	Memo           string             `json:"memo"`
	Direction      payments.Direction `json:"direction"`
	AmountSat      int64              `json:"amountSat"`
	AmountMSat     int64              `json:"amountMSat"`
	SettledAt      string             `json:"settledAt"`
}

func convertToGetInvoiceResponse(payments []payments.Payment) []GetInvoiceResponse {
	var invResponse []GetInvoiceResponse

	for _, payment := range payments {
		invResponse = append(invResponse, GetInvoiceResponse{
			ID:             payment.ID,
			UserID:         payment.UserID,
			PaymentRequest: payment.PaymentRequest,
			Preimage:       *payment.Preimage,
			Hash:           payment.HashedPreimage,
			CallbackURL:    payment.CallbackURL,
			Status:         payment.Status,
			Memo:           payment.Memo,
			Direction:      payment.Direction,
			AmountSat:      payment.AmountSat,
			AmountMSat:     payment.AmountMSat,
			SettledAt:      payment.SettledAt.String(),
		})
	}

	return invResponse
}

// GetAllInvoices is a GET request that returns all the users in the database
func GetAllInvoices(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {
		skipFirst, err := strconv.ParseInt(c.Param("skipFirst"), 10, 64)
		if err != nil {
			log.Error(err)
			c.JSONP(404, gin.H{"error": "url param invoice id should be a integer"})
			return
		}
		count, err := strconv.ParseInt(c.Param("count"), 10, 64)
		if err != nil {
			log.Error(err)
			c.JSONP(404, gin.H{"error": "url param invoice id should be a integer"})
			return
		}

		filter := payments.GetAllInvoicesData{
			SkipFirst: int(skipFirst),
			Count:     int(count),
		}

		_, claim, err := parseBearerJWT(c.GetHeader("Authorization"))

		t, err := payments.GetAll(r.db, claim.UserID, filter)
		if err != nil {
			c.JSONP(http.StatusInternalServerError, gin.H{
				"error": "internal server error, please try again or contact support"})
			return
		}

		c.JSONP(200, convertToGetInvoiceResponse(t))
	}
}

// GetInvoice is a GET request that returns users that match the one specified in the body
func GetInvoice(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, claim, err := parseBearerJWT(c.GetHeader("Authorization"))

		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			log.Error(err)
			c.JSONP(404, gin.H{"error": "url param invoice id should be a integer"})
			return
		}
		t, err := payments.GetByID(r.db, uint(id), claim.UserID)
		if err != nil {
			c.JSONP(
				http.StatusNotFound,
				gin.H{"error": "invoice not found"},
			)
			return
		}
		// Return the user when it is found and no errors where encountered
		c.JSONP(200, &GetInvoiceResponse{
			ID:             t.ID,
			UserID:         t.UserID,
			PaymentRequest: t.PaymentRequest,
			Preimage:       *t.Preimage,
			Hash:           t.HashedPreimage,
			CallbackURL:    t.CallbackURL,
			Status:         t.Status,
			Memo:           t.Memo,
			AmountSat:      t.AmountSat,
			AmountMSat:     t.AmountMSat,
			SettledAt:      t.SettledAt.String(),
		})
	}
}

// CreateInvoice creates a new invoice on behalf of a user
func CreateInvoice(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {
		var newInvoice payments.CreateInvoiceData

		if err := c.ShouldBindJSON(&newInvoice); err != nil {
			log.Error(err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad request, see documentation"})
			return
		}

		_, claims, err := parseBearerJWT(c.GetHeader("Authorization"))
		if err != nil {
			c.JSONP(http.StatusInternalServerError, gin.H{
				"error": "internal server error, please try again or contact support"})
		}

		log.Debugf("received new request for CreateInvoice for user_id %d: %v\n",
			claims.UserID,
			newInvoice)

		t, err := payments.CreateInvoice(
			r.db, *r.lncli, newInvoice, claims.UserID)
		if err != nil {
			log.Error(err)
			c.JSONP(http.StatusInternalServerError, gin.H{
				"error": "internal server error, please try again or contact support "})
			return
		}

		if t.UserID != claims.UserID {
			c.JSONP(http.StatusInternalServerError, gin.H{
				"error": "create invoice internal server error, id's not equal",
			})
		}

		// Return as much info as possible
		c.JSONP(200, &CreateInvoiceResponse{
			ID:             t.ID,
			UserID:         t.UserID,
			PaymentRequest: t.PaymentRequest,
			HashedPreimage: t.HashedPreimage,
			CallbackURL:    t.CallbackURL,
			Status:         t.Status,
			Memo:           t.Memo,
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
			log.Error(err)
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "bad request, see documentation"})
			return
		}

		// authenticate the user by extracting the id from the jwt-token
		_, claims, err := parseBearerJWT(c.GetHeader("Authorization"))
		if err != nil {
			c.JSONP(http.StatusInternalServerError,
				gin.H{"error": "internal server error, try logging in again or refreshing your session"})
		}

		// Pays an invoice from claims.UserID's balance. This is secure because
		// the UserID is extracted from the JWT
		t, err := payments.PayInvoice(r.db, *r.lncli, newPayment, claims.UserID)
		if err != nil {
			c.JSONP(http.StatusInternalServerError, gin.H{
				"error": "internal server error, could not pay invoice"})
			return
		}

		// Return as much info as possible
		c.JSONP(200, &PayInvoiceResponse{
			ID:             t.Payment.ID,
			UserID:         t.User.ID,
			PaymentRequest: t.Payment.PaymentRequest,
			Preimage:       *t.Payment.Preimage,
			Hash:           t.Payment.HashedPreimage,
			Status:         t.Payment.Status,
			Memo:           t.Payment.Memo,
			AmountSat:      t.Payment.AmountSat,
			AmountMSat:     t.Payment.AmountMSat,
			SettledAt:      t.Payment.SettledAt.String(),
		})
	}
}
