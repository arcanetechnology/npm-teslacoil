package users

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/dchest/passwordreset"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/sirupsen/logrus"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
	"golang.org/x/crypto/bcrypt"
)

// User is a database table
type User struct {
	ID int `db:"id"`

	Email            string `db:"email"`
	HasVerifiedEmail bool   `db:"has_verified_email"`

	// Balance is the balance of the user, expressed in sats
	Balance        int64   `db:"balance"`
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

// ChangeBalance is the struct for changing a users balance
type ChangeBalance struct {
	UserID    int
	AmountSat int64
}

// UsersTable is the tablename of users, as saved in the DB
const (
	UsersTable = "users"
	// returningFromUsersTable is a SQL snippet that returns all the rows needed
	// to scan a user struct
	returningFromUsersTable = "RETURNING id, email, has_verified_email, balance, hashed_password, totp_secret, confirmed_totp_secret, updated_at, first_name, last_name"

	// selectFromUsersTable is a SQL snippet that selects all the rows needed to
	// get a full fledged user struct
	selectFromUsersTable = "SELECT id, email, has_verified_email, balance, hashed_password, totp_secret, confirmed_totp_secret, updated_at, first_name, last_name"

	// PasswordResetTokenDuration is how long our password reset tokens are valid
	PasswordResetTokenDuration = 1 * time.Hour

	// EmailVerificationTokenDuration is how long our email verification tokens are valid
	EmailVerificationTokenDuration = 1 * time.Hour

	//TotpIssuer is the name we issue 2FA TOTP tokens under
	TotpIssuer = "Teslacoil"
)

// TODO: Make this secure :-)
var (
	// Secret key used for resetting passwords.
	passwordResetSecretKey = []byte("assume we have a long randomly generated secret key here")

	// Secret key used for verifying emails
	emailVerificationSecretKey = []byte("assume we have a different long and also random secret key here")
)

var (
	Err2faAlreadyEnabled = errors.New("user already has 2FA credentials")
	Err2faNotEnabled     = errors.New("user does not have 2FA enabled")
	ErrInvalidTotpCode   = errors.New("invalid TOTP code")
)

// GetAll is a GET request that returns all the users in the database
// TODO: This endpoint should be restricted to the admin
func GetAll(d *db.DB) ([]User, error) {
	// Equivalent to SELECT * from users;
	queryResult := []User{}
	err := d.Select(&queryResult, fmt.Sprintf("SELECT * FROM %s", UsersTable))
	if err != nil {
		return queryResult, err
	}

	return queryResult, nil
}

// GetByID is a GET request that returns users that match the one specified
// in the body
func GetByID(d *db.DB, id int) (User, error) {
	userResult := User{}
	uQuery := fmt.Sprintf(`%s FROM %s WHERE id=$1 LIMIT 1`,
		selectFromUsersTable, UsersTable)

	if err := d.Get(&userResult, uQuery, id); err != nil {
		return User{}, errors.Wrapf(err, "GetByID(db, %d)", id)
	}

	return userResult, nil
}

// GetByEmail is a GET request that returns users that match the one specified
// in the body
func GetByEmail(d *db.DB, email string) (User, error) {
	userResult := User{}
	uQuery := fmt.Sprintf(`%s FROM %s WHERE email=$1 LIMIT 1`,
		selectFromUsersTable, UsersTable)

	if err := d.Get(&userResult, uQuery, email); err != nil {
		return User{}, err
	}

	return userResult, nil
}

// GetByCredentials retrieves a user from the database using the email and
// the salted/hashed password
func GetByCredentials(d *db.DB, email string, password string) (
	User, error) {

	userResult := User{}
	uQuery := fmt.Sprintf(`%s FROM %s WHERE email=$1 LIMIT 1`,
		selectFromUsersTable, UsersTable)

	if err := d.Get(&userResult, uQuery, email); err != nil {
		return User{}, err
	}

	err := bcrypt.CompareHashAndPassword(
		userResult.HashedPassword, []byte(password))
	if err != nil {
		return User{}, err
	}

	return userResult, nil
}

// getEmailVerificationTokenWithKey creates a token that can be used to verify
// the given email. This function is exposed for testing purposes, all other
// callers should use the exposed method which use a predefined key.
func getEmailVerificationTokenWithKey(d *db.DB, email string, key []byte) (string, error) {
	user, err := GetByEmail(d, email)
	if err != nil {
		return "", err
	}

	hashedEmail := sha256.Sum256([]byte(user.Email))

	// we use the passwordreset package here because resetting a password
	// and verifying an email is fundamentally the same operation: we need to
	// give the user a secret they can use at a later point in time, and that
	// token needs to depend on both input from the user (their email) as well
	// as something secret to us. A difference between this usage and when we're
	// resetting a password is that password reset tokens are single use, as they
	// depend on both the users email and password (which makes the token invalid
	// after the password has been changed). The tokens created here could be
	// used multiple times, but there doesn't seem to be any harm in this.
	token := passwordreset.NewToken(
		email, EmailVerificationTokenDuration,
		hashedEmail[:], key)
	return token, nil
}

// GetEmailVerificationToken creates a token that can be used to verify the given
// email.
func GetEmailVerificationToken(d *db.DB, email string) (string, error) {
	return getEmailVerificationTokenWithKey(d, email, emailVerificationSecretKey)
}

// verifyEmailVerificationToken verifies that the given token matches the signing
// key used to create tokens.
func verifyEmailVerificationToken(token string) (string, error) {
	getEmailHash := func(email string) ([]byte, error) {
		hash := sha256.Sum256([]byte(email))
		return hash[:], nil
	}

	// see comment in getEmailVerificationTokenWithKey for explanation on why
	// we use the passwordreset package.
	return passwordreset.VerifyToken(
		token,
		getEmailHash,
		emailVerificationSecretKey)
}

// VerifyEmail checks the given token, and if valid sets the users email as verified
func VerifyEmail(d *db.DB, token string) (User, error) {
	email, err := verifyEmailVerificationToken(token)
	if err != nil {
		return User{}, err
	}

	query := `UPDATE users SET has_verified_email = true WHERE email = $1 ` + returningFromUsersTable
	rows, err := d.Query(query, email)
	if err != nil {
		return User{}, err
	}

	return scanUser(rows)

}

// GetPasswordResetToken creates a valid password reset token for the user
// corresponding to the given email, if such an user exists. This token
// can later be used to send a reset password request to the API.
func GetPasswordResetToken(d *db.DB, email string) (string, error) {
	user, err := GetByEmail(d, email)
	if err != nil {
		return "", err
	}

	token := passwordreset.NewToken(
		email, PasswordResetTokenDuration,
		user.HashedPassword, passwordResetSecretKey)
	return token, nil
}

// VerifyPasswordResetToken verifies the given token against the hashed
// password and email of the associated user, as well as our private signing
// key. It returns the login (email) that's allowed to use this password
// reset token.
func VerifyPasswordResetToken(d *db.DB, token string) (string, error) {
	getPasswordHash := func(email string) ([]byte, error) {
		user, err := GetByEmail(d, email)
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

type CreateUserArgs struct {
	Email     string
	Password  string
	FirstName *string
	LastName  *string
}

// Create is a POST request and inserts the user in the body into the database
func Create(d *db.DB, args CreateUserArgs) (User, error) {
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

	tx := d.MustBegin()
	userResp, err := insertUser(tx, user)
	if err != nil {
		txErr := tx.Rollback()
		if txErr != nil {
			return User{}, errors.Wrap(txErr, "--> tx.Rollback()")
		}
		return User{}, err
	}
	err = tx.Commit()
	if err != nil {
		txErr := tx.Rollback()
		if txErr != nil {
			return User{}, errors.Wrap(txErr, "--> tx.Rollback()")
		}
		log.Errorf("Could not commit user creation: %v\n", err)
		return User{}, err
	}

	return userResp, nil
}

// IncreaseBalance increases the balance of user id x by y satoshis
// This is using a struct as a parameter because it is a critical operation
// and placing the arguments in the wrong order leads to increasing the wrong
// users balance
func IncreaseBalance(tx *sqlx.Tx, cb ChangeBalance) (User, error) {
	if cb.AmountSat <= 0 {
		return User{}, fmt.Errorf("amount cant be less than or equal to 0")
	}

	updateBalanceQuery := `UPDATE users
		SET balance = balance + $1
		WHERE id = $2 ` + returningFromUsersTable

	rows, err := tx.Query(updateBalanceQuery, cb.AmountSat, cb.UserID)
	if err != nil {
		return User{}, errors.Wrapf(
			err,
			"IncreaseBalance() -> tx.Query(%s, %d, %d) could not construct user update",
			updateBalanceQuery,
			cb.AmountSat,
			cb.UserID)
	}

	user, err := scanUser(rows)
	if err != nil {
		return User{}, err
	}

	return user, nil
}

// DecreaseBalance decreases the balance of user id x by y satoshis
// A constraint on the DB prevents us from decreasing further than 0 satoshis
func DecreaseBalance(tx *sqlx.Tx, cb ChangeBalance) (User, error) {
	if cb.AmountSat <= 0 {
		return User{},
			errors.New("amount cant be less than or equal to 0")
	}

	updateBalanceQuery := `UPDATE users
		SET balance = balance - $1  
		WHERE id = $2 ` + returningFromUsersTable

	rows, err := tx.Query(updateBalanceQuery, cb.AmountSat, cb.UserID)
	if err != nil {
		return User{}, errors.WithMessagef(err, "decrease by %d for user %d", cb.AmountSat, cb.UserID)
	}

	user, err := scanUser(rows)
	if err != nil {
		return User{}, errors.WithMessage(err, "decreasebalance")
	}
	log.WithFields(logrus.Fields{
		"userId":     user.ID,
		"newBalance": user.Balance,
		"amount":     cb.AmountSat,
	}).Info("Decreased users balance")

	return user, nil

}

type dbScanner interface {
	Next() bool
	Scan(dest ...interface{}) error
	Close() error
	Err() error
}

// scanUser tries to scan a User struct frm the given scannable interface
func scanUser(rows dbScanner) (User, error) {
	user := User{}

	if err := rows.Err(); err != nil {
		return user, err
	}

	if rows.Next() {
		if err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.HasVerifiedEmail,
			&user.Balance,
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

	if err := rows.Close(); err != nil {
		return user, err
	}

	return user, nil
}

func hashAndSalt(pwd string) ([]byte, error) {
	// Use GenerateFromPassword to hash & salt pwd.
	// MinCost is just an integer constant provided by the bcrypt
	// package along with DefaultCost & MaxCost.
	// The cost can be any value you want provided it isn't lower
	// than the MinCost (4)

	const hashPasswordCost = 12
	hash, err := bcrypt.GenerateFromPassword([]byte(pwd), hashPasswordCost)
	if err != nil {
		log.Errorf("Couldn't hash password: %v", err)
		return nil, err
	}

	// bcrypt returns a base64 encoded hash, therefore string(hash) works for
	// converting the password to a readable format
	log.Tracef("generated password %s", string(hash))

	return hash, nil
}

func insertUser(tx *sqlx.Tx, user User) (User, error) {
	userCreateQuery := `INSERT INTO users 
		(email, balance, hashed_password, totp_secret, confirmed_totp_secret, first_name, last_name)
		VALUES (:email, 0, :hashed_password, :totp_secret, false, :first_name, :last_name) ` + returningFromUsersTable

	log.Debugf("Executing query for creating user (%s): %s", user, userCreateQuery)

	rows, err := tx.NamedQuery(userCreateQuery, user)
	if err != nil {
		return User{}, err
	}

	userResp, err := scanUser(rows)
	if err != nil {
		return User{}, errors.Wrap(err, fmt.Sprintf("insertUser(tx, %v) failed", user))
	}
	return userResp, nil
}

// UpdateUserOptions represents the different actions `UpdateUser` can perform.
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
// what options we get passed in
func (u User) Update(db *db.DB, opts UpdateOptions) (User, error) {
	if u.ID == 0 {
		return User{}, errors.New("User ID cannot be 0!")
	}

	// no action needed
	if opts.NewFirstName == nil &&
		opts.NewLastName == nil && opts.NewEmail == nil {
		return User{}, errors.New("no actions given in UpdateOptions")
	}

	updateQuery := `UPDATE users SET `
	updates := []string{}
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
	log.Debugf("Executing SQL for updating user: %s with opts %+v", updateQuery, opts)

	rows, err := db.NamedQuery(
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

func (u User) ResetPassword(db *db.DB, password string) (User, error) {
	hashed, err := hashAndSalt(password)
	if err != nil {
		return User{}, errors.Wrap(err, "User.ChangePassword(): couldn't hash new password")
	}

	tx := db.MustBegin()
	query := `UPDATE users SET hashed_password = $1 WHERE id = $2 ` + returningFromUsersTable
	rows, err := tx.Query(query, hashed, u.ID)
	if err != nil {
		if txErr := tx.Rollback(); txErr != nil {
			return User{}, errors.Wrap(err, txErr.Error())
		}
		return User{}, errors.Wrap(err, "couldn't update user password")
	}
	user, err := scanUser(rows)
	if err != nil {
		if txErr := tx.Rollback(); txErr != nil {
			return User{}, errors.Wrap(err, txErr.Error())
		}
		return User{}, errors.Wrap(err, "couldn't scan user when changing password")
	}

	if err = tx.Commit(); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return User{}, errors.Wrap(err, rollbackErr.Error())
		}
		return User{}, err
	}
	return user, nil
}

// Create2faCredentials creates TOTP based 2FA credentials for the user.
// It fails if the user already has 2FA credentials set. It returns the updated
// user.
// TODO(torkelrogstad) if the user doesn't confirm TOTP code within a set
// time period, reverse this operation
func (u *User) Create2faCredentials(d *db.DB) (*otp.Key, error) {
	if u.TotpSecret != nil {
		return nil, Err2faAlreadyEnabled
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      TotpIssuer,
		AccountName: u.Email,
	})
	if err != nil {
		log.Errorf("Could not generate TOTP key for u %d: %v", u.ID, err)
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

// Delete2faCredentials disabled 2FA authorizaton, assuming the user already
// has requested and confirmed 2FA credentials.
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
	updated, err := scanUser(rows)
	if err != nil {
		return *u, err
	}
	return updated, nil

}

// Confirm2faCredentials enables 2FA authorization, assuming the user already
// has requested 2FA credentials.
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
	updated, err := scanUser(rows)
	if err != nil {
		return *u, err
	}
	return updated, nil
}

func (u User) String() string {
	fragments := []string{
		fmt.Sprintf("ID: %d", u.ID),
		fmt.Sprintf("Email: %s", u.Email),
		fmt.Sprintf("HasVerifiedEmail: %t", u.HasVerifiedEmail),
		fmt.Sprintf("Balance: %d", u.Balance),
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
