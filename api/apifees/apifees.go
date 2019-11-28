package apifees

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	"github.com/gin-gonic/gin"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/zpay32"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/api/apierr"
	"gitlab.com/arcanecrypto/teslacoil/bitcoind"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var log = build.AddSubLogger("APIF") // "API fees" logger

// services that gets initiated in RegisterRoutes
var (
	mu    sync.Mutex // guards lncli
	lncli lnrpc.LightningClient

	bitcoin bitcoind.TeslacoilBitcoind

	activeNetwork *chaincfg.Params
)

// SetLnd lets you change the LND client used for handling HTTP requests
func SetLnd(lnd lnrpc.LightningClient) {
	mu.Lock()
	defer mu.Unlock()
	lncli = lnd
}

func RegisterRoutes(server *gin.Engine, lnd lnrpc.LightningClient, bitcoind bitcoind.TeslacoilBitcoind, network *chaincfg.Params) {
	// assign services
	lncli = lnd
	bitcoin = bitcoind
	activeNetwork = network

	fees := server.Group("/fees")
	fees.GET("", blockChainFees())
	fees.GET("/:paymentRequest", lightningFees())
}

const averageTransactionSize = 250 // bytes
const defaultFeeSatsPerByte = 5    // this is most likely only going to be used on test/regtest

func blockChainFees() gin.HandlerFunc {
	type params struct {
		// TODO add complementary string enum option
		Target *int64 `form:"target" binding:"omitempty,gte=1"` // target block time
	}

	type response struct {
		SatsPerByte        float64 `json:"satsPerByte"`
		AverageTransaction float64 `json:"averageTransaction"` // satoshis paid in fees for an average transaction
	}

	return func(c *gin.Context) {
		var p params
		if c.BindQuery(&p) != nil {
			return
		}

		var target int64 = 6 // default to 6 blocks confirmation time
		if p.Target != nil {
			target = *p.Target
		}

		fees, err := bitcoin.Btcctl().EstimateSmartFee(target, nil)
		if err != nil {
			_ = c.Error(err)
			return
		}

		var isNotOnMainnet = activeNetwork.Name == chaincfg.TestNet3Params.Name ||
			activeNetwork.Name == chaincfg.RegressionNetParams.Name
		if fees.Errors != nil || fees.FeeRate == nil {
			var level = logrus.WarnLevel

			// this is not an issue if we're on testnet or regtest,
			// as getting fee data here typically wont work
			if isNotOnMainnet {
				level = logrus.DebugLevel
			}

			log.WithFields(logrus.Fields{
				"feeErrors": fees.Errors,
				"feeRate":   fees.FeeRate,
				"target":    target,
				"network":   activeNetwork.Name,
			}).Log(level, "Got error response when querying for onchain fees")

			if isNotOnMainnet {
				c.JSON(http.StatusOK, response{
					SatsPerByte:        defaultFeeSatsPerByte,
					AverageTransaction: defaultFeeSatsPerByte * averageTransactionSize,
				})
				return
			}
			_ = c.Error(fmt.Errorf("could not estimate onchain fee: %s", strings.Join(fees.Errors, ", ")))
			return
		}

		log.WithFields(logrus.Fields{
			"feeRate": *fees.FeeRate,
			"blocks":  fees.Blocks,
			"target":  target,
		}).Debug("Got fee rates from bitcoind")

		feePerKb, err := btcutil.NewAmount(*fees.FeeRate)
		if err != nil {
			_ = c.Error(err)
			return
		}

		satsPerByte := feePerKb.ToUnit(btcutil.AmountSatoshi) / 1000 // from per KB to per byte
		c.JSON(http.StatusOK, response{
			SatsPerByte:        satsPerByte,
			AverageTransaction: math.Ceil(satsPerByte * averageTransactionSize),
		})
	}
}

func lightningFees() gin.HandlerFunc {
	const RouteNotFoundErr = "unable to find a path to destination"

	type params struct {
		PayReq string `uri:"paymentRequest" binding:"paymentrequest,required"`
	}

	type response struct {
		MilliSats int64   `json:"milliSatoshis"`
		Sats      float64 `json:"satoshis"`
	}

	return func(c *gin.Context) {
		var p params
		if c.BindUri(&p) != nil {
			return
		}

		decoded, err := zpay32.Decode(p.PayReq, activeNetwork)
		if err != nil {
			log.WithError(err).Error("Could not decode payment request, even though it should have been validated previously")
			_ = c.Error(err)
			return
		}
		serializedPubKey := decoded.Destination.SerializeCompressed()
		routesResponse, err := lncli.QueryRoutes(context.Background(), &lnrpc.QueryRoutesRequest{
			PubKey: hex.EncodeToString(serializedPubKey),
			Amt:    int64(decoded.MilliSat.ToSatoshis().ToUnit(btcutil.AmountSatoshi)),
		})

		grpcErr, grpcErrOk := status.FromError(err)
		switch {
		// confusingly it's possible to construct a GRPC error from nil, where we end up with the code set to OK
		// so we have to check that we're not dealing with an OK error...
		case grpcErr != nil && grpcErr.Code() != codes.OK && grpcErrOk:
			log.WithFields(logrus.Fields{
				"message": grpcErr.Message(),
				"details": grpcErr.Details(),
				"code":    grpcErr.Code(),
			}).Error("Could not query for LN route")
			if grpcErr.Message() == RouteNotFoundErr {
				// TODO is this the best HTTP code?
				apierr.Public(c, http.StatusNotFound, apierr.ErrLnRouteNotFound)
				return
			}
			_ = c.Error(grpcErr.Err())
			return
		case err != nil:
			log.WithError(err).Error("Could not query for LN route")
			_ = c.Error(err)
			return
		}

		// for outputting nicely formatted JSON about found routes to the logs
		var jsonRoutes []string
		var minFeeMilliSats int64 = math.MaxInt64
		var hops int
		for _, route := range routesResponse.Routes {
			log.WithFields(logrus.Fields{
				"hops":              len(route.Hops),
				"totalFeeMilliSats": route.TotalFeesMsat,
			}).Debug("Found route for payment request")

			if route.TotalFeesMsat < minFeeMilliSats {
				minFeeMilliSats = route.TotalFeesMsat
				hops = len(route.Hops)
			}

			marshalled, err := json.Marshal(route)
			if err != nil {
				_ = c.Error(err)
				return
			}
			jsonRoutes = append(jsonRoutes, string(marshalled))
		}

		log.WithFields(logrus.Fields{
			"paymentRequest":  p.PayReq,
			"routes":          jsonRoutes,
			"minFeeMilliSats": minFeeMilliSats,
			"hops":            hops,
		}).Debug("Found LN routes")

		c.JSON(http.StatusOK, response{
			MilliSats: minFeeMilliSats,
			Sats:      float64(minFeeMilliSats / 1000),
		})
	}
}
