package ln

import (
	"context"
	"encoding/hex"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/btcsuite/btcutil"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/macaroons"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gopkg.in/macaroon.v2"
)

// AddInvoiceData is the data required to add a invoice
type AddInvoiceData struct {
	Memo   string `json:"memo"`
	Amount int64  `json:"amount"`
}

// LightningConfig is a struct containing all possible options for configuring
// a connection to lnd
type LightningConfig struct {
	LndDir       string
	TLSCertPath  string
	MacaroonPath string
	Network      string
	RPCServer    string
}

var (
	// DefaultLndDir is the default location of .lnd
	DefaultLndDir = btcutil.AppDataDir("lnd", false)
	// DefaultTLSCertPath is the default location of tls.cert
	DefaultTLSCertPath = filepath.Join(DefaultLndDir, "tls.cert")
	// DefaultMacaroonPath is the default dir of x.macaroon
	DefaultMacaroonPath = filepath.Join(DefaultLndDir, "data/chain/bitcoin/testnet/admin.macaroon")

	// DefaultCfg is a config interface with default values
	DefaultCfg = LightningConfig{
		LndDir:       DefaultLndDir,
		TLSCertPath:  DefaultTLSCertPath,
		MacaroonPath: DefaultMacaroonPath,
		Network:      DefaultNetwork,
		RPCServer:    DefaultRPCHostPort,
	}
)

const (
	// DefaultNetwork is the default network
	DefaultNetwork = "testnet"
	// DefaultRPCHostPort is the default host port of lnd
	DefaultRPCHostPort = "localhost:10009"
	// DefaultTLSCertFileName is the default filename of the tls certificate
	DefaultTLSCertFileName = "tls.cert"
)

// NewLNDClient opens a new connection to LND and returns the client
func NewLNDClient(options LightningConfig) (
	lnrpc.LightningClient, error) {
	cfg := LightningConfig{
		LndDir:       options.LndDir,
		TLSCertPath:  CleanAndExpandPath(options.TLSCertPath),
		MacaroonPath: CleanAndExpandPath(options.MacaroonPath),
		Network:      options.Network,
		RPCServer:    options.RPCServer,
	}

	if options.LndDir != DefaultLndDir {
		cfg.LndDir = options.LndDir
		cfg.TLSCertPath = filepath.Join(cfg.LndDir, DefaultTLSCertFileName)
		cfg.MacaroonPath = filepath.Join(cfg.LndDir,
			filepath.Join("data/chain/bitcoin",
				filepath.Join(cfg.Network, "admin.macaroon")))
	}

	tlsCreds, err := credentials.NewClientTLSFromFile(cfg.TLSCertPath, "")
	if err != nil {
		log.Errorf("Cannot get node tls credentials %v", err)
		return nil, err
	}

	macaroonBytes, err := ioutil.ReadFile(cfg.MacaroonPath)
	if err != nil {
		log.Errorf("Cannot read macaroon file %v", err)
		return nil, err
	}

	mac := &macaroon.Macaroon{}
	if err = mac.UnmarshalBinary(macaroonBytes); err != nil {
		log.Errorf("Cannot unmarshal macaroon %v", err)
		return nil, err
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(tlsCreds),
		grpc.WithBlock(),
		grpc.WithPerRPCCredentials(macaroons.NewMacaroonCredential(mac)),
		grpc.WithTimeout(5 * time.Second),
	}

	conn, err := grpc.Dial(cfg.RPCServer, opts...)
	if err != nil {
		log.Errorf("cannot dial to lnd: %v", err)
		return nil, err
	}
	client := lnrpc.NewLightningClient(conn)

	log.Debugf("opened connection to lnd on %s", cfg.RPCServer)

	return client, nil
}

// CleanAndExpandPath expands environment variables and leading ~ in the
// passed path, cleans the result, and returns it.
// This function is taken from https://github.com/btcsuite/btcd
func CleanAndExpandPath(path string) string {
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
func AddInvoice(lncli lnrpc.LightningClient, invoiceData lnrpc.Invoice) (
	*lnrpc.Invoice, error) {
	ctx := context.Background()
	inv, err := lncli.AddInvoice(ctx, &invoiceData)
	if err != nil {
		log.Error(err)
		return &lnrpc.Invoice{}, err
	}
	invoice, err := lncli.LookupInvoice(ctx, &lnrpc.PaymentHash{
		RHash: inv.RHash,
	})
	if err != nil {
		log.Error(err)
		return &lnrpc.Invoice{}, err
	}

	log.Debugf("added invoice %s with hash %s", inv.PaymentRequest, hex.EncodeToString(inv.RHash))

	return invoice, nil
}

// ListenInvoices subscribes to lnd invoices
func ListenInvoices(lncli lnrpc.LightningClient, msgCh chan lnrpc.Invoice) error {
	invoiceSubDetails := &lnrpc.InvoiceSubscription{}

	invoiceClient, err := lncli.SubscribeInvoices(
		context.Background(),
		invoiceSubDetails)
	if err != nil {
		log.Error(err)
		return err
	}

	for {
		invoice := lnrpc.Invoice{}
		err := invoiceClient.RecvMsg(&invoice)
		if err != nil {
			log.Error(err)
			return err
		}
		log.Debugf("invoice %s with hash %s was updated", invoice.PaymentRequest, hex.EncodeToString(invoice.RHash))
		msgCh <- invoice
	}
}
