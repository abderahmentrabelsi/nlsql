package services

import (
	"abdo/config"
	"abdo/models"
	"errors"

	"gorm.io/gorm"
)

type UserService struct {
	db *gorm.DB
}

func NewUserService() *UserService {
	return &UserService{
		db: config.DB,
	}
}

// CreateUser creates a new user
func (s *UserService) CreateUser(user *models.User) error {
	return s.db.Create(user).Error
}

// GetUserByID retrieves a user by ID with orders
func (s *UserService) GetUserByID(id uint) (*models.User, error) {
	var user models.User
	err := s.db.Preload("Orders").First(&user, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

// GetAllUsers retrieves all users with their orders
func (s *UserService) GetAllUsers() ([]models.User, error) {
	var users []models.User
	err := s.db.Preload("Orders").Find(&users).Error
	return users, err
}

// UpdateUser updates an existing user
func (s *UserService) UpdateUser(id uint, updatedUser *models.User) (*models.User, error) {
	var user models.User

	// Check if user exists
	if err := s.db.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	// Update fields
	user.Name = updatedUser.Name
	user.Email = updatedUser.Email
	user.Phone = updatedUser.Phone
	user.Address = updatedUser.Address

	// Save changes
	if err := s.db.Save(&user).Error; err != nil {
		return nil, err
	}

	// Return updated user with orders
	return s.GetUserByID(id)
}

// DeleteUser soft deletes a user and handles related orders
func (s *UserService) DeleteUser(id uint) error {
	var user models.User

	// Check if user exists
	if err := s.db.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("user not found")
		}
		return err
	}

	// Check if user has orders
	var orderCount int64
	s.db.Model(&models.Order{}).Where("user_id = ?", id).Count(&orderCount)

	if orderCount > 0 {
		return errors.New("cannot delete user with existing orders")
	}

	// Delete user (soft delete)
	return s.db.Delete(&user).Error
}

// GetUserByEmail retrieves a user by email
func (s *UserService) GetUserByEmail(email string) (*models.User, error) {
	var user models.User
	err := s.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}
