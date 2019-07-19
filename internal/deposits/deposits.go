package deposits

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"gitlab.com/arcanecrypto/lpp/internal/platform/db"

	"github.com/gorilla/mux"
)

// Deposits is a GET request that returns all the deposits in the database
func Deposits(d db.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Equivalent to SELECT * from deposits;
		queryResult := []Deposit{}
		d.Find(&queryResult)

		deposits, err := json.Marshal(queryResult)
		if err != nil {
			panic(err.Error())
		}

		w.Write(deposits)
	})
}

// GetDeposit is a GET request that returns deposits that match the one specified in the body
func GetDeposit(d db.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		queryResult := Deposit{}
		d.Where("id = ?", vars["id"]).First(&queryResult)
		deposit, err := json.Marshal(queryResult)
		if err != nil {
			panic(err.Error())
		}

		w.Write(deposit)
	})
}

// CreateDeposit is a POST request and inserts all the deposits in the body into the database
func CreateDeposit(d db.DB) http.Handler {
	// 1. Read the data from the body
	// 2. Create a new entry in the database with the data
	// 3. Write the query result to the ResponseWriter
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil {
			http.Error(w, "Please send a request body", 400)
			return
		}
		defer r.Body.Close()

		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			fmt.Errorf("Reading data from body in CreateDeposit() failed")
			panic(err.Error())
		}

		deposit := Deposit{}
		err = json.Unmarshal(body, &deposit)

		// Create a lightning request here (using lnd grpc), and save it in the deposit interface,
		// like deposit.PaymentRequest = `lncli addinvoice --amt=deposit.Amount`

		// Equivalent to "INSERT INTO deposit VALUES {deposit}"
		d.Create(&deposit)

		result, err := json.Marshal(deposit)
		if err != nil {
			panic(err.Error())
		}

		// TODO BO: Return something more useful, like a 200 code, not "SUCCESS"
		http.StatusText(200)
		w.Write([]byte(result))
	})
}
