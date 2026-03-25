package services

import (
	"errors"
	"strconv"

	"afrigoals.com/database"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// GetUserByID retrieves a user by their ID
func GetUserByID(c *fiber.Ctx) error {
	id := c.Params("id")

	userID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID",
		})
	}

	var user models.User
	if err := database.DB.Preload("League").Preload("Club").First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	return c.JSON(fiber.Map{
		"user": fiber.Map{
			"id":        user.ID,
			"uuid":      user.UUID.String(),
			"email":     user.Email,
			"role":      user.Role,
			"club_id":   user.ClubID,
			"league_id": user.LeagueID,
			"league":    user.League,
			"club":      user.Club,
		},
	})
}

// GetUserByUUID retrieves a user by their UUID
func GetUserByUUID(c *fiber.Ctx) error {
	uuid := c.Params("uuid")

	var user models.User
	if err := database.DB.Preload("League").Preload("Club").Where("uuid = ?", uuid).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	return c.JSON(fiber.Map{
		"user": fiber.Map{
			"id":        user.ID,
			"uuid":      user.UUID.String(),
			"email":     user.Email,
			"role":      user.Role,
			"club_id":   user.ClubID,
			"league_id": user.LeagueID,
			"league":    user.League,
			"club":      user.Club,
		},
	})
}

// ListUsers retrieves all users with pagination and filtering
func ListUsers(c *fiber.Ctx) error {
	// Parse pagination parameters
	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "10"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	offset := (page - 1) * perPage

	// Build query
	query := database.DB.Model(&models.User{})

	// Apply filters
	if role := c.Query("role"); role != "" {
		query = query.Where("role = ?", role)
	}
	if leagueID := c.Query("league_id"); leagueID != "" {
		query = query.Where("league_id = ?", leagueID)
	}
	if clubID := c.Query("club_id"); clubID != "" {
		query = query.Where("club_id = ?", clubID)
	}
	if email := c.Query("email"); email != "" {
		query = query.Where("email LIKE ?", "%"+email+"%")
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to count users",
		})
	}

	// Get paginated results
	var users []models.User
	if err := query.Preload("League").Preload("Club").
		Limit(perPage).Offset(offset).
		Order("created_at DESC").
		Find(&users).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list users",
		})
	}

	// Convert to response format
	userList := make([]fiber.Map, len(users))
	for i, user := range users {
		userList[i] = fiber.Map{
			"id":         user.ID,
			"uuid":       user.UUID.String(),
			"email":      user.Email,
			"role":       user.Role,
			"club_id":    user.ClubID,
			"league_id":  user.LeagueID,
			"created_at": user.CreatedAt,
			"updated_at": user.UpdatedAt,
		}
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	return c.JSON(fiber.Map{
		"users":       userList,
		"total":       total,
		"page":        page,
		"per_page":    perPage,
		"total_pages": totalPages,
	})
}

// CreateUser creates a new user (admin only)
func CreateUser(c *fiber.Ctx) error {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
		ClubID   *uint  `json:"club_id,omitempty"`
		LeagueID *uint  `json:"league_id,omitempty"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if req.Email == "" || req.Password == "" || req.Role == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Email, password, and role are required",
		})
	}

	role := models.UserRole(req.Role)

	// Validate role-specific requirements
	if role == models.ClubManager && req.ClubID == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "club_id is required for club_manager role",
		})
	}

	if (role == models.LeagueAdmin || role == models.ClubManager) && (req.LeagueID == nil || *req.LeagueID == 0) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "league_id is required for league and club_manager roles",
		})
	}

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

	// Create user with appropriate LeagueID

	user := models.User{
		Email:    req.Email,
		Password: string(hashedPassword),
		Role:     role,
		ClubID:   req.ClubID,
		LeagueID: req.LeagueID,
	}

	if err := database.DB.Create(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create user",
		})
	}

	// Load relationships for response
	database.DB.Preload("League").Preload("Club").First(&user, user.ID)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "User created successfully",
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

// UpdateUser updates a user's information (admin only)
func UpdateUser(c *fiber.Ctx) error {
	id := c.Params("id")

	userID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID",
		})
	}

	var req struct {
		Email    *string `json:"email,omitempty"`
		Password *string `json:"password,omitempty"`
		Role     *string `json:"role,omitempty"`
		ClubID   *uint   `json:"club_id,omitempty"`
		LeagueID *uint   `json:"league_id,omitempty"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Find user
	var user models.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	updates := make(map[string]interface{})

	// Update email if provided
	if req.Email != nil && *req.Email != user.Email {
		// Check if new email already exists
		var existingUser models.User
		if err := database.DB.Where("email = ? AND id != ?", *req.Email, userID).First(&existingUser).Error; err == nil {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "Email already in use",
			})
		}
		updates["email"] = *req.Email
	}

	// Update password if provided
	if req.Password != nil {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to process password",
			})
		}
		updates["password"] = string(hashedPassword)
	}

	// Update role if provided
	if req.Role != nil {
		role := models.UserRole(*req.Role)

		// Validate role-specific requirements
		if role == models.ClubManager {
			if req.ClubID == nil && user.ClubID == nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "club_id is required for club_manager role",
				})
			}
		}

		updates["role"] = role
	}

	// Update club_id if provided
	if req.ClubID != nil {
		updates["club_id"] = *req.ClubID
	}

	// Update league_id if provided
	if req.LeagueID != nil {
		updates["league_id"] = *req.LeagueID
	}

	// Perform update if there are changes
	if len(updates) > 0 {
		if err := database.DB.Model(&user).Updates(updates).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update user",
			})
		}
	}

	// Reload user with relationships
	database.DB.Preload("League").Preload("Club").First(&user, userID)

	return c.JSON(fiber.Map{
		"message": "User updated successfully",
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

// DeleteUser soft deletes a user (admin only)
func DeleteUser(c *fiber.Ctx) error {
	id := c.Params("id")

	userID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID",
		})
	}

	result := database.DB.Delete(&models.User{}, userID)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete user",
		})
	}

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	return c.JSON(fiber.Map{
		"message": "User deleted successfully",
	})
}

// GetUsersByRole retrieves all users with a specific role
func GetUsersByRole(c *fiber.Ctx) error {
	role := c.Params("role")

	var users []models.User
	if err := database.DB.Preload("League").Preload("Club").Where("role = ?", role).Find(&users).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get users by role",
		})
	}

	userList := make([]fiber.Map, len(users))
	for i, user := range users {
		userList[i] = fiber.Map{
			"id":         user.ID,
			"uuid":       user.UUID.String(),
			"email":      user.Email,
			"role":       user.Role,
			"club_id":    user.ClubID,
			"league_id":  user.LeagueID,
			"created_at": user.CreatedAt,
		}
	}

	return c.JSON(fiber.Map{
		"users": userList,
		"count": len(userList),
	})
}

// GetUsersByLeague retrieves all users for a specific league
func GetUsersByLeague(c *fiber.Ctx) error {
	leagueID := c.Params("league_id")

	var users []models.User
	if err := database.DB.Preload("Club").Where("league_id = ?", leagueID).Find(&users).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get users by league",
		})
	}

	userList := make([]fiber.Map, len(users))
	for i, user := range users {
		userList[i] = fiber.Map{
			"id":         user.ID,
			"uuid":       user.UUID.String(),
			"email":      user.Email,
			"role":       user.Role,
			"club_id":    user.ClubID,
			"league_id":  user.LeagueID,
			"club":       user.Club,
			"created_at": user.CreatedAt,
		}
	}

	return c.JSON(fiber.Map{
		"users": userList,
		"count": len(userList),
	})
}

// GetUsersByClub retrieves all users for a specific club
func GetUsersByClub(c *fiber.Ctx) error {
	clubID := c.Params("club_id")

	var users []models.User
	if err := database.DB.Preload("League").Where("club_id = ?", clubID).Find(&users).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get users by club",
		})
	}

	userList := make([]fiber.Map, len(users))
	for i, user := range users {
		userList[i] = fiber.Map{
			"id":         user.ID,
			"uuid":       user.UUID.String(),
			"email":      user.Email,
			"role":       user.Role,
			"club_id":    user.ClubID,
			"league_id":  user.LeagueID,
			"league":     user.League,
			"created_at": user.CreatedAt,
		}
	}

	return c.JSON(fiber.Map{
		"users": userList,
		"count": len(userList),
	})
}

// AssignUserToClub assigns a user to a club
func AssignUserToClub(c *fiber.Ctx) error {
	var req struct {
		UserID uint `json:"user_id"`
		ClubID uint `json:"club_id"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Find user
	var user models.User
	if err := database.DB.First(&user, req.UserID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "User not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	// Verify club exists
	var club models.Club
	if err := database.DB.First(&club, req.ClubID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Club not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	// Update user's club
	if err := database.DB.Model(&user).Update("club_id", req.ClubID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to assign user to club",
		})
	}

	return c.JSON(fiber.Map{
		"message": "User assigned to club successfully",
	})
}

// RemoveUserFromClub removes a user from their club
func RemoveUserFromClub(c *fiber.Ctx) error {
	id := c.Params("id")

	userID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid user ID",
		})
	}

	// Update user's club_id to nil
	if err := database.DB.Model(&models.User{}).Where("id = ?", userID).Update("club_id", nil).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to remove user from club",
		})
	}

	return c.JSON(fiber.Map{
		"message": "User removed from club successfully",
	})
}
