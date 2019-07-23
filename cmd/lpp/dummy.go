package main

import (
	"time"

	"github.com/brianvoe/gofakeit"
	"github.com/jinzhu/gorm"
	"gitlab.com/arcanecrypto/lpp/internal/transactions"
	"gitlab.com/arcanecrypto/lpp/internal/users"
)

// FillWithDummyData creates three entries in each table
func FillWithDummyData(d *gorm.DB) error {
	gofakeit.Seed(time.Now().UnixNano())

	userCount := 10

	for index := 1; index <= userCount; index++ {
		user, err := users.Create(d, users.UserNew{
			Email:    gofakeit.Email(),
			Password: gofakeit.Password(true, true, true, true, true, 32),
		})
		if err != nil {
			return err
		}

		user.Balance = gofakeit.Number(50000, 10000000000)
		d.Save(user)

		transactionCount := gofakeit.Number(1, 20)

		statusOptions := []string{"completed", "failed"}
		directionOptions := []string{"inbound", "outbound"}
		for index := 1; index <= transactionCount; index++ {
			_, err = transactions.Create(d, transactions.NewTransaction{
				UserID:      user.ID,
				Invoice:     "sadfsdafasdfasdfsdfsadf12321213123",
				Status:      gofakeit.RandString(statusOptions),
				Description: "Dummy data",
				Direction:   transactions.Direction(gofakeit.RandString(directionOptions)),
				Amount:      gofakeit.Number(50000, 4000000000),
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}
