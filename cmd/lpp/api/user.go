package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gitlab.com/arcanecrypto/teslacoil/internal/users"
)

// UserResponse is the type returned by the api to the front-end
type UserResponse struct {
	ID        int     `json:"id"`
	Email     string  `json:"email"`
	Balance   int64   `json:"balance"`
	Firstname *string `json:"firstName"`
	Lastname  *string `json:"lastName"`
}

// CreateUserRequest is the expected type to create a new user
type CreateUserRequest struct {
	Email     string  `json:"email" binding:"required"`
	Password  string  `json:"password" binding:"required"`
	FirstName *string `json:"firstName"`
	LastName  *string `json:"lastName"`
}

// LoginRequest is the expected type to find a user in the DB
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
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

// RefreshTokenResponse is the response from /auth/refresh
type RefreshTokenResponse struct {
	AccessToken string `json:"accessToken"`
}

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

var (
	badRequestResponse          = gin.H{"error": "Bad request, see documentation"}
	internalServerErrorResponse = gin.H{"error": "Internal server error, please try again or contact us"}
)

const (
	// Authorization is the name of the header where we expect the access token
	// to be found
	Authorization = "Authorization"
)

// UpdateUser takes in a JSON body with three optional fields (email, firstname,
// lastname), and updates the user in the header JWT accordingly
func (r *RestServer) UpdateUser() gin.HandlerFunc {

	type UpdateUserRequest struct {
		Email     *string `json:"email"`
		FirstName *string `json:"firstName"`
		LastName  *string `json:"lastName"`
	}

	return func(c *gin.Context) {
		claims, ok := getJWTOrReject(c)
		if !ok {
			return
		}

		var request UpdateUserRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSONP(http.StatusBadRequest, badRequestResponse)
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
	return func(c *gin.Context) {
		var req CreateUserRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			log.Error(err)
			c.JSONP(http.StatusBadRequest, badRequestResponse)
			return
		}

		log.Info("creating user with credentials: ", req)

		// because the email column in users table has the unique tag, we don't
		// double check the email is unique
		u, err := users.Create(r.db, users.CreateUserArgs{
			Email:     req.Email,
			Password:  req.Password,
			FirstName: req.FirstName,
			LastName:  req.LastName,
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

	return func(c *gin.Context) {
		var req LoginRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			log.Error(err)
			c.JSON(http.StatusBadRequest, badRequestResponse)
			return
		}

		log.Info("logging in user: ", req)

		user, err := users.GetByCredentials(r.db, req.Email, req.Password)
		if err != nil {
			log.Error(err)
			c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
			return
		}
		log.Info("found user: ", user)

		tokenString, err := createJWTToken(req.Email, user.ID)
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

// RefreshToken refreshes a jwt-token
func (r *RestServer) RefreshToken() gin.HandlerFunc {
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

func (r *RestServer) ChangePassword() gin.HandlerFunc {
	type ChangePasswordRequest struct {
		OldPassword string `json:"oldPassword" binding:"required"`
		NewPassword string `json:"newPassword" binding:"required"`
	}
	return func(c *gin.Context) {

		var request ChangePasswordRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			log.Errorf("Could not bind request: %v", err)
			c.JSON(http.StatusBadRequest, badRequestResponse)
			return
		}

		claims, ok := getJWTOrReject(c)
		if !ok {
			return
		}

		user, err := users.GetByID(r.db, claims.UserID)
		if err != nil {
			log.Errorf("Couldn't get user by ID when changing password: %v", err)
			c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
			return
		}

		if _, err := user.ChangePassword(r.db, request.OldPassword, request.NewPassword); err != nil {
			log.Errorf("Couldn't update user password: %v", err)
			c.JSONP(http.StatusInternalServerError, internalServerErrorResponse)
			return
		}

		c.Status(http.StatusOK)
	}
}
