package actions

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/gin-gonic/gin"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"gitlab.com/arcanecrypto/teslacoil/api"
	"gitlab.com/arcanecrypto/teslacoil/api/auth"
	"gitlab.com/arcanecrypto/teslacoil/async"
	"gitlab.com/arcanecrypto/teslacoil/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/cmd/tlc/flags"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/dummy"
	"gitlab.com/arcanecrypto/teslacoil/email"
	"gitlab.com/arcanecrypto/teslacoil/ln"
)

const (
	rpcAwaitAttempts = 5
	rpcAwaitDuration = time.Second
)

// awaitBitcoind tries to get a RPC response from bitcoind, returning an error
// if that isn't possible within a set of attempts
func awaitBitcoind(btc *bitcoind.Conn) error {
	retry := func() bool {
		_, err := btc.Btcctl().GetBlockChainInfo()
		if err != nil {
			wrapped := fmt.Errorf("awaitBitcoind: %w", err)
			log.WithError(wrapped).Debug("getblockchaininfo failed")
		}
		return err == nil
	}
	return async.Await(rpcAwaitAttempts, rpcAwaitDuration, retry, "couldn't reach bitcoind")
}

// awaitLnd tries to get a RPC response from lnd, returning an error
// if that isn't possible within a set of attempts
func awaitLnd(lncli lnrpc.LightningClient) error {
	retry := func() bool {
		_, err := lncli.GetInfo(context.Background(), &lnrpc.GetInfoRequest{})
		return err == nil
	}
	return async.Await(rpcAwaitAttempts, rpcAwaitDuration, retry, "couldn't reach lnd")
}

// awaitLndMacaroonFile waits for the creation of the macaroon file in the given
// configuration
func awaitLndMacaroonFile(config ln.LightningConfig) error {
	macaroon := config.MacaroonPath
	if macaroon == "" {
		macaroon = path.Join(config.LndDir,
			ln.DefaultRelativeMacaroonPath(config.Network))
	}
	retry := func() bool {
		_, err := os.Stat(macaroon)
		return err == nil
	}
	return async.Await(rpcAwaitAttempts, rpcAwaitDuration,
		retry, fmt.Sprintf("couldn't read macaroon file %q", macaroon))
}

type realHttpSender struct{}

func (s realHttpSender) Post(url, contentType string, reader io.Reader) (*http.Response, error) {
	return http.Post(url, contentType, reader)
}

func Serve() cli.Command {
	serve := cli.Command{
		Name:  "serve",
		Usage: "Starts the lightning payment processing api",
		Before: func(c *cli.Context) error {
			jwtPrivateKeyPath := c.String("rsa-jwt-key")
			if jwtPrivateKeyPath == "" {
				return errors.New("no RSA JWT key given")
			}

			jwtPrivateKeyBytes, err := ioutil.ReadFile(jwtPrivateKeyPath)
			if err != nil {
				return fmt.Errorf("could not read RSA JWT key: %w", err)
			}

			jwtPrivateKeyPass := c.String("rsa-jwt-key-pass")
			if jwtPrivateKeyPass == "" {
				log.Warn("No RSA JWT key password given")
			}

			if err := auth.SetRawJwtPrivateKey(jwtPrivateKeyBytes, []byte(jwtPrivateKeyPass)); err != nil {
				return err
			}
			log.Info("Set JWT signing key")
			return nil
		},
		Action: func(c *cli.Context) error {

			lnConfig, err := flags.ReadLnConf(c)
			if err != nil {
				return err
			}

			bitcoindConfig, err := flags.ReadBitcoindConf(c)
			if err != nil {
				return err
			}

			bitcoindConn, err := bitcoind.NewConn(bitcoindConfig, 1*time.Second)
			if err != nil {
				return err
			}

			log.WithFields(logrus.Fields{
				"network": bitcoindConfig.Network.Name,
				"host":    bitcoindConfig.RpcHost,
				"port":    bitcoindConfig.RpcPort,
			}).Info("Opened connection to bitcoind")

			dbConf := flags.ReadDbConf(c)
			database, err := db.Open(dbConf)
			if err != nil {
				return err
			}
			defer func() { err = database.Close() }()

			// we do a DB status check here, to verify that we can connect
			// to the DB. otherwise errors there won't get picked up until later
			status, err := database.MigrationStatus()
			if err != nil {
				return fmt.Errorf("could not query DB migration status: %w", err)
			}
			if c.Bool("db.migrateup") {
				if !status.Dirty {
					log.Debug("No migrations needed")
				} else if err := database.MigrateUp(); err != nil {
					return err
				}
			}

			if err := awaitBitcoind(bitcoindConn); err != nil {
				return err
			}
			log.Info("bitcoind is properly started")

			if err := awaitLndMacaroonFile(lnConfig); err != nil {
				return err
			}

			lncli, err := ln.NewLNDClient(lnConfig)
			if err != nil {
				return err
			}
			if err := awaitLnd(lncli); err != nil {
				return err
			}
			log.Info("lnd is properly started")

			config := api.Config{
				LogLevel: build.Log.Level,
				Network:  bitcoindConfig.Network,
			}

			var baseUrl string
			isRelease := os.Getenv(gin.EnvGinMode) == gin.ReleaseMode
			switch {
			case bitcoindConfig.Network.Name == chaincfg.MainNetParams.Name && isRelease:
				baseUrl = "https://teslacoil.io"
			case bitcoindConfig.Network.Name == chaincfg.TestNet3Params.Name && isRelease:
				baseUrl = "https://testnet.teslacoil.io"
			default:
				// not in relase mode, assume we're running locally
				baseUrl = "http://127.0.0.1:3000"
			}

			emailApiKey := c.String("sendgrid.api-key")
			emailSender := email.NewSendGridSender(baseUrl, emailApiKey)

			a, err := api.NewApp(database, lncli, emailSender,
				bitcoindConn, realHttpSender{}, config)
			if err != nil {
				return err
			}

			if c.Bool("dummy.gen-data") {
				if bitcoindConfig.Network.Name == chaincfg.RegressionNetParams.Name {
					force := c.Bool("dummy.force")
					if !force {
						fmt.Println("Are you sure you want to fill dummy data? y/n")
						if !askForConfirmation() {
							log.Info("Not populating DB with dummy data")
							return nil
						}
					}

					err := dummy.FillWithData(database, lncli, c.Bool("dummy.only-once"))
					if err != nil {
						return err
					}
				} else {
					log.Warn("dummy.gen-data flag is set, but network is not regtest")
				}
			}

			address := fmt.Sprintf(":%d", c.Int("port"))
			if os.Getenv(gin.EnvGinMode) == gin.ReleaseMode {
				// Set up to run TLS on lightningspin using certs
				// generated by certbot
				err = a.Router.RunTLS(address,
					c.String("tls-cert-file"),
					c.String("tls-key-file"))
			} else {
				err = a.Router.Run(address)
			}

			return err
		},
	}

	baseFlags := []cli.Flag{
		cli.IntFlag{
			Name:  "port",
			Value: 5000,
			Usage: "Port number to listen on",
		},

		// dummy data generation
		cli.BoolFlag{
			Name:  "dummy.gen-data",
			Usage: "If the Db should be populated with dummy data. Only happens if network is regtest",
		},
		cli.BoolFlag{
			Name:  "dummy.force",
			Usage: "Whether or not to ask for confirmation before populating with dummy data",
		},
		cli.BoolFlag{
			Name:  "dummy.only-once",
			Usage: "Only fill with dummy data if DB is empty",
		},

		// security keys
		cli.StringFlag{
			Name:      "rsa-jwt-key",
			EnvVar:    "TESLACOIL_RSA_JWT_KEY",
			Usage:     "File path to PEM encoded RSA private key used for signing JWTs",
			TakesFile: true,
			Required:  true,
		},
		cli.StringFlag{
			Name:   "rsa-jwt-key-pass",
			EnvVar: "TESLACOIL_RSA_JWT_KEY_PASS",
			Usage:  "The password used to decrypt the RSA private key used for signing JWTs",
		},
		cli.StringFlag{
			Name:      "tls-cert-file",
			EnvVar:    "TESLACOIL_TLS_CERT_FILE",
			Usage:     "Path to TLS cert file",
			TakesFile: true,
			Required:  os.Getenv(gin.EnvGinMode) == gin.ReleaseMode,
		},
		cli.StringFlag{
			Name:     "tls-key-file",
			EnvVar:   "TESLACOIL_TLS_KEY_FILE",
			Usage:    "Path to TLS key file",
			Required: os.Getenv(gin.EnvGinMode) == gin.ReleaseMode,
		},

		// API keys
		cli.StringFlag{
			Name:     "sendgrid.api-key",
			Usage:    "API key used to connect with Sendgrid",
			EnvVar:   "SENDGRID_API_KEY",
			Required: true,
		},
	}

	serve.Flags = flags.Concat(baseFlags, flags.Bitcoind, flags.Db, flags.Lnd)
	return serve
}
