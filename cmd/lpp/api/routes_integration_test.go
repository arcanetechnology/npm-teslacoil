//+build integration

package api_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/brianvoe/gofakeit"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/cmd/lpp/api"
	"gitlab.com/arcanecrypto/teslacoil/internal/payments"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
	"gitlab.com/arcanecrypto/teslacoil/internal/users"
	"gitlab.com/arcanecrypto/teslacoil/testutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/httptestutil"
	"gitlab.com/arcanecrypto/teslacoil/testutil/lntestutil"
)

var (
	databaseConfig = testutil.GetDatabaseConfig("routes_integration")
	testDB         *db.DB
	conf           = api.Config{LogLevel: logrus.InfoLevel}
)

func init() {
	// we're not closing DB connections here...
	// this probably shouldn't matter, as the conn.
	// closes when the process exits anyway
	testDB = testutil.InitDatabase(databaseConfig)
}

func TestCreateInvoiceRoute(t *testing.T) {
	lntestutil.RunWithLnd(t, func(lnd lnrpc.LightningClient) {
		app, err := api.NewApp(testDB, lnd, testutil.GetMockSendGridClient(), conf)
		if err != nil {
			testutil.FatalMsg(t, err)
		}

		h := httptestutil.NewTestHarness(app.Router)

		testutil.DescribeTest(t)

		password := gofakeit.Password(true, true, true, true, true, 32)
		accessToken := h.CreateAndLoginUser(t, users.CreateUserArgs{
			Email:    gofakeit.Email(),
			Password: password,
		})

		t.Run("Create an invoice without memo and description", func(t *testing.T) {
			testutil.DescribeTest(t)

			amountSat := gofakeit.Number(0,
				int(payments.MaxAmountSatPerInvoice))

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

			amountSat := gofakeit.Number(0,
				int(payments.MaxAmountSatPerInvoice))

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

		t.Run("Not create an invoice with non-positive amount ", func(t *testing.T) {
			testutil.DescribeTest(t)

			// gofakeit panics with too low value here...
			// https://github.com/brianvoe/gofakeit/issues/56
			amountSat := gofakeit.Number(math.MinInt64+2, -1)

			req := httptestutil.GetAuthRequest(t,
				httptestutil.AuthRequestArgs{
					AccessToken: accessToken,
					Path:        "/invoices/create",
					Method:      "POST",
					Body: fmt.Sprintf(`{
					"amountSat": %d
				}`, amountSat),
				})

			h.AssertResponseNotOk(t, req)
		})

		t.Run("Not create an invoice with too large amount", func(t *testing.T) {
			testutil.DescribeTest(t)

			amountSat := gofakeit.Number(
				int(payments.MaxAmountSatPerInvoice), math.MaxInt64)

			req := httptestutil.GetAuthRequest(t,
				httptestutil.AuthRequestArgs{
					AccessToken: accessToken,
					Path:        "/invoices/create",
					Method:      "POST",
					Body: fmt.Sprintf(`{
					"amountSat": %d
				}`, amountSat),
				})

			h.AssertResponseNotOk(t, req)
		})

		t.Run("Not create an invoice with zero amount ", func(t *testing.T) {
			testutil.DescribeTest(t)

			req := httptestutil.GetAuthRequest(t,
				httptestutil.AuthRequestArgs{
					AccessToken: accessToken,
					Path:        "/invoices/create",
					Method:      "POST",
					Body: `{
					"amountSat": 0
				}`,
				})

			h.AssertResponseNotOk(t, req)

		})
	})
}
