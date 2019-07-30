package api

import (
	"time"

	jwt "github.com/appleboy/gin-jwt"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/lightningnetwork/lnd/lnrpc"
	"gitlab.com/arcanecrypto/lpp/internal/platform/ln"
	"gitlab.com/arcanecrypto/lpp/internal/transactions"
)

// RestServer is the rest server for our app. It includes a Router,
// a JWT middleware and a db connection
type RestServer struct {
	Router *gin.Engine
	JWT    *jwt.GinJWTMiddleware
	db     *sqlx.DB
}

//NewApp creates a new app
func NewApp(d *sqlx.DB) (RestServer, error) {
	g := gin.Default()

	restServer := RestServer{
		Router: g,
		JWT: &jwt.GinJWTMiddleware{
			Realm:         "gin jwt",
			Key:           []byte("secret key"),
			Timeout:       time.Hour,
			MaxRefresh:    time.Hour,
			TokenHeadName: "Bearer",
			TimeFunc:      time.Now,
		},
		db: d,
	}

	invoiceUpdatesCh := make(chan lnrpc.Invoice)
	go ln.ListenInvoices(invoiceUpdatesCh)

	go transactions.UpdateInvoiceStatus(invoiceUpdatesCh, d)

	RegisterUserRoutes(&restServer)
	RegisterTransactionRoutes(&restServer)

	return restServer, nil
}

// RegisterUserRoutes registers all user routes on the router
func RegisterUserRoutes(r *RestServer) {
	r.Router.GET("/users", GetAllUsers(r))
	r.Router.GET("/users/:id", GetUser(r))
	r.Router.POST("/users", CreateUser(r))
}

// RegisterTransactionRoutes registers all transaction routes on the router
func RegisterTransactionRoutes(r *RestServer) {
	r.Router.GET("/transactions", GetAllTransactions(r))
	r.Router.GET("/transactions/:id", GetTransaction(r))
	r.Router.POST("/invoice/create", CreateNewInvoice(r))
	r.Router.POST("/invoice/pay", PayInvoice(r))
}
