package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/sirupsen/logrus"

	"gitlab.com/arcanecrypto/teslacoil/api/apierr"

	"gitlab.com/arcanecrypto/teslacoil/models/transactions"

	"github.com/gin-gonic/gin"
)

// getAllTransactions finds all payments for the given user. Takes two URL
// parameters, `limit` and `offset`
func (r *RestServer) getAllTransactions() gin.HandlerFunc {
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

// getTransactionByID is a GET request that returns users that match the one
// specified in the body
func (r *RestServer) getTransactionByID() gin.HandlerFunc {
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

// withdrawOnChain is a request handler used for withdrawing funds
// to an on-chain address
// TODO: verify dust limits
func (r *RestServer) withdrawOnChain() gin.HandlerFunc {
	type withdrawResponse struct {
		ID          int                    `json:"id"`
		Address     string                 `json:"address"`
		Txid        *string                `json:"txid"`
		Vout        *int                   `json:"vout"`
		Direction   transactions.Direction `json:"direction"`
		AmountSat   int64                  `json:"amountSat"`
		Description *string                `json:"description"`
		Confirmed   bool                   `json:"confirmed"`

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

		onchain, err := transactions.WithdrawOnChain(r.db, r.lncli, r.bitcoind, req)
		if err != nil {
			if errors.Is(err, transactions.ErrBalanceTooLowForWithdrawal) {
				apierr.Public(c, http.StatusBadRequest, apierr.ErrBalanceTooLowForWithdrawal)
				return
			}
			log.WithError(err).Errorf("Cannot withdraw onchain")
			_ = c.Error(err)
			return
		}

		c.JSONP(http.StatusOK, withdrawResponse{
			ID:          onchain.ID,
			Address:     onchain.Address,
			Txid:        onchain.Txid,
			Vout:        onchain.Vout,
			Direction:   onchain.Direction,
			AmountSat:   onchain.AmountSat,
			Description: onchain.Description,
			ConfirmedAt: onchain.ConfirmedAt,
		})
	}

}

// newDeposit is a request handler used for creating a new deposit
// If successful, response with an address, and the saved description
func (r *RestServer) newDeposit() gin.HandlerFunc {
	type request struct {
		// Whether to discard the old address and force create a new one
		ForceNewAddress bool `json:"forceNewAddress"`
		// A personal description for the transaction
		Description string `json:"description"`
	}

	type response struct {
		ID          int                    `json:"id"`
		Address     string                 `json:"address"`
		Direction   transactions.Direction `json:"direction"`
		Description *string                `json:"description"`
		Confirmed   bool                   `json:"confirmed"`
		CreatedAt   time.Time              `json:"createdAt"`
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
			Direction:   transactions.INBOUND,
			Description: transaction.Description,
			Confirmed:   false,
			CreatedAt:   transaction.CreatedAt,
		}
		c.JSONP(http.StatusOK, res)
	}
}

// createInvoice creates a new invoice
func (r *RestServer) createInvoice() gin.HandlerFunc {
	// createInvoiceRequest is a deposit
	type createInvoiceRequest struct {
		AmountSat   int64  `json:"amountSat" binding:"required,gt=0,lte=4294967"`
		Memo        string `json:"memo" binding:"max=256"`
		Description string `json:"description"`
		CallbackURL string `json:"callbackUrl" binding:"omitempty,url"`
		OrderId     string `json:"orderId" binding:"max=256"`
	}

	return func(c *gin.Context) {

		userID, ok := getUserIdOrReject(c)

		if !ok {
			return
		}

		var req createInvoiceRequest
		if c.BindJSON(&req) != nil {
			return
		}

		// TODO: rename this function to something like `NewLightningPayment` or `NewLightningInvoice`
		t, err := transactions.NewOffchain(
			r.db, r.lncli, transactions.NewOffchainOpts{
				UserID:      userID,
				AmountSat:   req.AmountSat,
				Memo:        req.Memo,
				Description: req.Description,
				CallbackURL: req.CallbackURL,
				OrderId:     req.OrderId,
			})

		if err != nil {
			log.WithError(err).Error("Could not add new payment")
			_ = c.Error(err)
			return
		}

		c.JSONP(http.StatusOK, t)
	}
}

// payInvoice pays a valid invoice on behalf of a user
func (r *RestServer) payInvoice() gin.HandlerFunc {
	// PayInvoiceRequest is the required and optional fields for initiating a
	// withdrawal.
	type payInvoiceRequest struct {
		PaymentRequest string `json:"paymentRequest" binding:"required,paymentrequest"`
		Description    string `json:"description"`
	}

	return func(c *gin.Context) {
		userID, ok := getUserIdOrReject(c)
		if !ok {
			return
		}

		var req payInvoiceRequest
		if c.BindJSON(&req) != nil {
			return
		}

		// Pays an invoice from userid found in authorization header.
		t, err := transactions.PayInvoiceWithDescription(r.db, r.lncli, userID,
			req.PaymentRequest, req.Description)
		if err != nil {
			// investigate details around failure
			go func() {
				origErr := err
				decoded, err := r.lncli.DecodePayReq(context.Background(), &lnrpc.PayReqString{
					PayReq: req.PaymentRequest,
				})
				if err != nil {
					log.WithError(err).Error("Could not decode payment request")
					return
				}

				channels, err := r.lncli.ListChannels(context.Background(), &lnrpc.ListChannelsRequest{
					ActiveOnly: true,
				})
				if err != nil {
					log.WithError(err).Error("Could not list active channels")
					return
				}

				balance, err := r.lncli.ChannelBalance(context.Background(), &lnrpc.ChannelBalanceRequest{})
				if err != nil {
					log.WithError(err).Error("Could not get channel balance")
				}

				log.WithFields(logrus.Fields{
					"destination":    decoded.Destination,
					"amountSat":      decoded.NumSatoshis,
					"activeChannels": len(channels.Channels),
					"channelBalance": balance.Balance,
					"routeHints":     decoded.RouteHints,
				}).WithError(origErr).Error("Could not pay invoice")

			}()
			_ = c.Error(err)
			return
		}

		c.JSONP(http.StatusOK, t)
	}
}
