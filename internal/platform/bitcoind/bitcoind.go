package bitcoind

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/lightninglabs/gozmq"

	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	"gitlab.com/arcanecrypto/teslacoil/build"
)

var (
	log = build.Log
)

// Config contains everything we need to reliably start a bitcoind node.
// RpcNetwork is not defined as bitcoin core MUST be running locally
type Config struct {
	RpcPort      int
	User         string
	Password     string
	ZmqTxHost    string
	ZmqBlockHost string
}

// ToConnConfig converts this BitcoindConfig to the format the rpcclient
// library expects.
func (conf *Config) ToConnConfig() *rpcclient.ConnConfig {
	return &rpcclient.ConnConfig{
		Host:         fmt.Sprintf("127.0.0.1:%d", conf.RpcPort),
		User:         conf.User,
		Pass:         conf.Password,
		DisableTLS:   true, // Bitcoin Core doesn't do TLS
		HTTPPostMode: true, // Bitcoin Core only supports HTTP POST mode
	}
}

// Conn represents a persistent client connection to a bitcoind node
// that listens for events read from a ZMQ connection
type Conn struct {
	// client is the RPC client to bitcoind
	Client *rpcclient.Client
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
}

// NewConn returns a BitcoindConn corresponding to
// the given configuration, that consists of a bitcoind RPC client,
// a zmqBlockConnection and a zmqTxConnection
func NewConn(conf Config, zmqPollInterval time.Duration,
	zmqTxCh chan *wire.MsgTx, zmqBlockCh chan *wire.MsgBlock) (
	*Conn, error) {
	// Bitcoin Core doesn't do notifications
	var notificationHandler *rpcclient.NotificationHandlers = nil

	client, err := rpcclient.New(conf.ToConnConfig(), notificationHandler)
	if err != nil {
		return nil, fmt.Errorf("could not create new bitcoind rpcclient,"+
			"is bitcoind running?: %+v", err)
	}

	// Establish two different ZMQ connections to bitcoind to retrieve block
	// and transaction event notifications. We'll use two as a separation of
	// concern to ensure one type of event isn't dropped from the connection
	// queue due to another type of event filling it up.
	zmqBlockConn, err := gozmq.Subscribe(
		conf.ZmqBlockHost, []string{"rawblock"}, zmqPollInterval)
	if err != nil {
		return nil, fmt.Errorf("unable to subscribe to zmq block events: %+v", err)
	}
	zmqTxConn, err := gozmq.Subscribe(
		conf.ZmqTxHost, []string{"rawtx"}, zmqPollInterval)
	if err != nil {
		closeErr := zmqBlockConn.Close()
		if closeErr != nil {
			log.Errorf("could not close zmqBlockConn: %+v", closeErr)
		}
		return nil, fmt.Errorf("unable to subscribe to zmq tx events: %+v", err)
	}

	log.Info("block: ", zmqBlockConn)
	log.Info("tx: ", zmqTxConn)

	conn := &Conn{
		Client:       client,
		zmqBlockConn: zmqBlockConn,
		zmqTxConn:    zmqTxConn,
		// We register the channels on the connection to make them accessible
		// to the blockEventHandler and txEventHandler functions
		zmqTxCh:    zmqTxCh,
		zmqBlockCh: zmqBlockCh,
	}

	return conn, nil
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

// blockEventHandler reads raw blocks events from the ZMQ block socket and
// forwards them along to the current rescan clients.
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
			continue
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
// forwards them to the zmqTxCh found in the BitcoindConn
//
// NOTE: This must be run as a goroutine.
func (c *Conn) txEventHandler() {
	log.Info("Started listening for bitcoind transaction notifications via ZMQ")

	for {
		// Poll an event from the ZMQ socket. This is where the goroutine
		// will hang
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
			continue
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

// ListenTxs receives readily parsed txs and prints them
//
// NOTE: This must be run as a goroutine
func ListenTxs(zmqRawTxCh chan *wire.MsgTx) {
	for {
		tx := <-zmqRawTxCh

		log.Error("received new TX: ", tx)
	}
}

//ListenBlocks receives readily parsed blocks and prints them
//
// NOTE: This must be run as a goroutine
func ListenBlocks(zmqRawBlockCh chan *wire.MsgBlock) {
	for {
		block := <-zmqRawBlockCh

		log.Error("received new block: ", block)
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
