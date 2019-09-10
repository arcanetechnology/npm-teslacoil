package users

import (
	"fmt"
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
	uQuery := fmt.Sprintf(`SELECT id, email, balance, updated_at
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
	uQuery := fmt.Sprintf(`SELECT id, email, balance, updated_at
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
	uQuery := fmt.Sprintf(`SELECT id, email, balance, hashed_password, updated_at
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
		return User{}, err
	}
	err = tx.Commit()
	if err != nil {
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
		WHERE id = $2
		RETURNING id, email, balance, updated_at`

	rows, err := tx.Query(updateBalanceQuery, cb.AmountSat, cb.UserID)
	if err != nil {
		return User{}, errors.Wrap(
			err,
			"IncreaseBalance(): could not construct user update",
		)
	}
	defer rows.Close()

	user := User{}
	if rows.Next() {
		if err = rows.Scan(
			&user.ID,
			&user.Email,
			&user.Balance,
			&user.UpdatedAt,
		); err != nil {
			return User{}, errors.Wrap(
				err, "Could not scan user returned from db")
		}
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
		WHERE id = $2
		RETURNING id, email, balance, updated_at`

	rows, err := tx.Query(updateBalanceQuery, cb.AmountSat, cb.UserID)
	if err != nil {
		return User{}, errors.Wrap(
			err,
			"DecreaseBalance(): could not construct user update",
		)
	}
	defer rows.Close()

	user := User{}
	if rows.Next() {
		if err = rows.Scan(
			&user.ID,
			&user.Email,
			&user.Balance,
			&user.UpdatedAt,
		); err != nil {
			return User{}, errors.Wrap(
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
		(email, balance, hashed_password)
		VALUES (:email, 0, :hashed_password)
		RETURNING id, email, balance, updated_at`

	rows, err := tx.NamedQuery(userCreateQuery, user)
	if err != nil {
		return User{}, errors.Wrapf(
			err, "users.Create(db, %s, %s)",
			user.Email, string(user.HashedPassword))
	}

	userResp := User{}
	if rows.Next() {
		if err = rows.Scan(&userResp.ID,
			&userResp.Email,
			&userResp.Balance,
			&userResp.UpdatedAt); err != nil {
			return User{}, errors.Wrap(err, fmt.Sprintf("insertUser(tx, %v) failed", user))
		}
	}

	rows.Close()
	return userResp, nil
}

// UpdateEmail updates the users email
func (u User) UpdateEmail(db *db.DB, email string) (User, error) {
	var out User
	if u.ID == 0 {
		return out, errors.New("User ID cannot be 0!")
	}

	queryUser := User{
		ID:    u.ID,
		Email: email,
	}

	rows, err := db.NamedQuery(
		`UPDATE users SET email = :email WHERE id = :id RETURNING id, email`,
		queryUser,
	)

	if err != nil {
		return out, errors.Wrap(err, "could not update email")
	}

	if err = rows.Err(); err != nil {
		return out, err
	}

	if !rows.Next() {
		return out, errors.Errorf("did not find user with ID %d", u.ID)
	}

	if err = rows.Scan(&out.ID,
		&out.Email); err != nil {
		msg := fmt.Sprintf("updating user with ID %d failed", u.ID)
		return out, errors.Wrap(err, msg)
	}

	rows.Close()
	return out, nil

}

func (u User) String() string {
	str := fmt.Sprintf("ID: %d\n", u.ID)
	str += fmt.Sprintf("Email: %s\n", u.Email)
	str += fmt.Sprintf("Balance: %d\n", u.Balance)
	str += fmt.Sprintf("HashedPassword: %v\n", u.HashedPassword)
	str += fmt.Sprintf("CreatedAt: %v\n", u.CreatedAt)
	str += fmt.Sprintf("UpdatedAt: %v\n", u.UpdatedAt)
	str += fmt.Sprintf("DeletedAt: %v\n", u.DeletedAt)

	return str
}
