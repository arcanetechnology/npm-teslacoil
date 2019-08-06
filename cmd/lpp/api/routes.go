package api

import (
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/lpp/internal/payments"
	"gitlab.com/arcanecrypto/lpp/internal/platform/ln"
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
		db:     d,
		lncli:  &lncli,
	}

	invoiceUpdatesCh := make(chan lnrpc.Invoice)
	go ln.ListenInvoices(lncli, invoiceUpdatesCh)

	go payments.UpdateInvoiceStatus(invoiceUpdatesCh, d)

	// We register /login separately to require jwt-tokens on every other endpoint
	// than /login
	restServer.Router.POST("/login", Login(&restServer))
	RegisterAuthRoutes(&restServer)
	RegisterUserRoutes(&restServer)
	RegisterPaymentRoutes(&restServer)

	return restServer, nil
}

// RegisterAuthRoutes registers all auth routes
func RegisterAuthRoutes(r *RestServer) {
	auth := r.Router.Group("")
	auth.Use(authenticateJWT)

	auth.GET("/auth/refresh_token", RefreshToken(r))
}

// RegisterUserRoutes registers all user routes on the router
func RegisterUserRoutes(r *RestServer) {
	// We group on empty paths to apply middlewares to everything but the
	// /login route. The group path is empty because it is easier to read
	users := r.Router.Group("")
	users.Use(authenticateJWT)

	users.GET("/users", GetAllUsers(r))
	users.GET("/users/:id", GetUser(r))
	users.POST("/users", CreateUser(r))
}

// RegisterPaymentRoutes registers all payment routes on the router
func RegisterPaymentRoutes(r *RestServer) {
	payments := r.Router.Group("")
	payments.Use(authenticateJWT)

	payments.GET("/payments", GetAllInvoices(r))
	payments.GET("/payments/:id", GetInvoice(r))
	payments.POST("/invoices/create", CreateInvoice(r))
	payments.POST("/invoices/pay", PayInvoice(r))
}

// authenticateJWT is the middleware applied to every request to authenticate
// the jwt is issued by us. It aborts the following request if the supplied jwt
// is not valid or has expired
func authenticateJWT(c *gin.Context) {
	// Here we extract the token from the header
	tokenString := c.GetHeader("Authorization")

	_, _, err := parseToken(tokenString)
	if err != nil {
		c.JSONP(http.StatusForbidden, gin.H{"error": err})
		c.Abort() // cancels the following request
	}

	log.Infof("jwt-token is valid: %s", tokenString)
}

// parseToken parses a string representation of a jwt-token, and validates
// it is signed by us. It returns the token and the extracted claims.
// If anything goes wrong, an error with a descriptive reason is returned.
func parseToken(tokenString string) (*jwt.Token, jwt.MapClaims, error) {
	claims := jwt.MapClaims{}
	// Here we decode the token, verify it is signed with our secret key, and
	// extract the claims
	token, err := jwt.ParseWithClaims(tokenString, claims,
		func(token *jwt.Token) (interface{}, error) {
			return []byte("secret_key"), nil
		})
	if err != nil {
		log.Errorf("parsing jwt-token %s failed %v", tokenString, err)
		return nil, nil, errors.Wrap(err, "invalid request, restricted endpoint")
	}

	if !token.Valid {
		log.Errorf("jwt-token invalid %s", tokenString)
		return nil, nil, errors.New("invalid token, restricted endpoint. log in first")
	}

	// convert Claims to a map-type we can extract fields from
	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, nil, errors.New("invalid token, could not extract claims")
	}

	return token, mapClaims, nil
}

// createJWTToken creates a new JWT token with the supplied email as the
// claim, a specific expiration time, and signed with our secret key.
// It returns the string representation of the token
func createJWTToken(email string) (string, error) {
	expiresAt := time.Now().Add(5 * time.Minute).Unix()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256,
		&JWTClaims{
			Email: email,
			StandardClaims: jwt.StandardClaims{
				ExpiresAt: expiresAt,
			},
		},
	)

	log.Info(expiresAt)

	tokenString, err := token.SignedString([]byte("secret_key"))
	if err != nil {
		log.Errorf("signing jwt-token failed %v", err)
		return "", err
	}

	return tokenString, nil
}
