package api

// import (
// 	"encoding/json"
// 	"io/ioutil"
// 	"net/http"
// 	"net/http/httptest"
// 	"os"
// 	"testing"

// 	_ "github.com/lib/pq" // Import postgres
// )

// var withdrawals = []models.Withdrawal{
// 	{

// 		PaymentRequest: "lnbc10u1pwdh735pp5e3p5phcdzjhwc39yvm7jr3w2hvtnwpvdjmptm8829cjcqwvy5clqdqlxycrqvpqwdshgueqvfjhggr0dcsry7qcqzpgyrtvetq6044dtj7x9gf0stpp8c9nrvy2ac22eshyqarnkgv654ts7t3kc09yyjgcw05jeeu8syns5nh5fvc8y7w2aj0a548q6efa55cqy50lfx",
// 		Amount:         1000,
// 		Description:    "lightningspin",
// 	},
// 	{
// 		PaymentRequest: "lnbc10u1pwdh73lpp5xvlu0jhr3vsj0xyppuw6793qahdcjw56r3mk85jq5mj09w6alpcqdqlxycrqvpqwdshgueqvfjhggr0dcsry7qcqzpg2p6cm8ddmvgvcg3ct2uceseu07tjucvvkdujdds7lw9p6x7g0jy8a6rf3dnaa8yhejarhrzk304vuqjzchvq3pez5sekytn42aa7fvsq75g98j",
// 		Amount:         1000,
// 		Description:    "lightningspin",
// 	},
// 	{
// 		PaymentRequest: "lnbc10u1pwdh7jdpp5sh0ghtjm32yaqj7vv8dkx6ckx59snflhymyqvknswacey2vjqpcsdqlxycrqvpqwdshgueqvfjhggr0dcsry7qcqzpg42g69hmpc3ftufdtmx6sp27558vjgpmgukd8xlv64rc0g2chfft39vz3gedawt9c9uhqjxma2rzphet4tk2p0jnjlyk5unxxthelvpspxr9uyp",
// 		Amount:         1000,
// 		Description:    "lightningspin",
// 		Fee:            3,
// 	},
// }

// var deposits = []models.Deposit{
// 	{
// 		PaymentRequest: "lnbc10u1pwdh735pp5e3p5phcdzjhwc39yvm7jr3w2hvtnwpvdjmptm8829cjcqwvy5clqdqlxycrqvpqwdshgueqvfjhggr0dcsry7qcqzpgyrtvetq6044dtj7x9gf0stpp8c9nrvy2ac22eshyqarnkgv654ts7t3kc09yyjgcw05jeeu8syns5nh5fvc8y7w2aj0a548q6efa55cqy50lfx",
// 		Amount:         1000,
// 		Description:    "lightningspin",
// 	},
// 	{
// 		PaymentRequest: "lnbc10u1pwdh73lpp5xvlu0jhr3vsj0xyppuw6793qahdcjw56r3mk85jq5mj09w6alpcqdqlxycrqvpqwdshgueqvfjhggr0dcsry7qcqzpg2p6cm8ddmvgvcg3ct2uceseu07tjucvvkdujdds7lw9p6x7g0jy8a6rf3dnaa8yhejarhrzk304vuqjzchvq3pez5sekytn42aa7fvsq75g98j",
// 		Amount:         1000,
// 		Description:    "lightningspin",
// 	},
// 	{
// 		PaymentRequest: "lnbc10u1pwdh7jdpp5sh0ghtjm32yaqj7vv8dkx6ckx59snflhymyqvknswacey2vjqpcsdqlxycrqvpqwdshgueqvfjhggr0dcsry7qcqzpg42g69hmpc3ftufdtmx6sp27558vjgpmgukd8xlv64rc0g2chfft39vz3gedawt9c9uhqjxma2rzphet4tk2p0jnjlyk5unxxthelvpspxr9uyp",
// 		Amount:         1000,
// 		Description:    "lightningspin",
// 	},
// }

// var users = []models.User{
// 	{
// 		Balance: 50000,
// 		UUID:    "1234-1234-1234-1234",
// 	},
// 	{
// 		Balance: 0,
// 		UUID:    "1234-1234-1234-1234",
// 	},
// 	{
// 		Balance: 25000,
// 		UUID:    "1234-1234-1234-1234",
// 	},
// }

// var server = httptest.NewServer()

// func TestMain(m *testing.M) {
// 	// println("Tests are about to run")
// 	d := models.OpenTestDatabase()
// 	models.ResetDB(d)
// 	models.FillWithDummyData(d)

// 	// server = httptest.NewServer()
// 	// server.router.Use(contentTypeJSONMiddleware)
// 	// TODO ANYONE: Structure the app better than to pass the server as an argument
// 	// It is my understanding this is very bad form.. There must be a better way:(
// 	// server.router.RegisterDepositRoutes(d)
// 	// server.router.RegisterWithdrawalRoutes(d)
// 	// server.router.RegisterUserRoutes(d)
// 	// for i := range withdrawals {
// 	// 	serverObject.database.Save(&withdrawals[i])
// 	// }
// 	// for i := range users {
// 	// 	serverObject.database.Save(&users[i])
// 	// }
// 	// for i := range deposits {
// 	// 	serverObject.database.Save(&deposits[i])
// 	// }

// 	// Run tests
// 	result := m.Run()

// 	println("Tests done executing")

// 	os.Exit(result)
// }

// //Tests that the /withdrawals endpoint returns exactly what is entered into the database
// func TestWithdrawalsReturnsAllWithdrawals(t *testing.T) {
// 	d := models.OpenTestDatabase()

// 	request, err := http.NewRequest("GET", "http://localhost:3000/withdrawals", nil)
// 	if err != nil {
// 		t.Fatal("Creating 'GET withdrawals' request failed")
// 		t.Fail()
// 	}
// 	writer := httptest.NewRecorder()
// 	server := http.DefaultServeMux
// 	// server.withdrawals(writer, request)
// 	server.router.RegisterWithdrawalRoutes(models.DB{d})
// 	// withdrawals := Withdrawals(models.DB{d})
// 	server.withdrawals(writer, request)
// 	// Withdrawals(gorm.DB)

// 	body, err := ioutil.ReadAll(writer.Body)
// 	if err != nil {
// 		t.Error(err.Error())
// 	}

// 	var withdrawalFromAPI []models.Withdrawal
// 	err = json.Unmarshal(body, &withdrawalFromAPI)
// 	if err != nil {
// 		t.Error("Failed trying to unmarshal response from /withdrawals")
// 		t.Fatal(err.Error())
// 	}

// 	if len(withdrawalFromAPI) == 0 {
// 		t.FailNow()
// 	}

	// if withdrawalFromAPI[0].Amount != withdrawals[0].Amount {
	// 	t.Fail()
	// }
	// if withdrawalFromAPI[1].Amount != withdrawals[1].Amount {
	// 	t.Fail()
	// }
	// if withdrawalFromAPI[2].Amount != withdrawals[2].Amount {
	// 	t.Fail()
	// }

	// if withdrawalFromAPI[0].PaymentRequest != withdrawals[0].PaymentRequest {
	// 	t.Fail()
	// }
	// if withdrawalFromAPI[1].PaymentRequest != withdrawals[1].PaymentRequest {
	// 	t.Fail()
	// }
	// if withdrawalFromAPI[2].PaymentRequest != withdrawals[2].PaymentRequest {
	// 	t.Fail()
	// }

	// if withdrawalFromAPI[0].Description != withdrawals[0].Description {
	// 	t.Fail()
	// }
	// if withdrawalFromAPI[1].Description != withdrawals[1].Description {
	// 	t.Fail()
	// }
	// if withdrawalFromAPI[2].Description != withdrawals[2].Description {
	// 	t.Fail()
	// }

}

// //Tests that the /users endpoint returns exactly what is entered into the database
// func TestUsersReturnsAllUsers(t *testing.T) {

// 	request, err := http.NewRequest("GET", "/users", nil)
// 	if err != nil {
// 		t.Fatal("Creating 'GET users' request failed")
// 		t.Fail()
// 	}
// 	writer := httptest.NewRecorder()
// 	server.users(writer, request)

// 	body, err := ioutil.ReadAll(writer.Body)

// 	var usersFromAPI []models.User
// 	err = json.Unmarshal(body, &usersFromAPI)
// 	if err != nil {
// 		t.Error("Failed trying to unmarshal response from /users")
// 		t.Fatal(err.Error())
// 	}

// 	if len(usersFromAPI) == 0 {
// 		t.Fatal("Length of data returned from /users is 0")
// 	}

// 	if usersFromAPI[0].Balance != users[0].Balance {
// 		t.Error("Length of data returned from /users is 0")
// 		t.Fail()
// 	}
// 	if usersFromAPI[1].Balance != users[1].Balance {
// 		t.Error("Length of data returned from /users is 0")
// 		t.Fail()
// 	}
// 	if usersFromAPI[2].Balance != users[2].Balance {
// 		t.Error("Length of data returned from /users is 0")
// 		t.Fail()
// 	}

// 	if usersFromAPI[0].UUID != users[0].UUID {
// 		t.Errorf("%s != %s", usersFromAPI[0].UUID, users[0].UUID)
// 		t.Fail()
// 	}
// 	if usersFromAPI[1].UUID != users[1].UUID {
// 		t.Errorf("%d", usersFromAPI[1].Balance)
// 		t.Errorf("%s", usersFromAPI[1].UUID)
// 		t.Fail()
// 	}
// 	if usersFromAPI[2].UUID != users[2].UUID {
// 		t.Error("Length of data returned from /users is 0")
// 		t.Fail()
// 	}
// }

// //Tests that the /deposits endpoint returns exactly what is entered into the database
// func TestDepositsReturnsAllDeposits(t *testing.T) {
// 	request, err := http.NewRequest("GET", "/deposits", nil)
// 	if err != nil {
// 		t.Fatal("Creating 'GET deposits' request failed")
// 		t.Fail()
// 	}
// 	writer := httptest.NewRecorder()
// 	server.deposits(writer, request)

// 	body, err := ioutil.ReadAll(writer.Body)

// 	var depositFromAPI []models.Deposit
// 	err = json.Unmarshal(body, &depositFromAPI)
// 	if err != nil {
// 		t.Error("Failed trying to unmarshal response from /withdrawals")
// 		t.Fatal(err.Error())
// 	}

// 	if depositFromAPI[0].Amount != deposits[0].Amount {
// 		t.Fatal("1")
// 	}
// 	if depositFromAPI[1].Amount != deposits[1].Amount {
// 		t.Fatal("2")
// 	}
// 	if depositFromAPI[2].Amount != deposits[2].Amount {
// 		t.Fatal("3")
// 	}

// 	if depositFromAPI[0].PaymentRequest != deposits[0].PaymentRequest {
// 		t.Fatal("4")
// 	}
// 	if depositFromAPI[1].PaymentRequest != deposits[1].PaymentRequest {
// 		t.Fatal("5")
// 	}
// 	if depositFromAPI[2].PaymentRequest != deposits[2].PaymentRequest {
// 		t.Errorf("api: %+v\nvar: %+v", depositFromAPI[2].PaymentRequest, deposits[2].PaymentRequest)
// 		t.Fatal("6")
// 	}
// 	if depositFromAPI[0].Description != deposits[0].Description {
// 		t.Fatal("7")
// 	}
// 	if depositFromAPI[1].Description != deposits[1].Description {
// 		t.Fatal("8")
// 	}
// 	if depositFromAPI[2].Description != deposits[2].Description {
// 		t.Fatal("1")
// 	}
// }
