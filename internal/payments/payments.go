package payments

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/lpp/internal/platform/ln"
	"gitlab.com/arcanecrypto/lpp/internal/users"
)

// Direction is the direction of a lightning payment
type Direction string

// Status is the status of a lightning payment
type Status string

const (
	inbound  Direction = "INBOUND"
	outbound Direction = "OUTBOUND"

	succeeded Status = "SUCCEEDED"
	failed    Status = "FAILED"
	inflight  Status = "IN-FLIGHT"

	// OffchainTXTable is the tablename of offchaintx, as saved in the DB
	OffchainTXTable = "offchaintx"
)

// CreateInvoiceData is a deposit
type CreateInvoiceData struct {
	UserID    uint64 `json:"user_id"`
	Memo      string `json:"memo"`
	AmountSat int64  `json:"amount_sat"`
}

//PayInvoiceData is the required(and optional) fields for initiating a withdrawal
type PayInvoiceData struct {
	UserID         uint64 `json:"user_id"` // userID of the user this withdrawal belongs to
	PaymentRequest string `json:"payment_request"`
	Description    string `json:"description"`
	AmountSat      int64  `json:"amount_sat"`
}

// Payment is a database table
type Payment struct {
	ID             uint64    `db:"id"`
	UserID         uint64    `db:"user_id"`
	PaymentRequest string    `db:"payment_request"`
	Preimage       string    `db:"preimage"`
	HashedPreimage string    `db:"hashed_preimage"`
	CallbackURL    *string   `db:"callback_url"`
	Status         Status    `db:"status"`
	Description    string    `db:"description"`
	Direction      Direction `db:"direction"`
	AmountSat      int64     `db:"amount_sat"`
	AmountMSat     int64     `db:"amount_msat"`
	// SettledAt is a pointer because it can be null, and inserting null in
	// something not a pointer when querying the db is not possible
	SettledAt *time.Time `db:"settled_at"` // If not 0 or nul, it means the invoice is settled
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

//UserPaymentResponse is a user payment response
type UserPaymentResponse struct {
	Payment
	users.UserResponse
}

// GetAll fetches all payments
func GetAll(d *sqlx.DB) ([]Payment, error) {
	payments := []Payment{}
	tQuery := fmt.Sprintf(`SELECT *
		FROM %s
		ORDER BY created_at ASC`, OffchainTXTable)

	err := d.Select(&payments, tQuery)
	if err != nil {
		return payments, err
	}

	return payments, nil
}

// GetByID returns a single invoice based on the id given
func GetByID(d *sqlx.DB, id uint64) (Payment, error) {
	txResult := Payment{}
	tQuery := fmt.Sprintf("SELECT * FROM %s WHERE id=$1 LIMIT 1", OffchainTXTable)

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
			AmountSat:      invoiceData.AmountSat,
			AmountMSat:     invoiceData.AmountSat * 1000,
			PaymentRequest: invoice.PaymentRequest,
			HashedPreimage: hex.EncodeToString(invoice.RHash),
			Preimage:       hex.EncodeToString(invoice.RPreimage),
			Status:         inflight,
			Direction:      inbound,
		})
	if err != nil {
		tx.Rollback()
		return Payment{}, err
	}
	fmt.Println("INSERTED")

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
		&lnrpc.PayReqString{PayReq: withdrawalRequest.PaymentRequest})
	if err != nil {
		return UserPaymentResponse{}, err
	}

	sendRequest := &lnrpc.SendRequest{
		PaymentRequest: withdrawalRequest.PaymentRequest,
	}

	p := Payment{
		UserID:         withdrawalRequest.UserID,
		Direction:      outbound,
		PaymentRequest: withdrawalRequest.PaymentRequest,

		HashedPreimage: payreq.PaymentHash,
		Description:    payreq.Description,
		AmountSat:      payreq.NumSatoshis,
		AmountMSat:     int64(payreq.NumSatoshis * 1000),
	}
	// TODO(henrik): Need to improve this step to allow for slow paying invoices.
	// See logic in lightningspin-api for possible solution
	paymentResponse, err := lncli.SendPaymentSync(context.Background(), sendRequest)
	if err != nil {
		return UserPaymentResponse{}, nil
	}

	if paymentResponse.PaymentError == "" {
		t := time.Now()
		p.SettledAt = &t
		p.Status = "SETTLED"
		p.Preimage = hex.EncodeToString(paymentResponse.PaymentPreimage)
	} else {
		p.Status = failed
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
// TODO: Give better error message if payment was just created, but not yet
// inserted into the database
// This happens because the flow is this:
// 1. Payment is created
// 2. A notification is sent to the SubscribeInvoices stream
// 3. UpdateInvoiceStatus is run, looking for the newly created payment. It
// errors with 'sql: no rows in result set' on the first database.Get() because
// the invoice is not yet inserted into the database.
// 4. Payment is inserted into the database
func UpdateInvoiceStatus(invoiceUpdatesCh chan lnrpc.Invoice, database *sqlx.DB) {
	for {
		invoice := <-invoiceUpdatesCh

		type UserDetails struct {
			ID        uint64
			Balance   int64
			UpdatedAt *time.Time
		}

		// Define a custom response struct to include user details
		t := Payment{}

		tQuery := fmt.Sprintf("SELECT * FROM %s WHERE payment_request=$1", OffchainTXTable)

		if err := database.Get(&t, tQuery, invoice.PaymentRequest); err != nil {
			fmt.Println("1")
			// TODO: This is probably not a healthy way to deal with an error here
			log.Println(errors.Wrap(err, "UpdateInvoiceStatus: Could not find payment").Error())
			return
		}

		t.Status = Status(invoice.State.String())
		if invoice.Settled {
			fmt.Println("2")
			time := time.Now()
			t.SettledAt = &time

			updateOffchainTxQuery := fmt.Sprintf(`UPDATE %s 
				SET status = :status, settled_at = :settled_at 
				WHERE hashed_preimage = :hashed_preimage
				RETURNING id, user_id, payment_request, preimage, hashed_preimage,
						  description, direction, status, amount_sat, amount_msat,
						  created_at, updated_at`, OffchainTXTable)

			updateUserBalanceQuery := fmt.Sprintf(`UPDATE %s 
				SET balance = :amount + balance
				WHERE id = :user_id
				RETURNING id, balance, updated_at`, users.UsersTable)
			tx := database.MustBegin()
			rows, err := tx.NamedQuery(updateOffchainTxQuery, &t)
			if err != nil {
				fmt.Println("3")
				_ = tx.Rollback()
				log.Println(errors.Wrap(err,
					"UpdateInvoiceStatus: Could not update payment").Error())
				return
			}
			if rows.Next() {
				fmt.Println("4")
				if err = rows.Scan(
					&t.ID,
					&t.UserID,
					&t.PaymentRequest,
					&t.Preimage,
					&t.HashedPreimage,
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
					log.Println(errors.Wrap(err,
						"UpdateInvoiceStatus: Could not update payment").Error())
					return
				}
			}
			rows.Close() // Free up the database connection

			var u UserDetails
			rows, err = tx.NamedQuery(updateUserBalanceQuery, &t)
			if err != nil {
				fmt.Println("5")
				_ = tx.Rollback()
				// TODO: This is probably not a healthy way to deal with an error here
				log.Println(errors.Wrap(err,
					"UpdateInvoiceStatus: Could not update user balance").Error())
				return
			}
			if rows.Next() {
				fmt.Println("6")
				_ = tx.Rollback()
				if err = rows.Scan(
					&u.ID,
					&u.Balance,
					&u.UpdatedAt,
				); err != nil {
					_ = tx.Rollback()
					log.Println(errors.Wrap(err,
						"UpdateInvoiceStatus: Could not read updated user detials").Error())
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

	if payment.Direction == inbound {
		createOffchainTXQuery = fmt.Sprintf(`INSERT INTO %s
		(user_id, payment_request, preimage, hashed_preimage, description, direction, status, amount_sat, amount_msat)
		VALUES (:user_id, :payment_request, :preimage, :hashed_preimage, :description,
				:direction, :status, :amount_sat, :amount_msat)
		RETURNING id, user_id, payment_request, preimage, hashed_preimage,
		description, direction, status, amount_sat, amount_msat, created_at, updated_at`, OffchainTXTable)
	} else {
		createOffchainTXQuery = fmt.Sprintf(`INSERT INTO %s 
		(user_id, payment_request, description, direction, status, amount_sat, amount_msat)
		VALUES (:user_id, :payment_request, :description, :direction, :status, :amount_sat, :amount_msat)
		RETURNING id, user_id, payment_request, preimage, hashed_preimage,
		description, direction, status, amount_sat, amount_msat, created_at, updated_at`, OffchainTXTable)
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
			&tempPayment.Preimage,
			&tempPayment.HashedPreimage,
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
	if tempPayment.Preimage != payment.Preimage {
		return false
	}
	if tempPayment.HashedPreimage != payment.HashedPreimage {
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
	if tempPayment.AmountMSat != (payment.AmountSat * 1000) {
		return false
	}
	return true
}
