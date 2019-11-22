package users

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"gitlab.com/arcanecrypto/teslacoil/build"

	"github.com/lib/pq"
	"github.com/sirupsen/logrus"

	"github.com/dchest/passwordreset"
	"github.com/pkg/errors"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"

	"gitlab.com/arcanecrypto/teslacoil/db"
)

var log = build.AddSubLogger("USER")

// User is a database table
type User struct {
	ID int `db:"id"`

	Email            string `db:"email"`
	HasVerifiedEmail bool   `db:"has_verified_email"`

	Firstname      *string `db:"first_name"`
	Lastname       *string `db:"last_name"`
	HashedPassword []byte  `db:"hashed_password" json:"-"`
	// TotpSecret is the secret key the user uses for their 2FA setup
	TotpSecret *string `db:"totp_secret"`
	// ConfirmedTotpSecret is whether or not the user has confirmed the TOTP
	// secret they received by entering a code. If they don't to this in a
	// timely manner, we remove 2FA from their account.
	ConfirmedTotpSecret bool       `db:"confirmed_totp_secret"`
	CreatedAt           time.Time  `db:"created_at"`
	UpdatedAt           time.Time  `db:"updated_at"`
	DeletedAt           *time.Time `db:"deleted_at"`
}

// SQL related constants
const (
	// returningFromUsersTable is a SQL snippet that returns all the rows needed
	// to scan a user struct
	returningFromUsersTable = "RETURNING id, email, has_verified_email, hashed_password, totp_secret, confirmed_totp_secret, updated_at, first_name, last_name"
	// selectFromUsersTable is a SQL snippet that selects all the rows needed to
	// get a full fledged user struct
	selectFromUsersTable = "SELECT id, email, has_verified_email, hashed_password, totp_secret, confirmed_totp_secret, updated_at, first_name, last_name"

	uniqueEmailForeignKey = "users_email_key"
)

// Exported constants
const (
	// PasswordResetTokenDuration is how long our password reset tokens are valid
	PasswordResetTokenDuration = 1 * time.Hour
	// EmailVerificationTokenDuration is how long our email verification tokens are valid
	EmailVerificationTokenDuration = 1 * time.Hour
	// TotpIssuer is the name we issue 2FA TOTP tokens under
	TotpIssuer = "Teslacoil"
)

var (
	// Secret key used for resetting passwords.
	// TODO: Make this secure :-)
	passwordResetSecretKey = []byte("assume we have a long randomly generated secret key here")
	// Secret key used for verifying emails
	// TODO: Make this secure :-)
	emailVerificationSecretKey = []byte("assume we have a different long and also random secret key here")
)

// Exported errors
var (
	Err2faAlreadyEnabled = errors.New("user already has 2FA credentials")
	Err2faNotEnabled     = errors.New("user does not have 2FA enabled")
	ErrInvalidTotpCode   = errors.New("invalid TOTP code")

	// ErrEmailMustBeUnique is used to signify that an already existing user has the desired email
	ErrEmailMustBeUnique           = errors.New("user emails must be unique")
	ErrHashedPasswordMustBeDefined = errors.New(
		"property HashedPassword on user must be defined")
	ErrEmailMustBeDefined = errors.New(
		"property Email on user must be defined ")
)

// GetAll reads all users from the database
func GetAll(d *db.DB) ([]User, error) {
	var queryResult []User
	err := d.Select(&queryResult, "SELECT * FROM users")
	if err != nil {
		return queryResult, err
	}

	return queryResult, nil
}

// GetByID selects all columns for user where id=id
func GetByID(db *db.DB, id int) (User, error) {
	userResult := User{}
	uQuery := fmt.Sprintf(`%s FROM users WHERE id=$1 LIMIT 1`,
		selectFromUsersTable)

	if err := db.Get(&userResult, uQuery, id); err != nil {
		return User{}, errors.Wrapf(err, "GetByID(db, %d)", id)
	}

	return userResult, nil
}

// GetByEmail selects all columns for user where email=email
func GetByEmail(db *db.DB, email string) (User, error) {
	userResult := User{}
	uQuery := fmt.Sprintf(`%s FROM users WHERE email=$1 LIMIT 1`,
		selectFromUsersTable)

	if err := db.Get(&userResult, uQuery, email); err != nil {
		return User{}, err
	}

	return userResult, nil
}

// GetByCredentials retrieves a user from the database by taking in
// the email and the raw password, then with bcrypt, compares the hashed
// password stored in the db and the raw password.
// returns the user if and only if the password matches
func GetByCredentials(db *db.DB, email string, password string) (
	User, error) {

	userResult := User{}
	uQuery := fmt.Sprintf(`%s FROM users WHERE email=$1 LIMIT 1`,
		selectFromUsersTable)

	if err := db.Get(&userResult, uQuery, email); err != nil {
		return User{}, err
	}

	err := bcrypt.CompareHashAndPassword(
		userResult.HashedPassword, []byte(password))
	if err != nil {
		return User{}, err
	}

	return userResult, nil
}

// CreateUserArgs is the struct required to create a new user using
// the Create method
type CreateUserArgs struct {
	Email     string
	Password  string
	FirstName *string
	LastName  *string
}

// Create inserts a user with email, password, firstname,
// and lastname into the db. The password is hashed and salted
// before it is saved
func Create(db *db.DB, args CreateUserArgs) (User, error) {
	hashedPassword, err := hashAndSalt(args.Password)
	if err != nil {
		return User{}, err
	}
	user := User{
		Email:          args.Email,
		HashedPassword: hashedPassword,
		Firstname:      args.FirstName,
		Lastname:       args.LastName,
	}

	userResp, err := InsertUser(db, user)
	if err != nil {
		return User{}, err
	}

	return userResp, nil
}

// GetEmailVerificationTokenWithKey creates a token that can be used
// to verify the given email. This function is exposed for testing
// purposes, all other callers should use the exposed method which use
// a predefined key.
func GetEmailVerificationTokenWithKey(db *db.DB, email string, key []byte) (
	string, error) {

	user, err := GetByEmail(db, email)
	if err != nil {
		return "", err
	}

	hashedEmail := sha256.Sum256([]byte(user.Email))

	// we use the passwordreset package here because resetting a password
	// and verifying an email is fundamentally the same operation: we
	// need to give the user a secret they can use at a later point in
	// time, and that token needs to depend on both input from the user
	// (their email) as well as something secret to us. A difference
	// between this usage and when we're resetting a password is that
	// password reset tokens are single use, as they depend on both the
	// users email and password (which makes the token invalid after
	// the password has been changed). The tokens created here could
	// be used multiple times, but there doesn't seem to be any harm in this.
	token := passwordreset.NewToken(
		email, EmailVerificationTokenDuration,
		hashedEmail[:], key)
	return token, nil
}

// GetEmailVerificationToken creates a token that can be used to
// verify the given email.
func GetEmailVerificationToken(db *db.DB, email string) (string, error) {
	return GetEmailVerificationTokenWithKey(db, email, emailVerificationSecretKey)
}

// VerifyEmailVerificationToken verifies that the given token matches
// the signing key used to create tokens.
func VerifyEmailVerificationToken(token string) (string, error) {
	getEmailHash := func(email string) ([]byte, error) {
		hash := sha256.Sum256([]byte(email))
		return hash[:], nil
	}

	// see comment in getEmailVerificationTokenWithKey for explanation
	// on why we use the passwordreset package.
	return passwordreset.VerifyToken(
		token,
		getEmailHash,
		emailVerificationSecretKey)
}

// VerifyEmail checks the given token, and if valid sets the users
// email as verified
func VerifyEmail(db *db.DB, token string) (User, error) {
	email, err := VerifyEmailVerificationToken(token)
	if err != nil {
		return User{}, err
	}

	query := `UPDATE users SET has_verified_email = true WHERE email = $1
` + returningFromUsersTable

	rows, err := db.Query(query, email)
	if err != nil {
		return User{}, err
	}

	return scanUser(rows)

}

// NewPasswordResetToken creates a valid password reset token for the
// user corresponding to the given email, if such an user exists. This
// token can later be used to send a reset password request to the API.
func NewPasswordResetToken(db *db.DB, email string) (string, error) {
	user, err := GetByEmail(db, email)
	if err != nil {
		return "", err
	}

	token := passwordreset.NewToken(
		email, PasswordResetTokenDuration,
		user.HashedPassword, passwordResetSecretKey)
	return token, nil
}

// VerifyPasswordResetToken verifies the given token against the hashed
// password and email of the associated user, as well as our private
// signing key. It returns the login (email) that's allowed to use this
// password reset token.
func VerifyPasswordResetToken(db *db.DB, token string) (string, error) {
	getPasswordHash := func(email string) ([]byte, error) {
		user, err := GetByEmail(db, email)
		if err != nil {
			return nil, err
		}
		return user.HashedPassword, nil
	}

	return passwordreset.VerifyToken(
		token,
		getPasswordHash,
		passwordResetSecretKey)
}

// ChangePassword changes the password for a user
func (u User) ChangePassword(db *db.DB, newPassword string) (User, error) {
	hash, err := hashAndSalt(newPassword)
	if err != nil {
		return User{}, errors.Wrap(err, "could not hash new password")
	}

	// UPDATE user with new password
	query := `UPDATE users SET hashed_password = $1 WHERE id = $2 ` + returningFromUsersTable
	rows, err := db.Query(query, hash, u.ID)
	if err != nil {
		return User{}, errors.Wrap(err, "could not update user password")
	}

	// read updated user from db
	user, err := scanUser(rows)
	if err != nil {
		return User{}, errors.Wrap(err, "could not scan user when changing password")
	}

	return user, nil
}

// Create2faCredentials creates TOTP based 2FA credentials
// for the user. It fails if the user already has 2FA credentials
// set. It returns the totp key
// TODO(torkelrogstad) if the user doesn't confirm TOTP code within a set
//  time period, reverse this operation
func (u *User) Create2faCredentials(d *db.DB) (*otp.Key, error) {

	if u.TotpSecret != nil {
		return nil, Err2faAlreadyEnabled
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      TotpIssuer,
		AccountName: u.Email,
	})
	if err != nil {
		log.WithError(err).WithField("userID",
			u.ID).Error("could not generate TOTP key")
		return nil, err
	}

	updateTotpSecret := `UPDATE users 
		SET totp_secret = $1 
		WHERE id = $2`
	_, err = d.Query(updateTotpSecret, key.Secret(), u.ID)
	if err != nil {
		return nil, errors.Wrap(err, "could not update totp_secret in DB")
	}
	return key, nil
}

// Delete2faCredentials disabled 2FA authorizaton, assuming
// the user already has requested and confirmed 2FA credentials.
func (u *User) Delete2faCredentials(d *db.DB, passcode string) (User, error) {

	if u.TotpSecret == nil {
		return *u, Err2faNotEnabled
	}

	if !totp.Validate(passcode, *u.TotpSecret) {
		return *u, ErrInvalidTotpCode
	}

	unsetTotpQuery := `UPDATE users
		SET confirmed_totp_secret = false, totp_secret = NULL
		WHERE id = $1 ` + returningFromUsersTable
	rows, err := d.Query(unsetTotpQuery, u.ID)
	if err != nil {
		return *u, errors.Wrap(err, "could not unset TOTP status in DB")
	}

	updatedUser, err := scanUser(rows)
	if err != nil {
		return *u, err
	}
	return updatedUser, nil

}

// Confirm2faCredentials enables 2FA authorization, assuming
// the user already has requested 2FA credentials.
func (u *User) Confirm2faCredentials(d *db.DB, passcode string) (User, error) {

	if u.TotpSecret == nil {
		return *u, Err2faNotEnabled
	}
	if !totp.Validate(passcode, *u.TotpSecret) {
		return *u, ErrInvalidTotpCode
	}
	if u.ConfirmedTotpSecret {
		return *u, Err2faAlreadyEnabled
	}

	confirmTotpQuery := `UPDATE users
		SET confirmed_totp_secret = true
		WHERE id = $1 ` + returningFromUsersTable
	rows, err := d.Query(confirmTotpQuery, u.ID)
	if err != nil {
		return *u, errors.Wrap(err, "could not confirm TOTP status in DB")
	}

	updatedUser, err := scanUser(rows)
	if err != nil {
		return *u, err
	}
	return updatedUser, nil
}

// UpdateOptions represents the different actions `UpdateUser` can perform.
type UpdateOptions struct {
	// If set to nil, does nothing. If set to the empty string, deletes
	// firstName
	NewFirstName *string
	// If set to nil, does nothing. If set to the empty string, deletes
	// lastName
	NewLastName *string
	// If set to nil, does nothing, if set to the empty string, we return
	// an error
	NewEmail *string
}

// Update the users email, first name and last name, depending on
// what options are passed in
func (u User) Update(database *db.DB, opts UpdateOptions) (User, error) {
	if u.ID == 0 {
		return User{}, errors.New("UserID cannot be 0")
	}

	// no action needed
	if opts.NewFirstName == nil &&
		opts.NewLastName == nil && opts.NewEmail == nil {
		return User{}, errors.New("no actions given in UpdateOptions")
	}

	updateQuery := `UPDATE users SET `
	var updates []string
	queryUser := User{
		ID: u.ID,
	}

	if opts.NewEmail != nil {
		if *opts.NewEmail == "" {
			return User{}, errors.New("cannot delete email")
		}
		updates = append(updates, "email = :email")
		queryUser.Email = *opts.NewEmail
	}
	if opts.NewFirstName != nil {
		if *opts.NewFirstName == "" {
			updates = append(updates, "first_name = NULL")
		} else {
			updates = append(updates, "first_name = :first_name")
		}
		queryUser.Firstname = opts.NewFirstName
	}
	if opts.NewLastName != nil {
		if *opts.NewLastName == "" {
			updates = append(updates, "last_name = NULL")
		} else {
			updates = append(updates, "last_name = :last_name")
		}
		queryUser.Lastname = opts.NewLastName
	}

	updateQuery += strings.Join(updates, ",")
	updateQuery += ` WHERE id = :id ` + returningFromUsersTable
	log.WithFields(logrus.Fields{
		"userID":       queryUser.ID,
		"sqlQuery":     updateQuery,
		"newEmail":     opts.NewEmail,
		"newFirstName": opts.NewFirstName,
		"newLastName":  opts.NewLastName,
	}).Debug("executing SQL for updating user")

	rows, err := database.NamedQuery(
		updateQuery,
		&queryUser,
	)
	if err != nil {
		return User{}, errors.Wrap(err, "could not update user")
	}
	user, err := scanUser(rows)

	if err != nil {
		msg := fmt.Sprintf("updating user with ID %d failed", u.ID)
		return User{}, errors.Wrap(err, msg)
	}

	return user, nil

}

func (u User) String() string {
	fragments := []string{
		fmt.Sprintf("ID: %d", u.ID),
		fmt.Sprintf("Email: %s", u.Email),
		fmt.Sprintf("HasVerifiedEmail: %t", u.HasVerifiedEmail),
		fmt.Sprintf("CreatedAt: %s", u.CreatedAt),
	}

	if u.Firstname != nil {
		fragments = append(fragments, fmt.Sprintf(" Firstname: %s", *u.Firstname))
	} else {
		fragments = append(fragments, "Firstname: <nil>")
	}

	if u.Lastname != nil {
		fragments = append(fragments, fmt.Sprintf("Lastname: %s", *u.Lastname))
	} else {
		fragments = append(fragments, "Lastname: <nil>")
	}

	return strings.Join(fragments, ", ")
}

// scanUser tries to scan a User struct frm the given scannable interface
func scanUser(rows dbScanner) (User, error) {
	defer db.CloseRows(rows)
	user := User{}

	if err := rows.Err(); err != nil {
		return user, err
	}

	if rows.Next() {
		if err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.HasVerifiedEmail,
			&user.HashedPassword,
			&user.TotpSecret,
			&user.ConfirmedTotpSecret,
			&user.UpdatedAt,
			&user.Firstname,
			&user.Lastname,
		); err != nil {
			return user, errors.Wrap(
				err, "could not scan user returned from DB")
		}
	} else {
		return user, errors.New("given rows did not have any elements")
	}

	return user, nil
}

// hashAndSalt generates a bcrypt hash from a string
func hashAndSalt(password string) ([]byte, error) {
	// hashPasswordCost is how many rounds the password
	// should be hashed. rounds = 1 << hashPasswordCost
	const hashPasswordCost = 12

	hash, err := bcrypt.GenerateFromPassword([]byte(password), hashPasswordCost)
	if err != nil {
		log.WithError(err).Error("could not hash password")
		return nil, err
	}

	// bcrypt returns a base64 encoded hash, therefore string(hash)
	// works for converting the password to a readable format
	log.WithField("passwordHash", string(hash)).Trace("generated password")

	return hash, nil
}

type dbScanner interface {
	Next() bool
	Scan(dest ...interface{}) error
	Close() error
	Err() error
}

// InsertUser inserts fields from a user struct into the database.
func InsertUser(i db.Inserter, user User) (User, error) {
	userCreateQuery := `INSERT INTO users 
		(email, hashed_password, totp_secret, confirmed_totp_secret, 
first_name, last_name)
		VALUES (:email, :hashed_password, :totp_secret, false, :first_name, 
:last_name) ` + returningFromUsersTable

	if len(user.Email) == 0 {
		return User{}, ErrEmailMustBeDefined
	}

	if len(user.HashedPassword) == 0 {
		// hased password is not set
		return User{}, ErrHashedPasswordMustBeDefined
	}

	rows, err := i.NamedQuery(userCreateQuery, user)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Constraint == uniqueEmailForeignKey {
			err = ErrEmailMustBeUnique
		}
		return User{}, fmt.Errorf("could not insert user: %w", err)
	}

	userResp, err := scanUser(rows)
	if err != nil {
		return User{}, fmt.Errorf("could not scan user: %w", err)
	}
	return userResp, nil
}
