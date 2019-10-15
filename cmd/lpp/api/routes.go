package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	"gitlab.com/arcanecrypto/teslacoil/internal/apierr"
	"gitlab.com/arcanecrypto/teslacoil/internal/transactions"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/pkg/errors"
	"github.com/sendgrid/rest"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/internal/auth"
	"gitlab.com/arcanecrypto/teslacoil/validation"
	"gopkg.in/go-playground/validator.v8"

	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/internal/payments"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/ln"
)

// Config is the configuration for our API. Currently it just sets the
// log level.
type Config struct {
	// LogLevel specifies which level our application is going to log to
	LogLevel logrus.Level
	// The Bitcoin blockchain network we're on
	Network chaincfg.Params
}

type EmailSender interface {
	Send(email *mail.SGMailV3) (*rest.Response, error)
}

// RestServer is the rest server for our app. It includes a Router,
// a JWT middleware a db connection, and a grpc connection to lnd
type RestServer struct {
	Router      *gin.Engine
	db          *db.DB
	lncli       lnrpc.LightningClient
	bitcoind    bitcoind.TeslacoilBitcoind
	EmailSender EmailSender
}

func getCorsConfig() cors.Config {
	return cors.Config{
		AllowOrigins: []string{"https://teslacoil.io", "http://127.0.0.1:3000", "https://testnet.teslacoil.io"},
		AllowMethods: []string{
			http.MethodPut, http.MethodGet,
			http.MethodPost, http.MethodPatch,
			http.MethodDelete,
		},
		AllowHeaders: []string{
			"Accept", "Access-Control-Allow-Origin", "Content-Type", "Referer",
			"Authorization"},
	}
}

// getGinEngine creates a new Gin engine, and applies middlewares used by
// our API. This includes recovering from panics, logging with Logrus and
// applying CORS configuration.
func getGinEngine(config Config) *gin.Engine {
	engine := gin.New()

	log.Debug("Applying gin.Recovery middleware")
	engine.Use(gin.Recovery())

	log.Debug("Applying Gin logging middleware")
	engine.Use(build.GinLoggingMiddleWare(log,
		// TODO should we have a custom field for request logging in our config?
		config.LogLevel))

	log.Debug("Applying CORS middleware")
	corsConfig := getCorsConfig()
	engine.Use(cors.New(corsConfig))

	log.Debug("Applying error handler middleware")
	engine.Use(apierr.GetMiddleware(log))
	return engine
}

func checkBitcoindConnection(conn bitcoind.RpcClient, expected chaincfg.Params) error {

	info, err := conn.GetBlockChainInfo()
	if err != nil {
		return errors.Wrap(err, "could not get bitcoind info")
	}
	if !strings.HasPrefix(expected.Name, info.Chain) {
		return errors.Wrap(err, fmt.Sprintf("app (%s) and bitcoind (%s) are on different networks",
			expected.Name, info.Chain))
	}
	return nil
}

func checkLndConnection(lncli lnrpc.LightningClient, expected chaincfg.Params) error {
	info, err := lncli.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
	if err != nil {
		return errors.Wrap(err, "could not get lnd info:")
	}

	ok := false
	for _, chain := range info.Chains {
		if chain.Chain == "bitcoin" && strings.HasPrefix(expected.Name, chain.Network) {
			ok = true
		}
	}
	if !ok {
		return fmt.Errorf("app (%s) and lnd (%+v) are on different networks", expected.Name, info.Chains)
	}
	return nil
}

//NewApp creates a new app
func NewApp(db *db.DB, lncli lnrpc.LightningClient, sender EmailSender,
	bitcoin bitcoind.TeslacoilBitcoind, callbacks payments.HttpPoster,
	config Config) (RestServer, error) {
	build.SetLogLevel(config.LogLevel)

	if config.Network.Name == "" {
		return RestServer{}, errors.New("config.Network is not set")
	}

	g := getGinEngine(config)

	engine, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		return RestServer{}, fmt.Errorf(
			"gin validator engine (%s) was not validator.Validate",
			binding.Validator.Engine(),
		)
	}
	validators := validation.RegisterAllValidators(engine, config.Network)
	log.Infof("Registered custom validators: %s", validators)

	log.Info("Checking bitcoind connection")
	if err := checkBitcoindConnection(bitcoin.Btcctl(), config.Network); err != nil {
		return RestServer{}, err
	}
	log.Info("Checked bitcoind connection succesfully")

	if err := checkLndConnection(lncli, config.Network); err != nil {
		return RestServer{}, err
	}

	// Start two goroutines for listening to zmq events
	bitcoin.StartZmq()

	go transactions.TxListener(db, lncli, bitcoin.ZmqTxChannel(), config.Network)
	go transactions.BlockListener(db, bitcoin.Btcctl(), bitcoin.ZmqBlockChannel())

	invoiceUpdatesCh := make(chan *lnrpc.Invoice)
	// Start a goroutine for getting notified of newly added/settled invoices.
	go ln.ListenInvoices(lncli, invoiceUpdatesCh)
	// Start a goroutine for handling the newly added/settled invoices.

	go payments.InvoiceStatusListener(invoiceUpdatesCh, db, callbacks)

	r := RestServer{
		Router:      g,
		db:          db,
		lncli:       lncli,
		bitcoind:    bitcoin,
		EmailSender: sender,
	}

	// We register /login separately to require jwt-tokens on every other endpoint
	// than /login
	r.Router.POST("/login", r.Login())
	// Ping handler
	r.Router.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	r.Router.NoRoute(func(c *gin.Context) {
		apierr.Public(c, http.StatusNotFound, apierr.ErrRouteNotFound)
	})

	r.RegisterApiKeyRoutes()
	r.RegisterAdminRoutes()
	r.RegisterAuthRoutes()
	r.RegisterUserRoutes()
	r.RegisterPaymentRoutes()
	r.RegisterTransactionRoutes()

	return r, nil
}

// RegisterAdminRoutes registers routes related to administration of Teslacoil
// TODO: secure these routes with access control
func (r *RestServer) RegisterAdminRoutes() {
	getInfo := func(c *gin.Context) {
		chainInfo, err := r.bitcoind.Btcctl().GetBlockChainInfo()
		if err != nil {
			_ = c.Error(err).SetMeta("bitcoind.getblockchaininfo")
			return
		}

		bitcoindBalance, err := r.bitcoind.Btcctl().GetBalance("*")
		if err != nil {
			_ = c.Error(err).SetMeta("bitcoind.getbalance")
			return
		}

		lndWalletBalance, err := r.lncli.WalletBalance(context.Background(), &lnrpc.WalletBalanceRequest{})
		if err != nil {
			_ = c.Error(err).SetMeta("lncli.walletbalance")
			return
		}

		lndChannelBalance, err := r.lncli.ChannelBalance(context.Background(), &lnrpc.ChannelBalanceRequest{})
		if err != nil {
			_ = c.Error(err).SetMeta("lncli.channelbalance")
			return
		}

		lndInfo, err := r.lncli.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
		if err != nil {
			_ = c.Error(err).SetMeta("lncli.getinfo")
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"network":               chainInfo.Chain,
			"bestBlockHash":         chainInfo.BestBlockHash,
			"blockCount":            chainInfo.Blocks,
			"lnPeers":               lndInfo.NumPeers,
			"bitcoindBalanceSats":   bitcoindBalance.ToUnit(btcutil.AmountSatoshi),
			"lndWalletBalanceSats":  lndWalletBalance.TotalBalance,
			"lndChannelBalanceSats": lndChannelBalance.Balance,
			"activeChannels":        lndInfo.NumActiveChannels,
			"pendingChannels":       lndInfo.NumPendingChannels,
			"inactiveChannels":      lndInfo.NumInactiveChannels,
		})

	}

	r.Router.GET("/info", getInfo)
}

// RegisterAuthRoutes registers all auth routes
func (r *RestServer) RegisterAuthRoutes() {
	authGroup := r.Router.Group("auth")

	// Does not need auth token to reset password
	authGroup.PUT("reset_password", r.ResetPassword())
	authGroup.POST("reset_password", r.SendPasswordResetEmail())

	authGroup.Use(auth.GetMiddleware(r.db))

	// 2FA methods
	authGroup.POST("2fa", r.Enable2fa())
	authGroup.PUT("2fa", r.Confirm2fa())
	authGroup.DELETE("2fa", r.Delete2fa())

	authGroup.GET("refresh_token", r.RefreshToken())
	authGroup.PUT("change_password", r.ChangePassword())
}

func (r *RestServer) RegisterApiKeyRoutes() {
	keys := r.Router.Group("")
	keys.Use(auth.GetMiddleware(r.db))
	keys.POST("apikey", r.CreateApiKey())

}

// RegisterUserRoutes registers all user routes on the router
func (r *RestServer) RegisterUserRoutes() {
	// Creating a user doesn't require a JWT
	r.Router.POST("/users", r.CreateUser())

	// We group on empty paths to apply middlewares to everything but the
	// /login route. The group path is empty because it is easier to read
	users := r.Router.Group("")
	users.Use(auth.GetMiddleware(r.db))
	users.GET("/users", r.GetAllUsers())
	users.GET("/user", r.GetUser())
	users.PUT("/user", r.UpdateUser())
}

// RegisterPaymentRoutes registers all payment routes on the router.
// Payment is defined as a lightning transaction, so all things lightning
// can be found in payment packages
func (r *RestServer) RegisterPaymentRoutes() {
	payment := r.Router.Group("")
	payment.Use(auth.GetMiddleware(r.db))

	payment.GET("payments", r.GetAllPayments())
	payment.GET("payment/:id", r.GetPaymentByID())
	payment.POST("/invoices/create", r.CreateInvoice())
	payment.POST("/invoices/pay", r.PayInvoice())
}

// RegisterTransactionRoutes registers all transaction routes on the router.
// A transaction is defined as an on-chain transaction
func (r *RestServer) RegisterTransactionRoutes() {
	transaction := r.Router.Group("")
	transaction.Use(auth.GetMiddleware(r.db))

	transaction.GET("/transactions", r.GetAllTransactions())
	transaction.GET("/transaction/:id", r.GetTransactionByID())
	transaction.POST("/withdraw", r.WithdrawOnChain())
	transaction.POST("/deposit", r.NewDeposit())
}

// getUserIdOrReject retrieves the user ID associated with this request. This
// user ID should be set by the authentication middleware. This means that this
// method can safely be called by all endpoints that use the authentication
// middleware.
func getUserIdOrReject(c *gin.Context) (int, bool) {
	id, exists := c.Get(auth.UserIdVariable)
	if !exists {
		msg := "User ID is not set in request! This is a serious error, and means our authentication middleware did not set the correct variable."
		_ = c.AbortWithError(http.StatusInternalServerError, errors.New(msg))
		return -1, false
	}
	idInt, ok := id.(int)
	if !ok {
		msg := "User ID was not an int! This means our authentication middleware did something bad."
		_ = c.AbortWithError(http.StatusInternalServerError, errors.New(msg))
		return -1, false
	}

	return idInt, true
}
