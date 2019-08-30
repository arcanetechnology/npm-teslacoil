package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gitlab.com/arcanecrypto/teslacoil/internal/payments"
)

// PaymentResponse is the generic response for any GET /payment endpoint
type PaymentResponse struct {
	ID             int                `json:"id"`
	UserID         int                `json:"userId"`
	PaymentRequest string             `json:"paymentRequest"`
	Preimage       string             `json:"preimage"`
	Hash           string             `json:"hash"`
	CallbackURL    string             `json:"callbackUrl"`
	Status         payments.Status    `json:"status"`
	Memo           string             `json:"memo"`
	Direction      payments.Direction `json:"direction"`
	AmountSat      int                `json:"amountSat"`
	AmountMSat     int                `json:"amountMSat"`
	SettledAt      *time.Time         `json:"settledAt"`
}

// CreateInvoiceResponse is the request for the /invoice/create endpoint
type CreateInvoiceResponse struct {
	ID             int             `json:"id"`
	UserID         int             `json:"userId"`
	PaymentRequest string          `json:"paymentRequest"`
	HashedPreimage string          `json:"hashedPreimage"`
	CallbackURL    string          `json:"callbackUrl"`
	Status         payments.Status `json:"status"`
	Memo           string          `json:"memo"`
	AmountSat      int             `json:"amountSat"`
	AmountMSat     int             `json:"amountMSat"`
}

func convertToPaymentResponse(payments []payments.Payment) []PaymentResponse {
	var invResponse []PaymentResponse

	for _, payment := range payments {
		var preimage, callbackURL string
		if payment.Preimage.Valid {
			preimage = payment.Preimage.String
		}

		if payment.CallbackURL.Valid {
			callbackURL = payment.CallbackURL.String
		}

		invResponse = append(invResponse, PaymentResponse{
			ID:             payment.ID,
			UserID:         payment.UserID,
			PaymentRequest: payment.PaymentRequest,
			Preimage:       preimage,
			Hash:           payment.HashedPreimage,
			CallbackURL:    callbackURL,
			Status:         payment.Status,
			Memo:           payment.Memo,
			Direction:      payment.Direction,
			AmountSat:      payment.AmountSat,
			AmountMSat:     payment.AmountMSat,
			SettledAt:      payment.SettledAt,
		})
	}

	return invResponse
}

// GetAllPayments is a GET request that returns all the users in the database
// Takes two URL-params on the form ?limit=kek&offset=kek
func (r *RestServer) GetAllPayments() gin.HandlerFunc {
	return func(c *gin.Context) {
		URLParams := c.Request.URL.Query()
		limitStr := URLParams.Get("limit")
		offsetStr := URLParams.Get("offset")

		limit, err := strconv.ParseInt(limitStr, 10, 64)
		if err != nil {
			log.Error(err)
			c.JSONP(404, gin.H{"error": "url param \"limit\" should be a integer"})
			return
		}
		offset, err := strconv.ParseInt(offsetStr, 10, 64)
		if err != nil {
			log.Error(err)
			c.JSONP(404, gin.H{"error": "url param \"offset\" should be a integer"})
			return
		}

		_, claim, err := parseBearerJWT(c.GetHeader("Authorization"))

		// TODO: Make sure conversion from int64 to int is always safe and does
		// not overflow if limit > MAXINT32 {abort} if offset > MAXINT32 {abort}
		t, err := payments.GetAll(r.db, claim.UserID, int(limit), int(offset))
		if err != nil {
			c.JSONP(http.StatusInternalServerError, gin.H{
				"error": "internal server error, please try again or contact support"})
			return
		}

		c.JSONP(200, convertToPaymentResponse(t))
	}
}

// GetSinglePayment is a GET request that returns users that match the one specified in the body
func (r *RestServer) GetSinglePayment() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, claim, err := parseBearerJWT(c.GetHeader("Authorization"))

		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			log.Error(err)
			c.JSONP(404, gin.H{"error": "url param invoice id should be a integer"})
			return
		}
		log.Infof("find payment %d for user %d", id, claim.UserID)

		t, err := payments.GetByID(r.db, int(id), claim.UserID)
		if err != nil {
			c.JSONP(
				http.StatusNotFound,
				gin.H{"error": "invoice not found"},
			)
			return
		}

		log.Infof("found payment %v", t)
		// Return the user when it is found and no errors where encountered
		c.JSONP(200, PaymentResponse{
			ID:             t.ID,
			UserID:         t.UserID,
			PaymentRequest: t.PaymentRequest,
			Preimage:       t.Preimage.String,
			Hash:           t.HashedPreimage,
			CallbackURL:    t.CallbackURL.String,
			Status:         t.Status,
			Memo:           t.Memo,
			AmountSat:      t.AmountSat,
			AmountMSat:     t.AmountMSat,
			SettledAt:      t.SettledAt,
		})
	}
}

// CreateInvoice creates a new invoice on behalf of a user
func (r *RestServer) CreateInvoice() gin.HandlerFunc {
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

		log.Infof("received new request for CreateInvoice for user_id %d: %v\n",
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
			CallbackURL:    t.CallbackURL.String,
			Status:         t.Status,
			Memo:           t.Memo,
			AmountSat:      t.AmountSat,
			AmountMSat:     t.AmountMSat,
		})
	}
}

// PayInvoice pays a valid invoice on behalf of a user
func (r *RestServer) PayInvoice() gin.HandlerFunc {
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
		c.JSONP(200, &PaymentResponse{
			ID:             t.Payment.ID,
			UserID:         t.User.ID,
			PaymentRequest: t.Payment.PaymentRequest,
			Preimage:       t.Payment.Preimage.String,
			Hash:           t.Payment.HashedPreimage,
			Status:         t.Payment.Status,
			Memo:           t.Payment.Memo,
			AmountSat:      t.Payment.AmountSat,
			AmountMSat:     t.Payment.AmountMSat,
			SettledAt:      t.Payment.SettledAt,
		})
	}
}
