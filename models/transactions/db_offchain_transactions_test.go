package transactions_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/testutil/userstestutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/arcanecrypto/teslacoil/async"
	"gitlab.com/arcanecrypto/teslacoil/models/transactions"
	"gitlab.com/arcanecrypto/teslacoil/models/users/balance"
	"gitlab.com/arcanecrypto/teslacoil/testutil/txtest"

	"github.com/brianvoe/gofakeit"

	"gitlab.com/arcanecrypto/teslacoil/models/users"

	"github.com/lightningnetwork/lnd/lnrpc"

	"gitlab.com/arcanecrypto/teslacoil/ln"
	"gitlab.com/arcanecrypto/teslacoil/models/apikeys"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/lntestutil"

	"gitlab.com/arcanecrypto/teslacoil/db"
)

var (
	SamplePreimage = func() []byte {
		encoded, _ := hex.DecodeString(SamplePreimageHex)
		return encoded
	}()
	SamplePreimageHex = "0123456789abcdef0123456789abcdef"
	SampleHash        = func() [32]byte {
		first := sha256.Sum256(SamplePreimage)
		return sha256.Sum256(first[:])
	}()

	SampleHashHex = hex.EncodeToString(SampleHash[:])
	firstMemo     = "HiMisterHey"
	description   = "My personal description"
)

func TestNewOffchainTx(t *testing.T) {
	t.Parallel()
	user := CreateUserOrFail(t, testDB)

	amount1 := rand.Int63n(ln.MaxAmountSatPerInvoice)
	amount2 := rand.Int63n(ln.MaxAmountSatPerInvoice)
	amount3 := rand.Int63n(ln.MaxAmountSatPerInvoice)

	payReq1 := txtest.MockPaymentRequest()
	payReq2 := txtest.MockPaymentRequest()
	payReq3 := txtest.MockPaymentRequest()

	customerOrderId := "this is an order id"

	tests := []struct {
		memo        string
		description string
		amountSat   int64
		orderId     string

		lndInvoice lnrpc.Invoice
		want       transactions.Offchain
	}{
		{
			memo:        firstMemo,
			description: description,
			amountSat:   amount1,

			lndInvoice: lnrpc.Invoice{
				Value:          amount1,
				PaymentRequest: payReq1,
				RHash:          SampleHash[:],
				RPreimage:      SamplePreimage,
				Expiry:         1337,
				State:          lnrpc.Invoice_OPEN,
			},
			want: transactions.Offchain{
				UserID:         user.ID,
				PaymentRequest: payReq1,
				AmountSat:      amount1,
				AmountMilliSat: amount1 * 1000,
				HashedPreimage: SampleHash[:],
				Memo:           &firstMemo,
				Description:    &description,
				Status:         transactions.Offchain_CREATED,
				Direction:      transactions.INBOUND,
			},
		},
		{
			memo:        firstMemo,
			description: description,
			amountSat:   amount2,

			lndInvoice: lnrpc.Invoice{
				Value:          amount2,
				PaymentRequest: payReq2,
				RHash:          SampleHash[:],
				RPreimage:      SamplePreimage,
				Expiry:         1337,
				State:          lnrpc.Invoice_OPEN,
			},
			want: transactions.Offchain{
				UserID:         user.ID,
				AmountSat:      amount2,
				AmountMilliSat: amount2 * 1000,
				HashedPreimage: SampleHash[:],
				PaymentRequest: payReq2,
				Memo:           &firstMemo,
				Description:    &description,
				Status:         transactions.Offchain_CREATED,
				Direction:      transactions.INBOUND,
			},
		},
		{
			memo:        firstMemo,
			description: description,
			amountSat:   amount3,
			orderId:     customerOrderId,

			lndInvoice: lnrpc.Invoice{
				Value:          amount3,
				PaymentRequest: payReq3,
				RHash:          SampleHash[:],
				RPreimage:      SamplePreimage,
				Expiry:         1337,
				State:          lnrpc.Invoice_OPEN,
			},
			want: transactions.Offchain{
				UserID:          user.ID,
				AmountSat:       amount3,
				AmountMilliSat:  amount3 * 1000,
				Memo:            &firstMemo,
				HashedPreimage:  SampleHash[:],
				PaymentRequest:  payReq3,
				Description:     &description,
				Status:          transactions.Offchain_CREATED,
				Direction:       transactions.INBOUND,
				CustomerOrderId: &customerOrderId,
			},
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d create invoice with amount %d memo %s and"+
			" description %s",
			i, tt.amountSat, tt.memo, tt.description), func(t *testing.T) {
			// this test can not be run in parallell, messes up lncli

			// Create Mock LND client with preconfigured invoice response
			mockLNcli := lntestutil.LightningMockClient{
				InvoiceResponse: tt.lndInvoice,
			}

			offchainTx, err := transactions.CreateTeslacoilInvoice(testDB, mockLNcli, transactions.NewOffchainOpts{
				UserID:      tt.want.UserID,
				AmountSat:   tt.amountSat,
				Memo:        tt.memo,
				Description: tt.description,
				OrderId:     tt.orderId,
			})
			require.NoError(t, err)

			// Assertions
			got := offchainTx
			want := tt.want

			assert := assert.New(t)
			assert.Equal(want.AmountMilliSat, got.AmountMilliSat)
			assert.Equal(want.SettledAt, got.SettledAt)
			assert.Equal(want.Memo, got.Memo)
			assert.Equal(want.Description, got.Description)
			assert.Equal(want.CallbackURL, got.CallbackURL)
			assert.Equal(want.UserID, got.UserID)
			assert.Equal(want.CustomerOrderId, got.CustomerOrderId)
			assert.Equal(want.PaymentRequest, got.PaymentRequest)
			assert.Equal(want.Status, got.Status)
			assert.Equal(want.Direction, got.Direction)
			assert.Equal(want.HashedPreimage, got.HashedPreimage)
			assert.Equal(want.HashedPreimage, got.HashedPreimage)
			assert.Equal(want.Preimage, got.Preimage)
		})
	}

	t.Run("offchain TX without customer order id", func(t *testing.T) {
		t.Parallel()
		offchain, err := transactions.CreateTeslacoilInvoice(testDB, lntestutil.GetLightningMockClient(), transactions.NewOffchainOpts{
			UserID:    user.ID,
			AmountSat: int64(gofakeit.Number(1, ln.MaxAmountSatPerInvoice)),
		})
		require.NoError(t, err)
		assert.Nil(t, offchain.CustomerOrderId)
	})

	t.Run("offchain TX with customer order id", func(t *testing.T) {
		t.Parallel()
		order := gofakeit.Word()
		offchain, err := transactions.CreateTeslacoilInvoice(testDB, lntestutil.GetLightningMockClient(), transactions.NewOffchainOpts{
			UserID:    user.ID,
			AmountSat: int64(gofakeit.Number(1, ln.MaxAmountSatPerInvoice)),
			OrderId:   order,
		})
		require.NoError(t, err)
		assert.NotNil(t, offchain.CustomerOrderId)
		assert.Equal(t, order, *offchain.CustomerOrderId)
	})

	t.Run("offchain TX without description", func(t *testing.T) {
		t.Parallel()
		offchain, err := transactions.CreateTeslacoilInvoice(testDB, lntestutil.GetLightningMockClient(), transactions.NewOffchainOpts{
			UserID:    user.ID,
			AmountSat: int64(gofakeit.Number(1, ln.MaxAmountSatPerInvoice)),
		})
		require.NoError(t, err)
		assert.Nil(t, offchain.Description)
	})

	t.Run("offchain TX with description", func(t *testing.T) {
		t.Parallel()
		desc := gofakeit.Sentence(gofakeit.Number(1, 12))
		offchain, err := transactions.CreateTeslacoilInvoice(testDB, lntestutil.GetLightningMockClient(), transactions.NewOffchainOpts{
			UserID:      user.ID,
			AmountSat:   int64(gofakeit.Number(1, ln.MaxAmountSatPerInvoice)),
			Description: desc,
		})
		require.NoError(t, err)
		require.NotNil(t, offchain.Description)
		assert.Equal(t, desc, *offchain.Description)
	})

	t.Run("offchain TX without memo", func(t *testing.T) {
		t.Parallel()
		offchain, err := transactions.CreateTeslacoilInvoice(testDB, lntestutil.GetLightningMockClient(), transactions.NewOffchainOpts{
			UserID:    user.ID,
			AmountSat: int64(gofakeit.Number(1, ln.MaxAmountSatPerInvoice)),
		})
		require.NoError(t, err)
		assert.Nil(t, offchain.Memo)
	})

	t.Run("offchain TX with memo", func(t *testing.T) {
		t.Parallel()
		memo := gofakeit.Sentence(gofakeit.Number(1, 12))
		offchain, err := transactions.CreateTeslacoilInvoice(testDB, lntestutil.GetLightningMockClient(), transactions.NewOffchainOpts{
			UserID:    user.ID,
			AmountSat: int64(gofakeit.Number(1, ln.MaxAmountSatPerInvoice)),
			Memo:      memo,
		})
		require.NoError(t, err)
		require.NotNil(t, offchain.Memo)
		assert.Equal(t, memo, *offchain.Memo)
	})

	t.Run("offchain TX without callbackURL", func(t *testing.T) {
		t.Parallel()
		offchain, err := transactions.CreateTeslacoilInvoice(testDB, lntestutil.GetLightningMockClient(), transactions.NewOffchainOpts{
			UserID:    user.ID,
			AmountSat: int64(gofakeit.Number(1, ln.MaxAmountSatPerInvoice)),
		})
		require.NoError(t, err)
		assert.Nil(t, offchain.CallbackURL)
	})

	t.Run("offchain TX with callbackURL", func(t *testing.T) {
		t.Parallel()
		url := gofakeit.URL()
		offchain, err := transactions.CreateTeslacoilInvoice(testDB, lntestutil.GetLightningMockClient(), transactions.NewOffchainOpts{
			UserID:      user.ID,
			AmountSat:   int64(gofakeit.Number(1, ln.MaxAmountSatPerInvoice)),
			CallbackURL: url,
		})
		require.NoError(t, err)
		assert.NotNil(t, offchain.CallbackURL)
		assert.Equal(t, url, *offchain.CallbackURL)
	})
}
func TestInsertOffchainTransaction(t *testing.T) {
	t.Parallel()
	user := userstestutil.CreateUserOrFail(t, testDB)
	for i := 0; i < 20; i++ {
		t.Run("inserting arbitrary offchain "+strconv.Itoa(i), func(t *testing.T) {
			t.Parallel()
			offchain := txtest.MockOffchain(user.ID)

			inserted, err := transactions.InsertOffchain(testDB, offchain)
			require.NoError(t, err)

			offchain.CreatedAt = inserted.CreatedAt
			offchain.UpdatedAt = inserted.UpdatedAt

			if offchain.SettledAt != nil {
				if offchain.SettledAt.Sub(*inserted.SettledAt) > time.Millisecond*500 {
					assert.Equal(t, *offchain.SettledAt, *inserted.SettledAt)
				}
				offchain.SettledAt = inserted.SettledAt
			}

			// ID should be created by DB for us
			assert.NotEqual(t, offchain.ID, inserted.ID)
			offchain.ID = inserted.ID
			assert.Equal(t, offchain, inserted)

			foundTx, err := transactions.GetTransactionByID(testDB, inserted.ID, user.ID)
			require.NoError(t, err)

			foundOffChain, err := foundTx.ToOffchain()
			require.NoError(t, err)

			assert.Equal(t, foundOffChain, inserted)

			allTXs, err := transactions.GetAllTransactions(testDB, user.ID, transactions.GetAllParams{})
			require.NoError(t, err)
			found := false
			for _, tx := range allTXs {
				off, err := tx.ToOffchain()
				if err != nil {
					break
				}
				if cmp.Diff(off, inserted) == "" {
					found = true
					break
				}
			}
			assert.True(t, found, "Did not find TX when doing GetAll")
		})

	}
}

func TestOffchain_MarkAsCompleted(t *testing.T) {
	t.Parallel()
	user := CreateUserWithBalanceOrFail(t, testDB, ln.MaxAmountMsatPerInvoice*5)
	tx := txtest.MockOffchain(user.ID)
	inserted, err := transactions.InsertOffchain(testDB, tx)
	require.NoError(t, err)

	paid, err := inserted.MarkAsCompleted(testDB, inserted.Preimage, nil)
	require.NoError(t, err)
	assert.Equal(t, transactions.Offchain_COMPLETED, paid.Status)
	require.NotNil(t, paid.SettledAt)
	assert.WithinDuration(t, time.Now(), *paid.SettledAt, time.Second)
}

func TestOffchain_MarkAsFlopped(t *testing.T) {
	t.Parallel()
	user := CreateUserWithBalanceOrFail(t, testDB, ln.MaxAmountMsatPerInvoice*5)

	var tx transactions.Offchain
	// it only makes sense to mark open TXs as failed
	for tx.Status != transactions.Offchain_CREATED {
		tx = txtest.MockOffchain(user.ID)
	}

	inserted, err := transactions.InsertOffchain(testDB, tx)
	require.NoError(t, err)

	reason := gofakeit.Sentence(12)
	paid, err := inserted.MarkAsFlopped(testDB, reason)
	require.NoError(t, err)
	assert.Equal(t, transactions.Offchain_FLOPPED, paid.Status)
	require.NotNil(t, paid.Error)
	assert.Equal(t, reason, *paid.Error)

	assert.Nil(t, paid.SettledAt)
}

func TestPayInvoice(t *testing.T) {
	t.Parallel()

	amount := int64(gofakeit.Number(1, ln.MaxAmountSatPerInvoice))

	paymentRequest := "SomeOffchainRequest1"

	t.Run("paying invoice decreases balance of user", func(t *testing.T) {
		t.Parallel()
		user := CreateUserWithBalanceOrFail(t, testDB, ln.MaxAmountMsatPerInvoice*5)
		// Create Mock LND client with preconfigured invoice response
		mockLNcli := lntestutil.LightningMockClient{
			SendPaymentSyncResponse: lnrpc.SendResponse{
				PaymentPreimage: SamplePreimage,
				PaymentHash:     SampleHash[:],
			},
			// define what lncli.DecodePayReq returns
			DecodePayReqResponse: lnrpc.PayReq{
				PaymentHash: SampleHashHex,
				NumSatoshis: amount,
				Expiry:      1337,
			},
		}
		balancePrePayment, err := balance.ForUser(testDB, user.ID)
		require.NoError(t, err)

		_, err = transactions.PayInvoice(
			testDB, &mockLNcli, nil, user.ID, paymentRequest)
		require.NoError(t, err)

		_, err = users.GetByID(testDB, user.ID)
		require.NoError(t, err)

		bal, err := balance.ForUser(testDB, user.ID)
		require.NoError(t, err)

		assert.Equal(t, balancePrePayment.Sats()-amount, bal.Sats())
	})

	t.Run("paying invoice greater than balance fails", func(t *testing.T) {
		t.Parallel()
		build.SetLogLevels(logrus.DebugLevel)

		user := CreateUserWithBalanceOrFail(t, testDB, 5)
		balancePrePayment, err := balance.ForUser(testDB, user.ID)
		require.NoError(t, err)

		// Create Mock LND client with preconfigured invoice response
		mockLNcli := lntestutil.LightningMockClient{
			SendPaymentSyncResponse: lnrpc.SendResponse{
				PaymentPreimage: SamplePreimage,
				PaymentHash:     SampleHash[:],
			},
			// define what lncli.DecodePayReq returns
			DecodePayReqResponse: lnrpc.PayReq{
				PaymentHash: SampleHashHex,
				NumSatoshis: balancePrePayment.Sats() + 1,
			},
		}

		_, err = transactions.PayInvoice(testDB, &mockLNcli, nil, user.ID, paymentRequest)
		require.Error(t, err)
		testutil.AssertEqualErr(t, err, transactions.ErrBalanceTooLow)
	})

	t.Run("paying invoice with 0 amount fails with Err0AmountInvoiceNotSupported", func(t *testing.T) {
		t.Parallel()
		user := CreateUserWithBalanceOrFail(t, testDB, ln.MaxAmountMsatPerInvoice*5)
		mockLNcli := lntestutil.LightningMockClient{
			SendPaymentSyncResponse: lnrpc.SendResponse{
				PaymentPreimage: SamplePreimage,
				PaymentHash:     SampleHash[:],
			},
			// define what lncli.DecodePayReq returns
			DecodePayReqResponse: lnrpc.PayReq{
				PaymentHash: SampleHashHex,
				NumSatoshis: 0,
			},
		}

		_, err := transactions.PayInvoice(
			testDB, &mockLNcli, nil, user.ID, paymentRequest)
		if !errors.Is(err, transactions.Err0AmountInvoiceNotSupported) {
			assert.NoError(t, err, "Err was not Err0AmountInvoiceNotSupported")
		}

	})

	t.Run("successfully paying invoice marks invoice settledAt date", func(t *testing.T) {
		t.Parallel()
		user := CreateUserWithBalanceOrFail(t, testDB, ln.MaxAmountMsatPerInvoice*5)
		mockLNcli := lntestutil.LightningMockClient{
			SendPaymentSyncResponse: lnrpc.SendResponse{
				PaymentPreimage: SamplePreimage,
				PaymentHash:     SampleHash[:],
			},
			DecodePayReqResponse: lnrpc.PayReq{
				PaymentHash: SampleHashHex,
				NumSatoshis: amount,
				Expiry:      1000,
			},
		}

		got, err := transactions.PayInvoice(testDB, &mockLNcli, nil, user.ID, paymentRequest)
		require.NoError(t, err)

		assert.Equal(t, transactions.Offchain_COMPLETED, got.Status)
		assert.NotNil(t, got.SettledAt)
		assert.WithinDuration(t, *got.SettledAt, time.Now(), time.Second)
	})

	t.Run("a user cannot pay an invoice they created", func(t *testing.T) {
		t.Parallel()

		user := CreateUserWithBalanceOrFail(t, testDB, ln.MaxAmountSatPerInvoice*5)
		mockLNcli := lntestutil.LightningMockClient{
			DecodePayReqResponse: lnrpc.PayReq{
				PaymentHash: SampleHashHex,
				NumSatoshis: amount,
				Expiry:      1000,
			},
			InvoiceResponse: lnrpc.Invoice{
				PaymentRequest: paymentRequest,
			},
		}

		off, err := transactions.CreateTeslacoilInvoice(testDB, &mockLNcli, transactions.NewOffchainOpts{
			UserID:    user.ID,
			AmountSat: int64(gofakeit.Number(0, ln.MaxAmountSatPerInvoice)),
		})
		assert.Nil(t, err)
		assert.Equal(t, paymentRequest, off.PaymentRequest)

		paid, err := transactions.PayInvoice(testDB, &mockLNcli, nil, user.ID, off.PaymentRequest)
		assert.True(t, errors.Is(err, transactions.ErrCannotPayOwnInvoice))

		assert.Equal(t, transactions.Offchain{}, paid)
	})
	t.Run("internal transfer marks the transaction correctly", func(t *testing.T) {
		t.Parallel()

		payTo := CreateUserOrFail(t, testDB)
		user := CreateUserWithBalanceOrFail(t, testDB, ln.MaxAmountSatPerInvoice*3)
		balancePrePayment, err := balance.ForUser(testDB, user.ID)
		require.NoError(t, err)

		mockLNcli := lntestutil.LightningMockClient{
			DecodePayReqResponse: lnrpc.PayReq{
				PaymentHash: SampleHashHex,
				NumSatoshis: amount,
				Expiry:      1000,
			},
			InvoiceResponse: lnrpc.Invoice{
				PaymentRequest: "a payment request",
			},
		}

		off, err := transactions.CreateTeslacoilInvoice(testDB, &mockLNcli, transactions.NewOffchainOpts{
			UserID:    payTo.ID,
			AmountSat: int64(gofakeit.Number(0, ln.MaxAmountSatPerInvoice)),
		})
		assert.NoError(t, err)

		paid, err := transactions.PayInvoice(testDB, &mockLNcli, nil, user.ID, off.PaymentRequest)
		assert.NoError(t, err)

		assert.True(t, paid.InternalTransfer)
		assert.Equal(t, transactions.Offchain_COMPLETED, paid.Status)
		assert.NotNil(t, paid.SettledAt)

		// assert balance is decreased
		updatedBalance, err := balance.ForUser(testDB, user.ID)
		assert.NoError(t, err)
		assert.Equal(t, balancePrePayment.Sats()-amount, updatedBalance.Sats())
	})
}

func TestUpdateInvoiceStatus(t *testing.T) {
	t.Parallel()
	u := CreateUserOrFail(t, testDB)

	t.Run("update invoice status to settled", func(t *testing.T) {
		t.Parallel()
		lnMock := lntestutil.GetRandomLightningMockClient()
		httpPoster := testutil.GetMockHttpPoster()
		mockInvoice, _ := ln.AddInvoice(lnMock, lnrpc.Invoice{})
		offchainTx := CreateNewOffchainTxOrFail(t, testDB, lnMock, transactions.NewOffchainOpts{
			UserID:    u.ID,
			AmountSat: mockInvoice.Value,
		})

		invoice := lnrpc.Invoice{
			PaymentRequest: offchainTx.PaymentRequest,
			State:          lnrpc.Invoice_SETTLED,
		}

		updated, err := transactions.HandleSettledInvoice(invoice, testDB, httpPoster)
		require.NoError(t, err)
		assert.NotNil(t, updated.SettledAt)
	})

	t.Run("not settle an OPEN invoice", func(t *testing.T) {
		t.Parallel()
		lnMock := lntestutil.GetRandomLightningMockClient()
		httpPoster := testutil.GetMockHttpPoster()
		mockInvoice, _ := ln.AddInvoice(lnMock, lnrpc.Invoice{})
		offchainTx := CreateNewOffchainTxOrFail(t, testDB, lnMock, transactions.NewOffchainOpts{
			UserID:    u.ID,
			AmountSat: mockInvoice.Value,
		})

		invoice := lnrpc.Invoice{
			PaymentRequest: offchainTx.PaymentRequest,
			State:          lnrpc.Invoice_OPEN,
		}

		_, err := transactions.HandleSettledInvoice(invoice, testDB, httpPoster)
		require.EqualErrorf(t, err, transactions.ErrExpectedSettledStatus.Error(), "")
	})

	t.Run("callback URL should be called", func(t *testing.T) {
		t.Parallel()

		// for the callback to be executed, we need to create an API key for the
		// current user. this is because the callback body is hashed with the
		// users API key
		if _, _, err := apikeys.New(testDB, u.ID, apikeys.AllPermissions, ""); err != nil {
			require.NoError(t, err)
		}

		lnMock := lntestutil.GetRandomLightningMockClient()
		httpPoster := testutil.GetMockHttpPoster()
		mockInvoice, _ := ln.AddInvoice(lnMock, lnrpc.Invoice{})
		offchainTx := CreateNewOffchainTxOrFail(t, testDB, lnMock, transactions.NewOffchainOpts{
			UserID:      u.ID,
			AmountSat:   mockInvoice.Value,
			CallbackURL: gofakeit.URL(),
		})
		assert.NotNil(t, offchainTx.CallbackURL)

		invoice := lnrpc.Invoice{
			PaymentRequest: offchainTx.PaymentRequest,
			State:          lnrpc.Invoice_SETTLED,
		}

		_, err := transactions.HandleSettledInvoice(invoice, testDB, httpPoster)
		require.NoError(t, err)

		checkPostSent := func() bool {
			return httpPoster.GetSentPostRequests() == 1
		}
		// emails are sent in go-routine, so can't assume they're sent fast
		// enough for test to pick up
		err = async.AwaitNoBackoff(8, time.Millisecond*20, checkPostSent)
		require.NoError(t, err)
	})

	t.Run("callback URL should not be called with non-settled invoice", func(t *testing.T) {
		t.Parallel()
		lnMock := lntestutil.GetLightningMockClient()
		httpPoster := testutil.GetMockHttpPoster()
		mockInvoice, _ := ln.AddInvoice(lnMock, lnrpc.Invoice{})
		offchainTx := CreateNewOffchainTxOrFail(t, testDB, lnMock, transactions.NewOffchainOpts{
			UserID:      u.ID,
			AmountSat:   mockInvoice.Value,
			CallbackURL: "https://example.com",
		})

		invoice := lnrpc.Invoice{
			PaymentRequest: offchainTx.PaymentRequest,
			State:          lnrpc.Invoice_OPEN,
		}

		_, err := transactions.HandleSettledInvoice(invoice, testDB, httpPoster)
		testutil.AssertEqualErr(t, transactions.ErrExpectedSettledStatus, err)

		checkPostSent := func() bool {
			return httpPoster.GetSentPostRequests() > 0
		}

		// emails are sent in go-routing, so can't assume they're sent fast
		// enough for test to pick up
		err = async.Await(4,
			time.Millisecond*20, checkPostSent)
		assert.Error(t, err, "HTTP POSTer sent out callback for non-settled offchainTx")
	})
}

func TestOffchain_WithAdditionalFields(t *testing.T) {
	t.Parallel()
	user := CreateUserOrFail(t, testDB)

	t.Run("expiry field", func(t *testing.T) {
		offchainTx := transactions.Offchain{
			UserID:         user.ID,
			HashedPreimage: []byte("f747dbf93249644a71749b6fff7c5a9eb7c1526c52ad3414717e222470940c57"),
			Expiry:         1,
			Direction:      transactions.INBOUND,
			Status:         transactions.Offchain_CREATED,
			AmountMilliSat: 100000,
		}

		offchainTx, err := transactions.InsertOffchain(testDB, offchainTx)
		require.NoError(t, err)

		// Sleep for expiry to check if expired property is set
		// correctly. expired should be true
		time.Sleep(time.Second + time.Second*time.Duration(offchainTx.Expiry))

		assert.True(t, offchainTx.IsExpired())
		assert.Equal(t, offchainTx.ExpiresAt(), offchainTx.CreatedAt.Add(time.Second*time.Duration(offchainTx.Expiry)))
	})

	invoices := []transactions.Offchain{
		{
			UserID:         user.ID,
			HashedPreimage: []byte("f747dbf93249644a71749b6fff7c5a9eb7c1526c52ad3414717e222470940c57"),
			Expiry:         3600,
			Direction:      transactions.INBOUND,
			Status:         transactions.Offchain_CREATED,
			AmountMilliSat: 100000,
		},
		{
			UserID:         user.ID,
			HashedPreimage: []byte("f747dbf93249644a71749b6fff7c5a9eb7c1526c52ad3414717e222470940c57"),
			Expiry:         2,
			Direction:      transactions.INBOUND,
			Status:         transactions.Offchain_CREATED,
			AmountMilliSat: 100000,
		},
	}

	for _, invoice := range invoices {
		t.Run(fmt.Sprintf("invoice with expiry %d should not be expired", invoice.Expiry),
			func(t *testing.T) {
				payment, err := transactions.InsertOffchain(testDB, invoice)
				require.NoError(t, err)
				assert.False(t, payment.IsExpired())
			})
	}
}

// CreateNewOffchainTxOrFail creates a new offchain or fail
func CreateNewOffchainTxOrFail(t *testing.T, db *db.DB, ln ln.AddLookupInvoiceClient,
	opts transactions.NewOffchainOpts) transactions.Offchain {
	payment, err := transactions.CreateTeslacoilInvoice(db, ln, opts)
	require.NoError(t, err)
	return payment
}

func CreateUserOrFail(t *testing.T, db *db.DB) users.User {
	u, err := users.Create(db, users.CreateUserArgs{
		Email:    gofakeit.Email(),
		Password: gofakeit.Password(true, true, true, true, true, 32),
	})
	require.NoError(t, err)

	return u
}

// CreateUserWithBalanceOrFail creates a user with the given balance
func CreateUserWithBalanceOrFail(t *testing.T, db *db.DB, sats int64) users.User {
	u := CreateUserOrFail(t, db)
	tx := getIncomingTxFor(u.ID, sats)
	_, err := transactions.InsertOffchain(db, tx)
	require.NoError(t, err)

	return u
}

func getIncomingTxFor(userId int, sats int64) transactions.Offchain {
	tx := txtest.MockOffchain(userId)
	if tx.Direction != transactions.INBOUND {
		return getIncomingTxFor(userId, sats)
	}
	if tx.Status != transactions.Offchain_COMPLETED {
		return getIncomingTxFor(userId, sats)
	}

	tx.AmountSat = sats
	tx.AmountMilliSat = sats * 1000
	return tx
}
