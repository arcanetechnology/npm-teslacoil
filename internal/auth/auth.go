package auth

import (
	"errors"
	"fmt"
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

	// TODO change this before production
	// tokenSigningKey is the key we use to sign and verify our JWTs
	tokenSigningKey = []byte("secret_jwt_key")
)

// Header is the name of the header we check for authentication details
const Header = "Authorization"

// JWTClaims is the common form for our JWTs
type JWTClaims struct {
	Email  string `json:"email"`
	UserID int    `json:"user_id"`
	jwt.StandardClaims
}

// GetMiddleware generates a middleware that authenticates that the user
// supplies either a Bearer JWT or an API key in their authorization header.
func GetMiddleware(database *db.DB) func(c *gin.Context) {
	return func(c *gin.Context) {
		header := c.GetHeader(Header)
		if header == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Authorization header can't be empty"})
			c.Abort()
			return
		}
		if strings.HasPrefix(header, "Bearer ") {
			authenticateJWT(c)
		} else {
			authenticateApiKey(database, c)
		}
	}
}

func authenticateApiKey(database *db.DB, c *gin.Context) {
	uuidString := c.GetHeader(Header)
	parsedUuid, err := uuid.FromString(uuidString)
	if err != nil {
		log.WithError(err).Error("Bad authorization header for API key")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Malformed API key"})
		c.Abort()
		return
	}
	_, err = apikeys.Get(database, parsedUuid)
	if err != nil {
		log.WithError(err).WithField("key", parsedUuid).Error("Couldn't get API key")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "API key not found"})
		c.Abort()
	}
}

// authenticateJWT is the middleware applied to every request to authenticate
// the jwt is issued by us. It aborts the following request if the supplied jwt
// is not valid or has expired
func authenticateJWT(c *gin.Context) {
	// Here we extract the token from the header
	tokenString := c.GetHeader(Header)

	_, _, err := ParseBearerJwt(tokenString)
	if err != nil {
		var validationError *jwt.ValidationError
		if errors.As(err, &validationError) {
			switch validationError.Errors {
			case jwt.ValidationErrorMalformed:
				c.JSON(http.StatusBadRequest, gin.H{"error": "Malformed JWT"})
				c.Abort()
				return
			case jwt.ValidationErrorSignatureInvalid:
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid JWT signature"})
				c.Abort()
				return
			case jwt.ValidationErrorExpired:
				c.JSON(http.StatusUnauthorized, gin.H{"error": "JWT is expired"})
				c.Abort()
				return
			case jwt.ValidationErrorIssuedAt:
				c.JSON(http.StatusUnauthorized, gin.H{"error": "JWT is not valid yet"})
				c.Abort()
				return
			}
		}

		log.WithError(err).Info("Got unexpected error when parsing JWT")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Something went wrong..."})
		c.Abort()
		return
	}

	log.WithField("jwt", tokenString).Trace("JWT is valid")
}

func parseBearerJwtWithKey(tokenString string, key []byte) (*jwt.Token, *JWTClaims, error) {
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
			return key, nil
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
	return parseBearerJwtWithKey(tokenString, tokenSigningKey)
}

type createJwtArgs struct {
	email string
	id    int
	key   []byte
	now   func() time.Time
}

func createJwt(args createJwtArgs) (string, error) {
	if args.now == nil {
		args.now = time.Now
	}

	if args.key == nil {
		args.key = tokenSigningKey
	}

	expiresAt := args.now().Add(5 * time.Hour).Unix()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256,
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

	tokenString, err := token.SignedString(args.key)
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
		email: email,
		id:    id,
		key:   tokenSigningKey,
		now:   time.Now,
	})
}
