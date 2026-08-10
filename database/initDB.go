package database

import (
	"errors"
	"log"
	"strings"

	"afrigoals.com/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const minAdminPasswordLength = 12

// InitDB seeds the platform administrator account.
//
// The account is identified by ADMIN_EMAIL (ADMIN_USERNAME is still accepted for
// older configurations). Both the email and the password are required: booting
// with an unusable administrator is worse than refusing to boot at all, because
// there is no other way to create the first afrigoals_admin.
func InitDB() {
	adminEmail := strings.TrimSpace(getEnv("ADMIN_EMAIL", nil))
	if adminEmail == "" {
		adminEmail = strings.TrimSpace(getEnv("ADMIN_USERNAME", nil))
	}
	adminPassword := getEnv("ADMIN_PASSWORD", nil)

	if adminEmail == "" || adminPassword == "" {
		log.Fatal("ADMIN_EMAIL and ADMIN_PASSWORD are required")
	}
	if !strings.Contains(adminEmail, "@") {
		log.Fatalf("ADMIN_EMAIL must be an email address, got %q", adminEmail)
	}
	if len(adminPassword) < minAdminPasswordLength {
		log.Fatalf("ADMIN_PASSWORD must be at least %d characters", minAdminPasswordLength)
	}

	var existing models.User
	err := DB.Where("email = ?", adminEmail).First(&existing).Error

	switch {
	case err == nil:
		// Earlier versions of this function created the admin without a role,
		// leaving an account that fails every role check. Repair it in place.
		if existing.Role != models.AfrigoalsAdmin {
			if err := DB.Model(&existing).Update("role", models.AfrigoalsAdmin).Error; err != nil {
				log.Fatalf("Failed to set admin role on %s: %v", adminEmail, err)
			}
			log.Printf("Repaired role for existing admin user %s", adminEmail)
		}
		return

	case !errors.Is(err, gorm.ErrRecordNotFound):
		log.Fatalf("Failed to look up admin user: %v", err)
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash admin password: %v", err)
	}

	admin := models.User{
		Email:    adminEmail,
		Password: string(passwordHash),
		Role:     models.AfrigoalsAdmin,
	}

	if err := DB.Create(&admin).Error; err != nil {
		log.Fatalf("Failed to create admin user: %v", err)
	}

	log.Printf("Created platform admin %s", adminEmail)
}
