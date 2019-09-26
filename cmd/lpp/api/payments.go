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
		claim, ok := getJWTOrReject(c)
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

		t, err := payments.GetAll(r.db, claim.UserID, int(limit), int(offset))
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
		claims, ok := getJWTOrReject(c)
		if !ok {
			return
		}

		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			log.Error(err)
			c.JSONP(404, gin.H{"error": "url param invoice id should be a integer"})
			return
		}
		log.Infof("find payment %d for user %d", id, claims.UserID)

		t, err := payments.GetByID(r.db, int(id), claims.UserID)
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
	}

	return func(c *gin.Context) {
		claims, ok := getJWTOrReject(c)
		if !ok {
			return
		}

		var request CreateInvoiceRequest
		if ok := getJSONOrReject(c, &request); !ok {
			return
		}
		log.Debugf("received new request for CreateInvoice for user_id %d: %+v",
			claims.UserID,
			request)

		t, err := payments.NewPayment(
			r.db, *r.lncli, claims.UserID, request.AmountSat,
			request.Memo, request.Description)
		if err != nil {
			log.Error(errors.Wrapf(err, "could not add new payment"))
			c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
			return
		}

		if t.UserID != claims.UserID {
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
		claims, ok := getJWTOrReject(c)
		if !ok {
			return
		}

		var request PayInvoiceRequest
		if ok = getJSONOrReject(c, &request); !ok {
			return
		}
		log.Debugf("received new request for PayInvoice for userID %d: %+v",
			claims.UserID, request)

		// Pays an invoice from claims.UserID's balance. This is secure because
		// the UserID is extracted from the JWT
		t, err := payments.PayInvoiceWithDescription(r.db, *r.lncli, claims.UserID,
			request.PaymentRequest, request.Description)
		if err != nil {
			c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
			return
		}

		c.JSONP(http.StatusOK, t)
	}
}

// WithdrawOnChain is a request handler used for withdrawing funds
// to an on-chain address
// If the withdrawal is successful, it responds with the txid
func (r *RestServer) WithdrawOnChain() gin.HandlerFunc {
	type WithdrawResponse struct {
		Txid        string `json:"txid"`
		Address     string `json:"address"`
		AmountSat   int64  `json:"amountSat"`
		Description string `json:"description"`
	}

	return func(c *gin.Context) {
		claims, ok := getJWTOrReject(c)
		if !ok {
			return
		}

		var request payments.WithdrawOnChainArgs
		if ok := getJSONOrReject(c, &request); !ok {
			return
		}
		// add the userID to send coins from
		request.UserID = claims.UserID

		// TODO: Create a middleware for logging request body
		log.Infof("Received WithdrawOnChain request %+v\n", request)

		transaction, err := payments.WithdrawOnChain(r.db, *r.lncli, request)
		if err != nil {
			log.Errorf("cannot withdraw onchain: %v", err)
			c.JSONP(http.StatusInternalServerError,
				internalServerErrorResponse,
			)
			return
		}

		c.JSONP(http.StatusOK, WithdrawResponse{
			Txid:        *transaction.Txid,
			Address:     transaction.Address,
			AmountSat:   transaction.AmountSat,
			Description: transaction.Description,
		})
	}

}

// DepositOnChain is a request handler used for depositing funds
// to an on-chain address
// If the deposit is successful, it responds with an address
func (r *RestServer) DepositOnChain() gin.HandlerFunc {
	type NewDepositResponse struct {
		Address     string `json:"address"`
		Description string `json:"description"`
	}

	return func(c *gin.Context) {
		claims, ok := getJWTOrReject(c)
		if !ok {
			return
		}

		var request payments.DepositOnChainArgs
		if ok := getJSONOrReject(c, &request); !ok {
			return
		}
		log.Infof("Received DepositOnChain request %+v\n", request)

		transaction, err := payments.DepositOnChain(r.db, *r.lncli, claims.UserID, request)
		if err != nil {
			log.Errorf("cannot deposit onchain: %v", err)
			c.JSONP(http.StatusInternalServerError,
				internalServerErrorResponse,
			)
			return
		}

		c.JSONP(http.StatusOK, NewDepositResponse{
			Address:     transaction.Address,
			Description: transaction.Description,
		})
	}

}
