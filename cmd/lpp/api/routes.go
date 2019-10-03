package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/pkg/errors"
	"github.com/sendgrid/rest"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/validation"
	"gopkg.in/go-playground/validator.v8"

	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/internal/payments"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/ln"
)

// Config is the configuration for our API. Currently it just sets the
// log level.
type Config struct {
	// LogLevel specifies which level our application is going to log to
	LogLevel logrus.Level
}

type EmailSender interface {
	Send(email *mail.SGMailV3) (*rest.Response, error)
}

// RestServer is the rest server for our app. It includes a Router,
// a JWT middleware a db connection, and a grpc connection to lnd
type RestServer struct {
	Router      *gin.Engine
	db          *db.DB
	lncli       *lnrpc.LightningClient
	EmailSender EmailSender
}

// JWTClaims is the common form for our jwts
type JWTClaims struct {
	Email  string `json:"email"`
	UserID int    `json:"user_id"`
	jwt.StandardClaims
}

//NewApp creates a new app
func NewApp(d *db.DB, lncli lnrpc.LightningClient, email EmailSender,
	callbacks payments.HttpPoster, config Config) (RestServer, error) {
	build.SetLogLevel(config.LogLevel)

	g := gin.Default()

	engine, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		log.Fatalf("Gin validator engine (%s) was validator.Validate", binding.Validator.Engine())
	}
	validators := validation.RegisterAllValidators(engine)
	log.Infof("Registered custom validators: %s", validators)

	g.Use(cors.New(cors.Config{
		AllowOrigins: []string{"https://teslacoil.io", "http://127.0.0.1:3000"},
		AllowMethods: []string{
			http.MethodPut, http.MethodGet,
			http.MethodPost, http.MethodPatch,
			http.MethodDelete,
		},
		AllowHeaders: []string{
			"Accept", "Access-Control-Allow-Origin", "Content-Type", "Referer",
			"Authorization"},
	}))

	r := RestServer{
		Router:      g,
		db:          d,
		lncli:       &lncli,
		EmailSender: email,
	}

	invoiceUpdatesCh := make(chan *lnrpc.Invoice)

	// Start a goroutine for getting notified of newly added/settled invoices.
	go ln.ListenInvoices(lncli, invoiceUpdatesCh)
	// Start a goroutine for handling the newly added/settled invoices.

	go payments.InvoiceStatusListener(invoiceUpdatesCh, d, callbacks)

	// We register /login separately to require jwt-tokens on every other endpoint
	// than /login
	r.Router.POST("/login", r.Login())
	// Ping handler
	r.Router.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	r.RegisterApiKeyRoutes()
	r.RegisterAuthRoutes()
	r.RegisterUserRoutes()
	r.RegisterPaymentRoutes()
	r.RegisterTransactionRoutes()

	return r, nil
}

// RegisterAuthRoutes registers all auth routes
func (r *RestServer) RegisterAuthRoutes() {
	auth := r.Router.Group("auth")

	// Does not need auth token to reset password
	auth.PUT("reset_password", r.ResetPassword())
	auth.POST("reset_password", r.SendPasswordResetEmail())

	auth.Use(authenticateJWT)

	// 2FA methods
	auth.POST("2fa", r.Enable2fa())
	auth.PUT("2fa", r.Confirm2fa())
	auth.DELETE("2fa", r.Delete2fa())

	auth.GET("refresh_token", r.RefreshToken())
	auth.PUT("change_password", r.ChangePassword())
}

func (r *RestServer) RegisterApiKeyRoutes() {
	keys := r.Router.Group("")
	keys.Use(authenticateJWT)
	keys.POST("apikey", r.CreateApiKey())

}

// RegisterUserRoutes registers all user routes on the router
func (r *RestServer) RegisterUserRoutes() {
	// Creating a user doesn't require a JWT
	r.Router.POST("/users", r.CreateUser())

	// We group on empty paths to apply middlewares to everything but the
	// /login route. The group path is empty because it is easier to read
	users := r.Router.Group("")
	users.Use(authenticateJWT)
	users.GET("/users", r.GetAllUsers())
	users.GET("/user", r.GetUser())
	users.PUT("/user", r.UpdateUser())
}

// RegisterPaymentRoutes registers all payment routes on the router.
// Payment is defined as a lightning transaction, so all things lightning
// can be found in payment packages
func (r *RestServer) RegisterPaymentRoutes() {
	payment := r.Router.Group("")
	payment.Use(authenticateJWT)

	payment.GET("payments", r.GetAllPayments())
	payment.GET("payment/:id", r.GetPaymentByID())
	payment.POST("/invoices/create", r.CreateInvoice())
	payment.POST("/invoices/pay", r.PayInvoice())
}

// RegisterTransactionRoutes registers all transaction routes on the router.
// A transaction is defined as an on-chain transaction
func (r *RestServer) RegisterTransactionRoutes() {
	transaction := r.Router.Group("")
	transaction.Use(authenticateJWT)

	transaction.GET("/transactions", r.GetAllTransactions())
	transaction.GET("/transaction/:id", r.GetTransactionByID())
	transaction.POST("/withdraw", r.WithdrawOnChain())
	transaction.POST("/deposit", r.NewDeposit())
}

// authenticateJWT is the middleware applied to every request to authenticate
// the jwt is issued by us. It aborts the following request if the supplied jwt
// is not valid or has expired
func authenticateJWT(c *gin.Context) {
	// Here we extract the token from the header
	tokenString := c.GetHeader(Authorization)

	_, _, err := parseBearerJWT(tokenString)
	if err != nil {
		c.JSONP(http.StatusForbidden, gin.H{"error": "bad authorization"})
		c.Abort() // cancels the following request
		return
	}

	log.Infof("jwt-token is valid: %s", tokenString)
}

// parseBearerJWT parses a string representation of a jwt-token, and validates
// it is signed by us. It returns the token and the extracted claims.
// If anything goes wrong, an error with a descriptive reason is returned.
func parseBearerJWT(tokenString string) (*jwt.Token, *JWTClaims, error) {
	claims := jwt.MapClaims{}

	// Remove 'Bearer ' from tokenString. It is fine to do it this way because
	// a malicious actor will just create an invalid jwt-token if anything other
	// then Bearer is passed as the first 7 characters
	if len(tokenString) < 7 || tokenString[:7] != "Bearer " {
		return nil, nil, fmt.Errorf(
			"invalid jwt-token, please include token on form 'Bearer xx.xx.xx")
	}

	tokenString = tokenString[7:]

	// Here we decode the token, verify it is signed with our secret key, and
	// extract the claims
	token, err := jwt.ParseWithClaims(tokenString, claims,
		func(token *jwt.Token) (interface{}, error) {
			// TODO: This must be changed before production
			return []byte("secret_key"), nil
		})
	if err != nil {
		log.Errorf("parsing jwt-token %s failed %v", tokenString, err)
		return nil, nil, errors.Wrap(err, "invalid request, restricted endpoint")
	}

	if !token.Valid {
		log.Errorf("jwt-token invalid %s", tokenString)
		return nil, nil, fmt.Errorf("invalid token, restricted endpoint. log in first")
	}

	// convert Claims to a map-type we can extract fields from
	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, nil, fmt.Errorf("invalid token, could not extract claims")
	}

	// Extract fields from claims, and check they are of the correct type
	email, ok := mapClaims["email"].(string)
	if !ok {
		return nil, nil, fmt.Errorf("invalid token, could not extract email from claim")
	}

	// TODO(bo): For some reason, the UserID is converted to a float64 when extracted
	// We need to write tests for this, to make sure it always is the case the
	// UserID is a float64, not an int/int64 etc.
	id, ok := mapClaims["user_id"].(float64)
	if !ok {
		log.Error(id)
		return nil, nil, fmt.Errorf("invalid token, could not extract user_id from claim")
	}

	jwtClaims := &JWTClaims{
		Email:  email,
		UserID: int(id),
	}

	return token, jwtClaims, nil
}

// getJWTOrReject parses the bearer JWT of the request, rejecting it with
// a Bad Request response if this fails. Second return value indicates whether
// or not the operation succeded. The error is logged and sent to the user,
// so the callee does not need to do anything more.
func getJWTOrReject(c *gin.Context) (*JWTClaims, bool) {
	_, claims, err := parseBearerJWT(c.GetHeader(Authorization))
	if err != nil {
		log.Errorf("Could not parse bearer JWT: %v", err)
		c.JSONP(http.StatusBadRequest, badRequestResponse)
		return nil, false
	}
	return claims, true
}

// getJSONOrReject extracts fields from the context and inserts
// them into the passed body argument. If an error occurs, the
// error is logged and a response with StatusBadRequest is sent
// body MUST be an address to a variable, not a variable
func getJSONOrReject(c *gin.Context, body interface{}) bool {
	if err := c.ShouldBindJSON(body); err != nil {
		log.Errorf("%s could not bind JSON %+v", c.Request.URL.Path, err)
		c.JSON(http.StatusBadRequest, badRequestResponse)
		return false
	}

	return true
}

func getQueryOrReject(c *gin.Context, body interface{}) bool {
	if err := c.ShouldBindQuery(body); err != nil {
		err = errors.Wrapf(err, "wrong query parameter format, check the documentation")
		log.Error(err)
		c.JSON(http.StatusBadRequest, err.Error())
		return false
	}
	return true
}

// createJWTToken creates a new JWT token with the supplied email as the
// claim, a specific expiration time, and signed with our secret key.
// It returns the string representation of the token
func createJWTToken(email string, id int) (string, error) {
	expiresAt := time.Now().Add(5 * time.Hour).Unix()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256,
		&JWTClaims{
			Email:  email,
			UserID: id,
			StandardClaims: jwt.StandardClaims{
				ExpiresAt: expiresAt,
			},
		},
	)

	log.Info("created token: ", token)

	tokenString, err := token.SignedString([]byte("secret_key"))
	if err != nil {
		log.Errorf("signing jwt-token failed %v", err)
		return "", err
	}

	log.Infof("signed token making tokenString %s", tokenString)

	return "Bearer " + tokenString, nil
}
