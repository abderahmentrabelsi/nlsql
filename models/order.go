package models

import (
	"time"

	"gorm.io/gorm"
)

type Order struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`

	// Order details
	OrderNumber string  `json:"order_number" gorm:"size:100;uniqueIndex;not null" binding:"required"`
	Description string  `json:"description" gorm:"type:text"`
	Amount      float64 `json:"amount" gorm:"not null" binding:"required,min=0"`
	Status      string  `json:"status" gorm:"default:'pending'" binding:"oneof=pending processing shipped delivered cancelled"`

	// Order dates
	OrderDate time.Time  `json:"order_date" gorm:"not null" binding:"required"`
	ShipDate  *time.Time `json:"ship_date,omitempty"`

	// Foreign key relationship with User
	UserID uint `json:"user_id" gorm:"not null" binding:"required"`
	User   User `json:"user,omitempty" gorm:"foreignKey:UserID" binding:"-"`
}
