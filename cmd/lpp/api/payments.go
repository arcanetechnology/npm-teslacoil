package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/teslacoil/internal/payments"
)

// CreateInvoiceRequest is a deposit
type CreateInvoiceRequest struct {
	AmountSat   int64  `json:"amountSat"`
	Memo        string `json:"memo,omitempty"`
	Description string `json:"description,omitempty"`
}

// PayInvoiceRequest is the required and optional fields for initiating a
// withdrawal. fields tagged with omitEmpty are optional
type PayInvoiceRequest struct {
	PaymentRequest string `json:"paymentRequest"`
	Description    string `json:"description,omitempty"`
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

		claim, err := getJWTOrReject(c)
		if err != nil {
			return
		}

		t, err := payments.GetAll(r.db, claim.UserID, int(limit), int(offset))
		if err != nil {
			log.Errorf("Couldn't get payments: %v", err)
			c.JSONP(http.StatusInternalServerError, gin.H{
				"error": "internal server error, please try again or contact support"})
			return
		}

		c.JSONP(200, t)
	}
}

// GetPaymentByID is a GET request that returns users that match the one
// specified in the body
func (r *RestServer) GetPaymentByID() gin.HandlerFunc {
	return func(c *gin.Context) {
		claim, err := getJWTOrReject(c)
		if err != nil {
			return
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
		c.JSONP(200, t)
	}
}

// CreateInvoice creates a new invoice
func (r *RestServer) CreateInvoice() gin.HandlerFunc {
	return func(c *gin.Context) {
		var newPayment CreateInvoiceRequest

		if err := c.ShouldBindJSON(&newPayment); err != nil {
			log.Errorf("Could not bind invoice request: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad request, see documentation"})
			return
		}

		log.Tracef("Bound invoice request: %+v", newPayment)
		claims, err := getJWTOrReject(c)
		if err != nil {
			return
		}

		log.Errorf("received new request for CreateInvoice for user_id %d: %v\n",
			claims.UserID,
			newPayment)

		t, err := payments.NewPayment(
			r.db, *r.lncli, claims.UserID, newPayment.AmountSat,
			newPayment.Memo, newPayment.Description)
		if err != nil {
			log.Error(errors.Wrapf(err, "CreateInvoice() -> NewPayment(%d, %d, %v, %v)",
				claims.UserID,
				newPayment.AmountSat,
				newPayment.Memo,
				newPayment.Description))
			c.JSONP(http.StatusInternalServerError, gin.H{
				"error": "internal server error, please try again or contact support "})
			return
		}

		if t.UserID != claims.UserID {
			c.JSONP(http.StatusInternalServerError, gin.H{
				"error": "create invoice internal server error, id's not equal",
			})
		}

		c.JSONP(200, t)
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
		claims, err := getJWTOrReject(c)
		if err != nil {
			return
		}

		// Pays an invoice from claims.UserID's balance. This is secure because
		// the UserID is extracted from the JWT
		t, err := payments.PayInvoiceWithDescription(r.db, *r.lncli, claims.UserID,
			req.PaymentRequest, req.Description)
		if err != nil {
			c.JSONP(http.StatusInternalServerError, gin.H{
				"error": "internal server error, could not pay invoice"})
			return
		}

		c.JSONP(200, t)
	}
}
