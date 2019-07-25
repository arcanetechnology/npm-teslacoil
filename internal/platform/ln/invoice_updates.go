package ln

import (
	"context"

	"github.com/lightningnetwork/lnd/lnrpc"
)

// type InvoiceUpdates struct {
// 	Listeners: map[]
// }

// func (iu *InvoiceUpdates) Publish(invoice lnrpc.Invoice)  {

// }

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

// // MakeButton sdasdfsafd
// func MakeButton() *Button {
// 	result := new(Button)
// 	result.eventListeners = make(map[string][]chan string)
// 	return result
// }

// func (this *Button) AddEventListener(event string, responseChannel chan string) {
// 	if _, present := this.eventListeners[event]; present {
// 		this.eventListeners[event] =
// 			append(this.eventListeners[event], responseChannel)
// 	} else {
// 		this.eventListeners[event] = []chan string{responseChannel}
// 	}
// }

// func (this *Button) RemoveEventListener(event string, listenerChannel chan string) {
// 	if _, present := this.eventListeners[event]; present {
// 		for index, _ := range this.eventListeners[event] {
// 			if this.eventListeners[event][index] == listenerChannel {
// 				this.eventListeners[event] = append(
// 					this.eventListeners[event][:index],
// 					this.eventListeners[event][index+1:]...)
// 				break
// 			}
// 		}
// 	}
// }

// func (this *Button) TriggerEvent(event string, response string) {
// 	if _, present := this.eventListeners[event]; present {
// 		for _, handler := range this.eventListeners[event] {
// 			go func(handler chan string) {
// 				handler <- response
// 			}(handler)
// 		}
// 	}
// }
