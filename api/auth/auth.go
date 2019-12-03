package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gitlab.com/arcanecrypto/teslacoil/api/apierr"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/models/users"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	uuid "github.com/satori/go.uuid"

	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/models/apikeys"
)

const (
	// Header is the name of the header we check for authentication details
	Header = "Authorization"
	// userIdVariable is the Gin variable we store the authenticated user ID
	// as
	userIdVariable = "user-id"
	// permissionsVariable is the Gin variable we store the authenticated users
	// permissons set under
	permissionsVariable = "user-permissions"
)

var log = build.AddSubLogger("AUTH")

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

// SetRawJwtPrivateKey takes in a PEM encoded RSA private key, and set the JWT signing
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
			return fmt.Errorf("unable to decode PEM block: %w", err)
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

// jwtClaims is the common form for our JWTs
type jwtClaims struct {
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
		var permissions apikeys.Permissions
		if strings.HasPrefix(header, "Bearer ") {
			userID, err = authenticateJWT(c)
			if err != nil {
				return
			}
			permissions = apikeys.AllPermissions
		} else {
			var key apikeys.Key
			key, err = authenticateApiKey(database, c)
			if err != nil {
				return
			}
			permissions = key.Permissions
			userID = key.UserID
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

		c.Set(permissionsVariable, permissions)
		c.Set(userIdVariable, userID)

	}
}

// authenticateApiKey tries to extract a valid API key from the authorization
// header. If that doesn't succeed, it rejects the request. It returns the
// user ID of the authenticated user. If an error is returned, the request is
// responded to, and no further action is required.
func authenticateApiKey(database *db.DB, c *gin.Context) (apikeys.Key, error) {
	uuidString := c.GetHeader(Header)
	parsedUuid, err := uuid.FromString(uuidString)
	if err != nil {
		log.WithError(err).Error("Bad authorization header for API key")
		apierr.Public(c, http.StatusBadRequest, apierr.ErrMalformedApiKey)
		return apikeys.Key{}, err
	}
	key, err := apikeys.Get(database, parsedUuid)
	if err != nil {
		log.WithError(err).WithField("key", parsedUuid).Error("Couldn't get API key")
		apierr.Public(c, http.StatusUnauthorized, apierr.ErrApiKeyNotFound)
		return apikeys.Key{}, err
	}
	return key, nil
}

// authenticateJWT tries to extract and verify a JWT from the authorization
// header. If that doesn't succeed, it rejects the request. It returns the
// user ID of the authenticated user. If an error is returned, the request
// is responded to, and no further action is needed.
func authenticateJWT(c *gin.Context) (int, error) {
	// Here we extract the token from the header
	tokenString := c.GetHeader(Header)

	_, claims, err := parseBearerJwt(tokenString)
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

func parseBearerJwtWithKey(tokenString string, publicKey *rsa.PublicKey) (*jwt.Token, *jwtClaims, error) {
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

	jwtClaims := &jwtClaims{
		Email:  email,
		UserID: int(id),
	}

	return token, jwtClaims, nil
}

// parseBearerJwt parses a string representation of a JWT and validates
// it is signed by us. It returns the token and the extracted claims.
// If anything goes wrong, an error with a descriptive reason is returned.
func parseBearerJwt(tokenString string) (*jwt.Token, *jwtClaims, error) {
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
		&jwtClaims{
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

// Info holds information needed to authenticate a user request
type Info struct {
	apikeys.Permissions
	UserID int
}

// getInfoOrReject retrieves the authentication info associated with this request. This
// info should be set by the authentication middleware. This means that this
// method can safely be called by all endpoints that use the authentication
// middleware.
func getInfoOrReject(c *gin.Context) (Info, bool) {
	id, exists := c.Get(userIdVariable)
	if !exists {
		const msg = "user ID is not set in request! This is a serious error, and means our authentication middleware did not set the correct variable"
		_ = c.AbortWithError(http.StatusInternalServerError, errors.New(msg))
		return Info{}, false
	}
	idInt, ok := id.(int)
	if !ok {
		const msg = "user ID was not an int! This means our authentication middleware did something bad"
		_ = c.AbortWithError(http.StatusInternalServerError, errors.New(msg))
		return Info{}, false
	}

	maybePermissions, exists := c.Get(permissionsVariable)
	if !exists {
		const msg = "permissions is not in request! This is a serious error, and means our authentication middleware did not set the correct variable"
		_ = c.AbortWithError(http.StatusInternalServerError, errors.New(msg))
		return Info{}, false
	}

	permissions, ok := maybePermissions.(apikeys.Permissions)
	if !ok {
		const msg = "permissions was not apikeys.Permissions struct! This means our authentication middleware did something bad"
		_ = c.AbortWithError(http.StatusInternalServerError, errors.New(msg))
		return Info{}, false
	}

	return Info{
		Permissions: permissions,
		UserID:      idInt,
	}, true
}

type Scope int

func (s Scope) String() string {
	switch s {
	case ReadWallet:
		return "ReadWallet"
	case CreateInvoice:
		return "CreateInvoice"
	case EditAccount:
		return "EditAccount"
	case SendTransaction:
		return "SendTransaction"
	default:
		return "UnknownScope:" + strconv.Itoa(int(s))
	}
}

const (
	// ReadWallet correponds to the ReadWallet permissions field
	ReadWallet Scope = iota
	// CreateInvoice correponds to the CreateInvoice permissions field
	CreateInvoice
	// EditAccount correponds to the EditAccount permissions field
	EditAccount
	// SendTransaction correponds to the SendTransaction permissions field
	SendTransaction
)

// RequireScope extracts the authentication information associated with the given
// request, and confirms the given scope against the one found in the request.
// If the scope doesn't match, we reject the request, and no further action is
// needed by the caller of this function.
func RequireScope(c *gin.Context, scope Scope) (int, bool) {
	info, ok := getInfoOrReject(c)
	if !ok {
		return 0, false
	}

	switch scope {
	case ReadWallet:
		if info.ReadWallet {
			return info.UserID, true
		}
	case CreateInvoice:
		if info.CreateInvoice {
			return info.UserID, true
		}
	case EditAccount:
		if info.EditAccount {
			return info.UserID, true
		}
	case SendTransaction:
		if info.SendTransaction {
			return info.UserID, true
		}
	}
	apierr.Public(c, http.StatusUnauthorized, apierr.ErrBadApiKey)
	return 0, false
}
