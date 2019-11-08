package dummy

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"gitlab.com/arcanecrypto/teslacoil/build"

	"github.com/brianvoe/gofakeit"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/models/transactions"
	"gitlab.com/arcanecrypto/teslacoil/models/users"
	"gitlab.com/arcanecrypto/teslacoil/models/users/balance"
	"gitlab.com/arcanecrypto/teslacoil/testutil/txtest"
)

var log = build.AddSubLogger("DMMY")

func init() {
	rand.Seed(time.Now().Unix())
}

// FillWithData populates the database with dummy data
func FillWithData(d *db.DB, onlyOnce bool) error {
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
			return fmt.Errorf("could not get email verification token: %w", err)
		}

		verified, err := users.VerifyEmail(d, token)
		if err != nil {
			return fmt.Errorf("could not verify email: %w", err)
		}

		createTxsForUser(d, verified)
	} else {
		log.Debug("Not creating initial user")
	}

	userCount := 20

	var wg sync.WaitGroup
	for u := 1; u <= userCount; u++ {
		wg.Add(1)
		go createUser(d, &wg)
	}

	wg.Wait()
	log.WithField("userCount", userCount).Info("Created dummy data")

	return nil
}

func createUser(d *db.DB, wg *sync.WaitGroup) {
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

	go func() {
		createTxsForUser(d, verified)
		log.WithField("userId", verified.ID).Debug("Created payments for user")
		wg.Done()
	}()
}

const maxTxs = 40
const minTxs = 20

func genOffchain(user users.User) transactions.Offchain {
	tx := txtest.MockOffchain(user.ID)
	// we want a bias towards incoming TXs, so we end up with positive balance
	if rand.Intn(10) > 2 && tx.Direction == transactions.OUTBOUND {
		return genOffchain(user)
	}
	return tx
}

func genOnchain(user users.User) transactions.Onchain {
	tx := txtest.MockOnchain(user.ID)
	// we want a bias towards incoming TXs, so we end up with positive balance
	if rand.Intn(10) > 2 && tx.Direction == transactions.OUTBOUND {
		return genOnchain(user)
	}
	return tx

}

func createTxsForUser(db *db.DB, user users.User) {
	txCount := gofakeit.Number(minTxs, maxTxs)

	for p := 1; p <= txCount; p++ {
		if gofakeit.Bool() {
			off := genOffchain(user)
			inserted, err := transactions.InsertOffchain(db, off)
			if err != nil {
				log.WithError(err).Error("Could not insert offchain dummy TX")
			} else {
				log.WithFields(logrus.Fields{
					"userId": user.ID,
					"id":     inserted.ID,
				}).Debug("Inserted dummy offchain TX")
			}
		} else {
			on := genOnchain(user)
			inserted, err := transactions.InsertOnchain(db, on)
			if err != nil {
				log.WithError(err).Error("Could not insert onchain dummy TX")
			} else {
				log.WithFields(logrus.Fields{
					"userId": user.ID,
					"id":     inserted.ID,
				}).Debug("Inserted dummy on TX")
			}
		}
	}

	// verify that the user has positive balance, otherwise keep on chugging
	// until we're positive
	_, err := balance.ForUser(db, user.ID)
	if errors.Is(err, balance.ErrUserHasNegativeBalance) {
		createTxsForUser(db, user)
	}
}
