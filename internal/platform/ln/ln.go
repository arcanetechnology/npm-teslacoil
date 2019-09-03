package ln

import (
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/btcsuite/btcutil"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/macaroons"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gopkg.in/macaroon.v2"
)

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
	LndDir       string
	TLSCertPath  string
	MacaroonPath string
	Network      string
	RPCServer    string
}

func configDefaultLndDir() string {
	if len(os.Getenv("LND_DIR")) != 0 {
		return os.Getenv("LND_DIR")
	}
	return btcutil.AppDataDir("lnd", false)
}

func configDefaultLndNet() string {
	if len(os.Getenv("LND_NETWORK")) != 0 {
		return os.Getenv("LND_NETWORK")
	}
	return "testnet"

}
func configDefaultLndPort() string {
	if len(os.Getenv("LND_PORT")) != 0 {
		return os.Getenv("LND_PORT")
	}
	return "10009"
}

var (
	// DefaultNetwork is the default network
	DefaultNetwork = configDefaultLndNet()
	// DefaultPort is the default lnd port (10009)
	DefaultPort = configDefaultLndPort()
	// DefaultRPCHostPort is the default host port of lnd
	DefaultRPCHostPort = "localhost:" + DefaultPort
	// DefaultTLSCertFileName is the default filename of the tls certificate
	DefaultTLSCertFileName = "tls.cert"
)

var (
	// DefaultLndDir is the default location of .lnd
	DefaultLndDir = configDefaultLndDir()
	// LndNetwork is the default LND network (testnet)
	LndNetwork = configDefaultLndNet()
	// DefaultTLSCertPath is the default location of tls.cert
	DefaultTLSCertPath = filepath.Join(DefaultLndDir, "tls.cert")
	// DefaultMacaroonPath is the default dir of x.macaroon
	DefaultMacaroonPath = filepath.Join(DefaultLndDir, "data/chain/bitcoin",
		DefaultNetwork, "admin.macaroon")

	// DefaultCfg is a config interface with default values
	DefaultCfg = LightningConfig{
		LndDir:       DefaultLndDir,
		TLSCertPath:  DefaultTLSCertPath,
		MacaroonPath: DefaultMacaroonPath,
		Network:      DefaultNetwork,
		RPCServer:    DefaultRPCHostPort,
	}
)

// NewLNDClient opens a new connection to LND and returns the client
func NewLNDClient(options LightningConfig) (
	lnrpc.LightningClient, error) {
	cfg := LightningConfig{
		LndDir:       options.LndDir,
		TLSCertPath:  cleanAndExpandPath(options.TLSCertPath),
		MacaroonPath: cleanAndExpandPath(options.MacaroonPath),
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
		err = errors.Wrap(err, "Cannot get node tls credentials")
		return nil, err
	}

	macaroonBytes, err := ioutil.ReadFile(cfg.MacaroonPath)
	if err != nil {
		err = errors.Wrap(err, "Cannot read macaroon file")
		return nil, err
	}

	mac := &macaroon.Macaroon{}
	if err = mac.UnmarshalBinary(macaroonBytes); err != nil {
		err = errors.Wrap(err, "Cannot unmarshal macaroon")
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
		err = errors.Wrap(err, "cannot dial to lnd")
		return nil, err
	}
	client := lnrpc.NewLightningClient(conn)

	log.Infof("opened connection to lnd on %s", cfg.RPCServer)

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
	inv, err := lncli.AddInvoice(ctx, &invoiceData)
	if err != nil {
		err = errors.Wrap(err, "could not add invoice using lncli.AddInvoice()")
		return nil, err
	}
	invoice, err := lncli.LookupInvoice(ctx, &lnrpc.PaymentHash{
		RHash: inv.RHash,
	})
	if err != nil {
		err = errors.Wrap(err,
			"could not lookup invoice using lncli.LookupInvoice()")
		return nil, err
	}

	log.Infof("added invoice %s with hash %s",
		inv.PaymentRequest, hex.EncodeToString(inv.RHash))

	return invoice, nil
}

// ListenInvoices subscribes to lnd invoices
func ListenInvoices(lncli lnrpc.LightningClient, msgCh chan lnrpc.Invoice) {
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

		msgCh <- invoice
	}
}

func (l LightningConfig) String() string {
	str := fmt.Sprintf("LndDir: %s\n", l.LndDir)
	str += fmt.Sprintf("TLSCertPath: %s\n", l.TLSCertPath)
	str += fmt.Sprintf("MacaroonPath: %s\n", l.MacaroonPath)
	str += fmt.Sprintf("Network: %s\n", l.Network)
	str += fmt.Sprintf("RPCServer: %s\n", l.RPCServer)

	return str
}
