package api

import (
	"bytes"
	"encoding/base64"
	"image/png"
	"net/http"

	"github.com/dchest/passwordreset"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
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

var (
	badRequestResponse          = gin.H{"error": "Bad request, see documentation"}
	internalServerErrorResponse = gin.H{"error": "Internal server error, please try again or contact us"}
)

const (
	// Authorization is the name of the header where we expect the access token
	// to be found
	Authorization = "Authorization"
)

// GetAllUsers is a GET request that returns all the users in the database
// TODO: Restrict this to only the admin user
func (r *RestServer) GetAllUsers() gin.HandlerFunc {
	return func(c *gin.Context) {
		userResponse, err := users.GetAll(r.db)
		if err != nil {
			log.Error(err)
			c.JSONP(http.StatusInternalServerError, gin.H{"error": err})
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
		claims, ok := getJWTOrReject(c)
		if !ok {
			return
		}

		var request UpdateUserRequest
		if ok := getJSONOrReject(c, &request); !ok {
			return
		}

		// TODO debug
		log.Infof("Got update user request: %+v", request)

		user, err := users.GetByID(r.db, claims.UserID)
		if err != nil {
			c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
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
			log.Errorf("Could not update user: %v", err)
			c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
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

// GetUser is a GET request that returns users that match the one specified in the body
func (r *RestServer) GetUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := getJWTOrReject(c)
		if !ok {
			return
		}

		user, err := users.GetByID(r.db, claims.UserID)
		if err != nil {
			log.Error(err)
			c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
		}

		res := UserResponse{
			ID:        user.ID,
			Email:     user.Email,
			Balance:   user.Balance,
			Firstname: user.Firstname,
			Lastname:  user.Lastname,
		}

		log.Infof("GetUserResponse %v", res)

		// Return the user when it is found and no errors where encountered
		c.JSONP(200, res)
	}
}

// CreateUser is a POST request and inserts all the users in the body into the database
func (r *RestServer) CreateUser() gin.HandlerFunc {
	// CreateUserRequest is the expected type to create a new user
	type CreateUserRequest struct {
		Email     string  `json:"email" binding:"required,email"`
		Password  string  `json:"password" binding:"required"`
		FirstName *string `json:"firstName"`
		LastName  *string `json:"lastName"`
	}

	return func(c *gin.Context) {

		var request CreateUserRequest
		if ok := getJSONOrReject(c, &request); !ok {
			return
		}

		log.Info("creating user with credentials: ", request)

		// because the email column in users table has the unique tag, we don't
		// double check the email is unique
		u, err := users.Create(r.db, users.CreateUserArgs{
			Email:     request.Email,
			Password:  request.Password,
			FirstName: request.FirstName,
			LastName:  request.LastName,
		})
		if err != nil {
			log.Error(err)
			c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
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

		var request LoginRequest
		if ok := getJSONOrReject(c, &request); !ok {
			return
		}

		log.Info("logging in user: ", request)

		user, err := users.GetByCredentials(r.db, request.Email, request.Password)
		if err != nil {
			log.Error(err)
			c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
			return
		}
		log.Info("found user: ", user)

		// user has 2FA enabled
		if user.TotpSecret != nil {
			if request.TotpCode == "" {
				c.JSONP(http.StatusBadRequest, gin.H{"error": "Missing TOTP code"})
				return
			}

			if !totp.Validate(request.TotpCode, *user.TotpSecret) {
				log.Errorf("User provided invalid TOTP code")
				c.JSONP(http.StatusForbidden, gin.H{"error": "Bad TOTP code"})
				return
			}
		}

		tokenString, err := createJWTToken(request.Email, user.ID)
		if err != nil {
			c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
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
		log.Info("LoginResponse: ", res)

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
		claims, ok := getJWTOrReject(c)
		if !ok {
			return
		}

		user, err := users.GetByID(r.db, claims.UserID)
		if err != nil {
			log.Errorf("Could not find user %d: %v", claims.UserID, err)
			c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
			return
		}

		key, err := user.Create2faCredentials(r.db)
		if err != nil {
			log.Errorf("Could not create 2FA credentials for user %d: %v", claims.UserID, err)
			c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
			return
		}

		img, err := key.Image(200, 200)
		if err != nil {
			log.Errorf("Could not decode TOTP secret to image: %v", err)
			c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
			return
		}

		var imgBuf bytes.Buffer
		if err := png.Encode(&imgBuf, img); err != nil {
			log.Errorf("Could not encode TOTP secret image to base64: %v", err)
			c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
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
		claims, ok := getJWTOrReject(c)
		if !ok {
			return
		}

		var req Confirm2faRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Errorf("Could not bind Confirm2faRequest: %v", err)
			c.JSONP(http.StatusBadRequest, badRequestResponse)
			return
		}

		user, err := users.GetByID(r.db, claims.UserID)
		if err != nil {
			log.Errorf("Could not get user %d", claims.UserID)
			c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
			return
		}

		if _, err := user.Confirm2faCredentials(r.db, req.Code); err != nil {
			log.Errorf("Could not enable 2FA credentials for user %d: %v", claims.UserID, err)
			switch err {
			case users.Err2faNotEnabled:
				c.JSONP(http.StatusBadRequest, gin.H{"error": "2FA is not enabled"})
			case users.Err2faAlreadyEnabled:
				c.JSONP(http.StatusBadRequest, gin.H{"error": "2FA is already enabled"})
			case users.ErrInvalidTotpCode:
				c.JSONP(http.StatusForbidden, gin.H{"error": "Invalid TOTP code"})
			default:
				c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
			}
			return
		}

		log.Debugf("Confirmed 2FA setting for user %d", claims.UserID)
		c.Status(http.StatusOK)
	}
}

func (r *RestServer) Delete2fa() gin.HandlerFunc {
	type Delete2faRequest struct {
		Code string `json:"code" binding:"required"`
	}
	return func(c *gin.Context) {
		claims, ok := getJWTOrReject(c)
		if !ok {
			return
		}

		var req Delete2faRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Errorf("Could not bind Delete2faRequest: %v", err)
			c.JSONP(http.StatusBadRequest, badRequestResponse)
			return
		}

		user, err := users.GetByID(r.db, claims.UserID)
		if err != nil {
			log.Errorf("Could not get user %d", claims.UserID)
			c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
			return
		}

		if _, err := user.Delete2faCredentials(r.db, req.Code); err != nil {
			log.Errorf("Could not delete 2FA credentials for user %d: %v", claims.UserID, err)
			switch {
			case err == users.ErrInvalidTotpCode:
				c.JSONP(http.StatusForbidden, gin.H{"error": "Invalid TOTP code"})
			case err == users.Err2faNotEnabled:
				c.JSONP(http.StatusBadRequest, badRequestResponse)
			default:
				c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
			}
			return
		}

		log.Debugf("Removed 2FA setting for user %d", claims.UserID)
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
		claims, ok := getJWTOrReject(c)
		if !ok {
			return
		}

		tokenString, err := createJWTToken(claims.Email, claims.UserID)
		if err != nil {
			c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
		}

		res := &RefreshTokenResponse{
			AccessToken: tokenString,
		}

		log.Info("RefreshTokenResponse: ", res)

		c.JSONP(200, res)
	}
}

func (r *RestServer) ResetPassword() gin.HandlerFunc {
	type ResetPasswordRequest struct {
		Password string `json:"password" binding:"required"`
		Token    string `json:"token" binding:"required"`
	}

	return func(c *gin.Context) {
		var request ResetPasswordRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSONP(http.StatusBadRequest, badRequestResponse)
			return
		}

		login, err := users.VerifyPasswordResetToken(r.db, request.Token)
		if err != nil {
			switch {
			case err == passwordreset.ErrMalformedToken:
				c.JSONP(http.StatusBadRequest, gin.H{"error": "Token is malformed"})
				return

			case err == passwordreset.ErrExpiredToken:
				c.JSONP(http.StatusBadRequest, gin.H{"error": "Token is expired"})
				return
			case err == passwordreset.ErrWrongSignature:
				c.JSONP(http.StatusForbidden, gin.H{"error": "Token has bad signature"})
				return
			}
		}
		user, err := users.GetByEmail(r.db, login)
		if err != nil {
			c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
			return
		}

		if _, err := user.ResetPassword(r.db, request.Password); err != nil {
			c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
			return
		}

		c.JSONP(http.StatusOK, gin.H{"message": "Password reset successfully"})
	}
}

func (r *RestServer) ChangePassword() gin.HandlerFunc {
	type ChangePasswordRequest struct {
		OldPassword         string `json:"oldPassword" binding:"required"`
		NewPassword         string `json:"newPassword" binding:"required"`
		RepeatedNewPassword string `json:"repeatedNewPassword" binding:"required,eqfield=NewPassword"`
	}
	return func(c *gin.Context) {
		claims, ok := getJWTOrReject(c)
		if !ok {
			return
		}

		var request ChangePasswordRequest
		if ok := getJSONOrReject(c, &request); !ok {
			return
		}

		user, err := users.GetByID(r.db, claims.UserID)
		if err != nil {
			log.Errorf("Couldn't get user by ID when changing password: %v", err)
			c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
			return
		}

		if err := bcrypt.CompareHashAndPassword(user.HashedPassword, []byte(request.OldPassword)); err != nil {
			log.Errorf("Hashed password of user and given old password in request didn't match up!")
			c.JSONP(http.StatusForbidden, gin.H{"error": "Incorrect password"})
			return
		}

		if _, err := user.ResetPassword(r.db, request.NewPassword); err != nil {
			log.Errorf("Couldn't update user password: %v", err)
			c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
			return
		}

		c.Status(http.StatusOK)
	}
}
