package ln

import (
	"context"

	"github.com/lightningnetwork/lnd/lnrpc"
)

// ListenInvoices subscribes to lnd invoices
func ListenInvoices(lncli lnrpc.LightningClient, msgCh chan lnrpc.Invoice) error {
	invoiceSubDetails := &lnrpc.InvoiceSubscription{}

	invoiceClient, err := lncli.SubscribeInvoices(
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
}
