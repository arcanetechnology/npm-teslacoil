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

// GetByEmail is a GET request that returns users that match the one specified in the body
func GetByEmail(d *sqlx.DB, email string) (*UserResponse, error) {
	userResult := UserResponse{}
	uQuery := fmt.Sprintf(`SELECT id, email, balance, updated_at
		FROM %s WHERE email=$1 LIMIT 1`, UsersTable)

	if err := d.Get(&userResult, uQuery, email); err != nil {
		return nil, errors.Wrapf(err, "GetByEmail(db, %s)", email)
	}

	// log.Tracef("%s returned %v", uQuery, userResult)

	return &userResult, nil
}

// GetByCredentials retrieves a user from the database using the email and
// the salted/hashed password
func GetByCredentials(d *sqlx.DB, email string, password string) (*UserResponse, error) {
	userResult := UserResponse{}
	uQuery := fmt.Sprintf(`SELECT id, email, balance, hashed_password, updated_at
		FROM %s WHERE email=$1 LIMIT 1`, UsersTable)

	if err := d.Get(&userResult, uQuery, email); err != nil {
		return nil, errors.Wrapf(err, "GetByCredentials(db, %s, **password_not_logged**)", email)
	}

	err := bcrypt.CompareHashAndPassword(userResult.HashedPassword, []byte(password))
	if err != nil {
		return nil, errors.Wrap(err, "password authentication failed")
	}

	// log.Tracef("%s received user %v", uQuery, userResult)

	return &userResult, nil
}

// Create is a POST request and inserts the user in the body into the database
func Create(d *sqlx.DB, email, password string) (*UserResponse, error) {
	hashedPassword, err := hashAndSalt(password)
	if err != nil {
		return nil, err
	}
	user := User{
		Email:          email,
		HashedPassword: hashedPassword,
	}

	userCreateQuery := `INSERT INTO users 
		(email, balance, hashed_password)
		VALUES (:email, 0, :hashed_password)
		RETURNING id, email, balance, updated_at`

	rows, err := d.NamedQuery(userCreateQuery, user)
	if err != nil {
		return nil, errors.Wrapf(err, "users.Create(db, %s, %s)", email, string(hashedPassword))
	}
	defer rows.Close()

	uResp := UserResponse{}
	if rows.Next() {
		if err = rows.Scan(&uResp.ID, &uResp.Email, &uResp.Balance, &uResp.UpdatedAt); err != nil {
			return nil, errors.Wrap(err, "users.Create- rows.Scan() failed")
		}
	}

	// log.Tracef("%s inserted %v", userCreateQuery, uResp)

	return &uResp, nil
}

// UpdateUserBalance updates the users balance
func UpdateUserBalance(d *sqlx.DB, userID uint, amountSat int64) (*UserResponse, error) {
	if amountSat == 0 {
		return nil, errors.New(
			"No point in updating users balance with 0 satoshi")
	}

	type UpdateBalanceQuery struct {
		UserID    uint  `db:"user_id"`
		AmountSat int64 `db:"amount_sat"`
	}

	updateBalanceQuery := fmt.Sprintf(`UPDATE %s 
		SET balance = balance + :amount_sat
		WHERE id = :user_id
		RETURNING id, email, balance`, UsersTable)

	rows, err := d.NamedQuery(updateBalanceQuery, &UpdateBalanceQuery{
		UserID:    userID,
		AmountSat: amountSat,
	})
	if err != nil {
		// log.Error(err)
		// TODO: This is probably not a healthy way to deal with an error here
		return nil, errors.Wrap(
			err, "UpdateUserBalance(): could not construct user update")
	}

	var user UserResponse

	if rows.Next() {
		if err = rows.Scan(
			&user.ID,
			&user.Email,
			&user.Balance,
		); err != nil {
			// log.Error(err)
			return nil, err
		}
	}
	// log.Tracef("%s inserted %v", updateBalanceQuery, user)

	return &user, nil
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
