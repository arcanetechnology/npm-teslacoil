package payments

import (
	"context"
	"encoding/hex"
	"log"
	"time"

	"github.com/btcsuite/btcutil"
	"github.com/jmoiron/sqlx"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/lpp/internal/platform/ln"
	"gitlab.com/arcanecrypto/lpp/internal/users"
)

// Direction is the direction of a lightning payment
type Direction string

const (
	inbound  Direction = "inbound"  //nolint
	outbound Direction = "outbound" //nolint
)

// CreateInvoiceData is a deposit
type CreateInvoiceData struct {
	UserID    uint64 `json:"user_id"`
	Memo      string `json:"memo"`
	AmountSat int64  `json:"amount_sat"`
}

//PayInvoiceData is the required(and optional) fields for initiating a withdrawal
type PayInvoiceData struct {
	UserID      uint64 `json:"user_id"` // userID of the user this withdrawal belongs to
	Invoice     string
	Status      string
	Description string
	Direction   Direction
	AmountSat   int64
	AmountMSat  lnwire.MilliSatoshi
}

// Payment is a database table
type Payment struct {
	ID             uint64              `db:"id"`
	UserID         uint64              `db:"user_id"`
	PaymentRequest string              `db:"invoice"`
	PreImage       string              `db:"pre_image"`
	HashedPreImage string              `db:"hashed_pre_image"`
	CallbackURL    *string             `db:"callback_url"`
	Status         string              `db:"status"`
	Description    string              `db:"description"`
	Direction      Direction           `db:"direction"`
	AmountSat      int64               `db:"amount_sat"`
	AmountMSat     lnwire.MilliSatoshi `db:"amount_msat"`
	SettledAt      time.Time           `db:"settled_at"` // If not 0 or null, it means the invoice is settled
	CreatedAt      time.Time           `db:"created_at"`
	UpdatedAt      time.Time           `db:"updated_at"`
	DeletedAt      *time.Time          `db:"deleted_at"`
}

//UserPaymentResponse is a user payment response
type UserPaymentResponse struct {
	Payment
	users.UserResponse
}

// GetAll fetches all payments
func GetAll(d *sqlx.DB) ([]Payment, error) {
	payments := []Payment{}
	const tQuery = `SELECT t.* 
		FROM payments AS t 
		ORDER BY t.created_at ASC`

	err := d.Select(&payments, tQuery)
	if err != nil {
		return payments, err
	}

	return payments, nil
}

// GetByID returns a single invoice based on the id given
func GetByID(d *sqlx.DB, id uint64) (Payment, error) {
	txResult := Payment{}
	tQuery := `SELECT * FROM payments WHERE id=$1 LIMIT 1`

	if err := d.Get(&txResult, tQuery, id); err != nil {
		return txResult, errors.Wrap(err, "Could not get payment")
	}

	return txResult, nil
}

// CreateInvoice creates a new invoice using lnd based on the function parameter
// and inserts the newly created invoice into the database
func CreateInvoice(d *sqlx.DB, lncli lnrpc.LightningClient,
	invoiceData CreateInvoiceData) (Payment, error) {

	// First we add an invoice given the given parameters using the ln package
	invoice, err := ln.AddInvoice(lncli, lnrpc.Invoice{
		Memo: invoiceData.Memo, Value: invoiceData.AmountSat,
	})
	if err != nil {
		return Payment{}, err
	}

	// Sanity check the invoice we just created
	if invoice.Value != invoiceData.AmountSat {
		log.Fatal("could not insert invoice, created invoice amount not equal request.Amount")
	}

	// Insert the payment into the database. Should anything inside insertPayment
	// fail, we use tx.Rollback() to revert any change made
	tx := d.MustBegin()
	payment, err := insertPayment(tx,
		// Payment struct with all required data to add to the DB
		Payment{
			UserID:         invoiceData.UserID,
			Description:    invoiceData.Memo,
			AmountMSat:     lnwire.MilliSatoshi(invoiceData.AmountSat),
			AmountSat:      invoiceData.AmountSat,
			PaymentRequest: invoice.PaymentRequest,
			HashedPreImage: hex.EncodeToString(invoice.RHash),
			PreImage:       hex.EncodeToString(invoice.RPreimage),
			Status:         "unpaid",
			Direction:      Direction("inbound"), // All created invoices are inbound
		})
	if err != nil {
		tx.Rollback()
		return Payment{}, err
	}

	if err = tx.Commit(); err != nil {
		tx.Rollback()
		return Payment{}, err
	}

	return payment, nil
}

// PayInvoice pays an invoice on behalf of the user
func PayInvoice(d *sqlx.DB, lncli lnrpc.LightningClient,
	withdrawalRequest PayInvoiceData) (UserPaymentResponse, error) {

	payreq, err := lncli.DecodePayReq(
		context.Background(),
		&lnrpc.PayReqString{PayReq: withdrawalRequest.Invoice})
	if err != nil {
		return UserPaymentResponse{}, err
	}

	sendRequest := &lnrpc.SendRequest{
		PaymentRequest: withdrawalRequest.Invoice,
	}

	p := Payment{
		UserID:         withdrawalRequest.UserID,
		Direction:      Direction("outbound"),
		PaymentRequest: withdrawalRequest.Invoice,

		HashedPreImage: payreq.PaymentHash,
		Description:    payreq.Description,
		AmountSat:      payreq.NumSatoshis,
		AmountMSat:     lnwire.MilliSatoshi(payreq.NumSatoshis),
	}
	// TODO(henrik): Need to improve this step to allow for slow paying invoices.
	// See logic in lightningspin-api for possible solution
	paymentResponse, err := lncli.SendPaymentSync(context.Background(), sendRequest)
	if err != nil {
		return UserPaymentResponse{}, nil
	}

	if paymentResponse.PaymentError == "" {
		p.SettledAt = time.Now()
		p.Status = "SETTLED"
		p.PreImage = hex.EncodeToString(paymentResponse.PaymentPreimage)
	} else {
		p.Status = paymentResponse.PaymentError
		p.Status = "FAILED"
		return UserPaymentResponse{}, nil
	}

	tx := d.MustBegin()

	payment, err := insertPayment(tx, p)
	if err != nil {
		tx.Rollback()
		return UserPaymentResponse{}, err
	}

	var user users.UserResponse
	if payment.Status == "SETTLED" {
		user, err = users.UpdateUserBalance(d, payment.UserID)
		if err != nil {
			tx.Rollback()
			return UserPaymentResponse{}, err
		}
	}

	if err = tx.Commit(); err != nil {
		return UserPaymentResponse{}, errors.Wrap(err, "PayInvoice: Cound not commit")
	}

	return UserPaymentResponse{
		UserResponse: user,
		Payment:      payment,
	}, nil
}

// UpdateInvoiceStatus continually listens for messages and updated the user balance
// PS: This is most likely done in a horrible way. Must be refactored.
// We also need to keep track of the last received messages from lnd
func UpdateInvoiceStatus(invoiceUpdatesCh chan lnrpc.Invoice, database *sqlx.DB) {
	for {
		invoice := <-invoiceUpdatesCh

		type UserDetails struct {
			ID        uint64
			Balance   int64
			UpdatedAt *time.Time
		}

		// Define a custom response struct to include user details
		t := struct {
			Payment
			User UserDetails
		}{}

		tQuery := `SELECT * FROM payments as t WHERE invoice=$1 LIMIT 1`
		if err := database.Get(&t, tQuery, invoice.PaymentRequest); err != nil {
			// TODO: This is probably not a healthy way to deal with an error here
			log.Println(errors.Wrap(err, "UpdateInvoiceStatus: Could not find payment").Error())
			return
		}

		t.Status = invoice.State.String()
		if invoice.Settled {
			t.SettledAt = time.Now()

			updateOffchainTxQuery := `UPDATE offchaintx 
				SET status = :status, settled_at = :settled_at 
				WHERE hashed_pre_image = :hashed_pre_image
				RETURNING id, user_id, invoice, pre_image, hashed_pre_image,
						  description, direction, status, amount,
						  created_at, updated_at`

			updateUserBalanceQuery := `UPDATE users 
				SET balance = :amount + balance
				WHERE id = :user_id
				RETURNING id, balance, updated_at`
			tx := database.MustBegin()
			rows, err := tx.NamedQuery(updateOffchainTxQuery, &t)
			if err != nil {
				_ = tx.Rollback()
				log.Println(errors.Wrap(err, "UpdateInvoiceStatus: Could not update payment").Error())
				return
			}
			if rows.Next() {
				if err = rows.Scan(
					&t.ID,
					&t.UserID,
					&t.PaymentRequest,
					&t.PreImage,
					&t.HashedPreImage,
					&t.Description,
					&t.Direction,
					&t.Status,
					// TOOD: Danger we need to split this into Msats and sats
					&t.AmountSat,
					&t.AmountMSat,
					&t.CreatedAt,
					&t.UpdatedAt,
				); err != nil {
					_ = tx.Rollback()
					log.Println(errors.Wrap(err, "UpdateInvoiceStatus: Could not update payment").Error())
					return
				}
			}
			rows.Close() // Free up the database connection

			rows, err = tx.NamedQuery(updateUserBalanceQuery, &t)
			if err != nil {
				_ = tx.Rollback()
				// TODO: This is probably not a healthy way to deal with an error here
				log.Println(errors.Wrap(err, "UpdateInvoiceStatus: Could not update user balance").Error())
				return
			}
			if rows.Next() {
				if err = rows.Scan(
					&t.User.ID,
					&t.User.Balance,
					&t.User.UpdatedAt,
				); err != nil {
					_ = tx.Rollback()
					log.Println(errors.Wrap(err, "UpdateInvoiceStatus: Could not read updated user detials").Error())
					return
				}
			}
			rows.Close() // Free up the database connection
			_ = tx.Commit()
			// TODO: Here we need to call the callback with the response.
		}
	}
}

func insertPayment(tx *sqlx.Tx, payment Payment) (Payment, error) {
	var createOffchainTXQuery string

	if payment.Direction == Direction("inbound") {
		createOffchainTXQuery = `INSERT INTO offchaintx
		(user_id, payment_request, pre_image, hashed_pre_image, description, direction, status, amount)
		VALUES (:user_id, :payment_request, :pre_image, :hashed_pre_image, :description,
				:direction, :status, :amount)
		RETURNING id, user_id, payment_request, pre_image, hashed_pre_image,
		description, direction, status, amount_sat, amount_msat, created_at, updated_at`
	} else {
		createOffchainTXQuery = `INSERT INTO payments 
		(user_id, invoice, description, direction, status, amount)
		VALUES (:user_id, :invoice, :description, :direction, :status, :amount)
		RETURNING id, user_id, payment_request, pre_image, hashed_pre_image,
		description, direction, status, amount_sat, amount_msat, created_at, updated_at`
	}

	// Using the above query, NamedQuery() will extract VALUES from the payment
	// variable and insert them into the query
	rows, err := tx.NamedQuery(createOffchainTXQuery, payment)
	if err != nil {
		return Payment{}, err
	}
	if rows.Next() {
		var tempPayment Payment
		if err = rows.Scan(
			&tempPayment.ID,
			&tempPayment.UserID,
			&tempPayment.PaymentRequest,
			&tempPayment.PreImage,
			&tempPayment.HashedPreImage,
			&tempPayment.Description,
			&tempPayment.Direction,
			&tempPayment.Status,
			&tempPayment.AmountSat,
			&tempPayment.AmountMSat,
			&tempPayment.CreatedAt,
			&tempPayment.UpdatedAt,
		); err != nil {
			return Payment{}, err
		}
		if !sanityCheckPayment(tempPayment, payment) {
			return Payment{}, errors.New("Sanity check for inserted invoice failed")
		}
		payment = tempPayment
	}
	rows.Close() // Free up the database connection

	return payment, nil
}

func sanityCheckPayment(tempPayment Payment, payment Payment) bool {
	if tempPayment.UserID != payment.UserID {
		return false
	}
	if tempPayment.PaymentRequest != payment.PaymentRequest {
		return false
	}
	if tempPayment.PreImage != payment.PreImage {
		return false
	}
	if tempPayment.HashedPreImage != payment.HashedPreImage {
		return false
	}
	if tempPayment.Description != payment.Description {
		return false
	}
	if tempPayment.Direction != payment.Direction {
		return false
	}
	if tempPayment.Status != payment.Status {
		return false
	}
	if tempPayment.AmountSat != payment.AmountSat {
		return false
	}
	if tempPayment.AmountMSat != payment.AmountMSat {
		return false
	}
	if tempPayment.AmountMSat.ToSatoshis().ToUnit(btcutil.AmountSatoshi) != float64(payment.AmountSat) {
		return false
	}
	return true
}
