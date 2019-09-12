package main

import (
	"math/rand"
	"time"

	"github.com/brianvoe/gofakeit"
	"github.com/lightningnetwork/lnd/lnrpc"
	"gitlab.com/arcanecrypto/teslacoil/internal/payments"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
	"gitlab.com/arcanecrypto/teslacoil/internal/users"
)

// FillWithDummyData creates three entries in each table
func FillWithDummyData(d *db.DB, lncli lnrpc.LightningClient) error {
	gofakeit.Seed(time.Now().UnixNano())

	// Initial user
	email := "test_user@example.com"
	pass := "password"
	_, err := users.GetByEmail(d, email)
	// user does not exist
	if err != nil {
		log.Debug("Creating initial user")
		firstUser, err := users.Create(d,
			email,
			pass,
		)
		if err != nil {
			return err
		}
		if err = createPaymentsForUser(d, lncli, firstUser); err != nil {
			return err
		}
	} else {
		log.Debug("Not creating initial user")
	}

	userCount := 20

	for u := 1; u <= userCount; u++ {
		user, err := users.Create(d,
			gofakeit.Email(),
			gofakeit.Password(true, true, true, true, true, 32),
		)
		if err != nil {
			return err
		}

		log.Debugf("Generated user %+v", user)
		err = createPaymentsForUser(d, lncli, user)
		if err != nil {
			return err
		}

	}

	return nil
}

func createPaymentsForUser(db *db.DB, lncli lnrpc.LightningClient,
	user users.User) error {
	paymentCount := rand.Intn(20)

	for p := 1; p <= paymentCount; p++ {
		amountSat := rand.Int63n(4294967)
		var description *string = nil
		if rand.Int()%2 == 0 {
			desc := gofakeit.HipsterSentence(8)
			description = &desc
		}

		var memo *string = nil
		if rand.Int()%2 == 0 {
			mem := gofakeit.HipsterSentence(6)
			memo = &mem
		}

		inv, err := payments.CreateInvoice(db, lncli, user.ID,
			amountSat, description, memo)

		log.Debugf("Generated invoice for user %d: %v", user.ID, inv)

		if rand.Int()%2 == 0 {

			// 60 seconds x 60 minutes x 24 hours x 7 days
			// x 12 weeks x 1000000000 nanoseconds in a second
			nanos := rand.Intn(60 * 60 * 24 * 7 * 12 * 1000000000)
			duration := time.Duration(nanos)
			paidAt := inv.CreatedAt.Add(duration)

			err := payments.MarkInvoiceAsPaid(db, user.ID,
				inv.PaymentRequest,
				paidAt)

			if err != nil {
				log.Debugf("Could not mark invoice as paid: %s", err)
				return err
			} else {
				log.Debugf("Updated invoice for user with settled_at %+v", paidAt)
			}
		}

		if err != nil {
			return err
		}
	}
	return nil
}
