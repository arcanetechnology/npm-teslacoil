package api

import (
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/lightningnetwork/lnd/lnrpc"
	"gitlab.com/arcanecrypto/lpp/internal/platform/db"
	"gitlab.com/arcanecrypto/lpp/internal/platform/ln"
	"gitlab.com/arcanecrypto/lpp/internal/transactions"
)

func NewApp() *gin.Engine {
	d := db.OpenDatabase()
	r := gin.Default()

	invoiceUpdatesCh := make(chan lnrpc.Invoice)
	go ln.ListenInvoices(invoiceUpdatesCh)

	go transactions.UpdateUserBalance(invoiceUpdatesCh, d)

	RegisterUserRoutes(r, d)
	RegisterTransactionRoutes(r, d)

	// router.GET("/", func(c *gin.Context) {
	// 	c.JSONP(200, gin.H{
	// 		"message": "It's happening!",
	// 	})
	// })
	return r
}

// func API(d gorm.DB) http.Handler {

// 	router := mux.NewRouter()

// 	s.router.RegisterDepositRoutes(s.database)
// 	s.router.RegisterWithdrawalRoutes(s.database)
// 	s.router.RegisterUserRoutes(s.database)

// 	return
// }

// // RegisterRoutes registers all routes
// func (s *web.Server) RegisterRoutes() {
// 	s.router.Use(contentTypeJSONMiddleware)
// 	// TODO ANYONE: Structure the app better than to pass the server as an argument
// 	// It is my understanding this is very bad form.. There must be a better way:(
// 	s.router.RegisterDepositRoutes(s.database)
// 	s.router.RegisterWithdrawalRoutes(s.database)
// 	s.router.RegisterUserRoutes(s.database)

// 	http.Handle("/", s.router)
// }

// // RegisterDepositRoutes registers all user routes on the router
// func (r *web.Router) RegisterDepositRoutes(d db.DB) {
// 	r.Handle("/deposits", deposits.Deposits(d)).Methods("GET")
// 	r.Handle("/deposits", deposits.CreateDeposit(d)).Methods("POST")
// 	r.Handle("/deposits/{id:[0-9]+}", deposits.GetDeposit(d)).Methods("GET")
// }

// // RegisterWithdrawalRoutes registers all user routes on the router
// func (r *web.Router) RegisterWithdrawalRoutes(d db.DB) {
// 	r.Handle("/withdrawals", withdrawals.Withdrawals(d)).Methods("GET")
// 	r.Handle("/withdrawals", withdrawals.CreateWithdrawal(d)).Methods("POST")
// 	r.Handle("/withdrawals/{id:[0-9]+}", withdrawals.GetWithdrawal(d)).Methods("GET")
// }

// RegisterUserRoutes registers all user routes on the router
func RegisterUserRoutes(r *gin.Engine, d *gorm.DB) {
	r.GET("/users", AllUsers(d))
	r.GET("/users/:id", GetUser(d))
	r.POST("/users", CreateUser(d))
}

func RegisterTransactionRoutes(r *gin.Engine, d *gorm.DB) {
	r.GET("/transactions", AllTransactions(d))
	r.GET("/transactions/:id", GetTransaction(d))
	r.POST("/invoice/create", CreateNewInvoice(d))
	r.POST("/invoice/pay", PayInvoice(d))
}

// func contentTypeJSONMiddleware(next http.Handler) http.Handler {
// 	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		w.Header().Add("Content-Type", "application/json")
// 		next.ServeHTTP(w, r)
// 	})
// }
