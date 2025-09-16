package services

import (
	"abdo/config"
	"abdo/models"
	"errors"
	"time"

	"gorm.io/gorm"
)

type OrderService struct {
	db *gorm.DB
}

func NewOrderService() *OrderService {
	return &OrderService{
		db: config.DB,
	}
}

// CreateOrder creates a new order
func (s *OrderService) CreateOrder(order *models.Order) error {
	// Verify user exists
	var user models.User
	if err := s.db.First(&user, order.UserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("user not found")
		}
		return err
	}

	return s.db.Create(order).Error
}

// GetOrderByID retrieves an order by ID with user details
func (s *OrderService) GetOrderByID(id uint) (*models.Order, error) {
	var order models.Order
	err := s.db.Preload("User").First(&order, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("order not found")
		}
		return nil, err
	}
	return &order, nil
}

// GetAllOrders retrieves all orders with user details
func (s *OrderService) GetAllOrders() ([]models.Order, error) {
	var orders []models.Order
	err := s.db.Preload("User").Find(&orders).Error
	return orders, err
}

// GetOrdersByUserID retrieves all orders for a specific user
func (s *OrderService) GetOrdersByUserID(userID uint) ([]models.Order, error) {
	var orders []models.Order
	err := s.db.Preload("User").Where("user_id = ?", userID).Find(&orders).Error
	return orders, err
}

// UpdateOrder updates an existing order
func (s *OrderService) UpdateOrder(id uint, updatedOrder *models.Order) (*models.Order, error) {
	var order models.Order

	// Check if order exists
	if err := s.db.First(&order, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("order not found")
		}
		return nil, err
	}

	// If user is being changed, verify new user exists
	if updatedOrder.UserID != order.UserID {
		var user models.User
		if err := s.db.First(&user, updatedOrder.UserID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("user not found")
			}
			return nil, err
		}
	}

	// Update fields
	order.OrderNumber = updatedOrder.OrderNumber
	order.Description = updatedOrder.Description
	order.Amount = updatedOrder.Amount
	order.Status = updatedOrder.Status
	order.OrderDate = updatedOrder.OrderDate
	order.ShipDate = updatedOrder.ShipDate
	order.UserID = updatedOrder.UserID

	// Save changes
	if err := s.db.Save(&order).Error; err != nil {
		return nil, err
	}

	// Return updated order with user
	return s.GetOrderByID(id)
}

// DeleteOrder soft deletes an order
func (s *OrderService) DeleteOrder(id uint) error {
	var order models.Order

	// Check if order exists
	if err := s.db.First(&order, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("order not found")
		}
		return err
	}

	// Delete order (soft delete)
	return s.db.Delete(&order).Error
}

// UpdateOrderStatus updates only the status of an order
func (s *OrderService) UpdateOrderStatus(id uint, status string) (*models.Order, error) {
	var order models.Order

	// Check if order exists
	if err := s.db.First(&order, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("order not found")
		}
		return nil, err
	}

	// Update status
	order.Status = status

	// If status is shipped, set ship date
	if status == "shipped" && order.ShipDate == nil {
		now := time.Now()
		order.ShipDate = &now
	}

	// Save changes
	if err := s.db.Save(&order).Error; err != nil {
		return nil, err
	}

	// Return updated order with user
	return s.GetOrderByID(id)
}

// GetOrdersByStatus retrieves orders by status
func (s *OrderService) GetOrdersByStatus(status string) ([]models.Order, error) {
	var orders []models.Order
	err := s.db.Preload("User").Where("status = ?", status).Find(&orders).Error
	return orders, err
}
