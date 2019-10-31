package apiauth

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/dchest/passwordreset"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"gitlab.com/arcanecrypto/teslacoil/api/apierr"
	"gitlab.com/arcanecrypto/teslacoil/api/apiusers"
	"gitlab.com/arcanecrypto/teslacoil/api/auth"
	"gitlab.com/arcanecrypto/teslacoil/build"
	"gitlab.com/arcanecrypto/teslacoil/db"
	"gitlab.com/arcanecrypto/teslacoil/email"
	"gitlab.com/arcanecrypto/teslacoil/models/users"
	"golang.org/x/crypto/bcrypt"
)

var log = build.Log

// services that gets initiated in RegisterRoutes
var (
	database    *db.DB
	emailSender email.Sender
)

func RegisterRoutes(server *gin.Engine, db *db.DB, sender email.Sender, authmiddleware gin.HandlerFunc) *gin.RouterGroup {
	// assign the services given
	database = db
	emailSender = sender

	server.POST("/login", login())
	authGroup := server.Group("auth")

	// Does not need auth token to reset password
	authGroup.PUT("reset_password", resetPassword())
	authGroup.POST("reset_password", sendPasswordResetEmail())

	authGroup.Use(authmiddleware)

	authGroup.POST("2fa", enable2fa())
	authGroup.PUT("2fa", confirm2fa())
	authGroup.DELETE("2fa", delete2fa())

	authGroup.GET("refresh_token", refreshToken())
	authGroup.PUT("change_password", changePassword())

	return authGroup
}

// login is a POST request that retrieves a user with the
// credentials specified in the body
func login() gin.HandlerFunc {
	// loginRequest is the expected type to find a user in the DB
	type loginRequest struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
		TotpCode string `json:"totp"`
	}

	// loginResponse includes a jwt-token and the e-mail identifying the user
	type loginResponse struct {
		AccessToken string `json:"accessToken"`
		apiusers.Response
	}

	return func(c *gin.Context) {

		var req loginRequest
		if c.BindJSON(&req) != nil {
			return
		}

		user, err := users.GetByCredentials(database, req.Email, req.Password)
		if err != nil {
			switch {
			// we don't want to leak information about existing users, so
			// we respond with the same response for both errors
			case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
				apierr.Public(c, http.StatusUnauthorized, apierr.ErrNoSuchUser)

			case errors.Is(err, sql.ErrNoRows):
				apierr.Public(c, http.StatusUnauthorized, apierr.ErrNoSuchUser)

			default:
				log.WithError(err).Error("could not get by credentials")
				_ = c.Error(err)
				c.Abort()
			}
			return
		}

		if !user.HasVerifiedEmail {
			log.WithField("userId",
				user.ID).Error("user has not verified email")
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
				log.Error("user provided invalid TOTP code")
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

		res := loginResponse{
			AccessToken: tokenString,
			Response: apiusers.Response{
				ID:          user.ID,
				Email:       user.Email,
				BalanceSats: *user.BalanceSats,
				Firstname:   user.Firstname,
				Lastname:    user.Lastname,
			},
		}

		c.JSONP(200, res)
	}
}

func changePassword() gin.HandlerFunc {
	type changePasswordRequest struct {
		// we don't give this a "password" tag, as there's no point in validating
		// the users existing password
		OldPassword string `json:"oldPassword" binding:"required"`
		NewPassword string `json:"newPassword" binding:"required,password"`
		// we don't give this a "password" tag either, as this is checked to be
		// equal to `NewPassword`, and that has the "password" tag
		RepeatedNewPassword string `json:"repeatedNewPassword" binding:"required,eqfield=NewPassword"`
	}
	return func(c *gin.Context) {
		userID, ok := auth.GetUserIdOrReject(c)
		if !ok {
			return
		}

		var req changePasswordRequest
		if c.BindJSON(&req) != nil {
			return
		}

		user, err := users.GetByID(database, userID)
		if err != nil {
			log.WithError(err).Error("could not get user by ID when changing password")
			_ = c.Error(err)
			return
		}

		if err = bcrypt.CompareHashAndPassword(user.HashedPassword, []byte(req.OldPassword)); err != nil {
			apierr.Public(c, http.StatusForbidden, apierr.ErrIncorrectPassword)
			return
		}

		if _, err = user.ChangePassword(database, req.NewPassword); err != nil {
			log.WithError(err).Errorf("could not update user password")
			_ = c.Error(err)
			return
		}

		c.Status(http.StatusOK)
	}
}

func resetPassword() gin.HandlerFunc {
	type request struct {
		Password string `json:"password" binding:"required,password"`
		Token    string `json:"token" binding:"required"`
	}

	return func(c *gin.Context) {

		var req request
		if c.BindJSON(&req) != nil {
			return
		}

		login, err := users.VerifyPasswordResetToken(database, req.Token)
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
				log.WithError(err).Error("could not verify password reset token")
				_ = c.Error(err)
				return
			}
		}
		user, err := users.GetByEmail(database, login)
		if err != nil {
			log.WithError(err).WithField("email", login).Error("could not find user when resetting password")
			_ = c.Error(err)
			return
		}

		if _, err = user.ChangePassword(database, req.Password); err != nil {
			log.WithError(err).WithField("email", user.Email).Error("could not reset password")
			_ = c.Error(err)
			return
		}

		c.Status(http.StatusOK)
	}
}

// sendPasswordResetEmail takes in an email, and sends a password reset
// token to that destination. It is not an authenticated endpoint, as the
// user typically won't be able to sign in if this is something they are
// requesting.
func sendPasswordResetEmail() gin.HandlerFunc {
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
		user, err := users.GetByEmail(database, req.Email)
		if err != nil {
			log.WithError(err).Error("could not find recipient user")
			if errors.Is(err, sql.ErrNoRows) {
				c.Status(http.StatusOK)
				return
			}
			_ = c.Error(err)
			return
		}

		resetToken, err := users.NewPasswordResetToken(database, user.Email)
		if err != nil {
			_ = c.Error(err)
			return
		}

		if err := emailSender.SendPasswordReset(user, resetToken); err != nil {
			log.WithError(err).Error("could not send email")
			_ = c.Error(err)
			return
		}

		c.Status(http.StatusOK)
	}
}

// enable2fa takes in the user ID specified in the JWT, and enables 2FA for
// this user. The endpoint responds with a secret code that the user should
// use with their 2FA. They then need to confirm their 2FA setup by hitting
// a new endpoint.
func enable2fa() gin.HandlerFunc {
	type enable2faResponse struct {
		TotpSecret string `json:"secret"`
	}
	return func(c *gin.Context) {
		userID, ok := auth.GetUserIdOrReject(c)
		if !ok {
			return
		}

		user, err := users.GetByID(database, userID)
		if err != nil {
			log.WithError(err).Errorf("could not find user %d", userID)
			_ = c.Error(err)
			return
		}

		key, err := user.Create2faCredentials(database)
		if err != nil {
			log.Errorf("could not create 2FA credentials for user %d: %v", userID, err)
			_ = c.Error(err)
			return
		}

		response := enable2faResponse{
			TotpSecret: key.Secret(),
		}
		c.JSONP(http.StatusOK, response)

	}
}

// confirm2fa takes in a 2FA (TOTP) code and the user ID found in the JWT. It
// then checks the given 2FA code against the expected value for that user. If
// everything matches up, it marks the 2FA status of that user as confirmed.
func confirm2fa() gin.HandlerFunc {
	type confirm2faRequest struct {
		Code string `json:"code" binding:"required"`
	}
	return func(c *gin.Context) {
		userID, ok := auth.GetUserIdOrReject(c)
		if !ok {
			return
		}

		var req confirm2faRequest
		if c.BindJSON(&req) != nil {
			return
		}

		user, err := users.GetByID(database, userID)
		if err != nil {
			log.WithError(err).Errorf("could not get user %d", userID)
			_ = c.Error(err)
			return
		}

		if _, err := user.Confirm2faCredentials(database, req.Code); err != nil {
			log.Errorf("could not enable 2FA credentials for user %d: %v", userID, err)
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

		log.Debugf("confirmed 2FA setting for user %d", userID)
		c.Status(http.StatusOK)
	}
}

func delete2fa() gin.HandlerFunc {
	type delete2faRequest struct {
		Code string `json:"code" binding:"required"`
	}
	return func(c *gin.Context) {
		userID, ok := auth.GetUserIdOrReject(c)
		if !ok {
			return
		}

		var req delete2faRequest
		if c.BindJSON(&req) != nil {
			return
		}

		user, err := users.GetByID(database, userID)
		if err != nil {
			log.WithError(err).Errorf("could not get user %d", userID)
			_ = c.Error(err)
			return
		}

		if _, err := user.Delete2faCredentials(database, req.Code); err != nil {
			log.Errorf("could not delete 2FA credentials for user %d: %v", userID, err)
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

// refreshToken refreshes a jwt-token
func refreshToken() gin.HandlerFunc {
	// refreshTokenResponse is the response from /auth/refresh
	type refreshTokenResponse struct {
		AccessToken string `json:"accessToken"`
	}

	return func(c *gin.Context) {
		// The JWT is already authenticated, but here we parse the JWT to
		// extract the email as it is required to create a new JWT.
		userID, ok := auth.GetUserIdOrReject(c)
		if !ok {
			return
		}
		user, err := users.GetByID(database, userID)
		if err != nil {
			log.WithError(err).Error("could not refresh token")
			_ = c.Error(err)
			return
		}

		tokenString, err := auth.CreateJwt(user.Email, user.ID)
		if err != nil {
			log.WithError(err).Error("could not create JWT")
			_ = c.Error(err)
			return
		}

		res := refreshTokenResponse{
			AccessToken: tokenString,
		}

		c.JSONP(200, res)
	}
}
