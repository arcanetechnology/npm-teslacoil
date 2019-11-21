// Package apitxs provides HTTP handlers for querying for, creating and settling
// payments in our API.
package apitxs

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

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

var log = build.AddSubLogger("APIT")

// services that gets initiated in RegisterRoutes
var (
	database *db.DB
	lncli    lnrpc.LightningClient
	bitcoin  bitcoind.TeslacoilBitcoind
	sender   transactions.HttpPoster
)

// RegisterRoutes applies the authMiddleware to this packages routes
// and registers routes on the gin Engine parameter
func RegisterRoutes(server *gin.Engine, db *db.DB, lnd lnrpc.LightningClient,
	bitcoind bitcoind.TeslacoilBitcoind, poster transactions.HttpPoster,
	authmiddleware gin.HandlerFunc) {
	// assign the services given
	database = db
	lncli = lnd
	bitcoin = bitcoind
	sender = poster

	transaction := server.Group("")

	transaction.Use(authmiddleware)

	// common
	transaction.GET("/transactions", getAllTransactions())
	transaction.GET("/transactions/:id", getTransactionByID())

	// onchain transactions
	transaction.POST("/withdraw", withdrawOnChain())
	transaction.POST("/deposit", newDeposit())

	// offchain transactions
	transaction.POST("/invoices/create", createInvoice())
	transaction.POST("/invoices/pay", payInvoice())
	transaction.GET("/invoices/:paymentrequest", getOffchainByPaymentRequest())
}

// getAllTransactions finds all payments for the given user. Takes two URL
// parameters, `limit` and `offset`
func getAllTransactions() gin.HandlerFunc {
	type Params struct {
		Limit  int        `form:"limit" binding:"gte=0"`
		Offset int        `form:"offset" binding:"gte=0"`
		Max    *int64     `form:"max" binding:"omitempty"` // Sats
		Min    *int64     `form:"min" binding:"omitempty"` // Sats
		End    *time.Time `form:"end" binding:"omitempty"`
		Start  *time.Time `form:"start" binding:"omitempty"`

		// looks like it's not possible to use custom types here:  https://github.com/gin-gonic/gin/issues/1152
		Direction *string `form:"direction" binding:"omitempty,oneof=asc ASC desc DESC"`
	}

	type response struct {
		Transactions []transactions.Transaction `json:"transactions"`
		Total        int                        `json:"total"`
		Limit        int                        `json:"limit"`
		Offset       int                        `json:"offset"`
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

		results := make(chan interface{}, 2)
		errs := make(chan error, 2)
		var wg sync.WaitGroup

		txParams := transactions.GetAllParams{
			Offset: params.Offset,
			Limit:  params.Limit,
			End:    params.End,
			Start:  params.Start,
		}
		if params.Min != nil {
			i := *params.Min * 1000 // sats to msats conversion
			txParams.MinMilliSats = &i
		}
		if params.Max != nil {
			i := *params.Max * 1000 // sats to msats conversion
			txParams.MaxMilliSats = &i
		}
		if params.Direction != nil {
			switch strings.ToLower(*params.Direction) {
			case "asc":
				txParams.SortDirection = transactions.SortAscending
			case "desc":
				txParams.SortDirection = transactions.SortDescending
			default:
				// should never reach this thanks to binding check above, `oneof`
				log.WithField("direction", params.Direction).Error("Reached unreachable point")
				_ = c.Error(fmt.Errorf("bad sorting direction: %s", *params.Direction))
				return
			}
		}

		// fetch transactions
		wg.Add(1)
		go func() {
			defer wg.Done()

			txs, err := transactions.GetAllTransactions(database, userID, txParams)
			if err != nil {
				log.WithError(err).WithFields(logrus.Fields{
					"limit":  params.Limit,
					"offset": params.Offset,
					"userID": userID,
				}).Error("could not get transactions")
				errs <- err
				return
			}
			results <- txs
		}()

		// fetch the count
		wg.Add(1)
		go func() {
			defer wg.Done()

			count, err := transactions.CountForUser(database, userID)
			if err != nil {
				log.WithError(err).WithField("userId", userID).
					Error("Could not get TX count for user")
				errs <- err
				return
			}
			results <- count
		}()

		// close the channels
		go func() {
			wg.Wait()
			close(results)
			close(errs)
		}()

		for err := range errs {
			_ = c.Error(err)
			return
		}

		var txs []transactions.Transaction
		var total int
		for res := range results {
			switch r := res.(type) {
			case []transactions.Transaction:
				txs = r
			case int:
				total = r
			default:
				_ = c.Error(fmt.Errorf("unexpected result type when fetching TXs: %v", r))
				return
			}
		}

		c.JSONP(http.StatusOK, response{
			Transactions: txs,
			Total:        total,
			Limit:        params.Limit,
			Offset:       params.Offset,
		})
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

		// Return the transaction when it is found and no errors where encountered
		c.JSONP(http.StatusOK, t)
	}
}

// withdrawOnChain is a request handler used for withdrawing funds
// to an on-chain address
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
		t, err := transactions.CreateTeslacoilInvoice(
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

		c.JSON(http.StatusOK, t)
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

		t, err := transactions.PayInvoiceWithDescription(database, lncli, sender,
			userID, req.PaymentRequest, req.Description)
		if err != nil {
			if errors.Is(err, transactions.ErrCannotPayOwnInvoice) {
				apierr.Public(c, http.StatusBadRequest, apierr.ErrCannotPayOwnInvoice)
				return
			}
			_ = c.Error(err)
			return
		}

		c.JSON(http.StatusOK, t)
	}
}

// getOffchainByPaymentRequest takes in a paymentrequest path parameter,
// and fetches that from the DB
func getOffchainByPaymentRequest() gin.HandlerFunc {
	type request struct {
		PaymentRequest string `uri:"paymentrequest" binding:"required,paymentrequest"`
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

		t, err := transactions.GetOffchainByPaymentRequest(database, req.PaymentRequest, userID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				apierr.Public(c, http.StatusNotFound, apierr.ErrTransactionNotFound)
				return
			}
			_ = c.Error(err)
			return
		}

		c.JSONP(http.StatusOK, t)
	}
}
