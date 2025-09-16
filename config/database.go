package config

import (
	"fmt"
	"log"
	"os"
	"abdo/models"
	
	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDatabase() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: No .env file found, using system environment variables")
	}
	
	// Get database configuration from environment variables
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "3306")
	dbUser := getEnv("DB_USER", "root")
	dbPassword := getEnv("DB_PASSWORD", "")
	dbName := getEnv("DB_NAME", "abdo")
	
	// Build connection string
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		dbUser, dbPassword, dbHost, dbPort, dbName)
	
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

// getEnv gets an environment variable with a fallback value
func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
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