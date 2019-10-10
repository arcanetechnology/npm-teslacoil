package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gitlab.com/arcanecrypto/teslacoil/internal/errhandling"
	"gitlab.com/arcanecrypto/teslacoil/internal/httptypes"
	"gitlab.com/arcanecrypto/teslacoil/internal/payments"
)

// GetAllPayments finds all payments for the given user. Takes two URL
// parameters, `limit` and `offset`
func (r *RestServer) GetAllPayments() gin.HandlerFunc {
	type Params struct {
		Limit  int `form:"limit" binding:"gte=0"`
		Offset int `form:"offset" binding:"gte=0"`
	}
	return func(c *gin.Context) {
		userID, ok := getUserIdOrReject(c)
		if !ok {
			return
		}

		var params Params
		if c.BindQuery(&params) != nil {
			return
		}

		t, err := payments.GetAll(r.db, userID, params.Limit, params.Offset)
		if err != nil {
			log.WithError(err).Errorf("Couldn't get payments")
			_ = c.Error(err)
			return
		}

		c.JSONP(http.StatusOK, httptypes.Response(t))
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
			err := c.AbortWithError(http.StatusNotFound, errors.New("invoice not found"))
			_ = err.SetType(gin.ErrorTypePublic)
			_ = err.SetMeta(errhandling.ErrInvoiceNotFound)
			return
		}

		log.Infof("found payment %v", t)

		// Return the user when it is found and no errors where encountered
		c.JSONP(http.StatusOK, httptypes.Response(t))
	}
}

// CreateInvoice creates a new invoice
func (r *RestServer) CreateInvoice() gin.HandlerFunc {
	// CreateInvoiceRequest is a deposit
	type CreateInvoiceRequest struct {
		AmountSat   int64  `json:"amountSat" binding:"required,gt=0,lte=4294967"`
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
		if c.BindJSON(&request) != nil {
			return
		}

		log.Debugf("received new request for CreateInvoice for user_id %d: %+v",
			userID,
			request)

		t, err := payments.NewPayment(
			r.db, r.lncli, payments.NewPaymentOpts{
				UserID:      userID,
				AmountSat:   request.AmountSat,
				Memo:        request.Memo,
				Description: request.Description,
				CallbackURL: request.CallbackURL,
			})

		if err != nil {
			log.WithError(err).Error("Could not add new payment")
			_ = c.Error(err)
			return
		}

		if t.UserID != userID {
			_ = c.Error(err)
			return
		}

		c.JSONP(http.StatusOK, httptypes.Response(t))
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
		if c.BindJSON(&request) != nil {
			return
		}
		log.Debugf("received new request for PayInvoice for userID %d: %+v",
			userID, request)

		// Pays an invoice from claims.UserID's balance. This is secure because
		// the UserID is extracted from the JWT
		t, err := payments.PayInvoiceWithDescription(r.db, r.lncli, userID,
			request.PaymentRequest, request.Description)
		if err != nil {
			_ = c.Error(err)
			return
		}

		c.JSONP(http.StatusOK, httptypes.Response(t))
	}
}
