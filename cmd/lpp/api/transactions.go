package api

import (
	"net/http"
	"strconv"

	"gitlab.com/arcanecrypto/teslacoil/internal/transactions"

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
		claim, ok := getJWTOrReject(c)
		if !ok {
			return
		}

		var params Params
		if !getQueryOrReject(c, &params) {
			return
		}

		log.Debugf("received request for %d: %+v", claim.UserID, params)

		var t []transactions.Transaction
		var err error
		if params.Limit == 0 && params.Offset == 0 {
			t, err = transactions.GetAllTransactions(r.db, claim.UserID)
		}
		if params.Limit == 0 {
			t, err = transactions.GetAllTransactionsOffset(r.db, claim.UserID, params.Offset)
		} else {
			t, err = transactions.GetAllTransactionsLimitOffset(r.db, claim.UserID, params.Limit, params.Offset)
		}
		if err != nil {
			log.Errorf("Couldn't get transactions: %v", err)
			c.JSONP(http.StatusInternalServerError, gin.H{
				"error": "internal server error, please try again or contact support"})
			return
		}

		c.JSONP(http.StatusOK, t)
	}
}

// GetTransactionByID is a GET request that returns users that match the one
// specified in the body
func (r *RestServer) GetTransactionByID() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := getJWTOrReject(c)
		if !ok {
			return
		}

		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			log.Error(err)
			c.JSONP(http.StatusBadRequest, gin.H{"error": "url param invoice id should be a integer"})
			return
		}

		log.Debugf("find transaction %d for user %d", id, claims.UserID)
		t, err := transactions.GetTransactionByID(r.db, int(id), claims.UserID)
		if err != nil {
			c.JSONP(
				http.StatusNotFound,
				gin.H{"error": "invoice not found"},
			)
			return
		}

		log.Debugf("found transaction %v", t)

		// Return the user when it is found and no errors where encountered
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

		var request transactions.WithdrawOnChainArgs
		if !getJSONOrReject(c, &request) {
			return
		}
		// add the userID to send coins from
		request.UserID = claims.UserID

		// TODO: Create a middleware for logging request body
		log.Infof("Received WithdrawOnChain request %+v\n", request)

		transaction, err := transactions.WithdrawOnChain(r.db, *r.lncli, request)
		if err != nil {
			log.Errorf("cannot withdraw onchain: %v", err)
			c.JSONP(http.StatusInternalServerError,
				internalServerErrorResponse,
			)
			return
		}

		c.JSONP(http.StatusOK, WithdrawResponse{
			Txid:        transaction.Txid,
			Address:     transaction.Address,
			AmountSat:   transaction.AmountSat,
			Description: transaction.Description,
		})
	}

}

// NewDeposit is a request handler used for creating a new deposit
// If successful, response with an address, and the saved description
func (r *RestServer) NewDeposit() gin.HandlerFunc {
	type NewDepositResponse struct {
		ID          int    `json:"id"`
		Address     string `json:"address"`
		Description string `json:"description"`
	}

	return func(c *gin.Context) {
		claims, ok := getJWTOrReject(c)
		if !ok {
			return
		}

		var request transactions.GetAddressArgs
		if ok := getJSONOrReject(c, &request); !ok {
			return
		}
		log.Infof("Received DepositOnChain request %+v\n", request)

		transaction, err := transactions.GetDeposit(r.db, *r.lncli, claims.UserID, request)
		if err != nil {
			log.Errorf("cannot deposit onchain: %v", err)
			c.JSONP(http.StatusInternalServerError,
				internalServerErrorResponse,
			)
			return
		}

		c.JSONP(http.StatusOK, NewDepositResponse{
			ID:          transaction.ID,
			Address:     transaction.Address,
			Description: transaction.Description,
		})
	}
}
