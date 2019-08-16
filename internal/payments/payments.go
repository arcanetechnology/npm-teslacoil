package payments

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/ln"
	"gitlab.com/arcanecrypto/teslacoil/internal/users"
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
	open      Status = "OPEN"

	// OffchainTXTable is the tablename of offchaintx, as saved in the DB
	OffchainTXTable = "offchaintx"
)

// CreateInvoiceData is a deposit
type CreateInvoiceData struct {
	Memo      string `json:"memo"`
	AmountSat int64  `json:"amount_sat"`
}

//PayInvoiceData is the required(and optional) fields for initiating a withdrawal
type PayInvoiceData struct {
	PaymentRequest string `json:"payment_request"`
	Description    string `json:"description"`
}

// Payment is a database table
type Payment struct {
	ID             uint      `db:"id"`
	UserID         uint      `db:"user_id"`
	PaymentRequest string    `db:"payment_request"`
	Preimage       *string   `db:"preimage"`
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

// UserResponse
type UserResponse struct {
	ID        uint      `db:"id"`
	Email     string    `db:"email"`
	Balance   int       `db:"balance"`
	UpdatedAt time.Time `db:"updated_at"`
}

//UserPaymentResponse is a user payment response
type UserPaymentResponse struct {
	Payment Payment
	User    UserResponse
}

// GetAll fetches all payments
func GetAll(d *sqlx.DB, userID uint) ([]Payment, error) {
	payments := []Payment{}
	tQuery := fmt.Sprintf(`SELECT *
		FROM %s
		WHERE user_id=$1
		ORDER BY created_at ASC`, OffchainTXTable)

	err := d.Select(&payments, tQuery, userID)
	if err != nil {
		// log.Error(err)
		return payments, err
	}

	// log.Debugf("query %s for user_id %d returned %v", tQuery, userID, payments)

	return payments, nil
}

// GetByID returns a single invoice based on the id given
// It only retrieves invoices whose user_id is the same as the supplied argument
func GetByID(d *sqlx.DB, id uint64, userID uint) (Payment, error) {
	txResult := Payment{}
	tQuery := fmt.Sprintf("SELECT * FROM %s WHERE id=$1 AND user_id=$2 LIMIT 1", OffchainTXTable)

	if err := d.Get(&txResult, tQuery, id, userID); err != nil {
		// log.Error(err)
		return txResult, errors.Wrap(err, "could not get payment")
	}

	// sanity check the query
	if txResult.UserID != userID {
		err := errors.New(fmt.Sprintf("db query retrieved unexpected value, expected payment with user_id %d but got %d", userID, txResult.UserID))
		// log.Errorf(err.Error())
		return Payment{}, err
	}

	// log.Debugf("query %s for user_id %d returned %v", tQuery, userID, txResult)

	return txResult, nil
}

// CreateInvoice creates a new invoice using lnd based on the function parameter
// and inserts the newly created invoice into the database
func CreateInvoice(d *sqlx.DB, lncli ln.AddLookupInvoiceClient,
	invoiceData CreateInvoiceData, userID uint) (Payment, error) {

	// First we add an invoice given the given parameters using the ln package
	invoice, err := ln.AddInvoice(
		lncli,
		lnrpc.Invoice{
			Memo:  invoiceData.Memo,
			Value: int64(invoiceData.AmountSat),
		})
	if err != nil {
		// log.Error(err)
		return Payment{}, err
	}

	// Sanity check the invoice we just created
	if invoice.Value != int64(invoiceData.AmountSat) {
		err = errors.New("could not insert invoice, created invoice amount not equal request.Amount")
		// log.Error(err)
		return Payment{}, err
	}

	// Insert the payment into the database. Should anything inside insertPayment
	// fail, we use tx.Rollback() to revert any change made
	tx := d.MustBegin()
	invoicePreimage := hex.EncodeToString(invoice.RPreimage)
	payment, err := insertPayment(tx,
		// Payment struct with all required data to add to the DB
		Payment{
			UserID:         userID,
			Description:    invoiceData.Memo,
			AmountSat:      invoiceData.AmountSat,
			AmountMSat:     invoiceData.AmountSat * 1000,
			PaymentRequest: invoice.PaymentRequest,
			HashedPreimage: hex.EncodeToString(invoice.RHash),
			Preimage:       &invoicePreimage,
			Status:         open,
			Direction:      inbound,
		})
	if err != nil {
		// log.Error(err)
		tx.Rollback()
		return Payment{}, err
	}

	if err = tx.Commit(); err != nil {
		// log.Error(err)
		tx.Rollback()
		return Payment{}, err
	}

	return payment, nil
}

// PayInvoice pays an invoice on behalf of the user
func PayInvoice(d *sqlx.DB, lncli ln.DecodeSendClient,
	payInvoiceRequest PayInvoiceData, userID uint) (UserPaymentResponse, error) {

	payreq, err := lncli.DecodePayReq(
		context.Background(),
		&lnrpc.PayReqString{PayReq: payInvoiceRequest.PaymentRequest})
	if err != nil {
		// log.Error(err)
		return UserPaymentResponse{}, err
	}

	sendRequest := &lnrpc.SendRequest{
		PaymentRequest: payInvoiceRequest.PaymentRequest,
	}

	p := Payment{
		UserID:         userID,
		Direction:      outbound,
		PaymentRequest: payInvoiceRequest.PaymentRequest,
		Status:         succeeded,
		HashedPreimage: payreq.PaymentHash,
		Description:    payreq.Description,
		AmountSat:      payreq.NumSatoshis,
		AmountMSat:     payreq.NumSatoshis * 1000,
	}

	tx := d.MustBegin()

	// We insert the transaction in the DB before we attempt to send it to
	// avoid issues with attempte updates to the payment before it is added to
	// the DB
	payment, err := insertPayment(tx, p)
	if err != nil {
		// log.Error(err)
		tx.Rollback()
		return UserPaymentResponse{}, errors.Wrapf(
			err, "insertPayment(db, %+v)", p)
	}
	// TODO(henrik): Need to improve this step to allow for slow paying invoices.
	// See logic in lightningspin-api for possible solution
	paymentResponse, err := lncli.SendPaymentSync(context.Background(), sendRequest)
	if err != nil {
		// log.Error(err)
		return UserPaymentResponse{}, err
	}

	preImg := hex.EncodeToString(paymentResponse.PaymentPreimage)
	if paymentResponse.PaymentError == "" {
		t := time.Now()
		payment.SettledAt = &t
		payment.Status = succeeded
		payment.Preimage = &preImg
	} else {
		// TODO: Here we need to commit or roll back failed payment
		// We sould also return the reason for the failed payment
		p.Status = failed
		return UserPaymentResponse{}, errors.New(paymentResponse.PaymentError)
	}

	user := UserResponse{}
	if payment.Status == succeeded {
		user, err = updateUserBalance(tx, p.UserID, -payment.AmountSat)
		if err != nil {
			// log.Error(err)
			tx.Rollback()
			return UserPaymentResponse{}, errors.Wrapf(err,
				"PayInvoice->UpdateUserBalance(tx, %d, %d)", p.UserID, -payment.AmountSat)
		}
	}

	if err = tx.Commit(); err != nil {
		// log.Error(err)
		return UserPaymentResponse{}, errors.Wrap(err, "PayInvoice: Cound not commit")
	}

	result := UserPaymentResponse{
		Payment: payment,
		User:    user,
	}

	return result, nil
}

// QueryExecutor is based on sqlx.DB, but is an interface that only requires "Query"
type QueryExecutor interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

// UpdateUserBalance updates the users balance
func updateUserBalance(queryEx QueryExecutor, userID uint, amountSat int64) (UserResponse, error) {
	if amountSat == 0 {
		return UserResponse{}, errors.New(
			"No point in updating users balance with 0 satoshi")
	}

	updateBalanceQuery := `UPDATE users
		SET balance = balance + $1
		WHERE id = $2
		RETURNING id, email, balance`

	rows, err := queryEx.Query(updateBalanceQuery, amountSat, userID)
	if err != nil {
		// log.Error(err)
		return UserResponse{}, errors.Wrap(
			err,
			"updateUserBalance(): could not construct user update",
		)
	}

	user := UserResponse{}
	if rows.Next() {
		if err = rows.Scan(
			&user.ID,
			&user.Email,
			&user.Balance,
		); err != nil {
			// log.Error(err)
			return UserResponse{}, errors.Wrap(err, "Could not scan user returned from db")
		}
	}
	rows.Close()
	// log.Tracef("%s inserted %v", updateBalanceQuery, user)

	return user, nil
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
			ID        uint
			Balance   int
			UpdatedAt *time.Time
		}

		tQuery := fmt.Sprintf("SELECT * FROM %s WHERE payment_request=$1", OffchainTXTable)

		// Define a custom response struct to include user details
		t := Payment{}
		if err := database.Get(&t, tQuery, invoice.PaymentRequest); err != nil {
			// TODO: This is probably not a healthy way to deal with an error here
			// log.Warnf("UpdateInvoiceStatus: could not find payment: %v", err)
		}

		t.Status = Status(invoice.State.String())
		if invoice.Settled {
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
				_ = tx.Rollback()
				// log.Errorf("UpdateInvoiceStatus: could not update payment: %v", err)
				return
			}
			if rows.Next() {
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
					// log.Errorf("UpdateInvoiceStatus: could not update payment: %v", err)
					_ = tx.Rollback()
					return
				}
			}
			rows.Close() // Free up the database connection

			var u UserDetails
			rows, err = tx.NamedQuery(updateUserBalanceQuery, &t)
			if err != nil {
				// TODO: This is probably not a healthy way to deal with an error here
				// log.Errorf("UpdateInvoiceStatus: could not update user balance: %v", err)
				_ = tx.Rollback()
				return
			}
			if rows.Next() {
				_ = tx.Rollback()
				if err = rows.Scan(
					&u.ID,
					&u.Balance,
					&u.UpdatedAt,
				); err != nil {
					// TODO: This is probably not a healthy way to deal with an error here
					// log.Errorf("UpdateInvoiceStatus: could not update user balance: %v", err)
					_ = tx.Rollback()
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

	createOffchainTXQuery = `INSERT INTO offchaintx
	(user_id, payment_request, preimage, hashed_preimage, description, direction, status, amount_sat, amount_msat)
	VALUES (:user_id, :payment_request, :preimage, :hashed_preimage, :description,
			:direction, :status, :amount_sat, :amount_msat)
	RETURNING id, user_id, payment_request, preimage, hashed_preimage,
	description, direction, status, amount_sat, amount_msat, created_at, updated_at`

	// Using the above query, NamedQuery() will extract VALUES from the payment
	// variable and insert them into the query
	rows, err := tx.NamedQuery(createOffchainTXQuery, payment)
	if err != nil {
		// log.Error(err)
		return Payment{}, errors.Wrapf(
			err,
			"insertPayment->tx.NamedQuery(%s, %+v)",
			createOffchainTXQuery,
			payment,
		)
	}
	var result Payment
	if rows.Next() {
		if err = rows.Scan(
			&result.ID,
			&result.UserID,
			&result.PaymentRequest,
			&result.Preimage,
			&result.HashedPreimage,
			&result.Description,
			&result.Direction,
			&result.Status,
			&result.AmountSat,
			&result.AmountMSat,
			&result.CreatedAt,
			&result.UpdatedAt,
		); err != nil {
			// log.Error(err)
			return result, errors.Wrapf(err,
				"insertPayment->rows.Next(), Problem row = %+v", result)
		}

	}
	rows.Close() // Free up the database connection

	// log.Debugf("query %s inserted %v", createOffchainTXQuery, payment)

	return result, nil
}

// func sanityCheckPayment(tempPayment Payment, payment Payment) error {
// 	if tempPayment.UserID != payment.UserID {
// 		return errors.New(fmt.Sprintf("tempPayment.UserID %d not equal payment.UserID %d", tempPayment.UserID, payment.UserID))
// 	}
// 	if tempPayment.PaymentRequest != payment.PaymentRequest {
// 		return errors.New(fmt.Sprintf("tempPayment.PaymentRequest %s not equal payment.PaymentRequest %s", tempPayment.PaymentRequest, payment.PaymentRequest))
// 	}
// 	// Temporarily disabled due to trouble with nil dereferencing
// 	// if tempPayment.Preimage != payment.Preimage {
// 	// 	return errors.New(fmt.Sprintf("tempPayment.Preimage %v not equal payment.Preimage %v", tempPayment.Preimage, payment.Preimage))
// 	// }
// 	if tempPayment.HashedPreimage != payment.HashedPreimage {
// 		return errors.New(fmt.Sprintf("tempPayment.HashedPreimage %s not equal payment.HashedPreimage %s", tempPayment.HashedPreimage, payment.HashedPreimage))
// 	}
// 	if tempPayment.Description != payment.Description {
// 		return errors.New(fmt.Sprintf("tempPayment.Description %s not equal payment.Description %s", tempPayment.Description, payment.Description))
// 	}
// 	if tempPayment.Direction != payment.Direction {
// 		return errors.New(fmt.Sprintf("tempPayment.Direction %v not equal payment.Direction %v", tempPayment.Direction, payment.Direction))
// 	}
// 	if tempPayment.Status != payment.Status {
// 		return errors.New(fmt.Sprintf("tempPayment.Status %v not equal payment.Status %v", tempPayment.Status, payment.Status))
// 	}
// 	if tempPayment.AmountSat != payment.AmountSat {
// 		return errors.New(fmt.Sprintf("tempPayment.AmountSat %d not equal payment.AmountSat %d", tempPayment.AmountSat, payment.AmountSat))
// 	}
// 	if tempPayment.AmountMSat != payment.AmountMSat {
// 		return errors.New(fmt.Sprintf("tempPayment.AmountMSat %d not equal payment.AmountMSat %d", tempPayment.AmountMSat, payment.AmountMSat))
// 	}
// 	if tempPayment.AmountMSat != (payment.AmountSat * 1000) {
// 		return errors.New(fmt.Sprintf("tempPayment.AmountMSat %d not equal (payment.AmountSat * 1000) %d", tempPayment.AmountMSat, payment.AmountSat*1000))
// 	}
// 	return nil
// }
