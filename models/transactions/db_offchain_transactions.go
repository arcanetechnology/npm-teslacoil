package transactions

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
	"strings"
	"time"

	"gitlab.com/arcanecrypto/teslacoil/async"

	"github.com/google/go-cmp/cmp"
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

// NewOffchain creates a new payment by first creating an invoice
// using lnd, then saving info returned from lnd to a new payment
func NewOffchain(d *db.DB, lncli ln.AddLookupInvoiceClient, opts NewOffchainOpts) (Offchain, error) {
	// We do not store the preimage until the payment is settled, to avoid the
	// user getting the preimage before the invoice is settled

	invoice, err := CreateInvoiceWithMemo(lncli, opts.AmountSat, opts.Memo)
	if err != nil {
		log.WithError(err).Error("Could not create invoice")
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
		return Offchain{}, err
	}

	return inserted, nil
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

// MarkInvoiceAsFailed marks the given payment request as paid at the given date
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
		return Offchain{}, Err0AmountInvoiceNotSupported
	}

	hashedPreimage, err := hex.DecodeString(payreq.PaymentHash)
	if err != nil {
		return Offchain{}, err
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
		if err := MarkInvoiceAsFailed(db, paymentRequest); err != nil {
			return Offchain{}, err
		}

		return Offchain{}, errors.New(paymentResponse.PaymentError)
	}

	settledAt := time.Now()
	if err := MarkInvoiceAsPaid(db, paymentRequest, settledAt); err != nil {
		log.WithError(err).Error("Could not mark invoice as paid")
		return Offchain{}, fmt.Errorf("could not mark invoice as paid; %w", err)
	}

	log.WithField("payment", payment.String()).Info("updated payment")

	// to always return the latest state of the payment, we retreive it from the DB
	found, err := GetTransactionByID(db, payment.ID, payment.UserID)
	if err != nil {
		return Offchain{}, ErrCouldNotGetByID
	}

	foundOffchain, err := found.ToOffChain()
	if err != nil {
		return Offchain{}, err
	}

	return foundOffchain, nil
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
				"id":      updated.ID,
				"settled": updated.SettledAt != nil},
			).Debug("Updated invoice status")
		}
	}
}

// UpdateInvoiceStatus receives messages from lnd's SubscribeInvoices
// (newly added/settled invoices). If received payment was successful, updates
// the payment stored in our db and increases the users balance
func UpdateInvoiceStatus(invoice lnrpc.Invoice, database *db.DB, sender HttpPoster) (
	*Offchain, error) {
	tQuery := "SELECT * FROM offchaintx WHERE payment_request=$1"

	var payment Offchain
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
		return &payment, nil
	}
	now := time.Now()
	payment.SettledAt = &now
	payment.Status = Status("SUCCEEDED")
	payment.Preimage = invoice.RPreimage

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

	return &payment, nil
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
func postCallback(database *db.DB, payment Offchain, sender HttpPoster) error {
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

func (p Offchain) String() string {
	fragments := []string{
		"Offchain: {",
		fmt.Sprintf("ID: %d", p.ID),
		fmt.Sprintf("UserID: %d", p.UserID),
		fmt.Sprintf("PaymentRequest: %s", p.PaymentRequest),
		fmt.Sprintf("Preimage: %v", p.Preimage),
		fmt.Sprintf("HashedPreimage: %s", p.HashedPreimage),
		fmt.Sprintf("CallbackURL: %v", p.CallbackURL),
		fmt.Sprintf("Status: %s", p.Status),
	}

	if p.Memo != nil {
		fragments = append(fragments, fmt.Sprintf("Memo: %s", *p.Memo))
	}
	if p.Description != nil {
		fragments = append(fragments, fmt.Sprintf("Description: %s", *p.Description))
	}

	fragments = append(fragments,
		fmt.Sprintf("Expiry: %d", p.Expiry),
		fmt.Sprintf("Direction: %s", p.Direction),
		fmt.Sprintf("AmountSat: %d", p.AmountSat),
		fmt.Sprintf("AmountMSat: %d", p.AmountMSat),
		fmt.Sprintf("Expired: %v", p.Expired),
		fmt.Sprintf("ExpiresAt: %v", p.ExpiresAt),
		fmt.Sprintf("SettledAt: %v", p.SettledAt),
		fmt.Sprintf("CreatedAt: %v", p.CreatedAt),
		fmt.Sprintf("UpdatedAt: %v", p.UpdatedAt),
		fmt.Sprintf("DeletedAt: %v", p.DeletedAt),
		"}",
	)

	return strings.Join(fragments, ", ")
}

func (p Offchain) Equal(other Offchain) (bool, string) {
	p.CreatedAt = other.CreatedAt
	p.UpdatedAt = other.UpdatedAt
	p.DeletedAt = other.DeletedAt
	p.ID = other.ID

	if !reflect.DeepEqual(p, other) {
		return false, cmp.Diff(p, other)
	}

	return true, ""
}
