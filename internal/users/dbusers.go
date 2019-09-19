package users

import (
	"fmt"
	"strings"
	"time"

	"github.com/dchest/passwordreset"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"gitlab.com/arcanecrypto/teslacoil/internal/platform/db"
	"golang.org/x/crypto/bcrypt"
)

// UserNew contains all fields used while constructing a new user
type UserNew struct {
	Email    string `json:"email"`
	Password string `json:"password" binding:"required"`
}

// User is a database table
type User struct {
	ID             int        `db:"id"`
	Email          string     `db:"email"`
	Balance        int64      `db:"balance"`
	Firstname      *string    `db:"first_name"`
	Lastname       *string    `db:"last_name"`
	HashedPassword []byte     `db:"hashed_password" json:"-"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
	DeletedAt      *time.Time `db:"deleted_at"`
}

// ChangeBalance is the struct for changing a users balance
type ChangeBalance struct {
	UserID    int   `json:"userId"`
	AmountSat int64 `json:"amountSat"`
}

// UsersTable is the tablename of users, as saved in the DB
const (
	UsersTable = "users"
	// returningFromUsersTable is a SQL snippet that returns all the rows needed
	// to scan a user struct
	returningFromUsersTable = "RETURNING id, email, balance, hashed_password, updated_at, first_name, last_name"

	// selectFromUsersTable is a SQL snippet that selects all the rows needed to
	// get a full fledged user struct
	selectFromUsersTable = "SELECT id, email, balance, hashed_password, updated_at, first_name, last_name"

	// PasswordResetTokenDuration is how long our password reset tokens are valid
	PasswordResetTokenDuration = 1 * time.Hour
)

// Secret key used for resetting passwords.
// TODO: Make this secure :-)
var passwordResetSecretKey = []byte("assume we have a long randomly generated secret key here")

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
		return User{}, errors.Wrapf(err, "GetByEmail(db, %s)", email)
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
		return User{}, errors.Wrapf(
			err, "GetByCredentials(db, %s, **password_not_logged**)", email)
	}

	err := bcrypt.CompareHashAndPassword(
		userResult.HashedPassword, []byte(password))
	if err != nil {
		return User{}, errors.Wrap(err, "password authentication failed")
	}

	log.Tracef("%s received user %v", uQuery, userResult)

	return userResult, nil
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
func DecreaseBalance(tx *sqlx.Tx, cb ChangeBalance) (User, error) {
	if cb.AmountSat <= 0 {
		return User{},
			fmt.Errorf("amount cant be less than or equal to 0")
	}

	updateBalanceQuery := `UPDATE users
		SET balance = balance - $1  
		WHERE id = $2 ` + returningFromUsersTable

	rows, err := tx.Query(updateBalanceQuery, cb.AmountSat, cb.UserID)
	if err != nil {
		return User{}, errors.Wrap(
			err,
			"DecreaseBalance(): could not construct user update",
		)
	}

	user, err := scanUser(rows)
	if err != nil {
		return User{}, err
	}
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
			&user.Balance,
			&user.HashedPassword,
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
		(email, balance, hashed_password, first_name, last_name)
		VALUES (:email, 0, :hashed_password, :first_name, :last_name) ` + returningFromUsersTable

	log.Debugf("Executing query for creating user (%s): %s", user, userCreateQuery)

	rows, err := tx.NamedQuery(userCreateQuery, user)
	if err != nil {
		return User{}, errors.Wrapf(
			err, "users.Create(db, %s, %s)",
			user.Email, string(user.HashedPassword))
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

// ChangePassword takes in a old and new password, and if the old password matches
// the one in our DB we update it to the given password.
func (u User) ChangePassword(db *db.DB, oldPassword, newPassword string) (User, error) {
	if u.HashedPassword == nil {
		return User{}, errors.New("user object lacks HashedPassword")
	}

	if err := bcrypt.CompareHashAndPassword(u.HashedPassword, []byte(oldPassword)); err != nil {
		return User{}, errors.Wrap(err, "given password didn't match up with hashed password in DB")
	}

	return u.ResetPassword(db, newPassword)

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

func (u User) String() string {
	str := fmt.Sprintf("ID: %d\n", u.ID)
	str += fmt.Sprintf("Email: %s\n", u.Email)
	str += fmt.Sprintf("Balance: %d\n", u.Balance)
	if u.Firstname != nil {
		str += fmt.Sprintf("Firstname: %s\n", *u.Firstname)
	} else {
		str += fmt.Sprintln("Firstname: <nil>")
	}
	if u.Lastname != nil {
		str += fmt.Sprintf("Lastname: %s\n", *u.Lastname)
	} else {
		str += fmt.Sprintln("Firstname: <nil>")
	}
	str += fmt.Sprintf("HashedPassword: %v\n", u.HashedPassword)
	str += fmt.Sprintf("CreatedAt: %v\n", u.CreatedAt)
	str += fmt.Sprintf("UpdatedAt: %v\n", u.UpdatedAt)
	str += fmt.Sprintf("DeletedAt: %v\n", u.DeletedAt)

	return str
}
