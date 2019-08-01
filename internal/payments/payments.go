package payments

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/btcsuite/btcutil"
	"github.com/jmoiron/sqlx"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/lpp/internal/ln"
)

// Direction is the direction of a lightning payment
type Direction string

const (
	inbound  Direction = "inbound"  //nolint
	outbound Direction = "outbound" //nolint
)

//NewDeposit is a new deposit
type NewDeposit struct {
	UserID     uint64              `json:"user_id"`
	Memo       string              `json:"memo"`
	AmountSat  int64               `json:"amount_sat"`
	AmountMSat lnwire.MilliSatoshi `json:"amount_msat"`
}

//NewWithdrawal is the required(and optional) fields for initiating a withdrawal
type NewWithdrawal struct {
	UserID      uint64 `json:"user_id"` // userID of the user this withdrawal belongs to
	Invoice     string
	Status      string
	Description string
	Direction   Direction
	AmountSat   int64
	AmountMSat  lnwire.MilliSatoshi
}

func Hi() {
	x := NewWithdrawal{
		AmountSat:  int64(btcutil.Amount(5).ToUnit(btcutil.AmountSatoshi)),
		AmountMSat: lnwire.NewMSatFromSatoshis(5),
	}

	fmt.Println("---------------------------------------------------------------")
	fmt.Println(x.AmountSat)
	fmt.Println(x.AmountMSat)
	fmt.Println("---------------------------------------------------------------")
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
	SettledAt      *time.Time          `db:"settled_at"` // If this is not 0 or null, it means the invoice is settled
	CreatedAt      time.Time           `db:"created_at"`
	UpdatedAt      time.Time           `db:"updated_at"`
	DeletedAt      *time.Time          `db:"deleted_at"`
}

//UserPaymentResponse is a user payment response
type UserPaymentResponse struct {
	Payment
	User struct {
		ID        uint64     `db:"u.id"`
		Balance   int64      `db:"u.balance"`
		UpdatedAt *time.Time `db:"u.updated_at"`
	}
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
	trxResult := Payment{}
	tQuery := `SELECT * FROM payments WHERE id=$1 LIMIT 1`

	if err := d.Get(&trxResult, tQuery, id); err != nil {
		return trxResult, errors.Wrap(err, "Could not get payment")
	}

	return trxResult, nil
}

// CreateInvoice inserts a invoice into the database creates a new invoice
func CreateInvoice(d *sqlx.DB, lncli lnrpc.LightningClient, newPayment NewDeposit) (
	Payment, error) {

	invoice, err := ln.AddInvoice(lncli, &ln.AddInvoiceData{
		Memo: newPayment.Memo, Amount: newPayment.AmountSat,
	})
	if err != nil {
		return Payment{}, err
	}

	// Sanity check the invoice we just created
	if invoice.Value != newPayment.AmountSat {
		log.Fatal("could not insert invoice, created invoice amount not equal request.Amount")
	}

	preimage := hex.EncodeToString(invoice.RPreimage)
	payment := Payment{
		UserID:         newPayment.UserID,
		PaymentRequest: invoice.PaymentRequest,
		Description:    newPayment.Memo,
		PreImage:       preimage,
		HashedPreImage: hex.EncodeToString(invoice.RHash),
		Status:         "unpaid",
		Direction:      Direction("inbound"), // All created invoices are inbound
		// TOOD: Danger we need to split this into Msats and sats
		AmountMSat: lnwire.MilliSatoshi(newPayment.AmountSat),
		AmountSat:  newPayment.AmountSat,
	}

	trxCreateQuery := `INSERT INTO payments 
		(user_id, invoice, pre_image, hashed_pre_image, description, direction, status, amount)
		VALUES (:user_id, :invoice, :pre_image, :hashed_pre_image, :description,
				:direction, :status, :amount)
		RETURNING id, user_id, invoice, pre_image, hashed_pre_image, description, direction, status, amount,
				  created_at, updated_at`

	rows, err := d.NamedQuery(trxCreateQuery, payment)
	if err != nil {
		return payment, err
	}
	if rows.Next() {
		if err = rows.Scan(
			&payment.ID,
			&payment.UserID,
			&payment.PaymentRequest,
			&payment.PreImage,
			&payment.HashedPreImage,
			&payment.Description,
			&payment.Direction,
			&payment.Status,
			// TOOD: Danger we need to split this into Msats and sats
			&payment.AmountSat,
			&payment.AmountMSat,
			&payment.CreatedAt,
			&payment.UpdatedAt,
		); err != nil {
			return payment, err
		}
	}
	rows.Close() // Free up the database connection

	return payment, nil
}

// PayInvoice pay an invoice on behalf of the user
func PayInvoice(d *sqlx.DB, lncli lnrpc.LightningClient, withdrawalRequest NewWithdrawal) (
	UserPaymentResponse, error) {

	// Define a custom response struct to include user details
	payment := UserPaymentResponse{}

	payment.UserID = withdrawalRequest.UserID
	payment.Direction = Direction("outbound") // All paid invoices are outbound
	payment.PaymentRequest = withdrawalRequest.Invoice

	payRequest, err := lncli.DecodePayReq(
		context.Background(),
		&lnrpc.PayReqString{PayReq: withdrawalRequest.Invoice})
	if err != nil {
		return payment, err
	}

	payment.HashedPreImage = payRequest.PaymentHash
	// payment.Destination = payRequest.Destination
	payment.Description = payRequest.Description
	payment.AmountSat = payRequest.NumSatoshis
	payment.AmountMSat = lnwire.MilliSatoshi(payRequest.NumSatoshis)

	sendRequest := &lnrpc.SendRequest{
		PaymentRequest: withdrawalRequest.Invoice,
	}

	// TODO: Need to improve this step to allow for slow paying invoices.
	paymentResponse, err := lncli.SendPaymentSync(context.Background(), sendRequest)
	if err != nil {
		return payment, nil
	}

	if paymentResponse.PaymentError == "" {
		tNow := time.Now()
		payment.SettledAt = &tNow
		payment.Status = "SETTLED"
		payment.PreImage = hex.EncodeToString(paymentResponse.PaymentPreimage)
	} else {
		payment.Status = paymentResponse.PaymentError
		payment.Status = "FAILED"
		return payment, nil
	}

	tx := d.MustBegin()

	uUpdateQuery := `UPDATE users 
		SET balance = balance - :amount
		WHERE id = :user_id
		RETURNING id, balance, updated_at`

	trxCreateQuery := `INSERT INTO payments 
		(user_id, invoice, description, direction, status, amount)
		VALUES (:user_id, :invoice, :description, :direction, :status, :amount)
		RETURNING id, user_id, invoice, description, direction, status, amount,
				  created_at, updated_at`

	rows, err := tx.NamedQuery(trxCreateQuery, payment)
	if err != nil {
		_ = tx.Rollback()
		return payment, err
	}
	if rows.Next() {
		if err = rows.Scan(
			&payment.ID,
			&payment.UserID,
			&payment.PaymentRequest,
			&payment.Description,
			&payment.Direction,
			&payment.Status,
			// TOOD: Danger we need to split this into Msats and sats
			&payment.AmountSat,
			&payment.AmountMSat,
			&payment.CreatedAt,
			&payment.UpdatedAt,
		); err != nil {
			_ = tx.Rollback()
			return payment, errors.Wrap(err, "PayInvoice: Paid but cold not create payment")
		}
	}
	rows.Close() // Free up the database connection

	if payment.Status == "SETTLED" {
		rows, err := d.NamedQuery(uUpdateQuery, &payment)
		if err != nil {
			// TODO: This is probably not a healthy way to deal with an error here
			tx.Rollback()
			return payment, errors.Wrap(err, "PayInvoice: Cold not construct user update")
		}
		if rows.Next() {
			if err = rows.Scan(
				&payment.User.ID,
				&payment.User.Balance,
				&payment.User.UpdatedAt,
			); err != nil {
				_ = tx.Rollback()
				return payment, errors.Wrap(err, "PayInvoice: Could not decrement user balance")
			}
		}
	}

	if err != nil {
		return payment, err
	}

	err = tx.Commit()
	if err != nil {
		return payment, errors.Wrap(err, "PayInvoice: Cound not commit")
	}

	// TODO: Here we need to store the payment hash
	// We also need to store the payment preimage once the invoice is settled

	// payment.PaymentPreImage = paymentResponse.PaymentPreimage

	return payment, nil
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
			tNow := time.Now()
			t.SettledAt = &tNow

			trxUpdateQuery := `UPDATE payments 
				SET status = :status, settled_at = :settled_at 
				WHERE hashed_pre_image = :hashed_pre_image
				RETURNING id, user_id, invoice, pre_image, hashed_pre_image,
						  description, direction, status, amount,
						  created_at, updated_at`

			uUpdateQuery := `UPDATE users 
				SET balance = :amount + balance
				WHERE id = :user_id
				RETURNING id, balance, updated_at`
			tx := database.MustBegin()
			rows, err := tx.NamedQuery(trxUpdateQuery, &t)
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

			rows, err = tx.NamedQuery(uUpdateQuery, &t)
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
