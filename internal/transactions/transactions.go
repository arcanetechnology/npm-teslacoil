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
		Direction:   nt.Direction,
		Status:      nt.Status,
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
