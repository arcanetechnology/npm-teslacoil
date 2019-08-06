package users

import (
	"encoding/hex"
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
	ID             uint64     `db:"id"`
	Email          string     `db:"email"`
	Balance        int        `db:"balance"`
	HashedPassword []byte     `db:"hashed_password" json:"-"`
	CreatedAt      time.Time  `db:"created_at"`
	UpdatedAt      time.Time  `db:"updated_at"`
	DeletedAt      *time.Time `db:"deleted_at"`
}

// UserResponse is a database table
type UserResponse struct {
	ID        uint64    `db:"id"`
	Email     string    `db:"email"`
	Balance   int       `db:"balance"`
	UpdatedAt time.Time `db:"updated_at"`
}

// UsersTable is the tablename of users, as saved in the DB
const UsersTable = "users"

// All is a GET request that returns all the users in the database
func All(d *sqlx.DB) ([]User, error) {
	// Equivalent to SELECT * from users;
	queryResult := []User{}
	err := d.Select(&queryResult, fmt.Sprintf("SELECT * FROM %s", UsersTable))
	if err != nil {
		log.Error(err)
		return queryResult, err
	}

	log.Tracef("SELECT * from users received %v from DB", queryResult)

	return queryResult, nil
}

// GetByID is a GET request that returns users that match the one specified in the body
func GetByID(d *sqlx.DB, id uint) (UserResponse, error) {
	userResult := UserResponse{}
	uQuery := fmt.Sprintf(`SELECT id, email, balance, updated_at
		FROM %s WHERE id=$1 LIMIT 1`, UsersTable)

	if err := d.Get(&userResult, uQuery, id); err != nil {
		log.Error(err)
		return userResult, err
	}

	log.Tracef("SELECT * from users WHERE id=%d received %v from %v", userResult)

	return userResult, nil
}

// GetByCredentials retrieves a user from the database using the email and
// the salted/hashed password
func GetByCredentials(d *sqlx.DB, email string) (UserResponse, error) {
	userResult := UserResponse{}
	uQuery := fmt.Sprintf(`SELECT id, email, balance, updated_at
		FROM %s WHERE email=$1 LIMIT 1`, UsersTable)

	if err := d.Get(&userResult, uQuery, email); err != nil {
		log.Error(err)
		return userResult, err
	}

	log.Tracef("%s received user %v", uQuery, userResult)

	return userResult, nil
}

// Create is a POST request and inserts all the users in the body into the database
func Create(d *sqlx.DB, email, password string) (UserResponse, error) {
	hashedPassword, err := hashAndSalt(password)
	if err != nil {
		log.Error(err)
		return UserResponse{}, err
	}
	user := User{
		Email:          email,
		HashedPassword: hashedPassword,
	}
	userCreateQuery := fmt.Sprintf(`INSERT INTO %s 
		(email, balance, hashed_password)
		VALUES (:email, 0, :hashed_password)
		RETURNING id, email, balance, updated_at`, UsersTable)

	rows, err := d.NamedQuery(userCreateQuery, user)
	if err != nil {
		log.Error(err)
		return UserResponse{}, err
	}
	defer rows.Close()

	uResp := UserResponse{}
	if rows.Next() {
		if err = rows.Scan(&uResp.ID, &uResp.Email, &uResp.Balance, &uResp.UpdatedAt); err != nil {
			log.Error(err)
			return uResp, err
		}
	}

	log.Tracef("%s inserted %v", userCreateQuery, uResp)

	return uResp, nil
}

// UpdateUserBalance updates the users balance
func UpdateUserBalance(d *sqlx.DB, userID uint64) (UserResponse, error) {
	updateBalanceQuery := fmt.Sprintf(`UPDATE %s 
		SET balance = balance - :amount
		WHERE id = :user_id
		RETURNING id, balance`, UsersTable)

	user := UserResponse{}

	rows, err := d.NamedQuery(updateBalanceQuery, &user)
	if err != nil {
		log.Error(err)
		// TODO: This is probably not a healthy way to deal with an error here
		return UserResponse{}, errors.Wrap(err, "PayInvoice: Cold not construct user update")
	}
	if rows.Next() {
		if err = rows.Scan(
			&user.ID,
			&user.Balance,
		); err != nil {
			log.Error(err)
			return UserResponse{}, err
		}
	}
	log.Tracef("%s inserted %v", updateBalanceQuery, user)

	return user, nil
}

func hashAndSalt(pwd string) ([]byte, error) {
	// Use GenerateFromPassword to hash & salt pwd.
	// MinCost is just an integer constant provided by the bcrypt
	// package along with DefaultCost & MaxCost.
	// The cost can be any value you want provided it isn't lower
	// than the MinCost (4)
	hash, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.MinCost)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	log.Tracef("generated %s", hex.EncodeToString(hash))
	// GenerateFromPassword returns a byte slice so we need to
	// convert the bytes to a string and return it
	return hash, nil
}
