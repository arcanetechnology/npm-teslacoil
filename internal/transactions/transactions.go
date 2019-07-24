package transactions

import (
	"context"

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
		Status:    "unpaid",
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
	// log.Printf("%v", payRequest)

	err = d.Create(&transaction).Error
	if err != nil {
		return transaction, err
	}

	sendRequest := &lnrpc.SendRequest{
		PaymentRequest: nt.Invoice,
	}
	// TODO: Need to improve this step to allow for slow paying invoices.
	sendResponse, err := client.SendPaymentSync(context.Background(), sendRequest)
	if err != nil {
		return transaction, err
	}
	transaction.Status = sendResponse.PaymentError
	d.Save(transaction)

	return transaction, nil
}
