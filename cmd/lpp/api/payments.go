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
	Preimage       *string            `json:"preimage"`
	Hash           string             `json:"hash"`
	CallbackURL    *string            `json:"callbackUrl"`
	Status         payments.Status    `json:"status"`
	Memo           *string            `json:"memo,omitempty"`
	Description    *string            `json:"description,omitempty"`
	Expiry         int64              `json:"expiry"`
	Direction      payments.Direction `json:"direction"`
	AmountSat      int64              `json:"amountSat"`
	AmountMSat     int64              `json:"amountMSat"`
	SettledAt      *time.Time         `json:"settledAt"`
	CreatedAt      time.Time          `json:"createdAt"`
}

// CreateInvoiceRequest is a deposit
type CreateInvoiceRequest struct {
	Memo        *string `json:"memo,omitempty"`
	Description *string `json:"description,omitempty"`
	AmountSat   int64   `json:"amountSat"`
}

// PayInvoiceRequest is the required(and optional) fields for initiating a
// withdrawal
type PayInvoiceRequest struct {
	PaymentRequest string `json:"paymentRequest"`
	Description    string `json:"description"`
	Memo           string `json:"memo"`
}

func convertToPaymentResponse(payments []payments.Payment) []PaymentResponse {
	invResponse := []PaymentResponse{}

	for _, p := range payments {
		invResponse = append(invResponse, PaymentResponse{
			ID:             p.ID,
			UserID:         p.UserID,
			PaymentRequest: p.PaymentRequest,
			Preimage:       p.Preimage,
			Hash:           p.HashedPreimage,
			Expiry:         p.Expiry,
			CallbackURL:    p.CallbackURL,
			Status:         p.Status,
			Memo:           p.Memo,
			Description:    p.Description,
			Direction:      p.Direction,
			AmountSat:      p.AmountSat,
			AmountMSat:     p.AmountMSat,
			SettledAt:      p.SettledAt,
			CreatedAt:      p.CreatedAt,
		})
	}

	return invResponse
}

// GetAllPayments finds all payments for the given user. Takes two URL
// parameters, `limit` and `offset`
func (r *RestServer) GetAllPayments() gin.HandlerFunc {
	return func(c *gin.Context) {
		URLParams := c.Request.URL.Query()
		limitStr := URLParams.Get("limit")
		offsetStr := URLParams.Get("offset")

		limit, err := strconv.ParseInt(limitStr, 10, 64)
		if err != nil {
			log.Errorf(`Couldn't parse "limit" to an integer: %v`, err)
			c.JSONP(http.StatusBadRequest, gin.H{"error": "url param \"limit\" should be a integer"})
			return
		}
		offset, err := strconv.ParseInt(offsetStr, 10, 64)
		if err != nil {
			log.Errorf(`Couldn't parse "offset" to an integer: %v`, offset)
			c.JSONP(http.StatusBadRequest, gin.H{"error": "url param \"offset\" should be a integer"})
			return
		}

		auth := c.GetHeader("Authorization")
		_, claim, err := parseBearerJWT(auth)
		if err != nil {
			log.Errorf("GetAllPayments()->ParseBearerJWT(%s): Couldn't parse auth header: %+v",
				auth, err)
			c.JSONP(http.StatusBadRequest,
				gin.H{
					"error": "bad authorization header, should be bearer auth (JWT)",
				})
			return
		}

		// TODO: Make sure conversion from int64 to int is always safe and does
		// not overflow if limit > MAXINT32 {abort} if offset > MAXINT32 {abort}
		t, err := payments.GetAll(r.db, claim.UserID, int(limit), int(offset))
		if err != nil {
			log.Errorf("Couldn't get payments: %v", err)
			c.JSONP(http.StatusInternalServerError, gin.H{
				"error": "internal server error, please try again or contact support"})
			return
		}

		json := convertToPaymentResponse(t)
		c.JSONP(200, json)
	}
}

// GetPaymentByID is a GET request that returns users that match the one
// specified in the body
func (r *RestServer) GetPaymentByID() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, claim, err := parseBearerJWT(c.GetHeader("Authorization"))
		if err != nil {
			log.Errorf("Could not parse bearer JWT: %v", err)
			c.JSONP(404, gin.H{"error": "Bearer JWT was invalid"})
		}

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
			Preimage:       t.Preimage,
			Hash:           t.HashedPreimage,
			Expiry:         t.Expiry,
			CallbackURL:    t.CallbackURL,
			Status:         t.Status,
			Memo:           t.Memo,
			Description:    t.Description,
			AmountSat:      t.AmountSat,
			AmountMSat:     t.AmountMSat,
			SettledAt:      t.SettledAt,
			CreatedAt:      t.CreatedAt,
		})
	}
}

// CreateInvoice creates a new invoice on behalf of a user
func (r *RestServer) CreateInvoice() gin.HandlerFunc {
	return func(c *gin.Context) {
		var newInvoice CreateInvoiceRequest

		if err := c.ShouldBindJSON(&newInvoice); err != nil {
			log.Errorf("Could not bind invoice request: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad request, see documentation"})
			return
		}

		log.Tracef("Bound invoice request: %+v", newInvoice)
		_, claims, err := parseBearerJWT(c.GetHeader("Authorization"))
		if err != nil {
			c.JSONP(http.StatusInternalServerError, gin.H{
				"error": "internal server error, please try again or contact support"})
		}

		log.Infof("received new request for CreateInvoice for user_id %d: %v\n",
			claims.UserID,
			newInvoice)

		t, err := payments.CreateInvoice(
			r.db, *r.lncli, claims.UserID, newInvoice.AmountSat,
			newInvoice.Description, newInvoice.Memo)
		if err != nil {
			log.Errorf("Could not create invoice: %v", err)
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
		c.JSONP(200, &PaymentResponse{
			ID:             t.ID,
			UserID:         t.UserID,
			PaymentRequest: t.PaymentRequest,
			Hash:           t.HashedPreimage,
			Preimage:       t.Preimage,
			CallbackURL:    t.CallbackURL,
			Status:         t.Status,
			Memo:           t.Memo,
			Description:    t.Description,
			Expiry:         t.Expiry,
			AmountSat:      t.AmountSat,
			AmountMSat:     t.AmountMSat,
			SettledAt:      t.SettledAt,
			CreatedAt:      t.CreatedAt,
		})
	}
}

// PayInvoice pays a valid invoice on behalf of a user
func (r *RestServer) PayInvoice() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req PayInvoiceRequest

		if err := c.ShouldBindJSON(&req); err != nil {
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
		t, err := payments.PayInvoice(r.db, *r.lncli, claims.UserID,
			req.PaymentRequest, req.Description, req.Memo)
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
			Preimage:       t.Payment.Preimage,
			Hash:           t.Payment.HashedPreimage,
			Expiry:         t.Payment.Expiry,
			Status:         t.Payment.Status,
			Memo:           t.Payment.Memo,
			Description:    t.Payment.Description,
			AmountSat:      t.Payment.AmountSat,
			AmountMSat:     t.Payment.AmountMSat,
		})
	}
}
