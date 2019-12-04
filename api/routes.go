package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/api/apiauth"
	"gitlab.com/arcanecrypto/teslacoil/api/apifees"
	"gitlab.com/arcanecrypto/teslacoil/api/apikeyroutes"
	"gitlab.com/arcanecrypto/teslacoil/api/apitxs"
	"gitlab.com/arcanecrypto/teslacoil/api/apiusers"
	"gitlab.com/arcanecrypto/teslacoil/api/auth"
	"gitlab.com/arcanecrypto/teslacoil/async"
	"gitlab.com/arcanecrypto/teslacoil/build"

	"github.com/gin-gonic/gin/binding"
	"gitlab.com/arcanecrypto/teslacoil/api/validation"
	"gitlab.com/arcanecrypto/teslacoil/ln"
	"gitlab.com/arcanecrypto/teslacoil/models/transactions"
	"gopkg.in/go-playground/validator.v9"

	"gitlab.com/arcanecrypto/teslacoil/api/apierr"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	"gitlab.com/arcanecrypto/teslacoil/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/email"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/pkg/errors"
)

var log = build.AddSubLogger("APIM")

// Config is the configuration for our API. Currently it just sets the
// log level.
type Config struct {
	// The Bitcoin blockchain network we're on
	Network  chaincfg.Params
	LnConfig *ln.LightningConfig // If set, we try to reconnect to LND on connection loss

	// Which level HTTP requests are logged on. Defaults to info.
	HttpLogLevel logrus.Level

	WhitelistedDomains []string
	WhitelistedEmails  []string
}

// RestServer is the rest server for our app. It includes a Router,
// a JWT middleware a db connection, and a grpc connection to lnd
type RestServer struct {
	Router   *gin.Engine
	db       *db.DB
	lncli    lnrpc.LightningClient
	bitcoind bitcoind.TeslacoilBitcoind
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

// we don't log the body of HTTP requests sent to these URLs
var routeBlackList = []string{
	"/login",
	"/auth/reset_password",
	"/users",
}

// getGinEngine creates a new Gin engine, and applies middlewares used by
// our API. This includes recovering from panics, logging with Logrus and
// applying CORS configuration.
func getGinEngine(config Config) *gin.Engine {
	engine := gin.New()

	log.Debug("Applying gin.Recovery middleware")
	engine.Use(gin.Recovery())

	log.Debug("Applying Gin logging middleware")
	engine.Use(build.GinLoggingMiddleWare(log, routeBlackList, config.HttpLogLevel))

	log.Debug("Applying CORS middleware")
	engine.Use(cors.New(corsConfig))

	// write JSON header to all responses
	engine.Use(func(c *gin.Context) {
		c.Header("Content-Type", "application/json")
	})

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

	if config.Network.Name == "" {
		return RestServer{}, errors.New("config.Network is not set")
	}

	// if log level is not set, default to info
	if config.HttpLogLevel == logrus.Level(0) {
		config.HttpLogLevel = logrus.InfoLevel
	}

	g := getGinEngine(config)

	engine, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		return RestServer{}, fmt.Errorf(
			"gin validator engine (%T) was not validator.Validate",
			binding.Validator.Engine(),
		)
	}
	validators := validation.RegisterAllValidators(engine, config.Network)
	log.WithField("validators", validators).Info("Registered custom validators")

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

	r := RestServer{
		Router:   g,
		db:       db,
		lncli:    lncli,
		bitcoind: bitcoin,
	}

	// Ping handler
	r.Router.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	r.Router.NoRoute(func(c *gin.Context) {
		apierr.Public(c, http.StatusNotFound, apierr.ErrRouteNotFound)
	})

	middleware := auth.GetMiddleware(r.db)

	r.registerAdminRoutes()

	apikeyroutes.RegisterRoutes(r.Router, r.db, middleware)
	apitxs.RegisterRoutes(r.Router, r.db, r.lncli, r.bitcoind, callbacks, middleware)
	apiusers.RegisterRoutes(r.Router, r.db, sender, middleware, config.WhitelistedDomains, config.WhitelistedEmails)
	apiauth.RegisterRoutes(r.Router, r.db, sender, middleware)
	apifees.RegisterRoutes(r.Router, lncli, bitcoin, &config.Network)

	r.startListeningToLnd(callbacks, sender, config)

	return r, nil
}

func (r *RestServer) startListeningToLnd(poster transactions.HttpPoster, emailSender email.Sender, config Config) {
	log.Info("Adding LND invoice listener")
	invoiceUpdatesCh := make(chan *lnrpc.Invoice)

	// Start a goroutine for getting notified of newly added/settled invoices.
	go ln.ListenInvoices(r.lncli, invoiceUpdatesCh)

	// Start a goroutine for handling the newly added/settled invoices.
	go transactions.InvoiceListener(invoiceUpdatesCh, r.db, poster)

	// Start a goroutine to handle LND connection issues
	go func() {
		log.Info("Adding LND shutdown listener")
		err := ln.ListenShutdown(r.lncli, func() {
			if config.LnConfig == nil {
				log.Warn("LND quit, not reconnecting")
				return
			}
			log.Info("LND quit, trying to reconnect")
			const attepmts = 10
			const sleep = time.Second * 2
			if err := async.RetryNoBackoff(attepmts, sleep, func() error {
				return r.reconnectLnd(*config.LnConfig)
			}); err != nil {
				log.WithError(err).Error("Could not reconnect to LND")
				return
			}

			log.Info("Got new connection to LND")
			apitxs.SetLnd(r.lncli)
			apifees.SetLnd(r.lncli)
			r.startListeningToLnd(poster, emailSender, config)
		})
		if err != nil {
			log.WithError(err).Error("Could not listen for LND shutdown")
		}
	}()

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

		migrationStatus, err := r.db.MigrationStatus()
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

func (r *RestServer) reconnectLnd(config ln.LightningConfig) error {
	lncli, err := ln.NewLNDClient(config)
	if err != nil {
		return err
	}

	err = checkLndConnection(lncli, config.Network)
	if err != nil {
		return fmt.Errorf("could not verify LND connection when reconnecting: %w", err)
	}

	r.lncli = lncli
	return nil
}
