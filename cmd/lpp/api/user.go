package api

import (
	"bytes"
	"encoding/base64"
	stderr "errors"
	"fmt"
	"image/png"
	"net/http"
	"net/url"
	"strings"

	"github.com/dchest/passwordreset"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/pquerna/otp/totp"
	uuid "github.com/satori/go.uuid"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"gitlab.com/arcanecrypto/teslacoil/internal/auth"
	"gitlab.com/arcanecrypto/teslacoil/internal/errhandling"
	"gitlab.com/arcanecrypto/teslacoil/internal/httptypes"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/apikeys"
	"gitlab.com/arcanecrypto/teslacoil/internal/users"
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
		c.JSONP(200, httptypes.Response(userResponse))
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

		var request UpdateUserRequest
		if c.BindJSON(&request) != nil {
			return
		}

		user, err := users.GetByID(r.db, userID)
		if err != nil {
			_ = c.Error(err)
			return
		}

		opts := users.UpdateOptions{}
		if request.Email != nil {
			opts.NewEmail = request.Email
		}
		if request.FirstName != nil {
			opts.NewFirstName = request.FirstName
		}
		if request.LastName != nil {
			opts.NewLastName = request.LastName
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

		c.JSONP(http.StatusOK, httptypes.Response(response))

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
		c.JSONP(200, httptypes.Response(res))
	}
}

// CreateUser is a POST request and inserts all the users in the body into the database
func (r *RestServer) CreateUser() gin.HandlerFunc {
	// CreateUserRequest is the expected type to create a new user
	type CreateUserRequest struct {
		Email     string  `json:"email" binding:"required,email"`
		Password  string  `json:"password" binding:"required,password"`
		FirstName *string `json:"firstName"`
		LastName  *string `json:"lastName"`
	}

	return func(c *gin.Context) {

		var request CreateUserRequest
		if c.BindJSON(&request) != nil {
			return
		}

		// because the email column in users table has the unique tag, we don't
		// double check the email is unique
		u, err := users.Create(r.db, users.CreateUserArgs{
			Email:     request.Email,
			Password:  request.Password,
			FirstName: request.FirstName,
			LastName:  request.LastName,
		})
		if err != nil {
			log.WithError(err).Error("Could not create user ")
			_ = c.Error(err)
			return
		}

		res := UserResponse{
			ID:        u.ID,
			Email:     u.Email,
			Balance:   u.Balance,
			Firstname: u.Firstname,
			Lastname:  u.Lastname,
		}
		log.Info("successfully created user: ", res)

		c.JSONP(200, httptypes.Response(res))
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

		var request LoginRequest
		if c.BindJSON(&request) != nil {
			return
		}

		user, err := users.GetByCredentials(r.db, request.Email, request.Password)
		if err != nil {
			switch {
			case stderr.Is(err, bcrypt.ErrMismatchedHashAndPassword):
				err := c.AbortWithError(http.StatusForbidden, errors.New("Invalid credentials"))
				_ = err.SetType(gin.ErrorTypePublic)
				_ = err.SetMeta(errhandling.ErrIncorrectPassword)
			default:
				log.WithError(err).Error("Couldn't get by credentials")
				_ = c.Error(err)
			}
			return
		}

		// user has 2FA enabled
		if user.TotpSecret != nil {
			if request.TotpCode == "" {
				err := c.AbortWithError(http.StatusBadRequest, errors.New("Missing TOTP code"))
				_ = err.SetType(gin.ErrorTypePublic)
				_ = err.SetMeta(errhandling.ErrMissingTotpCode)
				return
			}

			if !totp.Validate(request.TotpCode, *user.TotpSecret) {
				log.Error("User provided invalid TOTP code")
				err := c.AbortWithError(http.StatusForbidden, errors.New("Bad TOTP code"))
				_ = err.SetType(gin.ErrorTypePublic)
				_ = err.SetMeta(errhandling.ErrBadTotpCode)
				return
			}
		}

		tokenString, err := auth.CreateJwt(request.Email, user.ID)
		if err != nil {
			_ = c.Error(err)
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

		c.JSONP(200, httptypes.Response(res))
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
		c.JSONP(http.StatusOK, httptypes.Response(response))

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
				err := c.AbortWithError(http.StatusBadRequest, errors.New("2FA is not enabled"))
				_ = err.SetType(gin.ErrorTypePublic)
				_ = err.SetMeta(errhandling.Err2faNotEnabled)
			case users.Err2faAlreadyEnabled:
				err := c.AbortWithError(http.StatusBadRequest, errors.New("2FA is already enabled"))
				_ = err.SetType(gin.ErrorTypePublic)
				_ = err.SetMeta(errhandling.Err2faAlreadyEnabled)
			case users.ErrInvalidTotpCode:
				err := c.AbortWithError(http.StatusForbidden, errors.New("invalid TOTP code"))
				_ = err.SetType(gin.ErrorTypePublic)
				_ = err.SetMeta(errhandling.ErrInvalidTotpCode)
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
				err := c.AbortWithError(http.StatusForbidden, errors.New("Invalid TOTP code"))
				_ = err.SetType(gin.ErrorTypePublic)
				_ = err.SetMeta(errhandling.ErrInvalidTotpCode)
			case err == users.Err2faNotEnabled:
				// we don't want to leak that the user hasn't enabled 2fa
				err := c.AbortWithError(http.StatusBadRequest, errors.New("Bad request"))
				_ = err.SetType(gin.ErrorTypePublic)
				_ = err.SetMeta(errhandling.ErrBadRequest)
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

		c.JSONP(200, httptypes.Response(res))
	}
}

// SendPasswordResetEmail takes in an email, and sends a password reset
// token to that destination. It is not an authenticated endpoint, as the
// user typically won't be able to sign in if this is something they are
// requesting.
func (r *RestServer) SendPasswordResetEmail() gin.HandlerFunc {
	type SendPasswordResetEmailRequest struct {
		Email string `json:"email" binding:"required,email"`
	}

	return func(c *gin.Context) {
		var request SendPasswordResetEmailRequest
		if c.BindJSON(&request) != nil {
			return
		}

		// TODO: If the user doesn't exist, respond with 200 but don't send an
		// TODO: email. We don't want to leak what emails our users have.
		user, err := users.GetByEmail(r.db, request.Email)
		if err != nil {
			_ = c.Error(err)
			return
		}

		from := mail.NewEmail("Teslacoil", "noreply@teslacoil.io")
		subject := "Password reset"
		var recipientName string
		var names []string
		if user.Firstname != nil {
			names = append(names, *user.Firstname)
		}
		if user.Lastname != nil {
			names = append(names, *user.Lastname)
		}
		if len(names) == 0 {
			recipientName = user.Email
		} else {
			recipientName = strings.Join(names, " ")
		}

		to := mail.NewEmail(recipientName, user.Email)
		resetToken, err := users.GetPasswordResetToken(r.db, user.Email)
		if err != nil {
			_ = c.Error(err)
			return
		}

		resetPasswordUrl := fmt.Sprintf("https://teslacoil.io/reset-password?token=%s", url.QueryEscape(resetToken))
		htmlText := fmt.Sprintf(
			`<p>You have requested a password reset. Go to <a href="%s">%s</a> to complete this process.</p>`,
			resetPasswordUrl, resetPasswordUrl)
		message := mail.NewSingleEmail(from, subject, to, "", htmlText)
		log.Infof("Sending password reset email: %+v", message)

		response, err := r.EmailSender.Send(message)
		if err != nil {
			log.WithError(err).Errorf("Could not send email to %s", to.Address)
			_ = c.Error(err)
			return
		}
		log.Infof("Sent email successfully. Response: %+v", response)

		c.JSONP(http.StatusOK, gin.H{"message": fmt.Sprintf("Sent password reset email to %s", user.Email)})
	}
}

func (r *RestServer) ResetPassword() gin.HandlerFunc {
	type ResetPasswordRequest struct {
		Password string `json:"password" binding:"required,password"`
		Token    string `json:"token" binding:"required"`
	}

	return func(c *gin.Context) {
		var request ResetPasswordRequest
		if c.BindJSON(&request) != nil {
			return
		}

		login, err := users.VerifyPasswordResetToken(r.db, request.Token)
		if err != nil {
			switch {
			case err == passwordreset.ErrMalformedToken:
				err := c.AbortWithError(http.StatusBadRequest, errors.New("JWT is malformed"))
				_ = err.SetType(gin.ErrorTypePublic)
				_ = err.SetMeta(errhandling.ErrMalformedJwt)
				return

			case err == passwordreset.ErrExpiredToken:
				err := c.AbortWithError(http.StatusBadRequest, errors.New("JWT is expired"))
				_ = err.SetType(gin.ErrorTypePublic)
				_ = err.SetMeta(errhandling.ErrExpiredJwt)
				return
			case err == passwordreset.ErrWrongSignature:
				err := c.AbortWithError(http.StatusForbidden, errors.New("JWT has bad signature"))
				_ = err.SetType(gin.ErrorTypePublic)
				_ = err.SetMeta(errhandling.ErrInvalidJwtSignature)
				return
			}
		}
		user, err := users.GetByEmail(r.db, login)
		if err != nil {
			_ = c.Error(err)
			return
		}

		if _, err := user.ResetPassword(r.db, request.Password); err != nil {
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

		var request ChangePasswordRequest
		if c.BindJSON(&request) != nil {
			return
		}

		user, err := users.GetByID(r.db, userID)
		if err != nil {
			log.WithError(err).Errorf("Couldn't get user by ID when changing password")
			_ = c.Error(err)
			return
		}

		if err := bcrypt.CompareHashAndPassword(user.HashedPassword, []byte(request.OldPassword)); err != nil {
			err := c.AbortWithError(http.StatusForbidden, errors.New("incorrect password"))
			_ = err.SetType(gin.ErrorTypePublic)
			_ = err.SetMeta(errhandling.ErrIncorrectPassword)
			return
		}

		if _, err := user.ResetPassword(r.db, request.NewPassword); err != nil {
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

		c.JSON(http.StatusCreated, httptypes.Response(CreateApiKeyResponse{
			Key:    rawKey,
			UserID: key.UserID,
		}))
	}
}
