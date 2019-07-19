package users

import (
	"github.com/jinzhu/gorm"
)

// UserNew contains all fields used while constructing a new user
type UserNew struct {
	Balance  int    `json:"balance" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// User is a database table
type User struct {
	// To read more about gorm.Model, follow this link
	// http://gorm.io/docs/conventions.html
	gorm.Model
	Balance  int
	Password string `json:"-"`
}
