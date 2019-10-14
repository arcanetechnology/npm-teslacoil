package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/apikeys"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
)

var (
	log = build.Log

	// tokenSigningKey is the key we use to sign and verify our JWTs
	privateKey               *rsa.PrivateKey
	publicKey                *rsa.PublicKey
	ErrPrivateKeyIsNotInArgs = errors.New("private key not present in args")
)

func init() {
	privateKey, publicKey = loadPrivateKey("/home/bo/gocode/src/gitlab.com/arcanecrypto/teslacoil/key.pem", "")
}

// loadPrivateKey loads the RSA private key from `rsaPrivKeyPath` with password `rsaPrivKeyPassword`
// if the RSA private key does not have a password, pass the empty string ""
// Because this is necessary for the application to run, the function will panic should anything go wrong
func loadPrivateKey(rsaPrivKeyPath, rsaPrivKeyPassword string) (*rsa.PrivateKey, *rsa.PublicKey) {

	priv, err := ioutil.ReadFile(rsaPrivKeyPath)
	if err != nil {
		log.Error(err)
		panic("No RSA private key found")
	}

	privPem, _ := pem.Decode(priv)
	if privPem.Type != "RSA PRIVATE KEY" {
		panic("key is of wrong type, not a RSA key")
	}

	var privPemBytes []byte
	if rsaPrivKeyPassword == "" {
		privPemBytes = privPem.Bytes
	} else {
		privPemBytes, err = x509.DecryptPEMBlock(privPem, []byte(rsaPrivKeyPassword))
		if err != nil {
			log.Error(err)
			panic("unable to decode pem block, wrong password?")
		}
	}

	var parsedKey interface{}
	if parsedKey, err = x509.ParsePKCS1PrivateKey(privPemBytes); err != nil {
		if parsedKey, err = x509.ParsePKCS8PrivateKey(privPemBytes); err != nil {
			log.Error(err)
			panic("unable to parse RSA private key")
		}
	}

	privateKey, ok := parsedKey.(*rsa.PrivateKey)
	if !ok {
		panic("unable to parse convert interface to rsa.PrivateKey")
	}

	return privateKey, &privateKey.PublicKey
}

const (
	// Header is the name of the header we check for authentication details
	Header = "Authorization"
	// UserIdVariable is the Gin variable we store the authenticated user ID
	// as
	UserIdVariable = "user-id"
)

// JWTClaims is the common form for our JWTs
type JWTClaims struct {
	Email  string `json:"email"`
	UserID int    `json:"user_id"`
	jwt.StandardClaims
}

// GetMiddleware generates a middleware that authenticates that the user
// supplies either a Bearer JWT or an API key in their authorization header.
// It also inserts the user ID associated with the authenticated user as a
// request variable that can be retrieved later, after the request has
// passed through the middleware.
func GetMiddleware(database *db.DB) func(c *gin.Context) {

	return func(c *gin.Context) {
		header := c.GetHeader(Header)
		if header == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Authorization header can't be empty"})
			c.Abort()
			return
		}
		var userID int
		if strings.HasPrefix(header, "Bearer ") {
			userID = authenticateJWT(c)
		} else {
			userID = authenticateApiKey(database, c)
		}
		c.Set(UserIdVariable, userID)

	}
}

// authenticateApiKey tries to extract a valid API key from the authorization
// header. If that doesn't succeed, it rejects the request. It returns the
// user ID of the authenticated user.
func authenticateApiKey(database *db.DB, c *gin.Context) int {
	uuidString := c.GetHeader(Header)
	parsedUuid, err := uuid.FromString(uuidString)
	if err != nil {
		log.WithError(err).Error("Bad authorization header for API key")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Malformed API key"})
		c.Abort()
		return 0
	}
	key, err := apikeys.Get(database, parsedUuid)
	if err != nil {
		log.WithError(err).WithField("key", parsedUuid).Error("Couldn't get API key")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "API key not found"})
		c.Abort()
		return 0
	}
	return key.UserID
}

// authenticateJWT tries to extract and verify a JWT from the authorization
// header. If that doesn't succeed, it rejects the request. It returns the
// user ID of the authenticated user.
func authenticateJWT(c *gin.Context) int {
	// Here we extract the token from the header
	tokenString := c.GetHeader(Header)

	_, claims, err := ParseBearerJwt(tokenString)
	if err != nil {
		var validationError *jwt.ValidationError
		if errors.As(err, &validationError) {
			switch validationError.Errors {
			case jwt.ValidationErrorMalformed:
				c.JSON(http.StatusBadRequest, gin.H{"error": "Malformed JWT"})
				c.Abort()
				return 0
			case jwt.ValidationErrorSignatureInvalid:
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid JWT signature"})
				c.Abort()
				return 0
			case jwt.ValidationErrorExpired:
				c.JSON(http.StatusUnauthorized, gin.H{"error": "JWT is expired"})
				c.Abort()
				return 0
			case jwt.ValidationErrorIssuedAt:
				c.JSON(http.StatusUnauthorized, gin.H{"error": "JWT is not valid yet"})
				c.Abort()
				return 0
			}
		}

		log.WithError(err).Info("Got unexpected error when parsing JWT")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Something went wrong..."})
		c.Abort()
		return 0
	}

	log.WithField("jwt", tokenString).Trace("JWT is valid")
	return claims.UserID
}

func parseBearerJwtWithKey(tokenString string, publicKey *rsa.PublicKey) (*jwt.Token, *JWTClaims, error) {
	claims := jwt.MapClaims{}

	// Remove 'Bearer ' from tokenString. It is fine to do it this way because
	// a malicious actor will just create an invalid JWT if anything other
	// then Bearer is passed as the first 7 characters
	if len(tokenString) < 7 || tokenString[:7] != "Bearer " {
		return nil, nil, jwt.NewValidationError("malformed JWT", jwt.ValidationErrorMalformed)
	}

	tokenString = tokenString[7:]

	// Here we decode the token, verify it is signed with our secret key, and
	// extract the claims
	token, err := jwt.ParseWithClaims(tokenString, claims,
		func(token *jwt.Token) (interface{}, error) {
			return publicKey, nil
		})
	if err != nil {
		log.WithError(err).WithField("jwt", tokenString).Errorf("Parsing JWT failed")
		return nil, nil, err
	}

	if !token.Valid {
		log.WithField("jwt", tokenString).Error("Invalid JWT")
		return nil, nil, err
	}

	// convert Claims tao a map-type we can extract fields from
	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, nil, err
	}

	// Extract fields from claims, and check they are of the correct type
	email, ok := mapClaims["email"].(string)
	if !ok {
		return nil, nil, fmt.Errorf("invalid token, could not extract email from claim")
	}

	// The user ID is a float64 here because JWTs use JSON encoding, and
	// JSON doesn't have integers. This is okay up until a point, where
	// too large user IDs would suffer from imprecision issues. We should
	// have a check when we create JWTs that the user ID cannot be set to
	// a too high value
	id, ok := mapClaims["user_id"].(float64)
	if !ok {
		return nil, nil, fmt.Errorf("invalid token, could not extract user_id from claim")
	}

	jwtClaims := &JWTClaims{
		Email:  email,
		UserID: int(id),
	}

	return token, jwtClaims, nil
}

// ParseBearerJwt parses a string representation of a JWT and validates
// it is signed by us. It returns the token and the extracted claims.
// If anything goes wrong, an error with a descriptive reason is returned.
func ParseBearerJwt(tokenString string) (*jwt.Token, *JWTClaims, error) {
	return parseBearerJwtWithKey(tokenString, publicKey)
}

type createJwtArgs struct {
	email      string
	id         int
	privateKey *rsa.PrivateKey
	now        func() time.Time
}

func createJwt(args createJwtArgs) (string, error) {
	if args.now == nil {
		args.now = time.Now
	}

	if args.privateKey == nil {
		return "", ErrPrivateKeyIsNotInArgs
	}

	expiresAt := args.now().Add(5 * time.Hour).Unix()

	token := jwt.NewWithClaims(jwt.SigningMethodRS512,
		&JWTClaims{
			Email:  args.email,
			UserID: args.id,
			StandardClaims: jwt.StandardClaims{
				ExpiresAt: expiresAt,
				IssuedAt:  args.now().Unix(),
			},
		},
	)

	log.Trace("Created token: ", token)

	tokenString, err := token.SignedString(args.privateKey)
	if err != nil {
		log.WithError(err).Error("Signing JWT failed")
		return "", err
	}

	log.WithField("jwt", tokenString).Trace("Signed token successfully")

	return "Bearer " + tokenString, nil
}

// CreateJwt creates a new JWT with the supplied email as the
// claim, a specific expiration time, and signed with our secret key.
// It returns the string representation of the token.
func CreateJwt(email string, id int) (string, error) {
	return createJwt(createJwtArgs{
		email:      email,
		id:         id,
		privateKey: privateKey,
		now:        time.Now,
	})
}
