package ln

import (
	"context"
	"fmt"
	"io/ioutil"
	"os/user"
	"path"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/macaroons"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gopkg.in/macaroon.v2"
)

// NewLNDClient opens a new connection to LND and returns the client
func NewLNDClient() (lnrpc.LightningClient, error) {
	usr, err := user.Current()
	if err != nil {
		fmt.Println("Cannot get current user:", err)
		return nil, err
	}
	// ctx := context.Background()

	// Connect to lnd
	tlsCertPath := path.Join(usr.HomeDir, ".lnd/tls.cert")
	macaroonPath := path.Join(usr.HomeDir, ".lnd/data/chain/bitcoin/simnet/admin.macaroon")

	tlsCreds, err := credentials.NewClientTLSFromFile(tlsCertPath, "")
	if err != nil {
		fmt.Println("Cannot get node tls credentials", err)
		return nil, err
	}

	macaroonBytes, err := ioutil.ReadFile(macaroonPath)
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

	conn, err := grpc.Dial("localhost:10009", opts...)
	if err != nil {
		fmt.Println("cannot dial to lnd", err)
		return nil, err
	}
	client := lnrpc.NewLightningClient(conn)

	return client, nil
}

func ListenInvoices(msgCh chan lnrpc.Invoice) error {

	client, err := NewLNDClient()
	if err != nil {
		return err
	}

	invoiceSubDetails := &lnrpc.InvoiceSubscription{}

	invoiceClient, err := client.SubscribeInvoices(
		context.Background(),
		invoiceSubDetails)
	if err != nil {
		return err
	}

	for {
		invoice := lnrpc.Invoice{}
		err := invoiceClient.RecvMsg(&invoice)
		if err != nil {
			return err
		}
		msgCh <- invoice
	}

	return nil
}

// func ListenInvoices() (lnrpc.Lightning_SubscribeTransactionsClient, error) {

// 	// iu := &InvoiceUpdates{}

// 	client, err := NewLNDClient()
// 	if err != nil {
// 		return nil, err
// 	}

// 	transactionsDetails := &lnrpc.GetTransactionsRequest{}

// 	transactionsClient, err := client.SubscribeTransactions(
// 		context.Background(),
// 		transactionsDetails)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return transactionsClient, nil
// }
