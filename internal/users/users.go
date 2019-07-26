package users

import (
	"log"

	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

// All is a GET request that returns all the users in the database
func All(d *sqlx.DB) ([]User, error) {
	// Equivalent to SELECT * from users;
	queryResult := []User{}
	if err := d.Select(&queryResult, "SELECT * FROM users"); err != nil {
		return queryResult, err
	}

	return queryResult, nil
}

// GetByID is a GET request that returns users that match the one specified in the body
func GetByID(d *sqlx.DB, id uint) (UserResponse, error) {
	userResult := UserResponse{}
	uQuery := `SELECT id, email, balance FROM users WHERE id=$1 LIMIT 1`

	if err := d.Get(&userResult, uQuery, id); err != nil {
		return userResult, err
	}

	return userResult, nil
}

// Create is a POST request and inserts all the users in the body into the database
func Create(d *sqlx.DB, nu UserNew) (UserResponse, error) {
	uResp := UserResponse{}

	user := User{
		Email:          nu.Email,
		HashedPassword: hashAndSalt(nu.Password),
	}
	userCreateQuery := `INSERT INTO users 
		(email, balance, hashed_password)
		VALUES (:email, 0, :hashed_password)
		RETURNING id, email, balance`

	rows, err := d.NamedQuery(userCreateQuery, user)
	if err != nil {
		return uResp, err
	}
	defer rows.Close()
	if rows.Next() {
		if err = rows.Scan(&uResp.ID, &uResp.Email, &uResp.Balance); err != nil {
			return uResp, err
		}
	}

	return uResp, nil
}

func hashAndSalt(pwd string) []byte {

	// Use GenerateFromPassword to hash & salt pwd.
	// MinCost is just an integer constant provided by the bcrypt
	// package along with DefaultCost & MaxCost.
	// The cost can be any value you want provided it isn't lower
	// than the MinCost (4)
	hash, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.MinCost)
	if err != nil {
		log.Println(err)
	}
	// GenerateFromPassword returns a byte slice so we need to
	// convert the bytes to a string and return it
	return hash
}
