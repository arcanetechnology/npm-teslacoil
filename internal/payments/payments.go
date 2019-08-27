package payments

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/ln"
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
	Memo        string `json:"memo"`
	Description string `json:"description"`
	AmountSat   int64  `json:"amountSat"`
}

// GetAllInvoicesData is the body for the GetAll endpoint
type GetAllInvoicesData struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

//PayInvoiceData is the required(and optional) fields for initiating a withdrawal
type PayInvoiceData struct {
	PaymentRequest string `json:"paymentRequest"`
	Description    string `json:"description"`
	Memo           string `json:"memo"`
}

// Payment is a database table
type Payment struct {
	ID             uint   `db:"id"`
	UserID         uint   `db:"user_id"`
	PaymentRequest string `db:"payment_request"`
	// We use string and time pointers to make a nullable type
	Preimage       sql.NullString `db:"preimage"`
	HashedPreimage string         `db:"hashed_preimage"`
	CallbackURL    sql.NullString `db:"callback_url"`
	Status         Status         `db:"status"`
	Memo           string         `db:"memo"`
	Description    string         `db:"description"`
	Direction      Direction      `db:"direction"`
	AmountSat      int64          `db:"amount_sat"`
	AmountMSat     int64          `db:"amount_msat"`
	// SettledAt is a pointer because it can be null, and inserting null in
	// something not a pointer when querying the db is not possible
	SettledAt *time.Time `db:"settled_at"` // If not 0 or nul, it means the
	// invoice is settled
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

// UserResponse is
type UserResponse struct {
	ID        uint      `db:"id"`
	Email     string    `db:"email"`
	Balance   int       `db:"balance"`
	UpdatedAt time.Time `db:"updated_at"`
}

//UserPaymentResponse is a user payment response
type UserPaymentResponse struct {
	Payment Payment      `json:"payment"`
	User    UserResponse `json:"user"`
}

// GetAll fetches all payments
func GetAll(d *sqlx.DB, userID uint, filter GetAllInvoicesData) (
	[]Payment, error) {

	payments := []Payment{}

	// Using OFFSET is not ideal, but until we start seeing
	// problems, it is fine
	tQuery := `SELECT *
		FROM offchaintx
		WHERE user_id=$1
		ORDER BY created_at ASC
		LIMIT $2
		OFFSET $3`

	err := d.Select(&payments, tQuery, userID, filter.Limit, filter.Offset)
	if err != nil {
		log.Error(err)
		return payments, err
	}

	return payments, nil
}

// GetByID returns a single invoice based on the id given
// It only retrieves invoices whose user_id is the same as the supplied argument
func GetByID(d *sqlx.DB, id uint, userID uint) (Payment, error) {
	txResult := Payment{}
	tQuery := fmt.Sprintf(
		"SELECT * FROM %s WHERE id=$1 AND user_id=$2 LIMIT 1", OffchainTXTable)

	if err := d.Get(&txResult, tQuery, id, userID); err != nil {
		// log.Error(err)
		return txResult, errors.Wrap(err, "could not get payment")
	}

	// sanity check the query
	if txResult.UserID != userID {
		err := errors.New(
			fmt.Sprintf(
				"db query retrieved unexpected value, expected payment with user_id %d but got %d",
				userID, txResult.UserID))
		// log.Errorf(err.Error())
		return Payment{}, err
	}

	if !txResult.CallbackURL.Valid {
		txResult.CallbackURL.String = ""
	}
	if !txResult.Preimage.Valid {
		txResult.Preimage.String = ""
	}

	return txResult, nil
}

// CreateInvoice creates a new invoice using lnd based on the function parameter
// and inserts the newly created invoice into the database
func CreateInvoice(d *sqlx.DB, lncli ln.AddLookupInvoiceClient,
	invoiceData CreateInvoiceData, userID uint) (Payment, error) {

	if invoiceData.AmountSat <= 0 {
		return Payment{}, errors.New("amount cant be less than or equal to 0")
	}
	if len(invoiceData.Memo) > 256 {
		return Payment{}, errors.New("memo cant be longer than 256 characters")
	}

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
	payment, err := insertPayment(tx,
		// Payment struct with all required data to add to the DB
		Payment{
			UserID:         userID,
			Memo:           invoiceData.Memo,
			Description:    invoiceData.Description,
			AmountSat:      invoiceData.AmountSat,
			AmountMSat:     invoiceData.AmountSat * 1000,
			PaymentRequest: strings.ToUpper(invoice.PaymentRequest),
			HashedPreimage: hex.EncodeToString(invoice.RHash),
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

	if !payment.Preimage.Valid {
		payment.Preimage = db.ToNullString("")
	}
	if !payment.CallbackURL.Valid {
		payment.CallbackURL = db.ToNullString("")
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
		PaymentRequest: strings.ToUpper(payInvoiceRequest.PaymentRequest),
		Status:         open,
		HashedPreimage: payreq.PaymentHash,
		Memo:           payreq.Description,
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
	paymentResponse, err := lncli.SendPaymentSync(
		context.Background(), sendRequest)
	if err != nil {
		// log.Error(err)
		return UserPaymentResponse{}, err
	}

	if paymentResponse.PaymentError == "" {
		t := time.Now()
		payment.SettledAt = &t
		payment.Status = succeeded
		payment.Preimage = sql.NullString{
			String: hex.EncodeToString(paymentResponse.PaymentPreimage),
			Valid:  true}
	} else {
		tx.Rollback()
		p.Status = failed
		return UserPaymentResponse{}, errors.New(paymentResponse.PaymentError)
	}

	user := UserResponse{}
	if payment.Status == succeeded {
		user, err = updateUserBalance(tx, p.UserID, payment.AmountSat)
		if err != nil {
			// log.Error(err)
			tx.Rollback()
			return UserPaymentResponse{}, errors.Wrapf(err,
				"PayInvoice->updateUserBalance(tx, %d, %d)",
				p.UserID, -payment.AmountSat)
		}
	}

	if err = tx.Commit(); err != nil {
		// log.Error(err)
		return UserPaymentResponse{}, errors.Wrap(
			err, "PayInvoice: Cound not commit")
	}

	if !payment.CallbackURL.Valid {
		payment.CallbackURL = db.ToNullString("")
	}
	if !payment.Preimage.Valid {
		payment.Preimage = db.ToNullString("")
	}

	return UserPaymentResponse{
		Payment: payment,
		User:    user,
	}, nil
}

// QueryExecutor is based on sqlx.DB, but is an interface that only requires
// "Query"
type QueryExecutor interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

func updateUserBalance(queryEx QueryExecutor, userID uint, amountSat int64) (
	UserResponse, error) {

	if amountSat == 0 {
		return UserResponse{}, errors.New(
			"No point in updating users balance with 0 satoshi")
	}

	updateBalanceQuery := `UPDATE users
		SET balance = balance + $1
		WHERE id = $2
		RETURNING id, email, balance, updated_at`

	rows, err := queryEx.Query(updateBalanceQuery, amountSat, userID)
	if err != nil {
		// log.Error(err)
		return UserResponse{}, errors.Wrap(
			err,
			"UpdateUserBalance(): could not construct user update",
		)
	}
	defer rows.Close()

	user := UserResponse{}
	if rows.Next() {
		if err = rows.Scan(
			&user.ID,
			&user.Email,
			&user.Balance,
			&user.UpdatedAt,
		); err != nil {
			// log.Error(err)
			return UserResponse{}, errors.Wrap(
				err, "Could not scan user returned from db")
		}
	}

	return user, nil
}

// InvoiceStatusListener is
func InvoiceStatusListener(invoiceUpdatesCh chan lnrpc.Invoice,
	database *sqlx.DB) {

	for {
		invoice := <-invoiceUpdatesCh
		_, err := UpdateInvoiceStatus(invoice, database)
		if err != nil {
			// TODO: Here we need to handle the errors from UpdateInvoiceStatus
		}
	}
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
func UpdateInvoiceStatus(invoice lnrpc.Invoice, database *sqlx.DB) (
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

	user := UserResponse{}

	if invoice.Settled == false {
		log.Info("Invoice not settled")
		return &UserPaymentResponse{
			Payment: payment,
			User:    user,
		}, nil
	}
	time := time.Now()
	payment.SettledAt = &time
	payment.Status = Status("SUCCEEDED")
	preimage := hex.EncodeToString(invoice.RPreimage)
	log.Infof("preimage: %s", preimage)
	payment.Preimage = sql.NullString{
		String: preimage,
		Valid:  true,
	}

	log.Infof("payment.Preimage %v", payment.Preimage)

	log.Infof("Invoice is settled %v", payment)

	updateOffchainTxQuery := `UPDATE offchaintx 
		SET status = :status, settled_at = :settled_at, preimage = :preimage
		WHERE hashed_preimage = :hashed_preimage
		RETURNING id, user_id, payment_request, preimage, hashed_preimage,
	   			memo, description, direction, status, amount_sat, amount_msat,
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
	defer rows.Close() // Free up the database connection

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

	log.Info("updated: ", payment)

	updateUserBalanceQuery := `UPDATE users 
				SET balance = :amount_sat + balance
				WHERE id = :user_id
				RETURNING id, email, balance, updated_at`

	rows, err = tx.NamedQuery(updateUserBalanceQuery, &payment)
	if err != nil {
		_ = tx.Rollback()
		return nil, errors.Wrapf(
			err,
			"UpdateInvoiceStatus->tx.NamedQuery(&t, query, %+v)",
			user,
		)
	}
	defer rows.Close() // Free up the database connection

	if rows.Next() {
		if err = rows.Scan(
			&user.ID,
			&user.Email,
			&user.Balance,
			&user.UpdatedAt,
		); err != nil {
			_ = tx.Rollback()
			return nil, errors.Wrap(
				err,
				"UpdateInvoiceStatus->rows.Scan()",
			)
		}
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

// insertPayment persists a payment to the database
func insertPayment(tx *sqlx.Tx, payment Payment) (Payment, error) {
	var createOffchainTXQuery string

	if payment.Preimage.Valid && payment.HashedPreimage != "" {
	}

	createOffchainTXQuery = `INSERT INTO 
	offchaintx (user_id, payment_request, preimage, hashed_preimage, memo,
		description, direction, status, amount_sat,amount_msat)
	VALUES (:user_id, :payment_request, :preimage, :hashed_preimage, 
		    :memo, :description, :direction, :status, :amount_sat, :amount_msat)
	RETURNING id, user_id, payment_request, preimage, hashed_preimage,
			  memo, description, direction, status, amount_sat, amount_msat,
			  created_at, updated_at`

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
			&result.AmountMSat,
			&result.CreatedAt,
			&result.UpdatedAt,
		); err != nil {
			// log.Error(err)
			return result, errors.Wrapf(err,
				"insertPayment->rows.Next(), Problem row = %+v", result)
		}

	}

	return result, nil
}
