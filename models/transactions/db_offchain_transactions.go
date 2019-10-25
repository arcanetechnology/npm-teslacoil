package transactions

import (
	"bytes"
	"context"
	hmac2 "crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"gitlab.com/arcanecrypto/teslacoil/async"
	"gitlab.com/arcanecrypto/teslacoil/models/users/balance"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/ln"
	"gitlab.com/arcanecrypto/teslacoil/models/apikeys"
)

var (
	ErrCouldNotGetByID            = errors.New("could not get payment by ID")
	ErrUserBalanceTooLow          = errors.New("user balance too low, cant decrease")
	Err0AmountInvoiceNotSupported = errors.New("cant insert 0 amount invoice, not yet supported")
)

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

// NewOffchainOpts are the different options that dictates creation of a new payment
type NewOffchainOpts struct {
	UserID      int
	AmountSat   int64
	Memo        string
	Description string
	CallbackURL string
	OrderId     string
}

func (o NewOffchainOpts) toFields() logrus.Fields {
	return logrus.Fields{
		"userId":      o.UserID,
		"amountSat":   o.AmountSat,
		"memo":        o.Memo,
		"description": o.Description,
		"callbackUrl": o.CallbackURL,
		"orderId":     o.OrderId,
	}
}

// NewOffchain creates a new payment by first creating an invoice
// using lnd, then saving info returned from lnd to a new payment
func NewOffchain(d *db.DB, lncli ln.AddLookupInvoiceClient, opts NewOffchainOpts) (Offchain, error) {
	// We do not store the preimage until the payment is settled, to avoid the
	// user getting the preimage before the invoice is settled

	invoice, err := CreateInvoiceWithMemo(lncli, opts.AmountSat, opts.Memo)
	if err != nil {
		log.WithError(err).WithFields(opts.toFields()).Error("Could not create invoice")
		return Offchain{}, err
	}

	tx := Offchain{
		UserID:         opts.UserID,
		AmountSat:      invoice.Value,
		AmountMSat:     invoice.Value * 1000,
		Expiry:         invoice.Expiry,
		PaymentRequest: invoice.PaymentRequest,
		HashedPreimage: invoice.RHash,
		Status:         OPEN,
		Direction:      INBOUND,
	}
	if opts.Memo != "" {
		tx.Memo = &opts.Memo
	}
	if opts.Description != "" {
		tx.Description = &opts.Description
	}
	if opts.CallbackURL != "" {
		tx.CallbackURL = &opts.CallbackURL
	}
	if opts.OrderId != "" {
		tx.CustomerOrderId = &opts.OrderId
	}

	inserted, err := InsertOffchain(d, tx)
	if err != nil {
		log.WithError(err).WithFields(opts.toFields()).Error("Could not insert invoice")
		return Offchain{}, err
	}

	return inserted, nil
}

// PayInvoice is used to Pay an invoice without a description
func PayInvoice(d *db.DB, lncli ln.DecodeSendClient, userID int,
	paymentRequest string) (Offchain, error) {
	return PayInvoiceWithDescription(d, lncli, userID, paymentRequest, "")
}

// PayInvoiceWithDescription first persists an outbound payment with the supplied invoice to
// the database. Then attempts to pay the invoice using SendOffchainSync
// Should the payment fail, we rollback all changes made to the DB
func PayInvoiceWithDescription(db *db.DB, lncli ln.DecodeSendClient, userID int,
	paymentRequest string, description string) (Offchain, error) {
	payreq, err := lncli.DecodePayReq(
		context.Background(),
		&lnrpc.PayReqString{PayReq: paymentRequest})

	if err != nil {
		return Offchain{}, err
	}

	if payreq.NumSatoshis == 0 {
		log.WithFields(logrus.Fields{
			"userId":  userID,
			"invoice": paymentRequest,
		}).Warn("User tried to pay zero amount invoice")
		return Offchain{}, Err0AmountInvoiceNotSupported
	}

	hashedPreimage, err := hex.DecodeString(payreq.PaymentHash)
	if err != nil {
		return Offchain{}, err
	}

	userBalance, err := balance.ForUser(db, userID)
	if err != nil {
		return Offchain{}, nil
	}
	if userBalance.Sats() < payreq.NumSatoshis {
		log.WithFields(logrus.Fields{
			"userId":          userID,
			"balanceSats":     userBalance.MilliSats(),
			"requestedAmount": payreq.NumSatoshis,
		}).Warn("User tried to pay invoice for more than their balance")
		return Offchain{}, ErrUserBalanceTooLow
	}

	// insert pay_req into DB
	payment := Offchain{
		UserID:         userID,
		PaymentRequest: paymentRequest,
		HashedPreimage: hashedPreimage,
		Expiry:         payreq.Expiry,
		Status:         OPEN,
		Memo:           &payreq.Description,
		Description:    &description,
		Direction:      OUTBOUND,
		AmountSat:      payreq.NumSatoshis,
		AmountMSat:     payreq.NumSatoshis * 1000,
	}

	payment, err = InsertOffchain(db, payment)
	if err != nil {
		log.WithError(err).Error("Could not insert offchain TX into DB")
		return Offchain{}, fmt.Errorf("could not insert offchain TX into DB: %w", err)
	}

	// attempt to pay invoice
	// TODO make this non sync
	paymentResponse, err := lncli.SendPaymentSync(
		context.Background(), &lnrpc.SendRequest{
			PaymentRequest: paymentRequest,
		})
	if err != nil {
		log.WithError(err).Error("Could not send offchain TX")
		return Offchain{}, fmt.Errorf("could not send offchain TX: %w", err)
	}

	log.WithFields(logrus.Fields{
		"paymentError": paymentResponse.PaymentError,
		"paymentHash":  hex.EncodeToString(paymentResponse.PaymentHash),
		"paymentRoute": paymentResponse.PaymentRoute,
	}).Info("Tried sending payment")

	if paymentResponse.PaymentError != "" {
		failed, err := payment.MarkAsFailed(db)
		if err != nil {
			return Offchain{}, err
		}

		return failed, errors.New(paymentResponse.PaymentError)
	}

	paid, err := payment.MarkAsPaid(db, time.Now())
	if err != nil {
		return Offchain{}, err
	}

	return paid, nil
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
		).Info("Received invoice on invoice status listener")
		updated, err := UpdateInvoiceStatus(*invoice, database, sender)
		if err != nil {
			log.WithError(err).Error("Error when updating invoice status")
			// TODO: Here we need to handle the errors from UpdateInvoiceStatus
		} else {
			log.WithFields(logrus.Fields{"hash": hash,
				"id":      updated.ID,
				"settled": updated.SettledAt != nil},
			).Info("Updated invoice status")
		}
	}
}

// UpdateInvoiceStatus receives messages from lnd's SubscribeInvoices
// (newly added/settled invoices). If received payment was successful, updates
// the payment stored in our DB.
func UpdateInvoiceStatus(invoice lnrpc.Invoice, database db.InsertGetter, sender HttpPoster) (
	Offchain, error) {
	tQuery := "SELECT * FROM transactions WHERE payment_request=$1"

	var selectTx Transaction
	if err := database.Get(
		&selectTx,
		tQuery,
		invoice.PaymentRequest); err != nil {
		log.WithError(err).WithField("paymentRequest",
			invoice.PaymentRequest).Error("Could not read TX from DB")
		return Offchain{}, errors.Wrapf(err,
			"UpdateInvoiceStatus->database.Get(&payment, query, %+v)",
			invoice.PaymentRequest,
		)
	}
	payment, err := selectTx.ToOffchain()
	if err != nil {
		return Offchain{}, err
	}

	log.WithFields(logrus.Fields{
		"id":      payment.ID,
		"settled": invoice.Settled,
	}).Debug("Updating invoice status of payment")

	if !invoice.Settled {
		return Offchain{}, nil
	}

	now := time.Now()
	payment.SettledAt = &now
	payment.Status = SUCCEEDED
	payment.Preimage = invoice.RPreimage

	updatedTx := payment.ToTransaction()
	updateOffchainTxQuery := `UPDATE transactions 
		SET invoice_status = :invoice_status, settled_at = :settled_at, preimage = :preimage
		WHERE hashed_preimage = :hashed_preimage ` + txReturningSql

	rows, err := database.NamedQuery(updateOffchainTxQuery, &updatedTx)
	if err != nil {
		return Offchain{}, err
	}
	defer rows.Close()

	if !rows.Next() {
		return Offchain{}, fmt.Errorf("could not update offchain TX: %w", sql.ErrNoRows)
	}
	var tx Transaction
	if err := rows.StructScan(&tx); err != nil {
		return Offchain{}, fmt.Errorf("could not read TX from DB: %w", err)
	}

	inserted, err := tx.ToOffchain()
	if err != nil {
		return Offchain{}, fmt.Errorf("could not convert TX to offchain TX: %w", err)
	}

	if inserted.CallbackURL != nil {
		if err := postCallback(database, inserted, sender); err != nil {
			// don't return here, we don't want this to fail the entire
			// operation
			log.WithError(err).Error("Could not POST to callback URL")
		}
	} else {
		log.WithField("id", payment.ID).Debug("Invoice did not have callback URL")
	}

	return inserted, nil
}

// CallbackBody is the shape of the body we send to a specified payment callback
// URL
type CallbackBody struct {
	Offchain Offchain `json:"payment"`
	Hash     []byte   `json:"hash"`
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
func postCallback(database db.Getter, payment Offchain, sender HttpPoster) error {
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
		Offchain: payment,
		Hash:     hmac.Sum(nil),
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
