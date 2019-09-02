package main

import (
	"math/rand"
	"time"

	"github.com/brianvoe/gofakeit"
	"github.com/jmoiron/sqlx"
	"github.com/lightningnetwork/lnd/lnrpc"
	"gitlab.com/arcanecrypto/teslacoil/internal/payments"
	"gitlab.com/arcanecrypto/teslacoil/internal/users"
)

// FillWithDummyData creates three entries in each table
func FillWithDummyData(d *sqlx.DB, lncli lnrpc.LightningClient) error {
	gofakeit.Seed(time.Now().UnixNano())

	// Initial user
	_, err := users.Create(d,
		"test_user@example.com",
		"password",
	)
	if err != nil {
		return err
	}

	userCount := 10

	for index := 1; index <= userCount; index++ {
		user, err := users.Create(d,
			gofakeit.Email(),
			gofakeit.Password(true, true, true, true, true, 32),
		)
		if err != nil {
			return err
		}

		paymentCount := rand.Intn(20)

		for index := 1; index <= paymentCount; index++ {
			_, err = payments.CreateInvoice(d, lncli, user.ID,
				rand.Int63n(4294967), "Dummy description"+string(index),
				"Dummy memo"+string(index))
			if err != nil {
				return err
			}
		}
	}

	return nil
}
