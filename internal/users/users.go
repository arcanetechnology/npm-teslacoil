package users

import (
	"log"

	"github.com/jinzhu/gorm"
	"golang.org/x/crypto/bcrypt"
)

// All is a GET request that returns all the users in the database
func All(d *gorm.DB) ([]User, error) {
	// Equivalent to SELECT * from users;
	queryResult := []User{}
	if err := d.Find(&queryResult).Error; err != nil {
		return queryResult, err
	}

	return queryResult, nil
}

// GetByID is a GET request that returns users that match the one specified in the body
func GetByID(d *gorm.DB, id uint64) (User, error) {
	queryResult := User{}
	if err := d.Where("id = ?", id).First(&queryResult).Error; err != nil {
		return queryResult, err
	}

	return queryResult, nil
}

// Create is a POST request and inserts all the users in the body into the database
func Create(d *gorm.DB, nu UserNew) (User, error) {

	user := User{
		Email:          nu.Email,
		HashedPassword: hashAndSalt(nu.Password),
	}

	if err := d.Create(&user).Error; err != nil {
		return user, err
	}
	return user, nil
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
