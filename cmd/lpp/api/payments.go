package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/teslacoil/internal/payments"
)

// GetAllPayments finds all payments for the given user. Takes two URL
// parameters, `limit` and `offset`
func (r *RestServer) GetAllPayments() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := getUserIdOrReject(c)
		if !ok {
			return
		}

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

		t, err := payments.GetAll(r.db, userID, int(limit), int(offset))
		if err != nil {
			log.Errorf("Couldn't get payments: %v", err)
			c.JSONP(http.StatusInternalServerError, gin.H{
				"error": "internal server error, please try again or contact support"})
			return
		}

		c.JSONP(http.StatusOK, t)
	}
}

// GetPaymentByID is a GET request that returns users that match the one
// specified in the body
func (r *RestServer) GetPaymentByID() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := getUserIdOrReject(c)
		if !ok {
			return
		}

		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			log.Error(err)
			c.JSONP(404, gin.H{"error": "url param invoice id should be a integer"})
			return
		}
		log.Infof("find payment %d for user %d", id, userID)

		t, err := payments.GetByID(r.db, int(id), userID)
		if err != nil {
			c.JSONP(
				http.StatusNotFound,
				gin.H{"error": "invoice not found"},
			)
			return
		}

		log.Infof("found payment %v", t)

		// Return the user when it is found and no errors where encountered
		c.JSONP(http.StatusOK, t)
	}
}

// CreateInvoice creates a new invoice
func (r *RestServer) CreateInvoice() gin.HandlerFunc {
	// CreateInvoiceRequest is a deposit
	type CreateInvoiceRequest struct {
		AmountSat   int64  `json:"amountSat" binding:"required,gt=0"`
		Memo        string `json:"memo" binding:"max=256"`
		Description string `json:"description"`
		CallbackURL string `json:"callbackUrl" binding:"omitempty,url"`
	}

	return func(c *gin.Context) {
		userID, ok := getUserIdOrReject(c)
		if !ok {
			return
		}

		var request CreateInvoiceRequest
		if ok := getJSONOrReject(c, &request); !ok {
			return
		}

		log.Debugf("received new request for CreateInvoice for user_id %d: %+v",
			userID,
			request)

		t, err := payments.NewPayment(
			r.db, *r.lncli, payments.NewPaymentOpts{
				UserID:      userID,
				AmountSat:   request.AmountSat,
				Memo:        request.Memo,
				Description: request.Description,
				CallbackURL: request.CallbackURL,
			})

		if err != nil {
			log.Error(errors.Wrapf(err, "could not add new payment"))
			c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
			return
		}

		if t.UserID != userID {
			c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
		}

		c.JSONP(http.StatusOK, t)
	}
}

// PayInvoice pays a valid invoice on behalf of a user
func (r *RestServer) PayInvoice() gin.HandlerFunc {
	// PayInvoiceRequest is the required and optional fields for initiating a
	// withdrawal.
	type PayInvoiceRequest struct {
		PaymentRequest string `json:"paymentRequest" binding:"required"`
		Description    string `json:"description"`
	}

	return func(c *gin.Context) {
		// authenticate the user by extracting the id from the jwt-token
		userID, ok := getUserIdOrReject(c)
		if !ok {
			return
		}

		var request PayInvoiceRequest
		if ok = getJSONOrReject(c, &request); !ok {
			return
		}
		log.Debugf("received new request for PayInvoice for userID %d: %+v",
			userID, request)

		// Pays an invoice from claims.UserID's balance. This is secure because
		// the UserID is extracted from the JWT
		t, err := payments.PayInvoiceWithDescription(r.db, *r.lncli, userID,
			request.PaymentRequest, request.Description)
		if err != nil {
			c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
			return
		}

		c.JSONP(http.StatusOK, t)
	}
}
