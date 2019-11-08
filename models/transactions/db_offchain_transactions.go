package transactions

import (
	"bytes"
	"context"
	hmac2 "crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"gitlab.com/arcanecrypto/teslacoil/async"
	"gitlab.com/arcanecrypto/teslacoil/models/users/balance"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/sirupsen/logrus"

	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/ln"
	"gitlab.com/arcanecrypto/teslacoil/models/apikeys"
)

var (
	ErrCouldNotGetByID            = errors.New("could not get payment by ID")
	Err0AmountInvoiceNotSupported = errors.New("cant insert 0 amount invoice, not yet supported")
	ErrExpectedOpenStatus         = fmt.Errorf("expected invoice status to be %s", lnrpc.Invoice_InvoiceState_name[int32(lnrpc.Invoice_OPEN)])
	ErrExpectedSettledStatus      = fmt.Errorf("expected invoice status to be %s", lnrpc.Invoice_InvoiceState_name[int32(lnrpc.Invoice_SETTLED)])
	ErrCannotPayOwnInvoice        = errors.New("cannot pay own invoice")
	ErrCouldNotDecodePayReq       = errors.New("could not decode payment request")
)

// InsertOffchain inserts the given offchain TX into the DB
func InsertOffchain(db db.Inserter, offchain Offchain) (Offchain, error) {
	tx, err := insertTransaction(db, offchain.ToTransaction())
	if err != nil {
		return Offchain{}, err
	}
	insertedOffchain, err := tx.ToOffchain()
	if err != nil {
		return Offchain{}, fmt.Errorf("could not convert inserted TX to offchain TX: %w", err)
	}

	// if preimage is NULL in DB, default is empty slice and not null
	if tx.Preimage != nil && len(*tx.Preimage) == 0 {
		insertedOffchain.Preimage = nil
	}

	return insertedOffchain, nil
}

// GetOffchainByID retrieves a transaction with `ID` for `userID` .
// if the transaction cannot be converted to an Offchain transaction
// an error is returned
func GetOffchainByID(database *db.DB, id int, userID int) (Offchain, error) {
	tx, err := GetTransactionByID(database, id, userID)
	if err != nil {
		return Offchain{}, err
	}
	offchain, err := tx.ToOffchain()
	if err != nil {
		return Offchain{}, fmt.Errorf("requested TX was not offchain TX: %w", err)
	}
	return offchain, nil
}

// GetOffchainByPaymentRequest retrieves a Offchain transaction from the database
// with the specified paymentRequest and userID
func GetOffchainByPaymentRequest(database *db.DB, paymentRequest string, userID int) (Offchain, error) {
	query := "SELECT * FROM transactions WHERE user_id=$1 AND payment_request=$2"

	var selectedTx Transaction
	if err := database.Get(&selectedTx, query, userID, paymentRequest); err != nil {
		log.WithError(err).WithField("paymentRequest",
			paymentRequest).Error("could not get TX from DB")
		return Offchain{}, err
	}

	offchain, err := selectedTx.ToOffchain()
	if err != nil {
		log.WithError(err).WithField("ID", offchain.ID).Error("could not convert to offchain")
		return Offchain{}, err
	}

	return offchain, nil
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
			Value: amountSat,
		})
	if err != nil {
		err = fmt.Errorf("could not add invoice to lnd: %w", err)
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

// CreateTeslacoilInvoice creates a new lightning invoice by first creating an
// invoice using lnd, then saving info returned from lnd to a new offchain tx
func CreateTeslacoilInvoice(database *db.DB, lncli ln.AddLookupInvoiceClient, opts NewOffchainOpts) (
	Offchain, error) {

	invoice, err := CreateInvoiceWithMemo(lncli, opts.AmountSat, opts.Memo)
	if err != nil {
		log.WithError(err).WithFields(opts.toFields()).Error("Could not create invoice")
		return Offchain{}, err
	}

	// We do not store the preimage until the payment is settled, to avoid the
	// user getting the preimage before the invoice is settled
	tx := Offchain{
		UserID:         opts.UserID,
		AmountMilliSat: invoice.Value * 1000,
		Expiry:         invoice.Expiry,
		PaymentRequest: invoice.PaymentRequest,
		HashedPreimage: invoice.RHash,
		Status:         InvoiceStateToTeslaState[invoice.State],
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

	inserted, err := InsertOffchain(database, tx)
	if err != nil {
		log.WithError(err).WithFields(opts.toFields()).WithField("expiry",
			invoice.Expiry).Error("Could not insert invoice")
		return Offchain{}, err
	}

	return inserted, nil
}

// payReqBelongsToTeslacoilUser checks whether a payment request belongs
// to teslacoil by SELECTING INBOUND transactions from the db. Returns the INBOUND
// offchain transaction if it exists
func payReqBelongsToTeslacoilUser(db *db.DB, paymentRequest string, userID int) (*Offchain, error) {
	query := "SELECT * FROM transactions WHERE payment_request=$1 AND direction = $2"

	var selectedTx Transaction
	err := db.Get(&selectedTx, query, paymentRequest, INBOUND)
	if errors.Is(err, sql.ErrNoRows) {
		// does not belong to us
		return nil, err
	}
	if err != nil {
		log.WithError(err).WithField("paymentRequest",
			paymentRequest).Error("could not get TX from DB")
		return nil, err
	}

	if selectedTx.UserID == userID {
		return nil, ErrCannotPayOwnInvoice
	}

	offchain, err := selectedTx.ToOffchain()
	if err != nil {
		return nil, err
	}

	return &offchain, nil
}

func sendOffchain(db *db.DB, lncli ln.DecodeSendClient, callbacker HttpPoster, payment Offchain) (Offchain, error) {
	// TODO(bo): Add a websocket here, sending a message to the user that
	//  the payment is initiated

	paymentResponse, err := lncli.SendPaymentSync(
		context.Background(), &lnrpc.SendRequest{
			PaymentRequest: payment.PaymentRequest,
		})
	if err != nil {
		return Offchain{}, fmt.Errorf("could not send offchain TX: %w", err)
	}

	log.WithFields(logrus.Fields{
		"paymentError": paymentResponse.PaymentError,
		"paymentHash":  hex.EncodeToString(paymentResponse.PaymentHash),
		"paymentRoute": paymentResponse.PaymentRoute,
	}).Info("tried sending payment")

	if paymentResponse.PaymentError != "" {
		failed, err := payment.MarkAsFlopped(db)
		if err != nil {
			return Offchain{}, err
		}

		return failed, errors.New(paymentResponse.PaymentError)
	}

	paid, err := payment.MarkAsCompleted(db, paymentResponse.PaymentPreimage, callbacker)
	if err != nil {
		return Offchain{}, err
	}

	// TODO(bo): Add a websocket here, sending a message to the user that
	//  the payment is completed

	return paid, nil
}

// PayInvoice is used to Pay an invoice without a description
func PayInvoice(d *db.DB, lncli lnrpc.LightningClient, callbacker HttpPoster, userID int,
	paymentRequest string) (Offchain, error) {
	return PayInvoiceWithDescription(d, lncli, callbacker, userID, paymentRequest, "")
}

// PayInvoiceWithDescription first persists an outbound payment with the supplied invoice to
// the database. Then attempts to pay the invoice using SendOffchainSync
// Should the payment fail, we rollback all changes made to the DB
func PayInvoiceWithDescription(database *db.DB, lncli lnrpc.LightningClient, callbacker HttpPoster,
	userID int, paymentRequest string, description string) (Offchain, error) {

	decoded, err := lncli.DecodePayReq(
		context.Background(),
		&lnrpc.PayReqString{PayReq: paymentRequest})
	if err != nil {
		return Offchain{}, fmt.Errorf("%v: %w", ErrCouldNotDecodePayReq, err)
	}

	log.WithFields(logrus.Fields{
		"paymentRequest":  paymentRequest,
		"memo":            decoded.Description,
		"hash":            decoded.PaymentHash,
		"fallbackAddress": decoded.FallbackAddr,
		"expiry":          decoded.Expiry,
		"numSats":         decoded.NumSatoshis,
		"destination":     decoded.Destination,
		"cltvExpiry":      decoded.CltvExpiry,
	}).Infof("paying payment request")

	if decoded.NumSatoshis == 0 {
		log.WithFields(logrus.Fields{
			"userId":  userID,
			"invoice": paymentRequest,
		}).Warn("user tried to pay zero amount invoice")
		return Offchain{}, Err0AmountInvoiceNotSupported
	}

	hashedPreimage, err := hex.DecodeString(decoded.PaymentHash)
	if err != nil {
		return Offchain{}, err
	}

	// insert pay_req into DB
	payment := Offchain{
		UserID:         userID,
		PaymentRequest: paymentRequest,
		HashedPreimage: hashedPreimage,
		Expiry:         decoded.Expiry,
		Status:         Offchain_CREATED,
		Memo:           &decoded.Description,
		Description:    &description,
		Direction:      OUTBOUND,
		AmountMilliSat: decoded.NumSatoshis * 1000,
	}

	// we insert the payment before calculating balance to ensure
	// all outgoing payments are included in the balance calculation
	payment, err = InsertOffchain(database, payment)
	if err != nil {
		log.WithError(err).Error("Could not insert offchain TX into DB")
		return Offchain{}, fmt.Errorf("could not insert offchain TX into DB: %w", err)
	}

	userBalance, err := balance.ForUser(database, userID)
	if err != nil {
		return Offchain{}, nil
	}
	if userBalance.Sats() < decoded.NumSatoshis {
		log.WithFields(logrus.Fields{
			"userId":          userID,
			"balanceSats":     userBalance.Sats(),
			"requestedAmount": decoded.NumSatoshis,
		}).Warn("User tried to pay invoice for more than their balance")
		return Offchain{}, ErrBalanceTooLow
	}

	inboundTransaction, err := payReqBelongsToTeslacoilUser(database, payment.PaymentRequest, payment.UserID)
	// first we check for specific errors we need to send up the chain
	if errors.Is(err, ErrCannotPayOwnInvoice) {
		log.WithError(err).Error("cannot pay own invoice")
		return Offchain{}, err
	}

	// if inboundTransaction is nil, the paymentRequest was not found in our
	// database, or something somewhere went wrong in payReqBelongsToTeslacoilUser
	// however, we don't care what or if anything went wrong, we just want to pay
	// the invoice
	if inboundTransaction == nil {
		payment, err = sendOffchain(database, lncli, callbacker, payment)
		if err != nil {
			return Offchain{}, fmt.Errorf("could not send offchain payment: %w", err)
		}
	} else {
		payment, err = settleInternalTransfer(database, lncli, payment, *inboundTransaction, callbacker)
		if err != nil {
			return Offchain{}, fmt.Errorf("could not settle internal transfer: %w", err)
		}
	}

	return payment, nil
}

func lookupPreimage(lncli ln.AddLookupInvoiceClient, rHash []byte) ([]byte, error) {
	invoice, err := lncli.LookupInvoice(context.Background(), &lnrpc.PaymentHash{
		RHash: rHash,
	})
	if err != nil {
		return nil, fmt.Errorf("could not lookup invoice: %w", err)
	}

	return invoice.RPreimage, nil
}

func settleInternalTransfer(database *db.DB, lncli ln.AddLookupInvoiceClient, outbound Offchain,
	inbound Offchain, callbacker HttpPoster) (Offchain, error) {

	if len(outbound.HashedPreimage) == 0 {
		return Offchain{}, fmt.Errorf("outbound offchain transaction does not have a hashed preimage")
	}

	preimage, err := lookupPreimage(lncli, outbound.HashedPreimage)
	if err != nil {
		return Offchain{}, err
	}

	tx := database.MustBegin()
	outbound, err = outbound.MarkAsCompleted(tx, preimage, callbacker)
	if err != nil {
		_ = tx.Rollback()
		return Offchain{}, err
	}

	_, err = inbound.MarkAsCompleted(tx, preimage, callbacker)
	if err != nil {
		_ = tx.Rollback()
		return Offchain{}, err
	}
	if err = tx.Commit(); err != nil {
		return Offchain{}, err
	}

	return outbound, nil
}

// InvoiceListener receives lnrpc.Invoices on a channel and handles them
// according to their State
func InvoiceListener(invoiceUpdatesCh chan *lnrpc.Invoice,
	database *db.DB, callbacker HttpPoster) {
	for {
		invoice := <-invoiceUpdatesCh
		if invoice == nil {
			log.Errorf("got invoice <nil> from invoiceUpdatesCh")
			return
		}

		log.WithField("hash", hex.EncodeToString(invoice.RHash)).
			Info("received invoice on invoice status listener")

		switch invoice.State {
		case lnrpc.Invoice_OPEN: // created, not yet confirmed paid
			log.WithField("paymentRequest", invoice.PaymentRequest).
				Tracef("no action required for an OPEN invoice, logic handled in CreateTeslacoilInvoice")

		case lnrpc.Invoice_SETTLED: // deposit success!
			offchain, err := HandleSettledInvoice(*invoice, database, callbacker)
			if err != nil {
				log.WithError(err).Error("could not handle settled invoice")
				continue
			}

			log.WithFields(logrus.Fields{
				"hash":   hex.EncodeToString(offchain.HashedPreimage),
				"id":     offchain.ID,
				"status": offchain.Status,
			},
			).Info("updated invoice status")

		case lnrpc.Invoice_CANCELED | lnrpc.Invoice_ACCEPTED: // hold invoices
			// we panic because somewhere in our code we used lncli.AddHoldInvoice(),
			// but we're not ready for that yet
			log.Panicf("hold invoices are not implemented yet")
		default:
			log.WithField("invoice", invoice).Error("invoice has unknown state")
		}
	}
}

// HandleSettledInvoice allows you (at any point in time) to send in an invoice
// and update the state in the database
// invoices whose status is not settled is rejected and an error is returned
func HandleSettledInvoice(invoice lnrpc.Invoice, database db.InsertGetter,
	callbacker HttpPoster) (Offchain, error) {

	if invoice.State != lnrpc.Invoice_SETTLED {
		return Offchain{}, ErrExpectedSettledStatus
	}

	log.WithFields(logrus.Fields{
		"paymentRequest":  invoice.PaymentRequest,
		"amtPaidMilliSat": invoice.AmtPaidMsat,
		"addIndex":        invoice.AddIndex,
		"hash":            hex.EncodeToString(invoice.RHash),
	}).Info("updating status of SETTLED invoice")

	// select transactions
	tQuery := "SELECT * FROM transactions WHERE payment_request=$1"

	var selectTx Transaction
	if err := database.Get(&selectTx, tQuery,
		invoice.PaymentRequest); err != nil {

		log.WithError(err).WithField("paymentRequest",
			invoice.PaymentRequest).Error("Could not read TX from DB")
		return Offchain{}, fmt.Errorf("could not read TX from DB: %w", err)

	}

	offchainInvoice, err := selectTx.ToOffchain()
	if err != nil {
		return Offchain{}, err
	}

	now := time.Now()
	offchainInvoice.SettledAt = &now
	offchainInvoice.Status = Offchain_COMPLETED
	offchainInvoice.Preimage = invoice.RPreimage

	// TODO: In the lightning spec, it is allowed to pay up to 2x the invoice
	//  amount (and the node should accept it). How do we make this clear to
	//  the merchant? I imagine searching in amounts is pretty important
	//  Should we add a new field to the db, e.g. overpaidAmount? and give it
	//  to them every month? Keep it ourself?
	if invoice.AmtPaidMsat != offchainInvoice.AmountMilliSat {
		log.WithFields(logrus.Fields{
			"expected": offchainInvoice.AmountMilliSat,
			"paid":     invoice.AmtPaidMsat,
		}).Warn("amount paid not equal to expected amount")
	}
	offchainInvoice.AmountMilliSat = invoice.AmtPaidMsat

	updatedTx := offchainInvoice.ToTransaction()
	updateOffchainTxQuery := `UPDATE transactions 
		SET invoice_status = :invoice_status, settled_at = :settled_at, preimage = :preimage
		WHERE hashed_preimage = :hashed_preimage ` + txReturningSql

	rows, err := database.NamedQuery(updateOffchainTxQuery, &updatedTx)
	if err != nil {
		return Offchain{}, err
	}

	if !rows.Next() {
		return Offchain{}, fmt.Errorf("could not update offchain TX: %w", sql.ErrNoRows)
	}
	var tx Transaction
	if err = rows.StructScan(&tx); err != nil {
		return Offchain{}, fmt.Errorf("could not read TX from DB: %w", err)
	}
	db.CloseRows(rows)

	inserted, err := tx.ToOffchain()
	if err != nil {
		return Offchain{}, fmt.Errorf("could not convert TX to offchain TX: %w", err)
	}

	// call the callback URL(if exists)
	if inserted.CallbackURL != nil {
		if err = postCallback(database, inserted, callbacker); err != nil {
			// don't return here, we don't want this to fail the entire
			// operation
			log.WithError(err).Error("Could not POST to callback URL")
		}
	} else {
		log.WithField("id", inserted.ID).Debug("invoice did not have callback URL")
	}

	log.Tracef("invoice is settled: %+v", inserted)

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
func postCallback(database db.Getter, payment Offchain, callbacker HttpPoster) error {
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
				res, err := callbacker.Post(*payment.CallbackURL, "application/json",
					bytes.NewReader(paymentBytes))
				response = res
				return err
			}
			err = async.RetryBackoff(5, time.Millisecond*1000, retry)
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
