package dummy

import (
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/models/transactions"

	"github.com/brianvoe/gofakeit"
	"github.com/lightningnetwork/lnd/lnrpc"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/models/users"
)

var log = build.Log

// FillWithData populates the database with dummy data
func FillWithData(d *db.DB, lncli lnrpc.LightningClient, onlyOnce bool) error {
	log.WithField("onlyOnce", onlyOnce).Info("Populating DB with dummy data")
	gofakeit.Seed(time.Now().UnixNano())

	if foundUsers, _ := users.GetAll(d); onlyOnce {
		if len(foundUsers) != 0 {
			log.Info("DB has data, not populating with further data")
			return nil
		}
	}
	// Initial user
	email := "test_user@example.com"
	pass := "password"
	_, err := users.GetByEmail(d, email)
	// user does not exist
	if err != nil {
		log.Debug("Creating initial user")
		first := gofakeit.FirstName()
		last := gofakeit.LastName()
		firstUser, err := users.Create(d, users.CreateUserArgs{
			Email:     email,
			Password:  pass,
			FirstName: &first,
			LastName:  &last,
		})
		if err != nil {
			return err
		}

		token, err := users.GetEmailVerificationToken(d, firstUser.Email)
		if err != nil {
			return errors.Wrap(err, "could not get email verification token")
		}

		verified, err := users.VerifyEmail(d, token)
		if err != nil {
			return errors.Wrap(err, "could not verify email")
		}

		if err = createPaymentsForUser(d, lncli, verified); err != nil {
			return err
		}
	} else {
		log.Debug("Not creating initial user")
	}

	userCount := 20
	createUser := func(wg *sync.WaitGroup) {
		defer wg.Done()

		var first string
		var last string
		if gofakeit.Bool() {
			first = gofakeit.FirstName()
		}

		if gofakeit.Bool() {
			last = gofakeit.LastName()
		}

		user, err := users.Create(d, users.CreateUserArgs{
			Email:     gofakeit.Email(),
			Password:  gofakeit.Password(true, true, true, true, true, 32),
			FirstName: &first,
			LastName:  &last,
		})
		if err != nil {
			log.WithError(err).Error("Could not create user")
			return
		}

		token, err := users.GetEmailVerificationToken(d, user.Email)
		if err != nil {
			log.WithError(err).Error("Could not get email verification token")
			return
		}

		verified, err := users.VerifyEmail(d, token)
		if err != nil {
			log.WithError(err).Error("Could not verify email")
			return
		}

		log.WithField("userId", verified.ID).Debug("Generated user")

		if err := createPaymentsForUser(d, lncli, verified); err != nil {
			log.WithError(err).WithField("user", verified).Error("Could not create payments")
			return
		}
		log.WithField("userId", verified.ID).Debug("Created payments for user")
	}

	var wg sync.WaitGroup
	for u := 1; u <= userCount; u++ {
		wg.Add(1)
		go createUser(&wg)
	}

	wg.Wait()
	log.WithField("userCount", userCount).Info("Created dummy data")

	return nil
}

func createPaymentsForUser(db *db.DB, lncli lnrpc.LightningClient,
	user users.User) error {
	paymentCount := gofakeit.Number(0, 20)

	for p := 1; p <= paymentCount; p++ {
		amountSat := gofakeit.Number(0, 4294967)
		var description string
		if gofakeit.Bool() {
			desc := gofakeit.HipsterSentence(8)
			description = desc
		}

		var memo string
		if gofakeit.Bool() {
			mem := gofakeit.HipsterSentence(6)
			memo = mem
		}

		inv, err := transactions.NewOffchain(db, lncli, transactions.NewOffchainOpts{
			UserID:      user.ID,
			AmountSat:   int64(amountSat),
			Memo:        memo,
			Description: description,
		})

		if err != nil {
			return err
		}

		log.Debugf("Generated invoice for user %d: %v", user.ID, inv)

		if gofakeit.Bool() {

			// 24 hours x 7 days x 12 weeks
			const twelveWeeks = time.Hour * 24 * 7 * 12
			twelveWeeksNanos := twelveWeeks.Nanoseconds()
			nanos := gofakeit.Number(0, int(twelveWeeksNanos))
			duration := time.Duration(nanos)
			paidAt := inv.CreatedAt.Add(duration)

			paid, err := inv.MarkAsPaid(db, paidAt)

			if err != nil {
				log.WithError(err).Debug("Could not mark invoice as paid")
				return err
			} else {
				log.WithFields(logrus.Fields{
					"settledAt": paid.SettledAt,
					"userId":    paid.UserID,
				}).Debug("Updated invoice for user")
			}
		}
	}
	return nil
}
