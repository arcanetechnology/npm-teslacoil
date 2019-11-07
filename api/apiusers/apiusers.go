// Package apiusers provides HTTP handlers for querying for, creating and modifying
// users in our API
package apiusers

import (
	"database/sql"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/api/apierr"
	"gitlab.com/arcanecrypto/teslacoil/api/auth"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/email"
	"gitlab.com/arcanecrypto/teslacoil/models/users"
	"gitlab.com/arcanecrypto/teslacoil/models/users/balance"
)

var log = build.Log

// services that gets initiated in RegisterRoutes
var (
	database    *db.DB
	emailSender email.Sender
)

func RegisterRoutes(server *gin.Engine, db *db.DB, sender email.Sender, authmiddleware gin.HandlerFunc) {
	// assign the services given
	database = db
	emailSender = sender

	// Creating a user doesn't require authentication
	server.POST("/users", createUser())

	// verifying an email doesn't require authentication beyond the
	// verification token
	server.PUT("/users/verify_email", verifyEmail())
	server.POST("/users/verify_email", sendEmailVerificationEmail())

	users := server.Group("")
	users.Use(authmiddleware)
	users.GET("/users", getUser())
	users.PUT("/users", updateUser())
}

// Response is the type returned by the API for user related request
type Response struct {
	ID          int     `json:"userId"`
	Email       string  `json:"email"`
	BalanceSats int64   `json:"balanceSats"`
	Firstname   *string `json:"firstName"`
	Lastname    *string `json:"lastName"`
}

// updateUser takes in a JSON body with three optional fields (email, firstname,
// lastname), and updates the user in the header JWT accordingly
func updateUser() gin.HandlerFunc {

	type request struct {
		Email     *string `json:"email" binding:"email"`
		FirstName *string `json:"firstName"`
		LastName  *string `json:"lastName"`
	}

	return func(c *gin.Context) {
		userID, ok := auth.RequireScope(c, auth.EditAccount)
		if !ok {
			return
		}

		var req request
		if c.BindJSON(&req) != nil {
			return
		}

		user, err := users.GetByID(database, userID)
		if err != nil {
			_ = c.Error(err)
			return
		}

		opts := users.UpdateOptions{}
		if req.Email != nil {
			opts.NewEmail = req.Email
		}
		if req.FirstName != nil {
			opts.NewFirstName = req.FirstName
		}
		if req.LastName != nil {
			opts.NewLastName = req.LastName
		}
		updated, err := user.Update(database, opts)
		if err != nil {
			log.WithError(err).Error("could not update user")
			_ = c.Error(err)
			return
		}

		updatedBalance, err := balance.ForUser(database, updated.ID)
		if err != nil {
			log.WithError(err).Error("could not get balance for user")
			_ = c.Error(err)
			return
		}

		response := Response{
			ID:          updated.ID,
			Email:       updated.Email,
			BalanceSats: updatedBalance.Sats(),
			Firstname:   updated.Firstname,
			Lastname:    updated.Lastname,
		}

		c.JSONP(http.StatusOK, response)
	}
}

// sendEmailVerificationEmail is a POST request that sends a
// verificationemail if and only if the email is not already
// verified. Should the email already be verified, we return
// a 200 response, to prevent leaking info about users
func sendEmailVerificationEmail() gin.HandlerFunc {
	type request struct {
		Email string `json:"email" binding:"required,email"`
	}

	return func(c *gin.Context) {
		var req request
		if c.BindJSON(&req) != nil {
			return
		}
		log.WithField("email", req.Email).Info(
			"got request to send email verification email")

		user, err := users.GetByEmail(database, req.Email)
		if err != nil {
			// we don't want to leak information about users, so we reply with 200
			// if the user doesn't exist
			if errors.Is(err, sql.ErrNoRows) {
				c.Status(http.StatusOK)
				return
			}
			log.WithError(err).Error("could not get user by email")
			_ = c.Error(err)
			return
		}

		// respond with 200 here to not leak information about our users
		if user.HasVerifiedEmail {
			log.WithField("userId",
				user.ID).Debug("user already has verified email, " +
				"not sending out new email")
			c.Status(http.StatusOK)
			return
		}

		token, err := users.GetEmailVerificationToken(database, req.Email)
		if err != nil {
			log.WithError(err).Error("could not email verification token")
			_ = c.Error(err)
			return
		}
		if err = emailSender.SendEmailVerification(user, token); err != nil {
			_ = c.Error(err)
			return
		}

		c.Status(http.StatusOK)
	}
}

// verifyEmail is a POST request that verifies the email, given the token is correct
func verifyEmail() gin.HandlerFunc {
	type request struct {
		Token string `json:"token" binding:"required"`
	}

	return func(c *gin.Context) {
		var req request
		if c.BindJSON(&req) != nil {
			return
		}

		user, err := users.VerifyEmail(database, req.Token)
		if err != nil {
			log.WithError(err).Error("could not verify email")
			_ = c.Error(err)
			return
		}

		log.WithField("userId", user.ID).Debug("verified email")
		c.Status(http.StatusOK)
	}
}

// getUser is a GET request that returns users that match
// the one specified in the body
func getUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := auth.RequireScope(c, auth.ReadWallet)
		if !ok {
			return
		}

		user, err := users.GetByID(database, userID)
		if err != nil {
			log.WithError(err).Error("could not get user")
			_ = c.Error(err)
			return
		}

		userBalance, err := balance.ForUser(database, user.ID)
		if err != nil {
			log.WithError(err).WithField("userID",
				user.ID).Error("could not get balance")
			_ = c.Error(err)
			return
		}

		res := Response{
			ID:          user.ID,
			Email:       user.Email,
			BalanceSats: userBalance.Sats(),
			Firstname:   user.Firstname,
			Lastname:    user.Lastname,
		}

		// Return the user when it is found and no errors where encountered
		c.JSONP(200, res)
	}
}

func emailInWhitelist(email string) bool {
	whitelist := []string{
		// add friends and family here
		"bojalbor@gmail.com",
	}
	email = strings.ToLower(email)

	for _, whitelistedEmail := range whitelist {
		if whitelistedEmail == email {
			return true
		}
	}

	if strings.HasSuffix(email, "arcane.no") {
		return true
	}
	if strings.HasSuffix(email, "arcanecrypto.no") {
		return true
	}
	if strings.HasSuffix(email, "tigerstaden.com") {
		return true
	}
	if strings.HasSuffix(email, "middelborg.no") {
		return true
	}
	if strings.HasSuffix(email, "kleingroup.no") {
		return true
	}

	return false
}

// createUser is a POST request that inserts a new user into the db
// required: email and password, optional: firstname and lastname
func createUser() gin.HandlerFunc {
	type request struct {
		Email     string  `json:"email" binding:"required,email"`
		Password  string  `json:"password" binding:"required,password"`
		FirstName *string `json:"firstName"`
		LastName  *string `json:"lastName"`
	}
	type response struct {
		ID    int    `json:"id"`
		Email string `json:"email"`
	}

	return func(c *gin.Context) {

		var req request
		if c.BindJSON(&req) != nil {
			return
		}

		if os.Getenv(gin.EnvGinMode) == gin.ReleaseMode {
			if !emailInWhitelist(req.Email) {
				apierr.Public(c, http.StatusForbidden, apierr.ErrNotYetOpenForBusiness)
				return
			}
		}

		// because the email column in users table has the unique tag, we don't
		// double check the email is unique
		u, err := users.Create(database, users.CreateUserArgs{
			Email:     req.Email,
			Password:  req.Password,
			FirstName: req.FirstName,
			LastName:  req.LastName,
		})
		if err != nil {
			if errors.Is(err, users.ErrEmailMustBeUnique) {
				apierr.Public(c, http.StatusBadRequest, apierr.ErrUserAlreadyExists)
				return
			}
			log.WithError(err).Error("could not create user")
			_ = c.Error(err)
			return
		}

		res := response{
			ID:    u.ID,
			Email: u.Email,
		}
		log.WithFields(logrus.Fields{
			"userId": u.ID,
			"email":  u.Email,
		}).Info("successfully created user")

		// spawn email verification in new goroutine
		go handleSendEmailVerification(u, emailSender)

		c.JSONP(200, res)
	}
}

func handleSendEmailVerification(user users.User, sender email.Sender) {
	log.WithFields(logrus.Fields{
		"userId": user.ID,
		"email":  user.Email,
	}).Info("sending email verification email")
	emailToken, err := users.GetEmailVerificationToken(database, user.Email)
	if err != nil {
		log.WithError(err).Error("could not get email verification token")
		return
	}

	if err = sender.SendEmailVerification(user, emailToken); err != nil {
		log.WithError(err).Error("could not send email verification email")
		return
	}
}
