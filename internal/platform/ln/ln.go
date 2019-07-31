package ln

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/btcsuite/btcutil"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/macaroons"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gopkg.in/macaroon.v2"
)

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
		fmt.Println("Cannot get node tls credentials", err)
		return nil, err
	}

	macaroonBytes, err := ioutil.ReadFile(cfg.MacaroonPath)
	if err != nil {
		fmt.Println("Cannot read macaroon file", err)
		return nil, err
	}

	mac := &macaroon.Macaroon{}
	if err = mac.UnmarshalBinary(macaroonBytes); err != nil {
		fmt.Println("Cannot unmarshal macaroon", err)
		return nil, err
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(tlsCreds),
		grpc.WithBlock(),
		grpc.WithPerRPCCredentials(macaroons.NewMacaroonCredential(mac)),
	}

	conn, err := grpc.Dial(cfg.RPCServer, opts...)
	if err != nil {
		fmt.Println("cannot dial to lnd", err)
		return nil, err
	}
	client := lnrpc.NewLightningClient(conn)

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
