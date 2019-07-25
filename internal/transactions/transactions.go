package transactions

import (
	"context"
	"log"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/lightningnetwork/lnd/lnrpc"
	"gitlab.com/arcanecrypto/lpp/internal/platform/ln"
)

// All fetches all transactions
func All(d *gorm.DB) ([]Transaction, error) {
	// Equivalent to SELECT * from users;
	queryResult := []Transaction{}
	if err := d.Find(&queryResult).Error; err != nil {
		return queryResult, err
	}

	return queryResult, nil
}

// GetByID is a GET request that returns deposits that match the one specified in the body
func GetByID(d *gorm.DB, id int) (Transaction, error) {

	transaction := Transaction{}
	if err := d.Where("id = ?", id).First(&transaction).Error; err != nil {
		return transaction, err
	}

	return transaction, nil
}

// Create a new transaction
func CreateInvoice(d *gorm.DB, nt NewTransaction) (Transaction, error) {

	transaction := Transaction{
		UserID:      nt.UserID,
		Description: nt.Description,
		Direction:   Direction("inbound"), // All created invoices are inbound
		Status:      "unpaid",
		Amount:      nt.Amount,
	}

	transaction.User.ID = nt.UserID

	client, err := ln.NewLNDClient()
	if err != nil {
		return transaction, err
	}
	invoice := &lnrpc.Invoice{
		Memo:  nt.Description,
		Value: nt.Amount,
	}

	newInvoice, err := client.AddInvoice(context.Background(), invoice)
	if err != nil {
		return transaction, err
	}

	transaction.Invoice = newInvoice.PaymentRequest

	err = d.Create(&transaction).Error
	if err != nil {
		return transaction, err
	}

	return transaction, nil
}

// PayInvoice pay an invoice on behalf of the user
func PayInvoice(d *gorm.DB, nt NewTransaction) (Transaction, error) {

	transaction := Transaction{
		UserID:    nt.UserID,
		Direction: Direction("outbound"), // All paid invoices are outbound
		Invoice:   nt.Invoice,
	}

	client, err := ln.NewLNDClient()
	if err != nil {
		return transaction, err
	}

	payRequest, err := client.DecodePayReq(
		context.Background(),
		&lnrpc.PayReqString{PayReq: nt.Invoice})
	if err != nil {
		return transaction, err
	}

	transaction.Description = payRequest.Description
	transaction.Amount = payRequest.NumSatoshis
	// TODO: Here we need to store the payment hash
	// We also need to store the payment preimage once the invoice is settled

	// log.Printf("%v", payRequest)

	err = d.Create(&transaction).Error
	if err != nil {
		return transaction, err
	}

	sendRequest := &lnrpc.SendRequest{
		PaymentRequest: nt.Invoice,
	}
	log.Println("Creating send client")
	// TODO: Need to improve this step to allow for slow paying invoices.
	paymentResponse, err := client.SendPaymentSync(context.Background(), sendRequest)
	if err != nil {
		return transaction, nil
	}

	if paymentResponse.PaymentError == "" {
		transaction.Status = "SETTLED"
	} else {
		transaction.Status = paymentResponse.PaymentError
	}
	transaction.SettledAt = time.Now()
	log.Println(transaction.User)
	transaction.User.ID = nt.UserID
	transaction.User.Balance -= int(payRequest.NumSatoshis * 1000)
	// transaction.PaymentPreImage = paymentResponse.PaymentPreimage
	d.Save(&transaction)
	return transaction, nil
}

// UpdateInvoiceStatus continually listens for messages and updated the user balance
// PS: This is most likely done in a horrible way. Must be refactored.
// We also need to keep track of the last received messages from lnd
func UpdateInvoiceStatus(invoiceUpdatesCh chan lnrpc.Invoice, database *gorm.DB) {

	for {
		invoice := <-invoiceUpdatesCh

		t := Transaction{}
		database.Preload("User").Where("invoice = ?", invoice.PaymentRequest).First(&t)
		t.Status = invoice.State.String()
		if invoice.Settled {
			t.SettledAt = time.Now()
			t.User.Balance += int(invoice.AmtPaidMsat)
		}
		database.Save(&t)
	}
}

