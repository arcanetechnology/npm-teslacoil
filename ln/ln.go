package ln

import (
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/build"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/macaroons"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gopkg.in/macaroon.v2"
)

var log = build.Log

// AddLookupInvoiceClient defines the required methods for adding an invoice
type AddLookupInvoiceClient interface {
	AddInvoice(ctx context.Context, in *lnrpc.Invoice, opts ...grpc.CallOption) (*lnrpc.AddInvoiceResponse, error)
	LookupInvoice(ctx context.Context, in *lnrpc.PaymentHash, opts ...grpc.CallOption) (*lnrpc.Invoice, error)
}

// DecodeSendClient defines the required methods for paying an invoice
type DecodeSendClient interface {
	DecodePayReq(ctx context.Context, in *lnrpc.PayReqString, opts ...grpc.CallOption) (*lnrpc.PayReq, error)
	SendPaymentSync(ctx context.Context, in *lnrpc.SendRequest, opts ...grpc.CallOption) (*lnrpc.SendResponse, error)
}

// AddInvoiceData is the data required to add a invoice
type AddInvoiceData struct {
	Memo   string `json:"memo"`
	Amount int    `json:"amount"`
}

// LightningConfig is a struct containing all possible options for configuring
// a connection to lnd
type LightningConfig struct {
	LndDir      string
	TLSCertPath string
	TLSKeyPath  string
	// MacaroonPath corresponds to the --adminmacaroonpath startup option of
	// lnd
	MacaroonPath string
	Network      chaincfg.Params
	RPCHost      string
	RPCPort      int
	// P2pPort is the port lnd listens to peer connections on
	P2pPort int
}

// DefaultRelativeMacaroonPath extracts the macaroon path using a specific network
func DefaultRelativeMacaroonPath(network chaincfg.Params) string {
	name := network.Name
	if name == "testnet3" {
		name = "testnet"
	}
	return filepath.Join("data", "chain",
		"bitcoin", name, "admin.macaroon")
}

const (
	DefaultRpcServer = "localhost:" + DefaultRpcPort
	DefaultRpcPort   = "10009"
)

// NewLNDClient opens a new connection to LND and returns the client
func NewLNDClient(options LightningConfig) (
	lnrpc.LightningClient, error) {
	cfg := LightningConfig{
		LndDir:       options.LndDir,
		TLSCertPath:  cleanAndExpandPath(options.TLSCertPath),
		MacaroonPath: cleanAndExpandPath(options.MacaroonPath),
		Network:      options.Network,
		RPCHost:      options.RPCHost,
		RPCPort:      options.RPCPort,
	}

	if cfg.TLSCertPath == "" {
		cfg.TLSCertPath = filepath.Join(cfg.LndDir, "tls.cert")
	}

	if cfg.MacaroonPath == "" {
		cfg.MacaroonPath = filepath.Join(cfg.LndDir,
			DefaultRelativeMacaroonPath(options.Network))
	}

	tlsCreds, err := credentials.NewClientTLSFromFile(cfg.TLSCertPath, "")
	if err != nil {
		err = fmt.Errorf("cannot get node tls credentials: %w", err)
		return nil, err
	}

	macaroonBytes, err := ioutil.ReadFile(cfg.MacaroonPath)
	if err != nil {
		err = fmt.Errorf("cannot read macaroon file: %w", err)
		return nil, err
	}

	mac := &macaroon.Macaroon{}
	if err = mac.UnmarshalBinary(macaroonBytes); err != nil {
		err = fmt.Errorf("cannot unmarshal macaroon: %w", err)
		return nil, err
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(tlsCreds),
		grpc.WithBlock(),
		grpc.WithPerRPCCredentials(macaroons.NewMacaroonCredential(mac)),
	}

	backgroundContext := context.Background()
	withTimeout, cancel := context.WithTimeout(backgroundContext, 5*time.Second)
	defer cancel()

	// grpc is having trouble connecting to host names that haven't been translated
	addrs, err := net.LookupHost(cfg.RPCHost)
	if err != nil {
		return nil, fmt.Errorf("could not lookup host %q: %w", cfg.RPCHost, err)
	}
	translatedAddress := addrs[0] // there's always at least one element here if no err

	log.WithFields(logrus.Fields{
		"certpath":     cfg.TLSCertPath,
		"macaroonpath": cfg.MacaroonPath,
		"network":      cfg.Network.Name,
		"rpchost":      cfg.RPCHost,
		"rpcport":      cfg.RPCPort,
		"rpcaddr":      translatedAddress,
	}).Info("Connecting to LND")

	conn, err := grpc.DialContext(withTimeout, fmt.Sprintf("%s:%d", translatedAddress, cfg.RPCPort), opts...)
	if err != nil {
		err = fmt.Errorf("cannot dial to lnd: %w", err)
		return nil, err
	}
	client := lnrpc.NewLightningClient(conn)

	log.WithFields(logrus.Fields{
		"rpchost": cfg.RPCHost,
		"rpcport": cfg.RPCPort,
	}).Info("opened connection to LND")

	return client, nil
}

// cleanAndExpandPath expands environment variables and leading ~ in the
// passed path, cleans the result, and returns it.
// This function is taken from https://github.com/btcsuite/btcd
func cleanAndExpandPath(path string) string {
	if path == "" {
		return ""
	}

	// Expand initial ~ to OS specific home directory.
	if strings.HasPrefix(path, "~") {
		var homeDir string
		user, err := user.Current()
		if err == nil {
			homeDir = user.HomeDir
		} else {
			homeDir = os.Getenv("HOME")
		}

		path = strings.Replace(path, "~", homeDir, 1)
	}

	// NOTE: The os.ExpandEnv doesn't work with Windows-style %VARIABLE%,
	// but the variables can still be expanded via POSIX-style $VARIABLE.
	return filepath.Clean(os.ExpandEnv(path))
}

// AddInvoice adds an invoice and looks up the invoice in the lnd DB to extract
// more useful data
func AddInvoice(lncli AddLookupInvoiceClient, invoiceData lnrpc.Invoice) (
	*lnrpc.Invoice, error) {
	ctx := context.Background()

	log.Tracef("Adding invoice: %+v", invoiceData)
	inv, err := lncli.AddInvoice(ctx, &invoiceData)
	if err != nil {
		err = fmt.Errorf("could not add invoice: %w", err)
		return nil, err
	}
	log.Tracef("Added invoice: %+v", *inv)

	invoice, err := lncli.LookupInvoice(ctx, &lnrpc.PaymentHash{
		RHash: inv.RHash,
	})
	if err != nil {
		err = fmt.Errorf("could not lookup invoice: %w", err)
		return nil, err
	}

	log.WithFields(logrus.Fields{
		"invoice": inv.PaymentRequest,
		"hash":    hex.EncodeToString(inv.RHash),
	}).Debug("added invoice")

	return invoice, nil
}

// ListenInvoices subscribes to lnd invoices
func ListenInvoices(lncli lnrpc.LightningClient, msgCh chan *lnrpc.Invoice) {
	invoiceSubDetails := &lnrpc.InvoiceSubscription{}

	invoiceClient, err := lncli.SubscribeInvoices(
		context.Background(),
		invoiceSubDetails)
	if err != nil {
		log.Error(err)
		return
	}

	for {
		invoice := lnrpc.Invoice{}
		err := invoiceClient.RecvMsg(&invoice)
		if err != nil {
			log.Error(err)
			return
		}
		log.Infof("invoice %s with hash %s was updated",
			invoice.PaymentRequest, hex.EncodeToString(invoice.RHash))

		msgCh <- &invoice
	}

}

func (l LightningConfig) String() string {
	return strings.Join([]string{
		fmt.Sprintf("LndDir: %s", l.LndDir),
		fmt.Sprintf("TLSCertPath: %s", l.TLSCertPath),
		fmt.Sprintf("MacaroonPath: %s", l.MacaroonPath),
		fmt.Sprintf("Network: %s", l.Network.Name),
		fmt.Sprintf("RPCHost: %s", l.RPCHost),
		fmt.Sprintf("RPCPort: %d", l.RPCPort),
	}, ", ")
}

const (
	// MaxAmountSatPerChannel is the maximum amount of satoshis a channel can be for
	// https://github.com/lightningnetwork/lnd/blob/b9816259cb520fc169cb2cd829edf07f1eb11e1b/fundingmanager.go#L64
	MaxAmountSatPerChannel = (1 << 24) - 1
	// MaxAmountMsatPerChannel is the maximum amount of millisatoshis a channel can be for
	MaxAmountMsatPerChannel = MaxAmountSatPerChannel * 1000
	// MaxAmountSatPerInvoice is the maximum amount of satoshis an invoice can be for
	MaxAmountSatPerInvoice = MaxAmountMsatPerInvoice / 1000
	// MaxAmountMsatPerInvoice is the maximum amount of millisatoshis an invoice can be for
	MaxAmountMsatPerInvoice = 4294967295
)
