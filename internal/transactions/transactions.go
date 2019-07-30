package transactions

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

// All fetches all transactions
func All(d *sqlx.DB) ([]Transaction, error) {
	transactions := []Transaction{}
	const tQuery = `SELECT t.* 
		FROM transactions AS t 
		ORDER BY t.created_at ASC`

	err := d.Select(&transactions, tQuery)
	if err != nil {
		return transactions, err
	}

	return transactions, nil
}

// GetByID returns a single transaction based on the id given
func GetByID(d *sqlx.DB, id uint64) (TransactionResponse, error) {
	trxResult := TransactionResponse{}
	tQuery := `SELECT * FROM transactions WHERE id=$1 LIMIT 1`

	if err := d.Get(&trxResult, tQuery, id); err != nil {
		return trxResult, errors.Wrap(err, "Could not get transaction")
	}

	return trxResult, nil
}

// Create a new transaction
func CreateInvoice(d *sqlx.DB, nt NewTransaction) (TransactionResponse, error) {

	transaction := TransactionResponse{
		UserID:      nt.UserID,
		Description: nt.Description,
		Direction:   Direction("inbound"), // All created invoices are inbound
		Status:      "unpaid",
		// TOOD: Danger we need to split this into Msats and sats
		Amount: nt.Amount * 1000,
	}

	transaction.UserID = nt.UserID

	client, err := ln.NewLNDClient()
	if err != nil {
		return transaction, err
	}
	// Generate random preimage.
	preimage := make([]byte, 32)
	if _, err := rand.Read(preimage); err != nil {
		return transaction, err
	}
	hexPreimage := hex.EncodeToString(preimage)

	invoice := &lnrpc.Invoice{
		Memo:      nt.Description,
		Value:     nt.Amount,
		RPreimage: preimage,
	}

	newInvoice, err := client.AddInvoice(context.Background(), invoice)
	if err != nil {
		return transaction, err
	}

	transaction.Invoice = newInvoice.PaymentRequest
	transaction.PreImage = hexPreimage
	transaction.HashedPreImage = hex.EncodeToString(newInvoice.RHash)

	trxCreateQuery := `INSERT INTO transactions 
		(user_id, invoice, pre_image, hashed_pre_image, description, direction, status, amount)
		VALUES (:user_id, :invoice, :pre_image, :hashed_pre_image, :description,
				:direction, :status, :amount)
		RETURNING id, user_id, invoice, pre_image, hashed_pre_image, description, direction, status, amount,
				  created_at, updated_at`

	rows, err := d.NamedQuery(trxCreateQuery, transaction)
	if err != nil {
		return transaction, err
	}
	if rows.Next() {
		if err = rows.Scan(
			&transaction.ID,
			&transaction.UserID,
			&transaction.Invoice,
			&transaction.PreImage,
			&transaction.HashedPreImage,
			&transaction.Description,
			&transaction.Direction,
			&transaction.Status,
			// TOOD: Danger we need to split this into Msats and sats
			&transaction.Amount,
			&transaction.CreatedAt,
			&transaction.UpdatedAt,
		); err != nil {
			return transaction, err
		}
	}
	rows.Close() // Free up the database connection

	return transaction, nil
}

// PayInvoice pay an invoice on behalf of the user
func PayInvoice(d *sqlx.DB, nt NewTransaction) (PaymentResponse, error) {

	// Define a custom response struct to include user details
	transaction := PaymentResponse{}

	transaction.UserID = nt.UserID
	transaction.Direction = Direction("outbound") // All paid invoices are outbound
	transaction.Invoice = nt.Invoice

	// TODO: the LND gRPC client should mayeb be shared, no need to create a
	// new for each transaction
	client, err := ln.NewLNDClient()
	if err != nil {
		return transaction, err
	}

	payRequest, err := client.DecodePayReq(
		context.Background(),
		&lnrpc.PayReqString{PayReq: nt.Invoice})
	if err != nil {
		return transaction, err
	}

	transaction.HashedPreImage = payRequest.PaymentHash
	// transaction.Destination = payRequest.Destination
	transaction.Description = payRequest.Description
	transaction.Amount = payRequest.NumSatoshis * 1000

	sendRequest := &lnrpc.SendRequest{
		PaymentRequest: nt.Invoice,
	}

	// TODO: Need to improve this step to allow for slow paying invoices.
	paymentResponse, err := client.SendPaymentSync(context.Background(), sendRequest)
	if err != nil {
		return transaction, nil
	}

	if paymentResponse.PaymentError == "" {
		tNow := time.Now()
		transaction.SettledAt = &tNow
		transaction.Status = "SETTLED"
		transaction.PreImage = hex.EncodeToString(paymentResponse.PaymentPreimage)
	} else {
		transaction.Status = paymentResponse.PaymentError
		transaction.Status = "FAILED"
		return transaction, nil
	}

	tx := d.MustBegin()

	uUpdateQuery := `UPDATE users 
		SET balance = balance - :amount
		WHERE id = :user_id
		RETURNING id, balance, updated_at`

	trxCreateQuery := `INSERT INTO transactions 
		(user_id, invoice, description, direction, status, amount)
		VALUES (:user_id, :invoice, :description, :direction, :status, :amount)
		RETURNING id, user_id, invoice, description, direction, status, amount,
				  created_at, updated_at`

	rows, err := tx.NamedQuery(trxCreateQuery, transaction)
	if err != nil {
		_ = tx.Rollback()
		return transaction, err
	}
	if rows.Next() {
		if err = rows.Scan(
			&transaction.ID,
			&transaction.UserID,
			&transaction.Invoice,
			&transaction.Description,
			&transaction.Direction,
			&transaction.Status,
			// TOOD: Danger we need to split this into Msats and sats
			&transaction.Amount,
			&transaction.CreatedAt,
			&transaction.UpdatedAt,
		); err != nil {
			_ = tx.Rollback()
			return transaction, errors.Wrap(err, "PayInvoice: Paid but cold not create transaction")
		}
	}
	rows.Close() // Free up the database connection

	if transaction.Status == "SETTLED" {
		rows, err := d.NamedQuery(uUpdateQuery, &transaction)
		if err != nil {
			// TODO: This is probably not a healthy way to deal with an error here
			tx.Rollback()
			return transaction, errors.Wrap(err, "PayInvoice: Cold not construct user update")
		}
		if rows.Next() {
			if err = rows.Scan(
				&transaction.User.ID,
				&transaction.User.Balance,
				&transaction.User.UpdatedAt,
			); err != nil {
				_ = tx.Rollback()
				return transaction, errors.Wrap(err, "PayInvoice: Could not decrement user balance")
			}
		}
	}

	if err != nil {
		return transaction, err
	}

	err = tx.Commit()
	if err != nil {
		return transaction, errors.Wrap(err, "PayInvoice: Cound not commit")
	}

	// TODO: Here we need to store the payment hash
	// We also need to store the payment preimage once the invoice is settled

	// transaction.PaymentPreImage = paymentResponse.PaymentPreimage

	return transaction, nil
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
			Transaction
			User UserDetails
		}{}

		tQuery := `SELECT * FROM transactions as t WHERE invoice=$1 LIMIT 1`
		if err := database.Get(&t, tQuery, invoice.PaymentRequest); err != nil {
			// TODO: This is probably not a healthy way to deal with an error here
			log.Println(errors.Wrap(err, "UpdateInvoiceStatus: Could not find transaction").Error())
			return
		}

		t.Status = invoice.State.String()
		if invoice.Settled {
			tNow := time.Now()
			t.SettledAt = &tNow

			trxUpdateQuery := `UPDATE transactions 
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
				log.Println(errors.Wrap(err, "UpdateInvoiceStatus: Could not update transaction").Error())
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
					log.Println(errors.Wrap(err, "UpdateInvoiceStatus: Could not update transaction").Error())
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
