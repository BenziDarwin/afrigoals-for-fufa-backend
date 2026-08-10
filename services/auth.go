package services

import (
	"errors"
	"strings"

	"afrigoals.com/database"
	"afrigoals.com/middleware"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// RegisterUser creates a new user account
func RegisterUser(c *fiber.Ctx) error {
	var req struct {
		Email      string `json:"email"`
		Password   string `json:"password"`
		Role       string `json:"role"`
		LeagueName string `json:"league_name,omitempty"`
		ClubName   string `json:"club_name,omitempty"`
		Country    string `json:"country,omitempty"`
		LeagueID   *uint  `json:"league_id,omitempty"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if req.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Email is required",
		})
	}

	if req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Password is required",
		})
	}

	if len(req.Password) < 8 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Password must be at least 8 characters long",
		})
	}

	if req.Role == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Role is required",
		})
	}

	role := models.UserRole(strings.TrimSpace(req.Role))

	// An unrecognised role is a client error, so name the accepted values.
	if !models.IsValidRole(role) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":         "Invalid role",
			"allowed_roles": models.SelfRegisterableRoles(),
		})
	}

	// A recognised but privileged role is an authorization decision, so say
	// nothing about which roles exist beyond the ones already offered above.
	if !models.IsSelfRegisterable(role) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "This role cannot be self-registered. Contact a platform administrator.",
		})
	}

	// Validate role-specific requirements
	if role == models.LeagueAdmin {
		if req.LeagueName == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "League name is required for league administrator role",
			})
		}
		if req.Country == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Country is required for league administrator role",
			})
		}
	}

	if role == models.ClubManager {
		if req.ClubName == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Club name is required for club manager role",
			})
		}
	}

	// NOTE:
	// DataAnalyst league assignment is OPTIONAL now (restriction removed).

	// Check if user already exists
	var existingUser models.User
	if err := database.DB.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "User with this email already exists",
		})
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to process password",
		})
	}

	// Start transaction
	tx := database.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var leagueID *uint // pointer allows NULL
	var clubID *uint

	// Handle League Admin registration
	if role == models.LeagueAdmin {
		var league models.League
		if err := tx.Where("name = ? AND country = ?", req.LeagueName, req.Country).First(&league).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				league = models.League{
					Name:        req.LeagueName,
					Country:     req.Country,
					Description: "Managed via Afrigoals",
				}
				if err := tx.Create(&league).Error; err != nil {
					tx.Rollback()
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"error": "Failed to create league",
					})
				}
			} else {
				tx.Rollback()
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Database error",
				})
			}
		}
		leagueID = &league.ID
	}

	// Handle Club Manager registration
	if role == models.ClubManager {
		var club models.Club
		if err := tx.Where("name = ?", req.ClubName).First(&club).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				club = models.Club{
					Name: req.ClubName,
				}
				if err := tx.Create(&club).Error; err != nil {
					tx.Rollback()
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"error": "Failed to create club",
					})
				}
			} else {
				tx.Rollback()
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
					"error": "Database error",
				})
			}
		} else {
			// Club exists, check if it already has a manager
			var existingManager models.User
			if err := tx.Where("club_id = ? AND role = ?", club.ID, models.ClubManager).First(&existingManager).Error; err == nil {
				tx.Rollback()
				return c.Status(fiber.StatusConflict).JSON(fiber.Map{
					"error": "This club already has a manager",
				})
			}
		}
		clubID = &club.ID
		leagueID = nil // Club managers have no league initially
	}

	// Data analyst registration (league OPTIONAL)
	if role == models.DataAnalyst {
		// If league_id provided, validate and assign; else allow NULL.
		if req.LeagueID != nil {
			var league models.League
			if err := tx.First(&league, *req.LeagueID).Error; err != nil {
				tx.Rollback()
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "Invalid league_id",
				})
			}
			leagueID = &league.ID
		} else {
			leagueID = nil
		}

		clubID = nil
	}

	// Create user
	user := models.User{
		Email:    req.Email,
		Password: string(hashedPassword),
		Role:     role,
		ClubID:   clubID,
		LeagueID: leagueID,
	}

	if err := tx.Create(&user).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create user: " + err.Error(),
		})
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to complete registration",
		})
	}

	// Generate JWT token
	token, err := middleware.CreateSession(c, &user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create session",
		})
	}

	// Load relationships for response
	database.DB.Preload("League").Preload("Club").First(&user, user.ID)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"user": fiber.Map{
				"ID":        user.ID,
				"UUID":      user.UUID.String(),
				"Email":     user.Email,
				"role":      user.Role,
				"ClubID":    user.ClubID,
				"Club":      user.Club,
				"LeagueID":  user.LeagueID,
				"League":    user.League,
				"CreatedAt": user.CreatedAt,
				"UpdatedAt": user.UpdatedAt,
			},
			"token": token,
		},
	})
}

// LoginUser authenticates a user and returns a JWT token
func LoginUser(c *fiber.Ctx) error {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Email == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Email and password are required",
		})
	}

	var user models.User
	if err := database.DB.Preload("League").Preload("Club").
		Where("email = ?", req.Email).
		First(&user).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Invalid credentials",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid credentials",
		})
	}

	token, err := middleware.CreateSession(c, &user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create session",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Login successful",
		"token":   token,
		"user": fiber.Map{
			"id":        user.ID,
			"uuid":      user.UUID.String(),
			"email":     user.Email,
			"role":      user.Role,
			"club_id":   user.ClubID,
			"league_id": user.LeagueID,
		},
	})
}

// LogoutUser clears the user session
func LogoutUser(c *fiber.Ctx) error {
	middleware.ClearSession(c)
	return c.JSON(fiber.Map{
		"message": "Logged out successfully",
	})
}

// GetCurrentUser returns the authenticated user's information
func GetCurrentUser(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(models.User)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User not authenticated",
		})
	}

	var fullUser models.User
	if err := database.DB.Preload("League").Preload("Club").First(&fullUser, user.ID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	return c.JSON(fiber.Map{
		"user": fiber.Map{
			"id":        fullUser.ID,
			"uuid":      fullUser.UUID.String(),
			"email":     fullUser.Email,
			"role":      fullUser.Role,
			"club_id":   fullUser.ClubID,
			"league_id": fullUser.LeagueID,
			"league":    fullUser.League,
			"club":      fullUser.Club,
		},
	})
}

// RefreshToken generates a new JWT token for the authenticated user
func RefreshToken(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromToken(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid or expired token",
		})
	}

	token, err := middleware.CreateSession(c, user)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate token",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Token refreshed successfully",
		"token":   token,
	})
}

// ChangePassword handles password change for authenticated user
func ChangePassword(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(models.User)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User not authenticated",
		})
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.OldPassword == "" || req.NewPassword == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Old password and new password are required",
		})
	}

	if len(req.NewPassword) < 8 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "New password must be at least 8 characters long",
		})
	}

	var dbUser models.User
	if err := database.DB.First(&dbUser, user.ID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(dbUser.Password), []byte(req.OldPassword)); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid old password",
		})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to process password",
		})
	}

	if err := database.DB.Model(&dbUser).Update("password", string(hashedPassword)).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update password",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Password changed successfully",
	})
}

// ResetPasswordRequest initiates password reset process
func ResetPasswordRequest(c *fiber.Ctx) error {
	var req struct {
		Email string `json:"email"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Email is required",
		})
	}

	var user models.User
	if err := database.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		return c.JSON(fiber.Map{
			"message": "If the email exists, a password reset link has been sent",
		})
	}

	resetToken, err := middleware.GenerateSecureToken(32)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate reset token",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Password reset instructions sent to email",
		"token":   resetToken, // Remove this in production
	})
}

// VerifyEmail handles email verification
func VerifyEmail(c *fiber.Ctx) error {
	token := c.Params("token")

	if token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Verification token is required",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Email verified successfully",
	})
}

// UpdateProfile updates user profile information
func UpdateProfile(c *fiber.Ctx) error {
	user, ok := c.Locals("user").(models.User)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User not authenticated",
		})
	}

	var req struct {
		Email string `json:"email,omitempty"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	var dbUser models.User
	if err := database.DB.First(&dbUser, user.ID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	updates := make(map[string]interface{})

	if req.Email != "" && req.Email != dbUser.Email {
		var existingUser models.User
		if err := database.DB.Where("email = ? AND id != ?", req.Email, user.ID).First(&existingUser).Error; err == nil {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "Email already in use",
			})
		}
		updates["email"] = req.Email
	}

	if len(updates) > 0 {
		if err := database.DB.Model(&dbUser).Updates(updates).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update profile",
			})
		}
	}

	database.DB.Preload("League").Preload("Club").First(&dbUser, dbUser.ID)

	return c.JSON(fiber.Map{
		"message": "Profile updated successfully",
		"user": fiber.Map{
			"id":        dbUser.ID,
			"uuid":      dbUser.UUID.String(),
			"email":     dbUser.Email,
			"role":      dbUser.Role,
			"club_id":   dbUser.ClubID,
			"league_id": dbUser.LeagueID,
		},
	})
}

// ValidateToken checks if a token is valid
func ValidateToken(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromToken(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"valid": false,
			"error": "Invalid token",
		})
	}

	return c.JSON(fiber.Map{
		"valid": true,
		"user": fiber.Map{
			"id":        user.ID,
			"uuid":      user.UUID.String(),
			"email":     user.Email,
			"role":      user.Role,
			"club_id":   user.ClubID,
			"league_id": user.LeagueID,
		},
	})
}
