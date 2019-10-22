package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"gitlab.com/arcanecrypto/teslacoil/api/apierr"

	"gitlab.com/arcanecrypto/teslacoil/models/payments"
	"gitlab.com/arcanecrypto/teslacoil/models/transactions"

	"github.com/gin-gonic/gin"
)

// GetAllTransactions finds all payments for the given user. Takes two URL
// parameters, `limit` and `offset`
func (r *RestServer) GetAllTransactions() gin.HandlerFunc {
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

		log.Debugf("received request for %d: %+v", userID, params)

		var t []transactions.Transaction
		var err error
		if params.Limit == 0 && params.Offset == 0 {
			t, err = transactions.GetAllTransactions(r.db, userID)
		} else if params.Limit == 0 {
			t, err = transactions.GetAllTransactionsOffset(r.db, userID, params.Offset)
		} else {
			t, err = transactions.GetAllTransactionsLimitOffset(r.db, userID, params.Limit, params.Offset)
		}
		if err != nil {
			log.WithError(err).Error("Couldn't get transactions")
			_ = c.Error(err)
			return
		}

		c.JSONP(http.StatusOK, t)
	}
}

// GetTransactionByID is a GET request that returns users that match the one
// specified in the body
func (r *RestServer) GetTransactionByID() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := getUserIdOrReject(c)
		if !ok {
			return
		}

		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			log.Error(err)
			c.JSONP(http.StatusBadRequest, gin.H{"error": "url param invoice id should be a integer"})
			return
		}

		log.Debugf("find transaction %d for user %d", id, userID)
		t, err := transactions.GetTransactionByID(r.db, int(id), userID)
		if err != nil {
			apierr.Public(c, http.StatusNotFound, apierr.ErrTransactionNotFound)
			return
		}

		// Return the user when it is found and no errors where encountered
		c.JSONP(http.StatusOK, t)
	}
}

// WithdrawOnChain is a request handler used for withdrawing funds
// to an on-chain address
// TODO: verify dust limits
func (r *RestServer) WithdrawOnChain() gin.HandlerFunc {
	type WithdrawResponse struct {
		ID          int                `json:"id"`
		Address     string             `json:"address"`
		Txid        *string            `json:"txid"`
		Vout        *int               `json:"vout"`
		Direction   payments.Direction `json:"direction"`
		AmountSat   int64              `json:"amountSat"`
		Description *string            `json:"description"`
		Confirmed   bool               `json:"confirmed"`

		ConfirmedAt *time.Time `json:"confirmedAt"`
	}

	return func(c *gin.Context) {
		userID, ok := getUserIdOrReject(c)
		if !ok {
			return
		}

		var req transactions.WithdrawOnChainArgs
		if c.BindJSON(&req) != nil {
			return
		}
		// add the userID to send coins from
		req.UserID = userID

		transaction, err := transactions.WithdrawOnChain(r.db, r.lncli, r.bitcoind, req)
		if err != nil {
			if errors.Is(err, transactions.ErrBalanceTooLowForWithdrawal) {
				apierr.Public(c, http.StatusBadRequest, apierr.ErrBalanceTooLowForWithdrawal)
				return
			}
			log.WithError(err).Errorf("Cannot withdraw onchain")
			_ = c.Error(err)
			return
		}

		c.JSONP(http.StatusOK, WithdrawResponse{
			ID:          transaction.ID,
			Address:     transaction.Address,
			Txid:        transaction.Txid,
			Vout:        transaction.Vout,
			Direction:   transaction.Direction,
			AmountSat:   transaction.AmountSat,
			Description: transaction.Description,
			Confirmed:   transaction.Confirmed,

			ConfirmedAt: transaction.ConfirmedAt,
		})
	}

}

// NewDeposit is a request handler used for creating a new deposit
// If successful, response with an address, and the saved description
func (r *RestServer) NewDeposit() gin.HandlerFunc {
	type request struct {
		// Whether to discard the old address and force create a new one
		ForceNewAddress bool `json:"forceNewAddress"`
		// A personal description for the transaction
		Description string `json:"description"`
	}

	type response struct {
		ID          int                `json:"id"`
		Address     string             `json:"address"`
		Direction   payments.Direction `json:"direction"`
		Description *string            `json:"description"`
		Confirmed   bool               `json:"confirmed"`
		CreatedAt   time.Time          `json:"createdAt"`
	}

	return func(c *gin.Context) {
		userID, ok := getUserIdOrReject(c)
		if !ok {
			return
		}

		var req request
		if c.BindJSON(&req) != nil {
			return
		}

		transaction, err := transactions.GetOrCreateDeposit(r.db, r.lncli, userID,
			req.ForceNewAddress, req.Description)
		if err != nil {
			log.WithError(err).Error("Cannot deposit onchain")
			_ = c.Error(err)
			return
		}
		res := response{
			ID:          transaction.ID,
			Address:     transaction.Address,
			Direction:   payments.INBOUND,
			Description: transaction.Description,
			Confirmed:   false,
			CreatedAt:   transaction.CreatedAt,
		}
		c.JSONP(http.StatusOK, res)
	}
}
