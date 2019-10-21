package api

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	stderr "errors"
	"image/png"
	"net/http"

	"gitlab.com/arcanecrypto/teslacoil/cmd/lpp/api/apierr"
	"gitlab.com/arcanecrypto/teslacoil/cmd/lpp/api/auth"
	"gitlab.com/arcanecrypto/teslacoil/models/users"

	"github.com/dchest/passwordreset"
	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"github.com/pquerna/otp/totp"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/apikeys"
	"golang.org/x/crypto/bcrypt"
)

// UserResponse is the type returned by the api to the front-end
type UserResponse struct {
	ID        int     `json:"id"`
	Email     string  `json:"email"`
	Balance   int64   `json:"balance"`
	Firstname *string `json:"firstName"`
	Lastname  *string `json:"lastName"`
}

// GetAllUsers is a GET request that returns all the users in the database
// TODO: Restrict this to only the admin user
func (r *RestServer) GetAllUsers() gin.HandlerFunc {
	return func(c *gin.Context) {
		userResponse, err := users.GetAll(r.db)
		if err != nil {
			log.WithError(err).Error("Couldn't get all users")
			_ = c.Error(err)
			return
		}
		c.JSONP(200, userResponse)
	}
}

// UpdateUser takes in a JSON body with three optional fields (email, firstname,
// lastname), and updates the user in the header JWT accordingly
func (r *RestServer) UpdateUser() gin.HandlerFunc {

	type UpdateUserRequest struct {
		Email     *string `json:"email" binding:"email"`
		FirstName *string `json:"firstName"`
		LastName  *string `json:"lastName"`
	}

	return func(c *gin.Context) {
		userID, ok := getUserIdOrReject(c)
		if !ok {
			return
		}

		var req UpdateUserRequest
		if c.BindJSON(&req) != nil {
			return
		}

		user, err := users.GetByID(r.db, userID)
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
		updated, err := user.Update(r.db, opts)
		if err != nil {
			log.WithError(err).Error("Could not update user")
			_ = c.Error(err)
			return
		}

		response := UserResponse{
			ID:        updated.ID,
			Email:     updated.Email,
			Balance:   updated.Balance,
			Firstname: updated.Firstname,
			Lastname:  updated.Lastname,
		}

		log.Infof("Update user result: %+v", response)

		c.JSONP(http.StatusOK, response)

	}
}

func (r *RestServer) SendEmailVerificationEmail() gin.HandlerFunc {
	type request struct {
		Email string `json:"email" binding:"required,email"`
	}

	return func(c *gin.Context) {
		var req request
		if c.BindJSON(&req) != nil {
			return
		}
		log.WithField("email", req.Email).Info("Got request to send email verification email")

		user, err := users.GetByEmail(r.db, req.Email)
		if err != nil {
			// we don't want to leak information about users, so we reply with 200
			// if the user doesn't exist
			if stderr.Is(err, sql.ErrNoRows) {
				c.Status(http.StatusOK)
				return
			}
			log.WithError(err).Error("Could not get user by email")
			_ = c.Error(err)
			return
		}

		// respond with 200 here to not leak information about our users
		if user.HasVerifiedEmail {
			log.WithField("userId", user.ID).Debug("User already has verified email, not sending out new email")
			c.Status(http.StatusOK)
			return
		}

		token, err := users.GetEmailVerificationToken(r.db, req.Email)
		if err != nil {
			log.WithError(err).Error("Could not email verification token")
			_ = c.Error(err)
			return
		}
		if err := r.EmailSender.SendEmailVerification(user, token); err != nil {
			_ = c.Error(err)
			return
		}

		c.Status(http.StatusOK)
	}
}

func (r *RestServer) VerifyEmail() gin.HandlerFunc {
	type Request struct {
		Token string `json:"token" binding:"required"`
	}

	return func(c *gin.Context) {
		var req Request
		if c.BindJSON(&req) != nil {
			return
		}

		user, err := users.VerifyEmail(r.db, req.Token)
		if err != nil {
			log.WithError(err).Error("Couldn't verify email")
			_ = c.Error(err)
			return
		}

		log.WithField("userId", user.ID).Debug("Verified email")
		c.Status(http.StatusOK)
	}
}

// GetUser is a GET request that returns users that match the one specified in the body
func (r *RestServer) GetUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := getUserIdOrReject(c)
		if !ok {
			return
		}

		user, err := users.GetByID(r.db, userID)
		if err != nil {
			log.WithError(err).Error("Could not get user")
			_ = c.Error(err)
			return
		}

		res := UserResponse{
			ID:        user.ID,
			Email:     user.Email,
			Balance:   user.Balance,
			Firstname: user.Firstname,
			Lastname:  user.Lastname,
		}

		// Return the user when it is found and no errors where encountered
		c.JSONP(200, res)
	}
}

func (r *RestServer) CreateUser() gin.HandlerFunc {
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

		// because the email column in users table has the unique tag, we don't
		// double check the email is unique
		u, err := users.Create(r.db, users.CreateUserArgs{
			Email:     req.Email,
			Password:  req.Password,
			FirstName: req.FirstName,
			LastName:  req.LastName,
		})
		if err != nil {
			log.WithError(err).Error("Could not create user ")
			var pqErr *pq.Error
			if stderr.As(err, &pqErr) && pqErr.Constraint == "users_email_key" {
				apierr.Public(c, http.StatusBadRequest, apierr.ErrUserAlreadyExists)
				return
			}
			_ = c.Error(err)
			return
		}

		res := response{
			ID:    u.ID,
			Email: u.Email,
		}
		log.Info("successfully created user: ", res)

		// spawn email verification in new goroutine
		go func() {
			log.WithFields(logrus.Fields{
				"userId": u.ID,
				"email":  u.Email,
			}).Info("Sending email verification email")
			emailToken, err := users.GetEmailVerificationToken(r.db, u.Email)
			if err != nil {
				log.WithError(err).Error("Could not get email verification token")
				return
			}

			if err := r.EmailSender.SendEmailVerification(u, emailToken); err != nil {
				log.WithError(err).Error("Could not send email verification email")
				return
			}
		}()

		c.JSONP(200, res)
	}
}

//Login logs in
func (r *RestServer) Login() gin.HandlerFunc {
	// LoginRequest is the expected type to find a user in the DB
	type LoginRequest struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
		TotpCode string `json:"totp"`
	}

	// LoginResponse includes a jwt-token and the e-mail identifying the user
	type LoginResponse struct {
		AccessToken string  `json:"accessToken"`
		Email       string  `json:"email"`
		UserID      int     `json:"userId"`
		Balance     int64   `json:"balance"`
		Firstname   *string `json:"firstName"`
		Lastname    *string `json:"lastName"`
	}

	return func(c *gin.Context) {

		var req LoginRequest
		if c.BindJSON(&req) != nil {
			return
		}

		user, err := users.GetByCredentials(r.db, req.Email, req.Password)
		if err != nil {
			switch {
			// we don't want to leak information about existing users, so
			// we respond with the same response for both errors
			case stderr.Is(err, bcrypt.ErrMismatchedHashAndPassword):
				apierr.Public(c, http.StatusUnauthorized, apierr.ErrNoSuchUser)
			case stderr.Is(err, sql.ErrNoRows):
				apierr.Public(c, http.StatusUnauthorized, apierr.ErrNoSuchUser)
			default:
				log.WithError(err).Error("Couldn't get by credentials")
				_ = c.Error(err)
				c.Abort()
			}
			return
		}

		if !user.HasVerifiedEmail {
			log.WithField("userId", user.ID).Error("User has not verified email")
			apierr.Public(c, http.StatusUnauthorized, apierr.ErrEmailNotVerified)
			return
		}

		// user has 2FA enabled
		if user.TotpSecret != nil {
			if req.TotpCode == "" {
				apierr.Public(c, http.StatusBadRequest, apierr.ErrMissingTotpCode)
				return
			}

			if !totp.Validate(req.TotpCode, *user.TotpSecret) {
				log.Error("User provided invalid TOTP code")
				apierr.Public(c, http.StatusForbidden, apierr.ErrBadTotpCode)
				return
			}
		}

		tokenString, err := auth.CreateJwt(req.Email, user.ID)
		if err != nil {
			_ = c.Error(err)
			c.Abort()
			return
		}

		res := LoginResponse{
			UserID:      user.ID,
			Email:       user.Email,
			AccessToken: tokenString,
			Balance:     user.Balance,
			Firstname:   user.Firstname,
			Lastname:    user.Lastname,
		}

		c.JSONP(200, res)
	}
}

// Enable2fa takes in the user ID specified in the JWT, and enables 2FA for
// this user. The endpoint responds with a secret code that the user should
// use with their 2FA. They then need to confirm their 2FA setup by hitting
// a new endpoint.
func (r *RestServer) Enable2fa() gin.HandlerFunc {
	type Enable2faResponse struct {
		TotpSecret string `json:"totpSecret"`
		Base64QR   string `json:"base64QrCode"`
	}
	return func(c *gin.Context) {
		userID, ok := getUserIdOrReject(c)
		if !ok {
			return
		}

		user, err := users.GetByID(r.db, userID)
		if err != nil {
			log.WithError(err).Errorf("Could not find user %d", userID)
			_ = c.Error(err)
			return
		}

		key, err := user.Create2faCredentials(r.db)
		if err != nil {
			log.Errorf("Could not create 2FA credentials for user %d: %v", userID, err)
			_ = c.Error(err)
			return
		}

		img, err := key.Image(200, 200)
		if err != nil {
			log.Errorf("Could not decode TOTP secret to image: %v", err)
			_ = c.Error(err)
			return
		}

		var imgBuf bytes.Buffer
		if err := png.Encode(&imgBuf, img); err != nil {
			log.Errorf("Could not encode TOTP secret image to base64: %v", err)
			_ = c.Error(err)
			return
		}

		base64Image := base64.StdEncoding.EncodeToString(imgBuf.Bytes())

		response := Enable2faResponse{
			TotpSecret: key.Secret(),
			Base64QR:   base64Image,
		}
		c.JSONP(http.StatusOK, response)

	}
}

// Confirm2fa takes in a 2FA (TOTP) code and the user ID found in the JWT. It
// then checks the given 2FA code against the expected value for that user. If
// everything matches up, it marks the 2FA status of that user as confirmed.
func (r *RestServer) Confirm2fa() gin.HandlerFunc {
	type Confirm2faRequest struct {
		Code string `json:"code" binding:"required"`
	}
	return func(c *gin.Context) {
		userID, ok := getUserIdOrReject(c)
		if !ok {
			return
		}

		var req Confirm2faRequest
		if c.BindJSON(&req) != nil {
			return
		}

		user, err := users.GetByID(r.db, userID)
		if err != nil {
			log.WithError(err).Errorf("Could not get user %d", userID)
			_ = c.Error(err)
			return
		}

		if _, err := user.Confirm2faCredentials(r.db, req.Code); err != nil {
			log.Errorf("Could not enable 2FA credentials for user %d: %v", userID, err)
			switch err {
			case users.Err2faNotEnabled:
				apierr.Public(c, http.StatusBadRequest, apierr.Err2faNotEnabled)
			case users.Err2faAlreadyEnabled:
				apierr.Public(c, http.StatusBadRequest, apierr.Err2faAlreadyEnabled)
			case users.ErrInvalidTotpCode:
				apierr.Public(c, http.StatusForbidden, apierr.ErrInvalidTotpCode)
			default:
				_ = c.Error(err)
			}
			return
		}

		log.Debugf("Confirmed 2FA setting for user %d", userID)
		c.Status(http.StatusOK)
	}
}

func (r *RestServer) Delete2fa() gin.HandlerFunc {
	type Delete2faRequest struct {
		Code string `json:"code" binding:"required"`
	}
	return func(c *gin.Context) {
		userID, ok := getUserIdOrReject(c)
		if !ok {
			return
		}

		var req Delete2faRequest
		if c.BindJSON(&req) != nil {
			return
		}

		user, err := users.GetByID(r.db, userID)
		if err != nil {
			log.WithError(err).Errorf("Could not get user %d", userID)
			_ = c.Error(err)
			return
		}

		if _, err := user.Delete2faCredentials(r.db, req.Code); err != nil {
			log.Errorf("Could not delete 2FA credentials for user %d: %v", userID, err)
			switch {
			case err == users.ErrInvalidTotpCode:
				apierr.Public(c, http.StatusForbidden, apierr.ErrInvalidTotpCode)
			case err == users.Err2faNotEnabled:
				// we don't want to leak that the user hasn't enabled 2fa
				apierr.Public(c, http.StatusBadRequest, apierr.ErrBadRequest)
			default:
				_ = c.Error(err)
			}
			return
		}

		log.Debugf("Removed 2FA setting for user %d", userID)
		c.Status(http.StatusOK)
	}
}

// RefreshToken refreshes a jwt-token
func (r *RestServer) RefreshToken() gin.HandlerFunc {
	// RefreshTokenResponse is the response from /auth/refresh
	type RefreshTokenResponse struct {
		AccessToken string `json:"accessToken"`
	}

	return func(c *gin.Context) {
		// The JWT is already authenticated, but here we parse the JWT to
		// extract the email as it is required to create a new JWT.
		userID, ok := getUserIdOrReject(c)
		if !ok {
			return
		}
		user, err := users.GetByID(r.db, userID)
		if err != nil {
			log.WithError(err).Error("Could not refresh token")
			_ = c.Error(err)
			return
		}

		tokenString, err := auth.CreateJwt(user.Email, user.ID)
		if err != nil {
			log.WithError(err).Error("Could not create JWT")
			_ = c.Error(err)
			return
		}

		res := &RefreshTokenResponse{
			AccessToken: tokenString,
		}

		c.JSONP(200, res)
	}
}

// SendPasswordResetEmail takes in an email, and sends a password reset
// token to that destination. It is not an authenticated endpoint, as the
// user typically won't be able to sign in if this is something they are
// requesting.
func (r *RestServer) SendPasswordResetEmail() gin.HandlerFunc {
	type request struct {
		Email string `json:"email" binding:"required,email"`
	}

	return func(c *gin.Context) {
		var req request
		if c.BindJSON(&req) != nil {
			return
		}

		// If the user doesn't exist, respond with 200 but don't send an
		// email. We don't want to leak what emails our users have.
		user, err := users.GetByEmail(r.db, req.Email)
		if err != nil {
			log.WithError(err).Error("Could not find recipient user")
			if stderr.Is(err, sql.ErrNoRows) {
				c.Status(http.StatusOK)
				return
			}
			_ = c.Error(err)
			return
		}

		resetToken, err := users.GetPasswordResetToken(r.db, user.Email)
		if err != nil {
			_ = c.Error(err)
			return
		}

		if err := r.EmailSender.SendPasswordReset(user, resetToken); err != nil {
			log.WithError(err).Error("Could not send email")
			_ = c.Error(err)
			return
		}

		c.Status(http.StatusOK)
	}
}

func (r *RestServer) ResetPassword() gin.HandlerFunc {
	type request struct {
		Password string `json:"password" binding:"required,password"`
		Token    string `json:"token" binding:"required"`
	}

	return func(c *gin.Context) {

		var req request
		if c.BindJSON(&req) != nil {
			return
		}

		login, err := users.VerifyPasswordResetToken(r.db, req.Token)
		if err != nil {
			switch {
			case err == passwordreset.ErrMalformedToken:
				apierr.Public(c, http.StatusBadRequest, apierr.ErrMalformedJwt)
				return
			case err == passwordreset.ErrExpiredToken:
				apierr.Public(c, http.StatusBadRequest, apierr.ErrExpiredJwt)
				return
			case err == passwordreset.ErrWrongSignature:
				apierr.Public(c, http.StatusForbidden, apierr.ErrInvalidJwtSignature)
				return
			default:
				log.WithError(err).Error("Could not verify password reset token")
				_ = c.Error(err)
				return
			}
		}
		user, err := users.GetByEmail(r.db, login)
		if err != nil {
			log.WithError(err).WithField("email", login).Error("Could not find user when resetting password")
			_ = c.Error(err)
			return
		}

		if _, err := user.ResetPassword(r.db, req.Password); err != nil {
			log.WithError(err).WithField("email", user.Email).Error("Could not reset password")
			_ = c.Error(err)
			return
		}

		c.JSONP(http.StatusOK, gin.H{"message": "Password reset successfully"})
	}
}

func (r *RestServer) ChangePassword() gin.HandlerFunc {
	type ChangePasswordRequest struct {
		// we don't give this a "password" tag, as there's no point in validating
		// the users existing password
		OldPassword string `json:"oldPassword" binding:"required"`
		NewPassword string `json:"newPassword" binding:"required,password"`
		// we don't give this a "password" tag either, as this is checked to be
		// equal to `NewPassword`, and that has the "password" tag
		RepeatedNewPassword string `json:"repeatedNewPassword" binding:"required,eqfield=NewPassword"`
	}
	return func(c *gin.Context) {
		userID, ok := getUserIdOrReject(c)
		if !ok {
			return
		}

		var req ChangePasswordRequest
		if c.BindJSON(&req) != nil {
			return
		}

		user, err := users.GetByID(r.db, userID)
		if err != nil {
			log.WithError(err).Errorf("Couldn't get user by ID when changing password")
			_ = c.Error(err)
			return
		}

		if err := bcrypt.CompareHashAndPassword(user.HashedPassword, []byte(req.OldPassword)); err != nil {
			apierr.Public(c, http.StatusForbidden, apierr.ErrIncorrectPassword)
			return
		}

		if _, err := user.ResetPassword(r.db, req.NewPassword); err != nil {
			log.WithError(err).Errorf("Couldn't update user password")
			_ = c.Error(err)
			return
		}

		c.Status(http.StatusOK)
	}
}

type CreateApiKeyResponse struct {
	Key    uuid.UUID `json:"key"`
	UserID int       `json:"userId"`
}

func (r *RestServer) CreateApiKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := getUserIdOrReject(c)
		if !ok {
			return
		}

		user, err := users.GetByID(r.db, userID)
		if err != nil {
			log.WithError(err).WithField("user", userID).Error("Could not get user")
			_ = c.Error(err)
			return
		}

		rawKey, key, err := apikeys.New(r.db, user)
		if err != nil {
			log.WithError(err).WithField("user", userID).Error("Could not create API key")
			_ = c.Error(err)
			return
		}

		c.JSON(http.StatusCreated, CreateApiKeyResponse{
			Key:    rawKey,
			UserID: key.UserID,
		})
	}
}
