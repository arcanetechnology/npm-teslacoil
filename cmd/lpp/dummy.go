package main

import (
	"time"

	"github.com/brianvoe/gofakeit"
	"github.com/jmoiron/sqlx"
	"gitlab.com/arcanecrypto/lpp/internal/transactions"
	"gitlab.com/arcanecrypto/lpp/internal/users"
)

// FillWithDummyData creates three entries in each table
func FillWithDummyData(d *sqlx.DB) error {
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

		transactionCount := gofakeit.Number(1, 20)

		directionOptions := []string{"inbound", "outbound"}
		for index := 1; index <= transactionCount; index++ {
			_, err = transactions.CreateInvoice(d, transactions.NewTransaction{
				UserID:      user.ID,
				Description: "Dummy data " + string(index),
				Direction: transactions.Direction(
					gofakeit.RandString(directionOptions)),
				Amount: int64(gofakeit.Number(50, 10000)),
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}
