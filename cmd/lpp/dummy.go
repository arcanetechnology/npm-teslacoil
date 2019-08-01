package main

import (
	"time"

	"github.com/brianvoe/gofakeit"
	"github.com/jmoiron/sqlx"
	"github.com/lightningnetwork/lnd/lnrpc"
	"gitlab.com/arcanecrypto/lpp/internal/payments"
	"gitlab.com/arcanecrypto/lpp/internal/users"
)

// FillWithDummyData creates three entries in each table
func FillWithDummyData(d *sqlx.DB, lncli lnrpc.LightningClient) error {
	gofakeit.Seed(time.Now().UnixNano())

	userCount := 10

	for index := 1; index <= userCount; index++ {
		user, err := users.Create(d,
			gofakeit.Email(),
			gofakeit.Password(true, true, true, true, true, 32),
		)
		if err != nil {
			return err
		}

		paymentCount := gofakeit.Number(1, 20)

		for index := 1; index <= paymentCount; index++ {
			_, err = payments.CreateInvoice(d, lncli, payments.NewDeposit{
				UserID: user.ID,
				Memo:   "Dummy data " + string(index),
				Amount: int64(gofakeit.Number(50, 10000)),
			})
			if err != nil {
				return err
			}
		}
	}

	return nil
}
