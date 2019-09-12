package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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

// RestServer is the rest server for our app. It includes a Router,
// a JWT middleware a db connection, and a grpc connection to lnd
type RestServer struct {
	Router *gin.Engine
	db     *db.DB
	lncli  *lnrpc.LightningClient
}

// JWTClaims is the common form for our jwts
type JWTClaims struct {
	Email  string `json:"email"`
	UserID int    `json:"user_id"`
	jwt.StandardClaims
}

//NewApp creates a new app
func NewApp(d *db.DB, lncli lnrpc.LightningClient, config Config) (RestServer, error) {
	build.SetLogLevel(config.LogLevel)

	g := gin.Default()

	g.Use(cors.New(cors.Config{
		AllowOrigins: []string{"http://127.0.0.1:3000"},
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
		Router: g,
		db:     d,
		lncli:  &lncli,
	}

	invoiceUpdatesCh := make(chan *lnrpc.Invoice)
	go ln.ListenInvoices(lncli, invoiceUpdatesCh)

	go payments.InvoiceStatusListener(invoiceUpdatesCh, d)

	// We register /login separately to require jwt-tokens on every other endpoint
	// than /login
	r.Router.POST("/login", r.Login())
	r.RegisterAuthRoutes()
	r.RegisterUserRoutes()
	r.RegisterPaymentRoutes()

	return r, nil
}

// RegisterAuthRoutes registers all auth routes
func (r *RestServer) RegisterAuthRoutes() {
	auth := r.Router.Group("")
	auth.Use(authenticateJWT)

	auth.GET("/auth/refresh_token", r.RefreshToken())
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

// RegisterPaymentRoutes registers all payment routes on the router
func (r *RestServer) RegisterPaymentRoutes() {
	payments := r.Router.Group("")
	payments.Use(authenticateJWT)

	payments.GET("/payments", r.GetAllPayments())
	payments.GET("/payments/:id", r.GetPaymentByID())
	payments.POST("/invoices/create", r.CreateInvoice())
	payments.POST("/invoices/pay", r.PayInvoice())
}

// authenticateJWT is the middleware applied to every request to authenticate
// the jwt is issued by us. It aborts the following request if the supplied jwt
// is not valid or has expired
func authenticateJWT(c *gin.Context) {
	// Here we extract the token from the header
	tokenString := c.GetHeader("Authorization")

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
