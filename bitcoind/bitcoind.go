package bitcoind

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/arcanecrypto/teslacoil/async"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/lightninglabs/gozmq"
	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/teslacoil/build"
)

var (
	log = build.Log

	// check the interface is satisfied
	_ TeslacoilBitcoind = &Conn{}
)

// Config contains everything we need to reliably start a bitcoind node.
type Config struct {
	RpcPort  int
	RpcHost  string
	P2pPort  int
	User     string
	Password string
	// ZmqPubRawTx is the host (and port) that bitcoind publishes raw TXs to
	ZmqPubRawTx string
	// ZmqPubRawBlock is the host (and port) that bitcoind publishes raw blocks to
	ZmqPubRawBlock string
	// Network is the network we're running on
	Network chaincfg.Params
}

// Conn represents a persistent client connection to a bitcoind node
// that listens for events read from a ZMQ connection
type Conn struct {
	// btcctl is a bitcoind rpc connection
	btcctl *rpcclient.Client
	// zmqBlockConn is the ZMQ connection we'll use to read raw block
	// events
	zmqBlockConn *gozmq.Conn
	// zmqBlockCh is the channel on which we return block events received
	// from zmq
	zmqBlockCh chan *wire.MsgBlock
	// zmqTxConn is the ZMQ connection we'll use to read raw new
	// transaction events
	zmqTxConn *gozmq.Conn
	// zmqTxCh is the channel on which we return tx events received
	// from zmq
	zmqTxCh chan *wire.MsgTx
	// config is the config used for this connection
	config Config
	// network is the network this cnonection is running on
	network chaincfg.Params
}

// ToConnConfig converts this BitcoindConfig to the format the rpcclient
// library expects.
func (conf *Config) ToConnConfig() *rpcclient.ConnConfig {
	host := conf.RpcHost
	if host == "" {
		host = "127.0.0.1"
	}
	return &rpcclient.ConnConfig{
		Host:         fmt.Sprintf("%s:%d", host, conf.RpcPort),
		User:         conf.User,
		Pass:         conf.Password,
		DisableTLS:   true, // Bitcoin Core doesn't do TLS
		HTTPPostMode: true, // Bitcoin Core only supports HTTP POST mode
	}
}

// DefaultRpcPort gets the default RPC port for the given chain parameters
func DefaultRpcPort(params chaincfg.Params) (int, error) {
	switch params.Name {
	case chaincfg.MainNetParams.Name:
		return 8332, nil
	case chaincfg.TestNet3Params.Name:
		return 18332, nil
	case chaincfg.RegressionNetParams.Name:
		return 18443, nil
	case "":
		return 0, errors.New("network is not set")
	default:
		return 0, fmt.Errorf("unknown network %q", params.Name)
	}
}

func (c *Conn) Btcctl() RpcClient {
	return c.btcctl
}
func (c *Conn) ZmqBlockChannel() chan *wire.MsgBlock {
	return c.zmqBlockCh
}
func (c *Conn) ZmqTxChannel() chan *wire.MsgTx {
	return c.zmqTxCh
}
func (c *Conn) Config() Config {
	return c.config
}
func (c *Conn) Network() chaincfg.Params {
	return c.network
}

func GenerateToAddress(bitcoin TeslacoilBitcoind, numBlocks uint32, address btcutil.Address) ([]*chainhash.Hash, error) {
	body := fmt.Sprintf(`{
		"jsonrpc": "1.0",
		"method": "generatetoaddress",
		"params": [%d, %q]
	}`, numBlocks, address)
	conf := bitcoin.Config()
	url := fmt.Sprintf("http://%s:%s@%s:%d", conf.User, conf.Password, conf.RpcHost, conf.RpcPort)
	req, err := http.Post(
		url,
		"application/json",
		bytes.NewReader([]byte(body)))
	if err != nil {
		return nil, errors.Wrap(err, "generatetoaddress")
	}

	bodyBytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, errors.Wrap(err, "could not read body")
	}

	type GenerateResponse struct {
		Hashes []string `json:"result"`
	}

	var res GenerateResponse
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, errors.Wrapf(err, "could not unmarshal JSON: %+v. Body : %s", err, string(bodyBytes))
	}
	var hashes []*chainhash.Hash
	for _, hash := range res.Hashes {
		newHash, err := chainhash.NewHashFromStr(hash)
		if err != nil {
			log.Errorf("could not create new hash from %s", hash)
			continue
		}
		hashes = append(hashes, newHash)
	}

	return hashes, nil
}

// NewConn returns a BitcoindConn corresponding to the given
// configuration
func NewConn(conf Config, zmqPollInterval time.Duration) (
	*Conn, error) {

	// Bitcoin Core doesn't do notifications
	var notificationHandler *rpcclient.NotificationHandlers = nil

	client, err := rpcclient.New(conf.ToConnConfig(), notificationHandler)
	if err != nil {
		return nil, errors.Wrap(err, "could not create new bitcoind rpcclient,"+
			"is bitcoind running?")
	}

	// Establish two different ZMQ connections to bitcoind to retrieve block
	// and transaction event notifications. We'll use two as a separation of
	// concern to ensure one type of event isn't dropped from the connection
	// queue due to another type of event filling it up.
	zmqBlockConn, err := gozmq.Subscribe(
		conf.ZmqPubRawBlock, []string{"rawblock"}, zmqPollInterval)
	if err != nil {
		return nil, errors.Wrap(err, "gozmq.Subscribe rawblock")
	}

	zmqTxConn, err := gozmq.Subscribe(
		conf.ZmqPubRawTx, []string{"rawtx"}, zmqPollInterval)
	if err != nil {
		closeErr := zmqBlockConn.Close()
		if closeErr != nil {
			log.Errorf("could not close zmqBlockConn: %+v", closeErr)
		}
		return nil, errors.Wrap(err, "gozmq.Subscribe rawtx")
	}

	zmqRawTxCh := make(chan *wire.MsgTx)
	zmqRawBlockCh := make(chan *wire.MsgBlock)

	conn := &Conn{
		btcctl:       client,
		zmqBlockConn: zmqBlockConn,
		zmqTxConn:    zmqTxConn,
		// We register the channels on the connection to make them accessible
		// to the blockEventHandler and txEventHandler functions
		zmqTxCh:    zmqRawTxCh,
		zmqBlockCh: zmqRawBlockCh,
		config:     conf,
		network:    conf.Network,
	}

	if err = awaitBitcoind(conn); err != nil {
		return nil, err
	}

	log.WithFields(logrus.Fields{
		"network": conf.Network.Name,
		"host":    conf.RpcHost,
		"port":    conf.RpcPort,
	}).Info("opened connection to bitcoind")

	return conn, nil
}

// awaitBitcoind tries to get a RPC response from bitcoind, returning an error
// if that isn't possible within a set of attempts
func awaitBitcoind(btc *Conn) error {
	retry := func() bool {
		_, err := btc.Btcctl().GetBlockChainInfo()
		if err != nil {
			wrapped := fmt.Errorf("awaitBitcoind: %w", err)
			log.WithError(wrapped).Debug("getblockchaininfo failed")
		}
		return err == nil
	}
	return async.Await(5, time.Second, retry, "couldn't reach bitcoind")
}

// StartZmq attempts to establish a ZMQ connection to a bitcoind node. If
// successful, a goroutine is spawned to read events from the ZMQ connection.
// It's possible for this function to fail due to a limited number of connection
// attempts. This is done to prevent waiting forever on the connection to be
// established in the case that the node is down.
func (c *Conn) StartZmq() {
	go c.blockEventHandler()
	go c.txEventHandler()
}

// Stop terminates the ZMQ connection to a bitcoind node
func (c *Conn) StopZmq() {
	// We don't care if the connection is actually closed or not, as this
	// method is only called at shutdown. Therefore we don't handle potential
	// errors
	_ = c.zmqBlockConn.Close()
	_ = c.zmqTxConn.Close()
}

// FindVout finds a vout for transaction with `txid` and `amountSat`
// by quering bitcoin core for the txid
func (c *Conn) FindVout(txid string, amountSat int64) (int, error) {

	txHash, err := chainhash.NewHashFromStr(txid)
	if err != nil {
		return -1, fmt.Errorf("could not convert txid to hash: %w", err)
	}

	transactionResult, err := c.Btcctl().GetRawTransactionVerbose(txHash)
	if err != nil {
		return -1, fmt.Errorf("could not get raw transaction from txhash: %w", err)
	}

	vout := -1
	for _, tx := range transactionResult.Vout {

		amount := math.Round(btcutil.SatoshiPerBitcoin * tx.Value)
		log.Tracef("found output with amountSat %f", amount)

		if amount == float64(amountSat) && vout != -1 {
			return -1, fmt.Errorf("found multiple outputs with amount %d", amountSat)
		}

		if amount == float64(amountSat) && vout == -1 {
			vout = int(tx.N)
		}
	}

	if vout == -1 {
		return -1, fmt.Errorf("did not find output with amount %d", amountSat)
	}

	return vout, nil
}

// blockEventHandler reads raw blocks events from the ZMQ block socket and
// forwards them to the channel registered on the Conn
//
// NOTE: This must be run as a goroutine.
func (c *Conn) blockEventHandler() {
	log.Info("Started listening for bitcoind block notifications via ZMQ")

	for {
		// Poll an event from the ZMQ socket. This is where the goroutine
		// will hang until new messages are received
		msgBytes, err := c.zmqBlockConn.Receive()
		if err != nil {
			// EOF should only be returned if the connection was
			// explicitly closed, so we can exit at this point.
			if err == io.EOF {
				return
			}

			// It's possible that the connection to the socket
			// continuously times out, so we'll prevent logging this
			// error to prevent spamming the logs.
			netErr, ok := err.(net.Error)
			if ok && netErr.Timeout() {
				continue
			}

			log.Errorf("Unable to receive ZMQ rawblock message: %v",
				err)

			// TODO: Silence error if it is of type 'cannot receive from a closed connection'
			return
		}

		// We have an event! We'll now ensure it is a block event,
		// deserialize it, and report it to the zmq block channel
		// the other end is (hopefully) listening at
		eventType := string(msgBytes[0])
		switch eventType {
		case "rawblock":
			block := &wire.MsgBlock{}
			r := bytes.NewReader(msgBytes[1])
			if err := block.Deserialize(r); err != nil {
				log.Errorf("Unable to deserialize block: %v",
					err)
				continue
			}

			log.Tracef("received new block %v", block.BlockHash())
			// send the deserialized block to the block channel
			c.zmqBlockCh <- block

		default:
			// It's possible that the message wasn't fully read if
			// bitcoind shuts down, which will produce an unreadable
			// event type. To prevent from logging it, we'll make
			// sure it conforms to the ASCII standard.
			if eventType == "" || !isASCII(eventType) {
				continue
			}

			log.Warnf("Received unexpected event type from "+
				"rawblock subscription: %v", eventType)
		}
	}
}

// txEventHandler reads raw blocks events from the ZMQ block socket and
// forwards them to the zmqTxCh found in the Conn
//
// NOTE: This must be run as a goroutine.
func (c *Conn) txEventHandler() {
	log.Info("Started listening for bitcoind transaction notifications via ZMQ")

	for {
		// Poll an event from the ZMQ socket
		msgBytes, err := c.zmqTxConn.Receive()
		if err != nil {
			// EOF should only be returned if the connection was
			// explicitly closed, so we can exit at this point.
			if err == io.EOF {
				return
			}

			// It's possible that the connection to the socket
			// continuously times out, so we'll prevent logging this
			// error to prevent spamming the logs.
			netErr, ok := err.(net.Error)
			if ok && netErr.Timeout() {
				continue
			}

			log.Errorf("Unable to receive ZMQ rawtx message: %v",
				err)
			// TODO: Silence error if it is of type 'cannot receive from a closed connection'
			//  and dont return here, but continue
			return
		}

		// We have an event! We'll now ensure it is a transaction event,
		// deserialize it, and report it to the different rescan
		// clients.
		eventType := string(msgBytes[0])
		switch eventType {
		case "rawtx":
			tx := &wire.MsgTx{}
			r := bytes.NewReader(msgBytes[1])
			// Deserialize the bytes from reader r into tx
			if err := tx.Deserialize(r); err != nil {
				log.Errorf("Unable to deserialize "+
					"transaction: %v", err)
				continue
			}

			// send the tx event to the channel
			c.zmqTxCh <- tx

		default:
			// It's possible that the message wasn't fully read if
			// bitcoind shuts down, which will produce an unreadable
			// event type. To prevent from logging it, we'll make
			// sure it conforms to the ASCII standard.
			if eventType == "" || !isASCII(eventType) {
				continue
			}

			log.Warnf("Received unexpected event type from rawtx "+
				"subscription: %v", eventType)
		}

	}
}

// isASCII is a helper method that checks whether all bytes in `data` would be
// printable ASCII characters if interpreted as a string.
func isASCII(s string) bool {
	for _, c := range s {
		if c < 32 || c > 126 {
			return false
		}
	}
	return true
}
