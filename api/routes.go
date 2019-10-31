package api

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin/binding"
	uuid "github.com/satori/go.uuid"
	"gitlab.com/arcanecrypto/teslacoil/api/apiauth"
	"gitlab.com/arcanecrypto/teslacoil/api/apitxs"
	"gitlab.com/arcanecrypto/teslacoil/api/apiusers"
	"gitlab.com/arcanecrypto/teslacoil/api/validation"
	"gitlab.com/arcanecrypto/teslacoil/ln"
	"gitlab.com/arcanecrypto/teslacoil/models/apikeys"
	"gitlab.com/arcanecrypto/teslacoil/models/transactions"
	"gitlab.com/arcanecrypto/teslacoil/models/users"
	"gopkg.in/go-playground/validator.v8"

	"gitlab.com/arcanecrypto/teslacoil/api/apierr"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	"gitlab.com/arcanecrypto/teslacoil/api/auth"
	"gitlab.com/arcanecrypto/teslacoil/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/email"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/build"
)

var log = build.Log

// Config is the configuration for our API. Currently it just sets the
// log level.
type Config struct {
	// LogLevel specifies which level our application is going to log to
	LogLevel logrus.Level
	// The Bitcoin blockchain network we're on
	Network chaincfg.Params
}

// RestServer is the rest server for our app. It includes a Router,
// a JWT middleware a db connection, and a grpc connection to lnd
type RestServer struct {
	Router      *gin.Engine
	db          *db.DB
	lncli       lnrpc.LightningClient
	bitcoind    bitcoind.TeslacoilBitcoind
	EmailSender email.Sender
}

var corsConfig = cors.Config{
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
func NewApp(db *db.DB, lncli lnrpc.LightningClient, sender email.Sender,
	bitcoin bitcoind.TeslacoilBitcoind, callbacks transactions.HttpPoster,
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

	go transactions.TxListener(db, bitcoin.ZmqTxChannel(), config.Network)
	go transactions.BlockListener(db, bitcoin.Btcctl(), bitcoin.ZmqBlockChannel())

	invoiceUpdatesCh := make(chan *lnrpc.Invoice)
	// Start a goroutine for getting notified of newly added/settled invoices.
	go ln.ListenInvoices(lncli, invoiceUpdatesCh)

	// Start a goroutine for handling the newly added/settled invoices.
	go transactions.InvoiceStatusListener(invoiceUpdatesCh, db, callbacks)

	r := RestServer{
		Router:      g,
		db:          db,
		lncli:       lncli,
		bitcoind:    bitcoin,
		EmailSender: sender,
	}

	// Ping handler
	r.Router.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	r.Router.NoRoute(func(c *gin.Context) {
		apierr.Public(c, http.StatusNotFound, apierr.ErrRouteNotFound)
	})

	middleware := auth.GetMiddleware(r.db)

	r.registerApiKeyRoutes()
	r.registerAdminRoutes()

	apitxs.RegisterRoutes(r.Router, r.db, r.lncli, r.bitcoind, middleware)
	apiusers.RegisterRoutes(r.Router, r.db, sender, middleware)
	apiauth.RegisterRoutes(r.Router, r.db, sender, middleware)

	return r, nil
}

// RegisterAdminRoutes registers routes related to administration of Teslacoil
// TODO: secure these routes with access control
func (r *RestServer) registerAdminRoutes() {
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

		migrationStatus, err := r.db.MigrationStatus(path.Join("file://", db.MigrationsPath))
		if err != nil {
			_ = c.Error(err)
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"network":                 chainInfo.Chain,
			"bestBlockHash":           chainInfo.BestBlockHash,
			"blockCount":              chainInfo.Blocks,
			"lnPeers":                 lndInfo.NumPeers,
			"bitcoindBalanceSats":     bitcoindBalance.ToUnit(btcutil.AmountSatoshi),
			"lndWalletBalanceSats":    lndWalletBalance.TotalBalance,
			"lndChannelBalanceSats":   lndChannelBalance.Balance,
			"activeChannels":          lndInfo.NumActiveChannels,
			"pendingChannels":         lndInfo.NumPendingChannels,
			"inactiveChannels":        lndInfo.NumInactiveChannels,
			"databaseMigrationStatus": migrationStatus,
		})

	}

	r.Router.GET("/info", getInfo)
}

func (r *RestServer) registerApiKeyRoutes() {
	keys := r.Router.Group("")
	keys.Use(auth.GetMiddleware(r.db))
	keys.POST("apikey", r.createApiKey())

}
func (r *RestServer) createApiKey() gin.HandlerFunc {
	type createApiKeyResponse struct {
		Key    uuid.UUID `json:"key"`
		UserID int       `json:"userId"`
	}

	return func(c *gin.Context) {
		userID, ok := auth.GetUserIdOrReject(c)
		if !ok {
			return
		}

		user, err := users.GetByID(r.db, userID)
		if err != nil {
			log.WithError(err).WithField("user", userID).Error("could not get user")
			_ = c.Error(err)
			return
		}

		rawKey, key, err := apikeys.New(r.db, user.ID)
		if err != nil {
			log.WithError(err).WithField("user", userID).Error("could not create API key")
			_ = c.Error(err)
			return
		}

		c.JSON(http.StatusCreated, createApiKeyResponse{
			Key:    rawKey,
			UserID: key.UserID,
		})
	}
}
