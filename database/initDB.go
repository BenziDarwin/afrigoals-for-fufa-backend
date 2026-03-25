package database

import (
	"log"

	"afrigoals.com/models"
	"golang.org/x/crypto/bcrypt"
)

func ptr(s string) *string {
	return &s
}

func InitDB() {

	adminUsername := getEnv("ADMIN_USERNAME", ptr("admin@example.com"))
	adminPassword := getEnv("ADMIN_PASSWORD", ptr("admin@123"))

	// Create default admin user
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash admin password: %v", err)
	}

	adminUser := models.User{
		Email:    adminUsername,
		Password: string(passwordHash),
	}

	if err := DB.FirstOrCreate(&adminUser, models.User{Email: adminUsername}).Error; err != nil {
		log.Fatalf("Failed to create admin user: %v", err)
	}

	log.Println("Database initialized with roles and admin user.")
}
