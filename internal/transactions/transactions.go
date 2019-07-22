package transactions

import (
	"github.com/jinzhu/gorm"
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
func Create(d *gorm.DB, nt NewTransaction) (Transaction, error) {

	transaction := Transaction{
		UserID:      nt.UserID,
		Invoice:     nt.Invoice,
		Description: nt.Description,
		Direction:   nt.Direction,
		Status:      nt.Status,
		Amount:      nt.Amount,
	}

	if err := d.Create(&transaction).Error; err != nil {
		return transaction, err
	}

	return transaction, nil
}
