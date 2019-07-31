package payments

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/lpp/internal/platform/ln"
)

// All fetches all payments
func All(d *sqlx.DB) ([]Payment, error) {
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
func GetByID(d *sqlx.DB, id uint64) (PaymentResponse, error) {
	trxResult := PaymentResponse{}
	tQuery := `SELECT * FROM payments WHERE id=$1 LIMIT 1`

	if err := d.Get(&trxResult, tQuery, id); err != nil {
		return trxResult, errors.Wrap(err, "Could not get payment")
	}

	return trxResult, nil
}

// CreateInvoice creates a new invoice
func CreateInvoice(d *sqlx.DB, nt NewPayment) (PaymentResponse, error) {

	payment := PaymentResponse{
		UserID:      nt.UserID,
		Description: nt.Description,
		Direction:   Direction("inbound"), // All created invoices are inbound
		Status:      "unpaid",
		// TOOD: Danger we need to split this into Msats and sats
		Amount: nt.Amount * 1000,
	}

	payment.UserID = nt.UserID

	client, err := ln.NewLNDClient()
	if err != nil {
		return payment, err
	}
	// Generate random preimage.
	preimage := make([]byte, 32)
	if _, err := rand.Read(preimage); err != nil {
		return payment, err
	}
	hexPreimage := hex.EncodeToString(preimage)

	invoice := &lnrpc.Invoice{
		Memo:      nt.Description,
		Value:     nt.Amount,
		RPreimage: preimage,
	}

	newInvoice, err := client.AddInvoice(context.Background(), invoice)
	if err != nil {
		return payment, err
	}

	payment.Invoice = newInvoice.PaymentRequest
	payment.PreImage = hexPreimage
	payment.HashedPreImage = hex.EncodeToString(newInvoice.RHash)

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
			&payment.Invoice,
			&payment.PreImage,
			&payment.HashedPreImage,
			&payment.Description,
			&payment.Direction,
			&payment.Status,
			// TOOD: Danger we need to split this into Msats and sats
			&payment.Amount,
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
func PayInvoice(d *sqlx.DB, nt NewPayment) (UserPaymentResponse, error) {

	// Define a custom response struct to include user details
	payment := UserPaymentResponse{}

	payment.UserID = nt.UserID
	payment.Direction = Direction("outbound") // All paid invoices are outbound
	payment.Invoice = nt.Invoice

	// TODO: the LND gRPC client should mayeb be shared, no need to create a
	// new for each payment
	client, err := ln.NewLNDClient()
	if err != nil {
		return payment, err
	}

	payRequest, err := client.DecodePayReq(
		context.Background(),
		&lnrpc.PayReqString{PayReq: nt.Invoice})
	if err != nil {
		return payment, err
	}

	payment.HashedPreImage = payRequest.PaymentHash
	// payment.Destination = payRequest.Destination
	payment.Description = payRequest.Description
	payment.Amount = payRequest.NumSatoshis * 1000

	sendRequest := &lnrpc.SendRequest{
		PaymentRequest: nt.Invoice,
	}

	// TODO: Need to improve this step to allow for slow paying invoices.
	paymentResponse, err := client.SendPaymentSync(context.Background(), sendRequest)
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
			&payment.Invoice,
			&payment.Description,
			&payment.Direction,
			&payment.Status,
			// TOOD: Danger we need to split this into Msats and sats
			&payment.Amount,
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
					&t.Invoice,
					&t.PreImage,
					&t.HashedPreImage,
					&t.Description,
					&t.Direction,
					&t.Status,
					// TOOD: Danger we need to split this into Msats and sats
					&t.Amount,
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
