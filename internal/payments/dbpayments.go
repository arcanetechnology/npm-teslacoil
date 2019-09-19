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

	// MaxAmountMsatPerInvoice is the maximum amount of milli satoshis an invoice
	// can be for
	MaxAmountMsatPerInvoice int64 = 4294967295

	// MaxAmountSatPerInvoice is the maximum amount of satoshis an invoice
	// can be for
	MaxAmountSatPerInvoice int64 = MaxAmountMsatPerInvoice / 1000
)

// Payment is a database table
type Payment struct {
	ID             int       `db:"id" json:"id"`
	UserID         int       `db:"user_id" json:"userId"`
	PaymentRequest string    `db:"payment_request" json:"paymentRequest"`
	Preimage       *string   `db:"preimage" json:"preimage"`
	HashedPreimage string    `db:"hashed_preimage" json:"hash"`
	CallbackURL    *string   `db:"callback_url" json:"callbackUrl"`
	Expiry         int64     `db:"expiry" json:"expiry"`
	Status         Status    `db:"status" json:"status"`
	Memo           *string   `db:"memo" json:"memo"`
	Description    *string   `db:"description" json:"description"`
	Direction      Direction `db:"direction" json:"direction"`
	AmountSat      int64     `db:"amount_sat" json:"amountSat"`
	AmountMSat     int64     `db:"amount_msat" json:"amountMSat"`
	// If defined, it means the  invoice is settled
	SettledAt *time.Time `db:"settled_at" json:"settledAt"`
	CreatedAt time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt time.Time  `db:"updated_at" json:"-"`
	DeletedAt *time.Time `db:"deleted_at" json:"-"`

	// ExpiresAt is the time at which the payment request expires.
	// NOT a db property
	ExpiresAt time.Time `json:"expiresAt"`
	// Expired is not stored in the DB, and can be added to a
	// payment struct by using the function WithAdditionalFields()
	Expired bool `json:"expired"`
}

// UserPaymentResponse is a user payment response
type UserPaymentResponse struct {
	Payment Payment    `json:"payment"`
	User    users.User `json:"user"`
}

// WithAdditionalFields adds useful fields for dealing with a payment
func (p Payment) WithAdditionalFields() Payment {
	p.ExpiresAt = p.GetExpiryDate()
	p.Expired = p.IsExpired()

	return p
}

// GetExpiryDAte adds the expiry(from lnd, in seconds) to CreatedAt
func (p Payment) GetExpiryDate() time.Time {
	return p.CreatedAt.Add(time.Second * time.Duration(p.Expiry))
}

// IsExpired calculates whether the invoice is expired or not
func (p Payment) IsExpired() bool {
	expiresAt := p.CreatedAt.Add(time.Second * time.Duration(p.Expiry))

	// Return whether the expiry date is before the time now
	// We get the UTC time because the db is in UTC time
	return expiresAt.Before(time.Now().UTC())
}

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

	createOffchainTXQuery := `INSERT INTO 
	offchaintx (user_id, payment_request, preimage, hashed_preimage, memo,
		description, expiry, direction, status, amount_sat,amount_msat)
	VALUES (:user_id, :payment_request, :preimage, :hashed_preimage, 
		    :memo, :description, :expiry, :direction, :status, :amount_sat, :amount_msat)
	RETURNING id, user_id, payment_request, preimage, hashed_preimage,
			  memo, description, expiry, direction, status, amount_sat, amount_msat,
			  created_at, updated_at`

	// Using the above query, NamedQuery() will extract VALUES from
	// the payment variable and insert them into the query
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
			&payment.CreatedAt,
			&payment.UpdatedAt,
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
		ORDER BY created_at ASC
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

	if amountSat > MaxAmountSatPerInvoice {
		return lnrpc.Invoice{}, fmt.Errorf(
			"amount (%d) was too large. Max: %d",
			amountSat, MaxAmountSatPerInvoice)
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

	// Sanity check the invoice we just created
	if invoice.Value != amountSat {
		err = fmt.Errorf(
			"could not insert invoice, created invoice amount (%d) not equal request.Amount (%d)",
			invoice.Value, amountSat)
		return lnrpc.Invoice{}, err
	}

	return *invoice, nil
}

// NewPayment creates a new payment by first creating an invoice
// using lnd, then saving info returned from lnd to a new payment
func NewPayment(d *db.DB, lncli ln.AddLookupInvoiceClient, userID int,
	amountSat int64, memo, description string) (Payment, error) {
	tx := d.MustBegin()
	// We do not store the preimage until the payment is settled, to avoid the
	// user getting the preimage before the invoice is settled

	invoice, err := CreateInvoiceWithMemo(lncli, amountSat, memo)
	if err != nil {
		log.Error(err)
		return Payment{}, err
	}

	p := Payment{
		UserID:         userID,
		Memo:           &memo,
		Description:    &description,
		AmountSat:      invoice.Value,
		AmountMSat:     invoice.Value * 1000,
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

	log.Debugf("NewPayment: %v", p)

	if err = tx.Commit(); err != nil {
		log.Error(err)
		_ = tx.Rollback()
		return Payment{}, err
	}

	return p, nil
}

// MarkInvoiceAsPaid marks the given payment request as paid at the given date
func MarkInvoiceAsPaid(d *db.DB, userID int,
	paymentRequest string, paidAt time.Time) error {
	updateOffchainTxQuery := `UPDATE offchaintx 
		SET settled_at = $1, status = $2
		WHERE payment_request = $3`

	tx := d.MustBegin()

	result, err := tx.Exec(updateOffchainTxQuery, paidAt, succeeded, paymentRequest)
	if err != nil {
		log.Errorf("Couldn't mark invoice as paid: %+v", err)
		return err
	}
	rows, _ := result.RowsAffected()
	log.Infof("Marking an invoice as paid resulted in %d updated rows", rows)

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil

}

// PayInvoice is used to Pay an invoice without a description
func PayInvoice(d *db.DB, lncli ln.DecodeSendClient, userID int,
	paymentRequest string) (UserPaymentResponse, error) {
	return PayInvoiceWithDescription(d, lncli, userID, paymentRequest, "")
}

// PayInvoiceWithDescription first persists an outbound payment with the supplied invoice to
// the database. Then attempts to pay the invoice using SendPaymentSync
// Should the payment fail, we do not decrease the users balance.
// This logic is completely fucked, as the user could initiate a payment for
// 10 000 000 satoshis, and the logic wouldn't complain until AFTER the payment
// is complete(that is, we no longer have the money)
// TODO: Decrease the users balance BEFORE attempting to send the payment.
// If at any point the payment/db transaction should fail, increase the users
// balance.
func PayInvoiceWithDescription(d *db.DB, lncli ln.DecodeSendClient, userID int,
	paymentRequest string, description string) (UserPaymentResponse, error) {
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
		Description:    &description,
		PaymentRequest: strings.ToUpper(paymentRequest),
		Status:         open,
		HashedPreimage: payreq.PaymentHash,
		Memo:           &payreq.Description,
		AmountSat:      payreq.NumSatoshis,
		AmountMSat:     payreq.NumSatoshis * 1000,
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
func InvoiceStatusListener(invoiceUpdatesCh chan *lnrpc.Invoice,
	database *db.DB) {
	for {
		invoice := <-invoiceUpdatesCh
		if invoice == nil {
			log.Errorf("InvoiceStatusListener(): got invoice <nil> from invoiceUpdatesCh")
			return
		}
		_, err := UpdateInvoiceStatus(*invoice, database)
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
	var payment Payment
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
			User:    users.User{},
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

// Transaction is the db and json type for an on-chain transaction
type Transaction struct {
	ID        int    `db:"confirmed_at" json:"id"`
	UserID    int    `db:"confirmed_at" json:"userId"`
	Address   string `db:"confirmed_at" json:"address"`
	Txid      string `db:"confirmed_at" json:"txid"`
	AmountSat int64  `db:"confirmed_at" json:"amountSat"`

	CallbackURL *string    `db:"confirmed_at"`
	ConfirmedAt time.Time  `db:"confirmed_at" json:"confirmedAt"`
	CreatedAt   time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time  `db:"updated_at" json:"-"`
	DeletedAt   *time.Time `db:"deleted_at" json:"-"`
}

func SendOnChain(lncli lnrpc.LightningClient, args *lnrpc.SendCoinsRequest) (
	string, error) {
	lnArgs := &lnrpc.SendCoinsRequest{
		Amount:     args.AmountSat,
		Addr:       args.Address,
		TargetConf: int32(args.TargetConf),
		SatPerByte: int64(args.SatPerByte),
	}

	res, err := lncli.SendCoins(context.Background(), lnArgs)
	if err != nil {
		return "", err
	}

	return res.Txid, nil
}

func insertTransaction(tx *sqlx.Tx, transaction Transaction) (Transaction, error) {

}

type WithdrawOnChainArgs struct {
	// The UserID that wants to withdraw funds, omit it from json
	UserID int `json:"-"`
	// The amount in satoshis to send
	AmountSat int64 `json:"amountSat" binding:"required"`
	// The address to send coins to
	Address string `json:"address" binding:"required"`
	// The target number of blocks the transaction should be confirmed by
	TargetConf int `json:"targetConf"`
	// A manual fee rate set in sat/byte that should be used
	SatPerByte int `json:"satPerByte"`
	// If set, amount field will be ignored, and the entire balance will be sent
	SendAll bool `json:"sendAll"`
}

func WithdrawOnChain(d *db.DB, lncli lnrpc.LightningClient,
	args WithdrawOnChainArgs) (string, error) {

	user, err := users.GetByID(d, args.UserID)
	if err != nil {
		return "", errors.Wrap(err, "withdrawonchain could not get user")
	}

	if user.Balance < args.AmountSat {
		return "", errors.New("cannot withdraw, not enough balance")
	}

	// We dont pass sendAll to lncli, as that would send the entire nodes
	// balance to the address
	if args.SendAll {
		args.AmountSat = user.Balance
	}

	SendOnChain(lncli)

	return res.Txid, nil
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
