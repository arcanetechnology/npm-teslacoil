package withdrawals

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"gitlab.com/arcanecrypto/lpp/internal/platform/db"

	"github.com/gorilla/mux"
)

// Withdrawals is a GET request that returns all the withdrawals in the database
func Withdrawals(d db.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Equivalent to SELECT * from withdrawals;
		queryResult := []Withdrawal{}
		d.Find(&queryResult)

		withdrawals, err := json.Marshal(queryResult)
		if err != nil {
			panic(err.Error())
		}

		w.Write(withdrawals)
	})
}

// GetWithdrawal is a GET request that returns withdrawals that match the one specified in the body
func GetWithdrawal(d db.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		queryResult := Withdrawal{}
		d.Where("id = ?", vars["id"]).First(&queryResult)
		withdrawal, err := json.Marshal(queryResult)
		if err != nil {
			panic(err.Error())
		}

		w.Write(withdrawal)
	})
}

// CreateWithdrawal is a POST request and inserts all the withdrawals in the body into the database
func CreateWithdrawal(d db.DB) http.Handler {
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
			fmt.Errorf("Reading data from body in CreateWithdrawal() failed")
			panic(err.Error())
		}

		withdrawal := Withdrawal{}

		err = json.Unmarshal(body, &withdrawal)
		if err != nil {
			http.Error(w, "Please send a correctly formatted request body", 400)
			return
		}

		// Create a lightning request here (using lnd grpc), and save it in the withdrawal interface,
		// like withdrawal.PaymentRequest = `lncli addinvoice --amt=withdrawal.Amount`

		// Equivalent to "INSERT INTO withdrawals VALUES {withdrawal}"
		d.Create(&withdrawal)

		// Convert the store object back to json before returning the result
		result, err := json.Marshal(withdrawal)
		if err != nil {
			panic(err.Error())
		}

		// Return the result
		http.StatusText(200)
		w.Write([]byte(result))
	})
}
