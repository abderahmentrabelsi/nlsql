package models

import (
	"time"
	"gorm.io/gorm"
)

type User struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
	
	Name     string `json:"name" gorm:"not null" binding:"required"`
	Email    string `json:"email" gorm:"size:255;uniqueIndex;not null" binding:"required,email"`
	Phone    string `json:"phone" gorm:"size:20"`
	Address  string `json:"address" gorm:"type:text"`
	
	// Relationship with orders
	Orders []Order `json:"orders" gorm:"foreignKey:UserID"`
}