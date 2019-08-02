package api

import (
	"net/http"
	"time"

	jwt "github.com/appleboy/gin-jwt"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/lightningnetwork/lnd/lnrpc"
	"gitlab.com/arcanecrypto/lpp/internal/payments"
	"gitlab.com/arcanecrypto/lpp/internal/platform/ln"
	"gitlab.com/arcanecrypto/lpp/internal/users"
)

// Config is a config
type Config struct {
	LightningConfig ln.LightningConfig
	DebugLevel      string
}

// RestServer is the rest server for our app. It includes a Router,
// a JWT middleware a db connection, and a grpc connection to lnd
type RestServer struct {
	Router *gin.Engine
	JWT    *jwt.GinJWTMiddleware
	db     *sqlx.DB
	lncli  *lnrpc.LightningClient
}

//NewApp creates a new app
func NewApp(d *sqlx.DB, config Config) (RestServer, error) {
	g := gin.Default()

	lncli, err := ln.NewLNDClient(config.LightningConfig)
	if err != nil {
		return RestServer{}, err
	}

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
		db:    d,
		lncli: &lncli,
	}

	invoiceUpdatesCh := make(chan lnrpc.Invoice)
	go ln.ListenInvoices(lncli, invoiceUpdatesCh)

	go payments.UpdateInvoiceStatus(invoiceUpdatesCh, d)

	RegisterUserRoutes(&restServer)
	RegisterPaymentRoutes(&restServer)

	return restServer, nil
}

//LoginAuthenticator is an authenticator
func (r *RestServer) LoginAuthenticator(ctx *gin.Context) (interface{}, error) {
	var params GetUserRequest
	if err := ctx.Bind(&params); err != nil {
		return "", jwt.ErrMissingLoginValues
	}

	// Get users by ID
	user, err := users.All(r.db)
	if err != nil {
		return nil, err
	}

	// Verify password
	// nah

	return user, nil
}

//Login logs in
func Login(r *RestServer) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := r.LoginAuthenticator(c)
		if err != nil {
			c.JSONP(http.StatusInternalServerError, gin.H{"error": err})
		}

		c.JSONP(200, user)
	}
}

// RegisterUserRoutes registers all user routes on the router
func RegisterUserRoutes(r *RestServer) {
	r.Router.POST("/login", Login(r))
	r.Router.GET("/users", GetAllUsers(r))
	r.Router.GET("/users/:id", GetUser(r))
	r.Router.POST("/users", CreateUser(r))
}

// RegisterPaymentRoutes registers all payment routes on the router
func RegisterPaymentRoutes(r *RestServer) {
	r.Router.GET("/payments", GetAllInvoices(r))
	r.Router.GET("/payments/:id", GetInvoice(r))
	r.Router.POST("/invoice/create", CreateInvoice(r))
	r.Router.POST("/invoice/pay", PayInvoice(r))
}
