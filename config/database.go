package config

import (
	"fmt"
	"log"
	"abdo/models"
	
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDatabase() {
	// Database connection string
	// Format: username:password@tcp(host:port)/database?charset=utf8mb4&parseTime=True&loc=Local
	dsn := "root:1234@tcp(localhost:3306)/abdo?charset=utf8mb4&parseTime=True&loc=Local"
	
	database, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	
	// Run auto-migration
	fmt.Println("Running database migrations...")
	err = AutoMigrate(database)
	if err != nil {
		log.Fatal("Failed to migrate database:", err)
	}
	
	DB = database
	fmt.Println("Database connection established successfully!")
}

// AutoMigrate handles all database migrations
func AutoMigrate(db *gorm.DB) error {
	// List all models that need migration
	modelsToMigrate := []interface{}{
		&models.User{},
		&models.Order{},
	}
	
	for _, model := range modelsToMigrate {
		fmt.Printf("Migrating %T...\n", model)
		if err := db.AutoMigrate(model); err != nil {
			return fmt.Errorf("failed to migrate %T: %v", model, err)
		}
	}
	
	fmt.Println("All migrations completed successfully!")
	return nil
}