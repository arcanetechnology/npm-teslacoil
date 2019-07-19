package users

import (
	"github.com/jinzhu/gorm"
)

// GetAll is a GET request that returns all the users in the database
func GetAll(d *gorm.DB) ([]User, error) {
	// Equivalent to SELECT * from users;
	queryResult := []User{}
	if err := d.Find(&queryResult).Error; err != nil {
		return queryResult, err
	}

	return queryResult, nil
}

// GetUser is a GET request that returns users that match the one specified in the body
func GetUser(d *gorm.DB, id uint64) (User, error) {
	queryResult := User{}
	if err := d.Where("id = ?", id).First(&queryResult).Error; err != nil {
		return queryResult, err
	}

	return queryResult, nil
}

// CreateUser is a POST request and inserts all the users in the body into the database
func CreateUser(d *gorm.DB, nu UserNew) (User, error) {

	user := User{
		Balance:  nu.Balance,
		Password: nu.Password,
	}

	if err := d.Create(&user).Error; err != nil {
		return user, err
	}
	return user, nil
}
