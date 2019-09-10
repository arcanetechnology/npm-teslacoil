package payments

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
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
	open      Status = "OPEN"

	// OffchainTXTable is the tablename of offchaintx, as saved in the DB
	OffchainTXTable = "offchaintx"
)

// Payment is a database table
type Payment struct {
	ID             int    `db:"id"`
	UserID         int    `db:"user_id"`
	PaymentRequest string `db:"payment_request"`
	// A pointer to a string means the type is nullable
	Preimage       *string   `db:"preimage"`
	HashedPreimage string    `db:"hashed_preimage"`
	CallbackURL    *string   `db:"callback_url"`
	Expiry         int64     `db:"expiry"`
	Status         Status    `db:"status"`
	Memo           string    `db:"memo"`
	Description    string    `db:"description"`
	Direction      Direction `db:"direction"`
	AmountSat      int64     `db:"amount_sat"`
	AmountMSat     int64     `db:"amount_msat"`
	// SettledAt is a pointer because it can be null, and inserting null in
	// something not a pointer when querying the db is not possible
	SettledAt *time.Time `db:"settled_at"` // If not 0 or nul, it means the
	// invoice is settled
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

//UserPaymentResponse is a user payment response
type UserPaymentResponse struct {
	Payment Payment            `json:"payment"`
	User    users.UserResponse `json:"user"`
}

// insert persists the supplied payment to the database. Returns the payment,
// as returned from the database
func insert(tx *sqlx.Tx, p Payment) (Payment, error) {
	var createOffchainTXQuery string

	if p.Preimage != nil && p.HashedPreimage != "" {
		return Payment{},
			fmt.Errorf("cant supply both a preimage and a hashed preimage")
	}

	createOffchainTXQuery = `INSERT INTO 
	offchaintx (user_id, payment_request, preimage, hashed_preimage, memo,
		description, expiry, direction, status, amount_sat,amount_msat)
	VALUES (:user_id, :payment_request, :preimage, :hashed_preimage, 
		    :memo, :description, :direction, :status, :amount_sat, :amount_msat)
	RETURNING id, user_id, payment_request, preimage, hashed_preimage,
			  memo, description, expiry, direction, status, amount_sat, amount_msat,
			  created_at, updated_at`

	// Using the above query, NamedQuery() will extract VALUES from the payment
	// variable and insert them into the query
	rows, err := tx.NamedQuery(createOffchainTXQuery, p)
	if err != nil {
		log.Error(err)
		return Payment{}, errors.Wrapf(
			err,
			"insertPayment->tx.NamedQuery(%s, %+v)",
			createOffchainTXQuery,
			p,
		)
	}
	defer rows.Close() // Free up the database connection

	var result Payment
	if rows.Next() {
		if err = rows.Scan(
			&result.ID,
			&result.UserID,
			&result.PaymentRequest,
			&result.Preimage,
			&result.HashedPreimage,
			&result.Memo,
			&result.Description,
			&result.Direction,
			&result.Status,
			&result.AmountSat,
			&result.Expiry,
			&result.AmountMSat,
			&result.CreatedAt,
			&result.UpdatedAt,
		); err != nil {
			log.Error(err)
			return result, errors.Wrapf(err,
				"insertPayment->rows.Next(), Problem row = %+v", result)
		}

	}

	return result, nil
}

// GetAll selects all payments for given userID from the DB.
func GetAll(d *db.DB, userID int, limit int, offset int) (
	[]Payment, error) {
	payments := []Payment{}

	// Using OFFSET is not ideal, but until we start seeing performance problems
	// it's fine
	tQuery := `SELECT *
		FROM offchaintx
		WHERE user_id=$1
		ORDER BY created_at ASC
		LIMIT $2
		OFFSET $3`

	err := d.Select(&payments, tQuery, userID, limit, offset)
	if err != nil {
		log.Error(err)
		return payments, err
	}

	return payments, nil
}

// GetByID performs this query:
// `SELECT * FROM offchaintx WHERE id=id AND user_id=userID`, where id is the
// primary key of the table(autoincrementing)
func GetByID(d *db.DB, id int, userID int) (Payment, error) {
	if id < 0 || userID < 0 {
		return Payment{}, fmt.Errorf("GetByID(): neither id nor userID can be less than 0")
	}

	txResult := Payment{}
	tQuery := fmt.Sprintf(
		"SELECT * FROM %s WHERE id=$1 AND user_id=$2 LIMIT 1", OffchainTXTable)

	if err := d.Get(&txResult, tQuery, id, userID); err != nil {
		log.Error(err)
		return txResult, errors.Wrap(err, "could not get payment")
	}

	// sanity check the query
	if txResult.UserID != userID {
		err := fmt.Errorf(
			"db query retrieved unexpected value, expected payment with user_id %d but got %d",
			userID, txResult.UserID)
		log.Errorf(err.Error())
		return Payment{}, err
	}

	return txResult, nil
}

// CreateInvoice creates and adds a new invoice to lnd and creates a new payment
// with the paymentRequest and RHash returned from lnd. After creation, inserts
// the payment into the database
func CreateInvoice(d *db.DB, lncli ln.AddLookupInvoiceClient, userID int,
	amountSat int64, description, memo string) (Payment, error) {

	if amountSat <= 0 {
		return Payment{}, fmt.Errorf("amount cant be less than or equal to 0")
	}
	if len(memo) > 256 {
		return Payment{}, fmt.Errorf("memo cant be longer than 256 characters")
	}

	// First we add an invoice given the given parameters using the ln package
	invoice, err := ln.AddInvoice(
		lncli,
		lnrpc.Invoice{
			Memo:  memo,
			Value: int64(amountSat),
		})
	if err != nil {
		err = errors.Wrap(err, "could not add invoice to lnd")
		log.Error(err)
		return Payment{}, err
	}

	// Sanity check the invoice we just created
	if invoice.Value != int64(amountSat) {
		err = fmt.Errorf("could not insert invoice, created invoice amount not equal request.Amount")
		log.Error(err)
		return Payment{}, err
	}

	// Insert the payment into the database. Should anything inside insertPayment
	// fail, we use tx.Rollback() to revert any change made
	tx := d.MustBegin()
	// We do not store the preimage until the payment is settled, to avoid the
	// user getting the preimage before the invoice is settled
	p := Payment{
		UserID:         userID,
		Memo:           memo,
		Description:    description,
		AmountSat:      amountSat,
		AmountMSat:     amountSat * 1000,
		Expiry:         invoice.Expiry,
		PaymentRequest: strings.ToUpper(invoice.PaymentRequest),
		HashedPreimage: hex.EncodeToString(invoice.RHash),
		Status:         open,
		Direction:      inbound,
	}
	p, err = insert(tx, p)
	if err != nil {
		log.Error(err)
		_ = tx.Rollback()
		return Payment{}, err
	}

	log.Infof("CreateInvoice payment: %v", p)

	if err = tx.Commit(); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		return Payment{}, err
	}

	return p, nil
}

// PayInvoice first persists an outbound payment with the supplied invoice to
// the database. Then attempts to pay the invoice using SendPaymentSync
// Should the payment fail, we do not decrease the users balance.
// This logic is completely fucked, as the user could initiate a payment for
// 10 000 000 satoshis, and the logic wouldn't complain until AFTER the payment
// is complete(that is, we no longer have the money)
// TODO: Decrease the users balance BEFORE attempting to send the payment.
// If at any point the payment/db transaction should fail, increase the users
// balance.
func PayInvoice(d *db.DB, lncli ln.DecodeSendClient, userID int,
	paymentRequest, description, memo string) (UserPaymentResponse, error) {

	payreq, err := lncli.DecodePayReq(
		context.Background(),
		&lnrpc.PayReqString{PayReq: paymentRequest})
	if err != nil {
		log.Error(err)
		return UserPaymentResponse{}, err
	}

	sendRequest := &lnrpc.SendRequest{
		PaymentRequest: paymentRequest,
	}

	// We instantiate an empty struct fill up information as we go, and return
	// all the information we have thus far should there be an error
	var upr UserPaymentResponse

	upr.Payment = Payment{
		UserID:         userID,
		Direction:      outbound,
		PaymentRequest: strings.ToUpper(paymentRequest),
		Status:         open,
		HashedPreimage: payreq.PaymentHash,
		Memo:           payreq.Description,
		// TODO: Make sure conversion from int64 to int is always safe and does
		// not overflow if limit > MAXINT32 {abort} if offset > MAXINT32 {abort}
		AmountSat:  payreq.NumSatoshis,
		AmountMSat: payreq.NumSatoshis * 1000,
	}

	tx := d.MustBegin()

	upr.User, err = users.DecreaseBalance(tx, users.ChangeBalance{
		UserID:    upr.Payment.UserID,
		AmountSat: upr.Payment.AmountSat,
	})
	if err != nil {
		_ = tx.Rollback()
		return upr, errors.Wrapf(err,
			"PayInvoice->updateUserBalance(tx, %d, %d)",
			upr.Payment.UserID, upr.Payment.AmountSat)
	}

	// We insert the transaction in the DB before we attempt to send it to
	// avoid issues with attempted updates to the payment before it is added to
	// the DB
	upr.Payment, err = insert(tx, upr.Payment)
	if err != nil {
		log.Error(err)
		_ = tx.Rollback()
		return upr, errors.Wrapf(
			err, "insertPayment(db, %+v)", upr.Payment)
	}

	// TODO(henrik): Need to improve this step to allow for slow paying invoices.
	// See logic in lightningspin-api for possible solution
	paymentResponse, err := lncli.SendPaymentSync(
		context.Background(), sendRequest)
	if err != nil {
		log.Error(err)
		return upr, err
	}

	if paymentResponse.PaymentError == "" {
		t := time.Now()
		upr.Payment.SettledAt = &t
		upr.Payment.Status = succeeded
		preimage := hex.EncodeToString(paymentResponse.PaymentPreimage)
		upr.Payment.Preimage = &preimage
	} else {
		err = tx.Rollback()
		if err != nil {
			return upr, errors.Wrap(err, "could not rollback DB")
		}

		upr.Payment.Status = failed
		return upr, errors.New(paymentResponse.PaymentError)
	}

	if err = tx.Commit(); err != nil {
		return UserPaymentResponse{}, errors.Wrap(
			err, "PayInvoice: could not commit")
	}

	return UserPaymentResponse{
		Payment: upr.Payment,
		User:    upr.User,
	}, nil
}

// InvoiceStatusListener is
func InvoiceStatusListener(invoiceUpdatesCh chan lnrpc.Invoice,
	database *db.DB) {

	for {
		invoice := <-invoiceUpdatesCh
		_, err := UpdateInvoiceStatus(invoice, database)
		if err != nil {
			log.Errorf("Error when updating invoice status: %v", err)
			// TODO: Here we need to handle the errors from UpdateInvoiceStatus
		}
	}
}

// UpdateInvoiceStatus receives messages from lnd's SubscribeInvoices
// (newly added/settled invoices). If received payment was successful, updates
// the payment stored in our db and increases the users balance
func UpdateInvoiceStatus(invoice lnrpc.Invoice, database *db.DB) (
	*UserPaymentResponse, error) {

	tQuery := "SELECT * FROM offchaintx WHERE payment_request=$1"

	// Define a custom response struct to include user details
	payment := Payment{}
	if err := database.Get(
		&payment,
		tQuery,
		strings.ToUpper(invoice.PaymentRequest)); err != nil {
		return nil, errors.Wrapf(err,
			"UpdateInvoiceStatus->database.Get(&payment, query, %+v)",
			invoice.PaymentRequest,
		)
	}

	if !invoice.Settled {
		return &UserPaymentResponse{
			Payment: payment,
			User:    users.UserResponse{},
		}, nil
	}
	time := time.Now()
	payment.SettledAt = &time
	payment.Status = Status("SUCCEEDED")
	preimage := hex.EncodeToString(invoice.RPreimage)
	payment.Preimage = &preimage

	updateOffchainTxQuery := `UPDATE offchaintx 
		SET status = :status, settled_at = :settled_at, preimage = :preimage
		WHERE hashed_preimage = :hashed_preimage
		RETURNING id, user_id, payment_request, preimage, hashed_preimage,
	   			memo, description, expiry, direction, status, amount_sat, amount_msat,
				created_at, updated_at`

	tx := database.MustBegin()

	rows, err := tx.NamedQuery(updateOffchainTxQuery, &payment)
	if err != nil {
		_ = tx.Rollback()
		return nil, errors.Wrapf(err,
			"UpdateInvoiceStatus->tx.NamedQuery(&t, query, %+v)",
			payment,
		)
	}
	rows.Close() // Free up the database connection

	if rows.Next() {
		if err = rows.Scan(
			&payment.ID,
			&payment.UserID,
			&payment.PaymentRequest,
			&payment.Preimage,
			&payment.HashedPreimage,
			&payment.Memo,
			&payment.Description,
			&payment.Direction,
			&payment.Status,
			&payment.Expiry,
			&payment.AmountSat,
			&payment.AmountMSat,
			&payment.CreatedAt,
			&payment.UpdatedAt,
		); err != nil {
			_ = tx.Rollback()
			return nil, errors.Wrap(
				err,
				"UpdateInvoiceStatus->rows.Scan()",
			)
		}
	}

	user, err := users.IncreaseBalance(tx, users.ChangeBalance{
		AmountSat: payment.AmountSat,
		UserID:    payment.UserID})
	if err != nil {
		return nil, errors.Wrapf(
			err,
			"UpdateInvoiceStatus->users.DecreaseBalance(tx, %d, %d)",
			payment.UserID, payment.AmountSat)
	}

	err = tx.Commit()
	if err != nil {
		return nil, errors.Wrap(
			err,
			"UpdateInvoiceStatus->tx.Commit()",
		)
	}
	// TODO: Here we need to call the callback with the response.

	return &UserPaymentResponse{
		Payment: payment,
		User:    user,
	}, nil
}

func (p Payment) String() string {
	str := "Payment: {\n"
	str += fmt.Sprintf("\tID: %d\n", p.ID)
	str += fmt.Sprintf("\tUserID: %d\n", p.UserID)
	str += fmt.Sprintf("\tPaymentRequest: %s\n", p.PaymentRequest)
	str += fmt.Sprintf("\tPreimage: %v\n", p.Preimage)
	str += fmt.Sprintf("\tHashedPreimage: %s\n", p.HashedPreimage)
	str += fmt.Sprintf("\tCallbackURL: %v\n", p.CallbackURL)
	str += fmt.Sprintf("\tStatus: %s\n", p.Status)
	str += fmt.Sprintf("\tMemo: %s\n", p.Memo)
	str += fmt.Sprintf("\tDescription: %s\n", p.Description)
	str += fmt.Sprintf("\tExpiry: %d\n", p.Expiry)
	str += fmt.Sprintf("\tDirection: %s\n", p.Direction)
	str += fmt.Sprintf("\tAmountSat: %d\n", p.AmountSat)
	str += fmt.Sprintf("\tAmountMSat: %d\n", p.AmountMSat)
	str += fmt.Sprintf("\tSettledAt: %v\n", p.SettledAt)
	str += fmt.Sprintf("\tCreatedAt: %v\n", p.CreatedAt)
	str += fmt.Sprintf("\tUpdatedAt: %v\n", p.UpdatedAt)
	str += fmt.Sprintf("\tDeletedAt: %v\n", p.DeletedAt)
	str += "}"

	return str
}
