package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gitlab.com/arcanecrypto/teslacoil/cmd/lpp/api/apierr"
	"gitlab.com/arcanecrypto/teslacoil/models/users"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	pkgerrors "github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/models/apikeys"
)

var (
	log = build.Log
)

var (
	ErrPrivateKeyIsNotInArgs = errors.New("private key not present in args")
	ErrInvalidKeyType        = errors.New("key is not a RSA key")
	ErrJwtKeyHasNotBeenSet   = errors.New("JWT public key is nil! You need to call SetJwtPrivateKey before using this package")
)

// keys used to sign JWTs
var (
	jwtPrivateKey *rsa.PrivateKey
	jwtPublicKey  *rsa.PublicKey
)

// SetJwtPrivateKey takes in a PEM encoded RSA private key, and set the JWT signing
// key used in this package to it. Password may be empty.
func SetRawJwtPrivateKey(key, password []byte) (err error) {

	privPem, _ := pem.Decode(key)
	if privPem == nil {
		return errors.New("could not decode PEM key")
	}
	if privPem.Type != "RSA PRIVATE KEY" {
		return ErrInvalidKeyType
	}

	var privPemBytes []byte
	if len(password) == 0 {
		privPemBytes = privPem.Bytes
	} else {
		privPemBytes, err = x509.DecryptPEMBlock(privPem, password)
		if err != nil {
			return pkgerrors.Wrap(err, "unable to decode PEM block")
		}
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(privPemBytes)
	if err != nil {
		return err
	}

	SetJwtPrivateKey(privateKey)
	return nil
}

// SetJwtPrivateKey takes in a RSA private key, and set the JWT signing
// key used in this package to it.
func SetJwtPrivateKey(key *rsa.PrivateKey) {
	jwtPrivateKey, jwtPublicKey = key, &key.PublicKey
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
			apierr.Public(c, http.StatusBadRequest, apierr.ErrMissingAuthHeader)
			return
		}
		var userID int
		var err error
		if strings.HasPrefix(header, "Bearer ") {
			userID, err = authenticateJWT(c)
		} else {
			userID, err = authenticateApiKey(database, c)
		}

		if err != nil {
			return
		}

		// check that email is verified
		user, err := users.GetByID(database, userID)
		if err != nil {
			log.WithError(err).WithField("userId", userID).Error("Couldn't find user")
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}

		if !user.HasVerifiedEmail {
			log.WithField("userId", userID).Error("User has not verified email")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user hasn't verified email"})
			return
		}

		c.Set(UserIdVariable, userID)

	}
}

// authenticateApiKey tries to extract a valid API key from the authorization
// header. If that doesn't succeed, it rejects the request. It returns the
// user ID of the authenticated user. If an error is returned, the request is
// responded to, and no further action is required.
func authenticateApiKey(database *db.DB, c *gin.Context) (int, error) {
	uuidString := c.GetHeader(Header)
	parsedUuid, err := uuid.FromString(uuidString)
	if err != nil {
		log.WithError(err).Error("Bad authorization header for API key")
		apierr.Public(c, http.StatusBadRequest, apierr.ErrMalformedApiKey)
		return 0, err
	}
	key, err := apikeys.Get(database, parsedUuid)
	if err != nil {
		log.WithError(err).WithField("key", parsedUuid).Error("Couldn't get API key")
		apierr.Public(c, http.StatusUnauthorized, apierr.ErrApiKeyNotFound)
		return 0, err
	}
	return key.UserID, nil
}

// authenticateJWT tries to extract and verify a JWT from the authorization
// header. If that doesn't succeed, it rejects the request. It returns the
// user ID of the authenticated user. If an error is returned, the request
// is responded to, and no further action is needed.
func authenticateJWT(c *gin.Context) (int, error) {
	// Here we extract the token from the header
	tokenString := c.GetHeader(Header)

	_, claims, err := ParseBearerJwt(tokenString)
	if err != nil {
		var validationError *jwt.ValidationError
		if errors.As(err, &validationError) {
			switch validationError.Errors {
			case jwt.ValidationErrorMalformed:
				apierr.Public(c, http.StatusBadRequest, apierr.ErrMalformedJwt)
				return 0, err
			case jwt.ValidationErrorSignatureInvalid:
				apierr.Public(c, http.StatusUnauthorized, apierr.ErrInvalidJwtSignature)
				return 0, err
			case jwt.ValidationErrorExpired:
				apierr.Public(c, http.StatusUnauthorized, apierr.ErrExpiredJwt)
				return 0, err
			case jwt.ValidationErrorIssuedAt:
				apierr.Public(c, http.StatusUnauthorized, apierr.ErrJwtNotValidYet)
				return 0, err
			}
		}

		log.WithError(err).Info("Got unexpected error when parsing JWT")
		_ = c.Error(err)
		c.Abort()
		return 0, err
	}

	log.WithField("jwt", tokenString).Trace("JWT is valid")
	return claims.UserID, nil
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
	if jwtPublicKey == nil {
		log.Panic(ErrJwtKeyHasNotBeenSet)
	}
	return parseBearerJwtWithKey(tokenString, jwtPublicKey)
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
	if jwtPrivateKey == nil {
		log.Panic(ErrJwtKeyHasNotBeenSet)
	}

	return createJwt(createJwtArgs{
		email:      email,
		id:         id,
		privateKey: jwtPrivateKey,
		now:        time.Now,
	})
}
