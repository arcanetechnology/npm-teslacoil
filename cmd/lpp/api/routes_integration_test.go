//+build integration

package api_test

import (
	"fmt"
	"testing"

	"gitlab.com/arcanecrypto/teslacoil/internal/platform/bitcoind"

	"github.com/brianvoe/gofakeit"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/cmd/lpp/api"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/ln"
	"gitlab.com/arcanecrypto/teslacoil/internal/users"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/httptestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/nodetestutil"
)

var (
	databaseConfig = testutil.GetDatabaseConfig("routes_integration")
	testDB         *db.DB
	conf           = api.Config{LogLevel: logrus.InfoLevel, Network: chaincfg.RegressionNetParams}
)

func init() {
	// we're not closing DB connections here...
	// this probably shouldn't matter, as the conn.
	// closes when the process exits anyway
	testDB = testutil.InitDatabase(databaseConfig)
}

func TestCreateInvoiceRoute(t *testing.T) {
	nodetestutil.RunWithLnd(t, func(lnd lnrpc.LightningClient) {
		app, err := api.NewApp(testDB,
			lnd,
			testutil.GetMockSendGridClient(),
			bitcoind.TeslacoilBitcoindMockClient{},
			testutil.GetMockHttpPoster(),
			conf)
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		h := httptestutil.NewTestHarness(app.Router)

		testutil.DescribeTest(t)

		password := gofakeit.Password(true, true, true, true, true, 32)
		accessToken := h.CreateAndAuthenticateUser(t, users.CreateUserArgs{
			Email:    gofakeit.Email(),
			Password: password,
		})

		t.Run("Create an invoice without memo and description", func(t *testing.T) {
			testutil.DescribeTest(t)

			amountSat := gofakeit.Number(0, ln.MaxAmountSatPerInvoice)

			req := httptestutil.GetAuthRequest(t,
				httptestutil.AuthRequestArgs{
					AccessToken: accessToken,
					Path:        "/invoices/create",
					Method:      "POST",
					Body: fmt.Sprintf(`{
					"amountSat": %d
				}`, amountSat),
				})

			res := h.AssertResponseOkWithJson(t, req)
			testutil.AssertMsg(t, res["memo"] == nil, "Memo was not empty")
			testutil.AssertMsg(t, res["description"] == nil, "Description was not empty")

		})

		t.Run("Create an invoice with memo and description", func(t *testing.T) {
			testutil.DescribeTest(t)

			amountSat := gofakeit.Number(0, ln.MaxAmountSatPerInvoice)

			memo := gofakeit.Sentence(gofakeit.Number(1, 20))
			description := gofakeit.Sentence(gofakeit.Number(1, 20))

			req := httptestutil.GetAuthRequest(t,
				httptestutil.AuthRequestArgs{
					AccessToken: accessToken,
					Path:        "/invoices/create",
					Method:      "POST",
					Body: fmt.Sprintf(`{
					"amountSat": %d,
					"memo": %q,
					"description": %q
				}`, amountSat, memo, description),
				})

			res := h.AssertResponseOkWithJson(t, req)
			testutil.AssertEqual(t, res["memo"], memo)
			testutil.AssertEqual(t, res["description"], description)

		})

	})
}
