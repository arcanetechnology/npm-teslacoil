package payments

import (
	"bytes"
	"context"
	hmac2 "crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"testing"
	"time"

	"gitlab.com/arcanecrypto/teslacoil/async"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/models/apikeys"
	"gitlab.com/arcanecrypto/teslacoil/testutil"

	"github.com/jmoiron/sqlx"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/ln"
	"gitlab.com/arcanecrypto/teslacoil/models/users"
)

// Payment is a database table
type Payment struct {
	ID     int `db:"id" json:"id"`
	UserID int `db:"user_id" json:"userId"`

	// ExpiresAt is the time at which the payment request expires.
	// NOT a db property
	ExpiresAt time.Time `json:"expiresAt"`
	// Expired is not stored in the DB, and can be added to a
	// payment struct by using the function WithAdditionalFields()
	Expired bool `json:"expired"`
}

var (
	ErrCouldNotGetByID            = errors.New("could not get payment by ID")
	ErrUserBalanceTooLow          = errors.New("user balance too low, cant decrease")
	Err0AmountInvoiceNotSupported = errors.New("cant insert 0 amount invoice, not yet supported")
)

// insert persists the supplied payment to the database.
// Returns the payment, as returned from the database
func insert(tx *sqlx.Tx, p Payment) (Payment, error) {
	if p.Preimage != nil && p.HashedPreimage != "" {
		return Payment{},
			fmt.Errorf("insert(tx, %+v): cant supply both a preimage and a hashed preimage", p)
	}
	if p.Description != nil && *p.Description == "" {
		p.Description = nil
	}
	if p.Memo != nil && *p.Memo == "" {
		p.Memo = nil
	}
	if p.AmountSat == 0 {
		return Payment{}, Err0AmountInvoiceNotSupported
	}

	createOffchainTXQuery := `INSERT INTO 
	offchaintx (user_id, payment_request, preimage, hashed_preimage, memo,
		callback_url, description, expiry, direction, status, amount_sat,amount_msat,
	            customer_order_id)
	VALUES (:user_id, :payment_request, :preimage, :hashed_preimage, 
		    :memo, :callback_url, :description, :expiry, :direction, :status, 
	        :amount_sat, :amount_msat, :customer_order_id)
	RETURNING id, user_id, payment_request, preimage, hashed_preimage,
			  memo, description, expiry, direction, status, amount_sat, amount_msat,
			  callback_url, created_at, updated_at, customer_order_id`

	// Using the above query, NamedQuery() will extract VALUES from
	// the payment variable and insert them into the query
	rows, err := tx.NamedQuery(createOffchainTXQuery, p)
	if err != nil {
		log.WithError(err).Error("Could not insert payment")
		return Payment{}, err
	}
	defer rows.Close()

	// Store the result of the query in the payment variable
	var payment Payment
	if rows.Next() {
		if err = rows.Scan(
			&payment.ID,
			&payment.UserID,
			&payment.PaymentRequest,
			&payment.Preimage,
			&payment.HashedPreimage,
			&payment.Memo,
			&payment.Description,
			&payment.Expiry,
			&payment.Direction,
			&payment.Status,
			&payment.AmountSat,
			&payment.AmountMSat,
			&payment.CallbackURL,
			&payment.CreatedAt,
			&payment.UpdatedAt,
			&payment.CustomerOrderId,
		); err != nil {
			log.Error(err)
			return payment, errors.Wrapf(err,
				"insert->rows.Next(), Problem row = %+v", payment)
		}
	}

	return payment.WithAdditionalFields(), nil
}

// GetAll selects all payments for given userID from the DB.
func GetAll(d *db.DB, userID int, limit int, offset int) (
	[]Payment, error) {
	// Using OFFSET is not ideal, but until we start seeing
	// performance problems it's fine
	tQuery := `SELECT *
		FROM offchaintx
		WHERE user_id=$1
		ORDER BY created_at
		LIMIT $2
		OFFSET $3`

	payments := []Payment{}
	err := d.Select(&payments, tQuery, userID, limit, offset)
	if err != nil {
		log.Error(err)
		return payments, err
	}

	for i, payment := range payments {
		payments[i] = payment.WithAdditionalFields()
	}

	return payments, nil
}

// GetByID performs this query:
// `SELECT * FROM offchaintx WHERE id=id AND user_id=userID`,
// where id is the primary key of the table(autoincrementing)
func GetByID(d *db.DB, id int, userID int) (Payment, error) {
	if id < 0 || userID < 0 {
		return Payment{}, fmt.Errorf("GetByID(): neither id nor userID can be less than 0")
	}

	tQuery := fmt.Sprintf(
		"SELECT * FROM %s WHERE id=$1 AND user_id=$2 LIMIT 1", OffchainTXTable)

	var payment Payment
	if err := d.Get(&payment, tQuery, id, userID); err != nil {
		log.Error(err)
		return payment, errors.Wrap(err, "could not get payment")
	}

	return payment.WithAdditionalFields(), nil
}

// CreateInvoice is used to Create an Invoice without a memo
func CreateInvoice(lncli ln.AddLookupInvoiceClient, amountSat int64) (
	lnrpc.Invoice, error) {
	return CreateInvoiceWithMemo(lncli, amountSat, "")
}

// CreateInvoiceWithMemo creates an invoice with a memo using lnd
func CreateInvoiceWithMemo(lncli ln.AddLookupInvoiceClient, amountSat int64,
	memo string) (lnrpc.Invoice, error) {

	if amountSat > ln.MaxAmountSatPerInvoice {
		return lnrpc.Invoice{}, fmt.Errorf(
			"amount (%d) was too large. Max: %d",
			amountSat, ln.MaxAmountSatPerInvoice)
	}

	if amountSat <= 0 {
		return lnrpc.Invoice{}, fmt.Errorf("amount cant be less than or equal to 0")
	}
	if len(memo) > 256 {
		return lnrpc.Invoice{}, fmt.Errorf("memo cant be longer than 256 characters")
	}

	// add an invoice to lnd with the given parameters using our ln package
	invoice, err := ln.AddInvoice(
		lncli,
		lnrpc.Invoice{
			Memo:  memo,
			Value: int64(amountSat),
		})
	if err != nil {
		err = errors.Wrap(err, "could not add invoice to lnd")
		log.Error(err)
		return lnrpc.Invoice{}, err
	}

	return *invoice, nil
}

// NewPaymentArgs are the different options that dictates creation of a new
// payment
type NewPaymentOpts struct {
	UserID      int
	AmountSat   int64
	Memo        string
	Description string
	CallbackURL string
	OrderId     string
}

// NewPayment creates a new payment by first creating an invoice
// using lnd, then saving info returned from lnd to a new payment
func NewPayment(d *db.DB, lncli ln.AddLookupInvoiceClient, opts NewPaymentOpts) (Payment, error) {
	tx := d.MustBegin()
	// We do not store the preimage until the payment is settled, to avoid the
	// user getting the preimage before the invoice is settled

	invoice, err := CreateInvoiceWithMemo(lncli, opts.AmountSat, opts.Memo)
	if err != nil {
		log.WithError(err).Error("Could not create invoice")
		return Payment{}, err
	}

	p := Payment{
		UserID:         opts.UserID,
		AmountSat:      invoice.Value,
		AmountMSat:     invoice.Value * 1000,
		Expiry:         invoice.Expiry,
		PaymentRequest: invoice.PaymentRequest,
		HashedPreimage: hex.EncodeToString(invoice.RHash),
		Status:         OPEN,
		Direction:      INBOUND,
	}
	if opts.Memo != "" {
		p.Memo = &opts.Memo
	}
	if opts.Description != "" {
		p.Description = &opts.Description
	}
	if opts.CallbackURL != "" {
		p.CallbackURL = &opts.CallbackURL
	}
	if opts.OrderId != "" {
		p.CustomerOrderId = &opts.OrderId
	}

	p, err = insert(tx, p)
	if err != nil {
		_ = tx.Rollback()
		return Payment{}, err
	}

	if err := tx.Commit(); err != nil {
		log.WithError(err).Error("Could not commit payment TX")
		_ = tx.Rollback()
		return Payment{}, err
	}

	return p, nil
}

// MarkInvoiceAsPaid marks the given payment request as paid at the given date
func MarkInvoiceAsPaid(db *db.DB, paymentRequest string, paidAt time.Time) error {
	updateOffchainTxQuery := `UPDATE offchaintx 
		SET settled_at = $1, status = $2
		WHERE payment_request = $3`

	log.Infof("marking %s as paid", paymentRequest)

	result, err := db.Exec(updateOffchainTxQuery, paidAt, SUCCEEDED, paymentRequest)
	if err != nil {
		log.Errorf("Couldn't mark invoice as paid: %+v", err)
		return err
	}
	rows, _ := result.RowsAffected()
	log.Infof("Marking an invoice as paid resulted in %d updated rows", rows)

	return nil
}

// MarkInvoiceAsPaid marks the given payment request as paid at the given date
func MarkInvoiceAsFailed(db *db.DB, paymentRequest string) error {
	updateOffchainTxQuery := `UPDATE offchaintx 
		SET status = $1
		WHERE payment_request = $2`

	log.Infof("marking %s as failed", paymentRequest)

	result, err := db.Exec(updateOffchainTxQuery, FAILED, paymentRequest)
	if err != nil {
		log.Errorf("Couldn't mark invoice as failed: %+v", err)
		return err
	}
	rows, _ := result.RowsAffected()
	log.Tracef("Marking an invoice as paid resulted in %d updated rows", rows)

	return nil
}

// PayInvoice is used to Pay an invoice without a description
func PayInvoice(d *db.DB, lncli ln.DecodeSendClient, userID int,
	paymentRequest string) (*Payment, error) {
	return PayInvoiceWithDescription(d, lncli, userID, paymentRequest, "")
}

// PayInvoiceWithDescription first persists an outbound payment with the supplied invoice to
// the database. Then attempts to pay the invoice using SendPaymentSync
// Should the payment fail, we rollback all changes made to the DB
func PayInvoiceWithDescription(db *db.DB, lncli ln.DecodeSendClient, userID int,
	paymentRequest string, description string) (*Payment, error) {
	payreq, err := lncli.DecodePayReq(
		context.Background(),
		&lnrpc.PayReqString{PayReq: paymentRequest})
	if err != nil {
		return nil, err
	}

	if payreq.NumSatoshis == 0 {
		return nil, Err0AmountInvoiceNotSupported
	}

	tx := db.MustBegin()

	// decrease users balance
	user, err := users.DecreaseBalance(tx, users.ChangeBalance{
		UserID:    userID,
		AmountSat: payreq.NumSatoshis,
	})
	if err != nil {
		_ = tx.Rollback()
		log.WithError(err).Errorf("could not decrease user %d balance by %d", userID, payreq.NumSatoshis)
		return nil, ErrUserBalanceTooLow
	}

	// insert pay_req into DB
	payment := Payment{
		UserID:         userID,
		PaymentRequest: paymentRequest,
		HashedPreimage: payreq.PaymentHash,
		Expiry:         payreq.Expiry,
		Status:         OPEN,
		Memo:           &payreq.Description,
		Description:    &description,
		Direction:      OUTBOUND,
		AmountSat:      payreq.NumSatoshis,
		AmountMSat:     payreq.NumSatoshis * 1000,
	}

	payment, err = insert(tx, payment)
	if err != nil {
		log.Error(err)
		_ = tx.Rollback()
		return nil, errors.Wrapf(err, "payinvoicewithdescription")
	}

	// attempt to pay invoice
	paymentResponse, err := lncli.SendPaymentSync(
		context.Background(), &lnrpc.SendRequest{
			PaymentRequest: paymentRequest,
		})
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	log.WithFields(logrus.Fields{
		"paymentError": paymentResponse.PaymentError,
		"paymentHash":  hex.EncodeToString(paymentResponse.PaymentHash),
		"paymentRoute": paymentResponse.PaymentRoute,
	}).Info("Tried sending payment")

	// if payment failed, mark it as failed and rollback
	if paymentResponse.PaymentError != "" {

		if err = tx.Rollback(); err != nil {
			return nil, errors.Wrap(err, "could not rollback DB")
		}
		if err = MarkInvoiceAsFailed(db, paymentRequest); err != nil {
			return nil, err
		}

		return nil, errors.New(paymentResponse.PaymentError)
	}

	if err = tx.Commit(); err != nil {
		_ = tx.Rollback()
		return nil, errors.Wrap(
			err, "PayInvoice: could not commit")
	}

	settledAt := time.Now()
	err = MarkInvoiceAsPaid(db, paymentRequest, settledAt)
	if err != nil {
		// we never want to be in this situation. we now have an
		// OPEN invoice in the db, but the use has decreased the balance
		log.Panicf("could not mark invoice %s as paid, although it was paid", paymentRequest)
	}

	log.WithField("payment", payment.String()).Info("updated payment")

	// to always return the latest state of the payment, we retreive it from the DB
	payment, err = GetByID(db, payment.ID, user.ID)
	if err != nil {
		return nil, ErrCouldNotGetByID
	}

	log.WithField("payment", payment.String()).Trace("state in db after being paid")

	return &payment, nil
}

// InvoiceStatusListener is
func InvoiceStatusListener(invoiceUpdatesCh chan *lnrpc.Invoice,
	database *db.DB, sender HttpPoster) {
	for {
		invoice := <-invoiceUpdatesCh
		if invoice == nil {
			log.Errorf("InvoiceStatusListener(): got invoice <nil> from invoiceUpdatesCh")
			return
		}
		hash := hex.EncodeToString(invoice.RHash)
		log.WithField("hash",
			hash,
		).Debug("Received invoice on invoice status listener")
		updated, err := UpdateInvoiceStatus(*invoice, database, sender)
		if err != nil {
			log.Errorf("Error when updating invoice status: %v", err)
			// TODO: Here we need to handle the errors from UpdateInvoiceStatus
		} else {
			log.WithFields(logrus.Fields{"hash": hash,
				"id":      updated.Payment.ID,
				"settled": updated.Payment.SettledAt != nil},
			).Debug("Updated invoice status")
		}
	}
}

// UpdateInvoiceStatus receives messages from lnd's SubscribeInvoices
// (newly added/settled invoices). If received payment was successful, updates
// the payment stored in our db and increases the users balance
func UpdateInvoiceStatus(invoice lnrpc.Invoice, database *db.DB, sender HttpPoster) (
	*PaymentResponse, error) {
	tQuery := "SELECT * FROM offchaintx WHERE payment_request=$1"

	// Define a custom response struct to include user details
	var payment Payment
	if err := database.Get(
		&payment,
		tQuery,
		invoice.PaymentRequest); err != nil {
		return nil, errors.Wrapf(err,
			"UpdateInvoiceStatus->database.Get(&payment, query, %+v)",
			invoice.PaymentRequest,
		)
	}

	log.WithFields(logrus.Fields{
		"id":      payment.ID,
		"settled": invoice.Settled,
	}).Debug("Updating invoice status of payment", payment.ID, invoice.Settled)

	if !invoice.Settled {
		return &PaymentResponse{
			Payment: payment,
		}, nil
	}
	now := time.Now()
	payment.SettledAt = &now
	payment.Status = Status("SUCCEEDED")
	preimage := hex.EncodeToString(invoice.RPreimage)
	payment.Preimage = &preimage

	updateOffchainTxQuery := `UPDATE offchaintx 
		SET status = :status, settled_at = :settled_at, preimage = :preimage
		WHERE hashed_preimage = :hashed_preimage
		RETURNING id, user_id, payment_request, preimage, hashed_preimage,
	   			memo, description, expiry, direction, status, amount_sat, amount_msat,
				callback_url, created_at, updated_at`

	tx := database.MustBegin()

	rows, err := tx.NamedQuery(updateOffchainTxQuery, &payment)
	if err != nil {
		_ = tx.Rollback()
		return nil, errors.Wrapf(err,
			"UpdateInvoiceStatus->tx.NamedQuery(&t, query, %+v)",
			payment,
		)
	}
	rows.Close()

	if rows.Next() {
		if err = rows.Scan(
			&payment.ID,
			&payment.UserID,
			&payment.PaymentRequest,
			&payment.Preimage,
			&payment.HashedPreimage,
			&payment.Memo,
			&payment.Description,
			&payment.Expiry,
			&payment.Direction,
			&payment.Status,
			&payment.AmountSat,
			&payment.AmountMSat,
			&payment.CallbackURL,
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

	_, err = users.IncreaseBalance(tx, users.ChangeBalance{
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
	if payment.CallbackURL != nil {
		if err := postCallback(database, payment, sender); err != nil {
			// don't return here, we don't want this to fail the entire
			// operation
			log.WithError(err).Error("Could not POST to callback URL")
		}
	} else {
		log.WithField("id", payment.ID).Debug("Invoice did not have callback URL")
	}

	return &PaymentResponse{
		Payment: payment,
	}, nil
}

// CallbackBody is the shape of the body we send to a specified payment callback
// URL
type CallbackBody struct {
	Payment Payment `json:"payment"`
	Hash    []byte  `json:"hash"`
}

// TODO: document exact format of callback API. This Node.js snippet replicates
// TODO: the HMAC functionality:
// import crypto from "crypto";
//
// const hashedKey = crypto
// .createHash("sha256")
// .update("my-api-key")
// .digest("hex");
//
// crypto
// .createHmac("sha256", hashedKey)
// .update(payment.id.toString())
// .digest("hex");
func postCallback(database *db.DB, payment Payment, sender HttpPoster) error {
	if payment.CallbackURL == nil {
		return errors.New("callback URL was nil")
	}

	key := apikeys.Key{}
	if err := database.Get(&key,
		`SELECT * FROM api_keys WHERE user_id = $1 LIMIT 1`,
		payment.UserID); err != nil {
		log.WithError(err).
			WithField("userId", payment.UserID).
			Error("Could not get API key for user")
		return err
	}
	hmac := hmac2.New(sha256.New, key.HashedKey)
	_, _ = hmac.Write([]byte(fmt.Sprintf("%d", payment.ID)))

	body := CallbackBody{
		Payment: payment,
		Hash:    hmac.Sum(nil),
	}

	if paymentBytes, err := json.Marshal(body); err != nil {
		log.WithError(err).Error("Could not marshal payment into JSON")
		return err
	} else {
		// naive callback implementation
		// TODO: add logging of when the URL was hit
		go func() {
			logger := log.WithFields(logrus.Fields{
				"url": *payment.CallbackURL,
			})
			var response *http.Response
			retry := func() error {
				res, err := sender.Post(*payment.CallbackURL, "application/json",
					bytes.NewReader(paymentBytes))
				response = res
				return err
			}
			err := async.Retry(5, time.Millisecond*1000, retry)
			if err != nil {
				logger.WithError(err).Error("Error when POSTing callback")
			} else {
				logger.WithField("status", response.StatusCode).Debug("POSTed callback")
			}
		}()
		return nil
	}
}

type HttpPoster interface {
	Post(url, contentType string, reader io.Reader) (*http.Response, error)
}

func (p Payment) String() string {
	str := "\nPayment: {\n"
	str += fmt.Sprintf("\tID: %d\n", p.ID)
	str += fmt.Sprintf("\tUserID: %d\n", p.UserID)
	str += fmt.Sprintf("\tPaymentRequest: %s\n", p.PaymentRequest)
	str += fmt.Sprintf("\tPreimage: %v\n", p.Preimage)
	str += fmt.Sprintf("\tHashedPreimage: %s\n", p.HashedPreimage)
	str += fmt.Sprintf("\tCallbackURL: %v\n", p.CallbackURL)
	str += fmt.Sprintf("\tStatus: %s\n", p.Status)
	if p.Memo != nil {
		str += fmt.Sprintf("\tMemo: %s\n", *p.Memo)
	}
	if p.Description != nil {
		str += fmt.Sprintf("\tDescription: %s\n", *p.Description)
	}
	str += fmt.Sprintf("\tExpiry: %d\n", p.Expiry)
	str += fmt.Sprintf("\tDirection: %s\n", p.Direction)
	str += fmt.Sprintf("\tAmountSat: %d\n", p.AmountSat)
	str += fmt.Sprintf("\tAmountMSat: %d\n", p.AmountMSat)
	str += fmt.Sprintf("\tExpired: %v\n", p.Expired)
	str += fmt.Sprintf("\tExpiresAt: %v\n", p.ExpiresAt)
	str += fmt.Sprintf("\tSettledAt: %v\n", p.SettledAt)
	str += fmt.Sprintf("\tCreatedAt: %v\n", p.CreatedAt)
	str += fmt.Sprintf("\tUpdatedAt: %v\n", p.UpdatedAt)
	str += fmt.Sprintf("\tDeletedAt: %v\n", p.DeletedAt)
	str += "}"

	return str
}

func (p Payment) Equal(other Payment) (bool, string) {
	p.CreatedAt = other.CreatedAt
	p.UpdatedAt = other.UpdatedAt
	p.DeletedAt = other.DeletedAt
	p.ID = other.ID

	if !reflect.DeepEqual(p, other) {
		return false, cmp.Diff(p, other)
	}

	return true, ""
}

func CreateNewPaymentOrFail(t *testing.T, db *db.DB, ln ln.AddLookupInvoiceClient,
	opts NewPaymentOpts) Payment {
	payment, err := NewPayment(db, ln, opts)
	if err != nil {
		testutil.FatalMsg(t,
			errors.Wrap(err, "wasn't able to create new payment"))
	}
	return payment
}
