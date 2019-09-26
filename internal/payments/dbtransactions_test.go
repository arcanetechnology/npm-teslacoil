package payments

import (
	"testing"

	"github.com/lightningnetwork/lnd/lnrpc"
	"gitlab.com/arcanecrypto/teslacoil/internal/users"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
)

func TestWithdrawOnChainBadOpts(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		scenario  string
		balance   int64
		amountSat int64
	}{
		{
			scenario:  "withdraw more than balance",
			balance:   1000,
			amountSat: 2000,
		},
		{
			scenario:  "withdraw negative amount",
			balance:   1000,
			amountSat: -5000,
		},
		{
			scenario:  "withdraw 0 amount",
			balance:   2000,
			amountSat: 0,
		},
	}
	mockLNcli := testutil.LightningMockClient{
		SendCoinsResponse: lnrpc.SendCoinsResponse{
			Txid: "owrgkpoaerkgpok",
		},
	}

	for _, test := range testCases {
		user := CreateUserWithBalanceOrFail(t, test.balance)

		t.Run(test.scenario, func(t *testing.T) {
			txid, err := WithdrawOnChain(testDB, mockLNcli, WithdrawOnChainArgs{
				UserID:    user.ID,
				AmountSat: test.amountSat,
				Address:   simnetAddress,
			})
			if err == nil || txid != nil {
				testutil.FatalMsgf(t, "should not send transaction, bad opts")
			}
		})
	}

}

func TestWithdrawOnChainSendAll(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		balance int64
		// We specify amountSat to make sure it is ignored when sendAll is true
		amountSat int64
	}{
		{
			balance:   10000,
			amountSat: 500000,
		},
		{
			balance:   20000,
			amountSat: -500000,
		},
		{
			balance:   500, // 20 000
			amountSat: 0,
		},
	}

	for _, test := range testCases {

		user := CreateUserWithBalanceOrFail(t, test.balance)

		t.Run("can withdraw on-chain", func(t *testing.T) {

			mockLNcli := testutil.LightningMockClient{
				SendCoinsResponse: lnrpc.SendCoinsResponse{
					Txid: "owrgkpoaerkgpok",
				},
			}

			_, err := WithdrawOnChain(testDB, mockLNcli, WithdrawOnChainArgs{
				UserID:    user.ID,
				AmountSat: test.amountSat,
				Address:   testnetAddress,
				SendAll:   true,
			})
			if err != nil {
				testutil.FatalMsgf(t, "could not WithdrawOnChain %+v", err)
			}

			// TODO: Test this creates transactions for the right amount
			// t.Run("withdrew the right amount", func(t *testing.T) {
			// Look up the txid on-chain, and check the amount
			// fmt.Println("txid: ", txid)
			// })
		})

		// Assert
		t.Run("users balance is 0", func(t *testing.T) {
			user, err := users.GetByID(testDB, user.ID)
			if err != nil {
				testutil.FatalMsgf(t, "could not get user %+v", err)
			}
			if user.Balance != 0 {
				testutil.FatalMsgf(t, "users balance was not 0 %+v", err)
			}
		})
	}
}
