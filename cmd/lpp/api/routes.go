package api

import (
	"net/http"
	"time"

	"github.com/btcsuite/btcd/wire"
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
}

type EmailSender interface {
	Send(email *mail.SGMailV3) (*rest.Response, error)
}

// RestServer is the rest server for our app. It includes a Router,
// a JWT middleware a db connection, and a grpc connection to lnd
type RestServer struct {
	Router      *gin.Engine
	db          *db.DB
	lncli       *lnrpc.LightningClient
	EmailSender EmailSender
}

func getCorsConfig() cors.Config {
	return cors.Config{
		AllowOrigins: []string{"https://teslacoil.io", "http://127.0.0.1:3000"},
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
	return engine
}

//NewApp creates a new app
func NewApp(d *db.DB, lncli lnrpc.LightningClient, email EmailSender,
	callbacks payments.HttpPoster, config Config) (RestServer, error) {
	build.SetLogLevel(config.LogLevel)

	g := getGinEngine(config)

	engine, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		log.Fatalf("Gin validator engine (%s) was validator.Validate", binding.Validator.Engine())
	}
	validators := validation.RegisterAllValidators(engine)
	log.Infof("Registered custom validators: %s", validators)

	r := RestServer{
		Router:      g,
		db:          d,
		lncli:       &lncli,
		EmailSender: email,
	}

	zmqRawTxCh := make(chan *wire.MsgTx)
	zmqBlockCh := make(chan *wire.MsgBlock)

	// Start a bitcoind conn using the standard testnet params
	// TODO(bo): read these parameters from ~/.bitcoin/bitcoin.conf.
	//  See example in extractBitcoindRPCParams in lnd-codebase
	bitcoindConn, err := bitcoind.NewConn(bitcoind.Config{
		RpcPort:      18332,
		User:         "kek",
		Password:     "kek",
		ZmqBlockHost: "tcp://127.0.0.1:28332",
		ZmqTxHost:    "tcp://127.0.0.1:28333",
	}, 1*time.Second, zmqRawTxCh, zmqBlockCh)
	if err != nil {
		panic(err)
	}
	log.Info("opened connection to bitcoind")

	// Start two goroutines for listening to zmq events
	bitcoindConn.StartZmq()

	go bitcoind.ListenTxs(zmqRawTxCh)
	go bitcoind.ListenBlocks(zmqBlockCh)

	invoiceUpdatesCh := make(chan *lnrpc.Invoice)
	// Start a goroutine for getting notified of newly added/settled invoices.
	go ln.ListenInvoices(lncli, invoiceUpdatesCh)
	// Start a goroutine for handling the newly added/settled invoices.

	go payments.InvoiceStatusListener(invoiceUpdatesCh, d, callbacks)

	// We register /login separately to require jwt-tokens on every other endpoint
	// than /login
	r.Router.POST("/login", r.Login())
	// Ping handler
	r.Router.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	r.Router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Route not found"})
	})

	r.RegisterApiKeyRoutes()
	r.RegisterAuthRoutes()
	r.RegisterUserRoutes()
	r.RegisterPaymentRoutes()
	r.RegisterTransactionRoutes()

	return r, nil
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
		c.JSON(http.StatusInternalServerError, internalServerErrorResponse)
		c.Abort()
		log.Panic("User ID is not set in request! This is a serious error, and means our authentication middleware did not set the correct variable.")
	}
	idInt, ok := id.(int)
	if !ok {
		c.JSON(http.StatusInternalServerError, internalServerErrorResponse)
		c.Abort()
		log.WithField("userID", id).Panic("User ID was not an int! This means our authentication middleware did something bad.")
	}

	return idInt, true
}

// getJSONOrReject extracts fields from the context and inserts
// them into the passed body argument. If an error occurs, the
// error is logged and a response with StatusBadRequest is sent
// body MUST be an address to a variable, not a variable
func getJSONOrReject(c *gin.Context, body interface{}) bool {
	if err := c.ShouldBindJSON(body); err != nil {
		log.Errorf("%s could not bind JSON %+v", c.Request.URL.Path, err)
		c.JSON(http.StatusBadRequest, badRequestResponse)
		return false
	}

	return true
}

func getQueryOrReject(c *gin.Context, body interface{}) bool {
	if err := c.ShouldBindQuery(body); err != nil {
		err = errors.Wrapf(err, "wrong query parameter format, check the documentation")
		log.Error(err)
		c.JSON(http.StatusBadRequest, err.Error())
		return false
	}
	return true
}
