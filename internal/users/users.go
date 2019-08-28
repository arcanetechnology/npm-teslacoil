package users

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

// UserNew contains all fields used while constructing a new user
type UserNew struct {
	Email    string `json:"email"`
	Password string `json:"password" binding:"required"`
}

// User is a database table
type User struct {
	ID             uint       `db:"id"`
	Email          string     `db:"email"`
	Balance        int        `db:"balance"`
	HashedPassword []byte     `db:"hashed_password" json:"-"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
	DeletedAt      *time.Time `db:"deleted_at"`
}

// UserResponse is a database table
type UserResponse struct {
	ID             uint      `db:"id"`
	Email          string    `db:"email"`
	Balance        int       `db:"balance"`
	HashedPassword []byte    `db:"hashed_password"`
	UpdatedAt      time.Time `db:"updated_at"`
}

// ChangeBalance is the type for increasing or decreasing the balance
type ChangeBalance struct {
	AmountSat int64 `db:"amountSat"`
	UserID    uint  `db:"userID"`
}

// UsersTable is the tablename of users, as saved in the DB
const UsersTable = "users"

// All is a GET request that returns all the users in the database
// TODO: This endpoint should be restricted to the admin
func All(d *sqlx.DB) ([]User, error) {
	// Equivalent to SELECT * from users;
	queryResult := []User{}
	err := d.Select(&queryResult, fmt.Sprintf("SELECT * FROM %s", UsersTable))
	if err != nil {
		return queryResult, err
	}

	// log.Tracef("SELECT * from users received %v from DB", queryResult)

	return queryResult, nil
}

// GetByID is a GET request that returns users that match the one specified
// in the body
func GetByID(d *sqlx.DB, id uint) (UserResponse, error) {
	userResult := UserResponse{}
	uQuery := fmt.Sprintf(`SELECT id, email, balance, updated_at
		FROM %s WHERE id=$1 LIMIT 1`, UsersTable)

	if err := d.Get(&userResult, uQuery, id); err != nil {
		return UserResponse{}, errors.Wrapf(err, "GetByID(db, %d)", id)
	}

	return userResult, nil
}

// GetByEmail is a GET request that returns users that match the one specified
// in the body
func GetByEmail(d *sqlx.DB, email string) (UserResponse, error) {
	userResult := UserResponse{}
	uQuery := fmt.Sprintf(`SELECT id, email, balance, updated_at
		FROM %s WHERE email=$1 LIMIT 1`, UsersTable)

	if err := d.Get(&userResult, uQuery, email); err != nil {
		return UserResponse{}, errors.Wrapf(err, "GetByEmail(db, %s)", email)
	}

	return userResult, nil
}

// GetByCredentials retrieves a user from the database using the email and
// the salted/hashed password
func GetByCredentials(d *sqlx.DB, email string, password string) (
	UserResponse, error) {

	userResult := UserResponse{}
	uQuery := fmt.Sprintf(`SELECT id, email, balance, hashed_password, updated_at
		FROM %s WHERE email=$1 LIMIT 1`, UsersTable)

	if err := d.Get(&userResult, uQuery, email); err != nil {
		return UserResponse{}, errors.Wrapf(
			err, "GetByCredentials(db, %s, **password_not_logged**)", email)
	}

	err := bcrypt.CompareHashAndPassword(
		userResult.HashedPassword, []byte(password))
	if err != nil {
		return UserResponse{}, errors.Wrap(err, "password authentication failed")
	}

	// log.Tracef("%s received user %v", uQuery, userResult)

	return userResult, nil
}

// Create is a POST request and inserts the user in the body into the database
func Create(d *sqlx.DB, email, password string) (UserResponse, error) {
	hashedPassword, err := hashAndSalt(password)
	if err != nil {
		return UserResponse{}, err
	}
	user := User{
		Email:          email,
		HashedPassword: hashedPassword,
	}

	tx := d.MustBegin()
	userResp, err := insertUser(tx, user)
	if err != nil {
		return UserResponse{}, err
	}
	tx.Commit()

	return userResp, nil
}

// IncreaseBalance increases the balance of user id x by y satoshis
func IncreaseBalance(tx *sqlx.Tx, cb ChangeBalance) (UserResponse, error) {
	if cb.AmountSat <= 0 {
		return UserResponse{}, errors.New("amount cant be less than or equal to 0")
	}

	updateBalanceQuery := `UPDATE users
		SET balance = balance + $1
		WHERE id = $2
		RETURNING id, email, balance, updated_at`

	rows, err := tx.Query(updateBalanceQuery, cb.AmountSat, cb.UserID)
	if err != nil {
		return UserResponse{}, errors.Wrap(
			err,
			"UpdateUserBalance(): could not construct user update",
		)
	}
	defer rows.Close()

	user := UserResponse{}
	if rows.Next() {
		if err = rows.Scan(
			&user.ID,
			&user.Email,
			&user.Balance,
			&user.UpdatedAt,
		); err != nil {
			return UserResponse{}, errors.Wrap(
				err, "Could not scan user returned from db")
		}
	}

	return user, nil
}

// DecreaseBalance decreases the balance of user id x by y satoshis
func DecreaseBalance(tx *sqlx.Tx, cb ChangeBalance) (UserResponse, error) {
	if cb.AmountSat <= 0 {
		return UserResponse{}, errors.New("amount cant be less than or equal to 0")
	}

	updateBalanceQuery := `UPDATE users
		SET balance = balance - $1
		WHERE id = $2
		RETURNING id, email, balance, updated_at`

	rows, err := tx.Query(updateBalanceQuery, cb.AmountSat, cb.UserID)
	if err != nil {
		return UserResponse{}, errors.Wrap(
			err,
			"UpdateUserBalance(): could not construct user update",
		)
	}
	defer rows.Close()

	user := UserResponse{}
	if rows.Next() {
		if err = rows.Scan(
			&user.ID,
			&user.Email,
			&user.Balance,
			&user.UpdatedAt,
		); err != nil {
			return UserResponse{}, errors.Wrap(
				err, "Could not scan user returned from db")
		}
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
		// log.Error(err)
		return nil, err
	}

	// bcrypt returns a base64 encoded hash, therefore string(hash) works for
	// converting the password to a readable format
	// log.Tracef("generated password %s", string(hash))

	return hash, nil
}

func insertUser(tx *sqlx.Tx, user User) (UserResponse, error) {
	userCreateQuery := `INSERT INTO users 
		(email, balance, hashed_password)
		VALUES (:email, 0, :hashed_password)
		RETURNING id, email, balance, updated_at`

	rows, err := tx.NamedQuery(userCreateQuery, user)
	if err != nil {
		return UserResponse{}, errors.Wrapf(
			err, "users.Create(db, %s, %s)",
			user.Email, string(user.HashedPassword))
	}

	userResp := UserResponse{}
	if rows.Next() {
		if err = rows.Scan(&userResp.ID,
			&userResp.Email,
			&userResp.Balance,
			&userResp.UpdatedAt); err != nil {
			return UserResponse{}, errors.Wrap(err, "users.Create- rows.Scan() failed")
		}
	}

	rows.Close()
	return userResp, nil
}
