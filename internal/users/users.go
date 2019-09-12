package users

import (
	"fmt"
	"strings"
	"time"

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
const UsersTable = "users"

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
	uQuery := fmt.Sprintf(`SELECT 
		id, email, balance, updated_at, first_name, last_name
		FROM %s WHERE id=$1 LIMIT 1`, UsersTable)

	if err := d.Get(&userResult, uQuery, id); err != nil {
		return User{}, errors.Wrapf(err, "GetByID(db, %d)", id)
	}

	return userResult, nil
}

// GetByEmail is a GET request that returns users that match the one specified
// in the body
func GetByEmail(d *db.DB, email string) (User, error) {
	userResult := User{}
	uQuery := fmt.Sprintf(`SELECT 
		id, email, balance, updated_at, first_name, last_name
		FROM %s WHERE email=$1 LIMIT 1`, UsersTable)

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
	uQuery := fmt.Sprintf(`SELECT 
		id, email, balance, hashed_password, updated_at, first_name, 
		last_name
		FROM %s WHERE email=$1 LIMIT 1`, UsersTable)

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

// Create is a POST request and inserts the user in the body into the database
func Create(d *db.DB, email, password string) (User, error) {
	hashedPassword, err := hashAndSalt(password)
	if err != nil {
		return User{}, err
	}
	user := User{
		Email:          email,
		HashedPassword: hashedPassword,
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
	var deferredError error = nil
	if cb.AmountSat <= 0 {
		return User{}, fmt.Errorf("amount cant be less than or equal to 0")
	}

	updateBalanceQuery := `UPDATE users
		SET balance = balance + $1
		WHERE id = $2
		RETURNING id, email, balance, updated_at, first_name, last_name`

	rows, err := tx.Query(updateBalanceQuery, cb.AmountSat, cb.UserID)
	if err != nil {
		return User{}, errors.Wrap(
			err,
			"IncreaseBalance(): could not construct user update",
		)
	}
	defer func() {
		deferredError = rows.Close()
	}()

	user := User{}
	if rows.Next() {
		if err = rows.Scan(
			&user.ID,
			&user.Email,
			&user.Balance,
			&user.UpdatedAt,
			&user.Firstname,
			&user.Lastname,
		); err != nil {
			return User{}, errors.Wrap(
				err, "Could not scan user returned from db")
		}
	}

	return user, deferredError
}

// DecreaseBalance decreases the balance of user id x by y satoshis
func DecreaseBalance(tx *sqlx.Tx, cb ChangeBalance) (User, error) {
	if cb.AmountSat <= 0 {
		return User{},
			fmt.Errorf("amount cant be less than or equal to 0")
	}

	updateBalanceQuery := `UPDATE users
		SET balance = balance - $1  
		WHERE id = $2 
		RETURNING id, email, balance, updated_at, first_name, last_name`

	rows, err := tx.Query(updateBalanceQuery, cb.AmountSat, cb.UserID)
	if err != nil {
		return User{}, errors.Wrap(
			err,
			"DecreaseBalance(): could not construct user update",
		)
	}

	user := User{}
	if rows.Next() {
		if err = rows.Scan(
			&user.ID,
			&user.Email,
			&user.Balance,
			&user.UpdatedAt,
			&user.Firstname,
			&user.Lastname,
		); err != nil {
			return User{}, errors.Wrap(
				err, "Could not scan user returned from db")
		}
	}
	if err = rows.Close(); err != nil {
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
	hash, err := bcrypt.GenerateFromPassword([]byte(pwd), 12)
	if err != nil {
		log.Error(err)
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
		VALUES (:email, 0, :hashed_password, :first_name, :last_name)
		RETURNING id, email, balance, updated_at, first_name, last_name`

	log.Debugf("Executing query for creating user (%s): %s", user, userCreateQuery)

	rows, err := tx.NamedQuery(userCreateQuery, user)
	if err != nil {
		return User{}, errors.Wrapf(
			err, "users.Create(db, %s, %s)",
			user.Email, string(user.HashedPassword))
	}

	userResp := User{}
	if rows.Next() {
		if err = rows.Scan(
			&userResp.ID,
			&userResp.Email,
			&userResp.Balance,
			&userResp.UpdatedAt,
			&userResp.Firstname,
			&userResp.Lastname); err != nil {
			return User{}, errors.Wrap(err, fmt.Sprintf("insertUser(tx, %v) failed", user))
		}
	}

	if err = rows.Close(); err != nil {
		return userResp, err
	}
	return userResp, nil
}

// UpdateUserOptions represents the different actions `UpdateUser` can perform.
type UpdateOptions struct {
	RemoveFirstName bool
	SetFirstName    bool
	NewFirstName    string

	RemoveLastName bool
	SetLastName    bool
	NewLastName    string

	UpdateEmail bool
	NewEmail    string
}

// Update the users email, first name and last name, depending on
// what options we get passed in
func (u User) Update(db *db.DB, opts UpdateOptions) (User, error) {
	var out User
	if u.ID == 0 {
		return out, errors.New("User ID cannot be 0!")
	}

	if opts.RemoveFirstName && opts.SetFirstName {
		return out, errors.New("cannot both set and remove first name")
	}

	if opts.RemoveLastName && opts.SetLastName {
		return out, errors.New("cannot both set and remove last name")
	}

	if opts.SetFirstName && opts.NewFirstName == "" {
		return out, errors.New("no new first name provided")
	}

	if opts.SetLastName && opts.NewLastName == "" {
		return out, errors.New("no new last name provided")
	}

	if opts.UpdateEmail && opts.NewEmail == "" {
		return out, errors.New("no new email provided")
	}

	// no action needed
	if !(opts.UpdateEmail ||
		opts.SetLastName || opts.SetFirstName ||
		opts.RemoveLastName || opts.RemoveFirstName) {
		return out, nil
	}

	updateQuery := `UPDATE users SET `
	updates := []string{}
	queryUser := User{
		ID: u.ID,
	}

	if opts.UpdateEmail {
		updates = append(updates, "email = :email")
		queryUser.Email = opts.NewEmail
	}
	if opts.SetFirstName {
		updates = append(updates, "first_name = :first_name")
		queryUser.Firstname = &opts.NewFirstName
	}
	if opts.RemoveFirstName {
		updates = append(updates, "first_name = NULL")
	}
	if opts.SetLastName {
		updates = append(updates, "last_name = :last_name")
		queryUser.Lastname = &opts.NewLastName
	}
	if opts.RemoveLastName {
		updates = append(updates, "last_name = NULL")
	}

	updateQuery += strings.Join(updates, ",")
	updateQuery += ` WHERE id = :id RETURNING id, email, first_name, last_name, balance`
	log.Debugf("Executing SQL for updating user: %s with opts %+v", updateQuery, opts)

	rows, err := db.NamedQuery(
		updateQuery,
		&queryUser,
	)

	if err != nil {
		return out, errors.Wrap(err, "could not update user")
	}

	if err = rows.Err(); err != nil {
		return out, err
	}

	if !rows.Next() {
		return out, errors.Errorf("did not find user with ID %d", u.ID)
	}

	if err = rows.Scan(
		&out.ID,
		&out.Email,
		&out.Firstname,
		&out.Lastname,
		&out.Balance,
	); err != nil {
		msg := fmt.Sprintf("updating user with ID %d failed", u.ID)
		return out, errors.Wrap(err, msg)
	}

	if err = rows.Close(); err != nil {
		return out, err
	}
	return out, nil

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
		str += fmt.Sprintf("Lastname: %d\n", u.Lastname)
	} else {
		str += fmt.Sprintln("Firstname: <nil>")
	}
	str += fmt.Sprintf("HashedPassword: %v\n", u.HashedPassword)
	str += fmt.Sprintf("CreatedAt: %v\n", u.CreatedAt)
	str += fmt.Sprintf("UpdatedAt: %v\n", u.UpdatedAt)
	str += fmt.Sprintf("DeletedAt: %v\n", u.DeletedAt)

	return str
}
