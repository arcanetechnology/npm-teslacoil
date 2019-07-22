package users

import (
	"github.com/jinzhu/gorm"
)

// UserNew contains all fields used while constructing a new user
type UserNew struct {
	Email    string `json:"email"`
	Password string `json:"password" binding:"required"`
}

// User is a database table
type User struct {
	// To read more about gorm.Model, follow this link
	// http://gorm.io/docs/conventions.html
	gorm.Model
	Email          string
	Balance        int
	HashedPassword []byte `json:"-"`
}
