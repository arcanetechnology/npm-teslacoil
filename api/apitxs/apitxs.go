// Package apitxs provides HTTP handlers for querying for, creating and settling
// payments in our API.
package apitxs

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/api/apierr"
	"gitlab.com/arcanecrypto/teslacoil/api/auth"
	"gitlab.com/arcanecrypto/teslacoil/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/models/transactions"
)

var log = build.Log

// services that gets initiated in RegisterRoutes
var (
	database *db.DB
	lncli    lnrpc.LightningClient
	bitcoin  bitcoind.TeslacoilBitcoind
)

func RegisterRoutes(server *gin.Engine, db *db.DB, lnd lnrpc.LightningClient,
	bitcoind bitcoind.TeslacoilBitcoind, authmiddleware gin.HandlerFunc) {
	// assign the services given
	database = db
	lncli = lnd
	bitcoin = bitcoind

	transaction := server.Group("")

	transaction.Use(authmiddleware)

	// common
	transaction.GET("/transactions", getAllTransactions())
	transaction.GET("/transaction/:id", getTransactionByID())

	// onchain transactions
	transaction.POST("/withdraw", withdrawOnChain())
	transaction.POST("/deposit", newDeposit())

	// offchain transactions
	transaction.POST("/invoices/create", createInvoice())
	transaction.POST("/invoices/pay", payInvoice())
}

// getAllTransactions finds all payments for the given user. Takes two URL
// parameters, `limit` and `offset`
func getAllTransactions() gin.HandlerFunc {
	type Params struct {
		Limit  int `form:"limit" binding:"gte=0"`
		Offset int `form:"offset" binding:"gte=0"`
	}

	return func(c *gin.Context) {
		userID, ok := auth.RequireScope(c, auth.ReadWallet)
		if !ok {
			return
		}

		var params Params
		if c.BindQuery(&params) != nil {
			return
		}

		var t []transactions.Transaction
		var err error
		if params.Limit == 0 && params.Offset == 0 {
			t, err = transactions.GetAllTransactions(database, userID)
		} else if params.Limit == 0 {
			t, err = transactions.GetAllTransactionsOffset(database, userID, params.Offset)
		} else {
			t, err = transactions.GetAllTransactionsLimitOffset(database, userID, params.Limit, params.Offset)
		}
		if err != nil {
			log.WithError(err).Error("Couldn't get transactions")
			_ = c.Error(err)
			return
		}

		c.JSONP(http.StatusOK, t)
	}
}

// getTransactionByID takes in a transaction ID path parameter, and fetches that from the DB
func getTransactionByID() gin.HandlerFunc {
	type request struct {
		ID int `uri:"id" binding:"required"`
	}
	return func(c *gin.Context) {
		userID, ok := auth.RequireScope(c, auth.ReadWallet)
		if !ok {
			return
		}

		var req request
		if c.BindUri(&req) != nil {
			return
		}

		t, err := transactions.GetTransactionByID(database, req.ID, userID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				apierr.Public(c, http.StatusNotFound, apierr.ErrTransactionNotFound)
				return
			}
			_ = c.Error(err)
			return
		}

		// Return the user when it is found and no errors where encountered
		c.JSONP(http.StatusOK, t)
	}
}

// withdrawOnChain is a request handler used for withdrawing funds
// to an on-chain address
// TODO: verify dust limits
func withdrawOnChain() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := auth.RequireScope(c, auth.SendTransaction)
		if !ok {
			return
		}

		var req transactions.WithdrawOnChainArgs
		if c.BindJSON(&req) != nil {
			return
		}
		// add the userID to send coins from
		req.UserID = userID

		onchain, err := transactions.WithdrawOnChain(database, lncli, bitcoin, req)
		if err != nil {
			if errors.Is(err, transactions.ErrBalanceTooLow) {
				apierr.Public(c, http.StatusBadRequest, apierr.ErrBalanceTooLow)
				return
			}
			log.WithError(err).Errorf("Cannot withdraw onchain")
			_ = c.Error(err)
			return
		}

		c.JSONP(http.StatusOK, onchain)
	}

}

// newDeposit is a request handler used for creating a new deposit
// If successful, response with an address, and the saved description
func newDeposit() gin.HandlerFunc {
	type request struct {
		// Whether to discard the old address and force create a new one
		ForceNewAddress bool `json:"forceNewAddress"`
		// A personal description for the transaction
		Description string `json:"description"`
	}

	return func(c *gin.Context) {
		userID, ok := auth.RequireScope(c, auth.CreateInvoice)
		if !ok {
			return
		}

		var req request
		if c.BindJSON(&req) != nil {
			return
		}

		transaction, err := transactions.GetOrCreateDeposit(database, lncli, userID,
			req.ForceNewAddress, req.Description)
		if err != nil {
			log.WithError(err).Error("Cannot deposit onchain")
			_ = c.Error(err)
			return
		}

		c.JSONP(http.StatusOK, transaction)
	}
}

func createInvoice() gin.HandlerFunc {
	type createInvoiceRequest struct {
		AmountSat   int64  `json:"amountSat" binding:"required,gt=0,lte=4294967"`
		Memo        string `json:"memo" binding:"max=256"`
		Description string `json:"description"`
		CallbackURL string `json:"callbackUrl" binding:"omitempty,url"`
		OrderId     string `json:"orderId" binding:"max=256"`
	}

	return func(c *gin.Context) {

		userID, ok := auth.RequireScope(c, auth.CreateInvoice)

		if !ok {
			return
		}

		var req createInvoiceRequest
		if c.BindJSON(&req) != nil {
			return
		}

		// TODO: rename this function to something like `NewLightningPayment` or `NewLightningInvoice`
		t, err := transactions.NewOffchain(
			database, lncli, transactions.NewOffchainOpts{
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

func payInvoice() gin.HandlerFunc {
	type payInvoiceRequest struct {
		PaymentRequest string `json:"paymentRequest" binding:"required,paymentrequest"`
		Description    string `json:"description"`
	}

	return func(c *gin.Context) {
		userID, ok := auth.RequireScope(c, auth.SendTransaction)
		if !ok {
			return
		}

		var req payInvoiceRequest
		if c.BindJSON(&req) != nil {
			return
		}

		t, err := transactions.PayInvoiceWithDescription(database, lncli, userID,
			req.PaymentRequest, req.Description)
		if err != nil {
			// investigate details around failure
			go func() {
				origErr := err
				decoded, err := lncli.DecodePayReq(context.Background(), &lnrpc.PayReqString{
					PayReq: req.PaymentRequest,
				})
				if err != nil {
					log.WithError(err).Error("Could not decode payment request")
					return
				}

				channels, err := lncli.ListChannels(context.Background(), &lnrpc.ListChannelsRequest{
					ActiveOnly: true,
				})
				if err != nil {
					log.WithError(err).Error("Could not list active channels")
					return
				}

				balance, err := lncli.ChannelBalance(context.Background(), &lnrpc.ChannelBalanceRequest{})
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
