package services

import (
	"errors"
	"strconv"

	"afrigoals.com/database"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// GetLeagueByID retrieves a league by their ID
func GetLeagueByID(c *fiber.Ctx) error {
	id := c.Params("id")

	leagueID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid league ID",
		})
	}

	var league models.League
	if err := database.DB.Preload("Clubs").Preload("Users").First(&league, leagueID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "League not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	return c.JSON(fiber.Map{
		"league": fiber.Map{
			"id":          league.ID,
			"uuid":        league.UUID.String(),
			"name":        league.Name,
			"country":     league.Country,
			"description": league.Description,
			"clubs":       league.Clubs,
			"users":       league.Users,
		},
	})
}

// GetLeagueByUUID retrieves a league by their UUID
func GetLeagueByUUID(c *fiber.Ctx) error {
	uuid := c.Params("uuid")

	var league models.League
	if err := database.DB.Preload("Clubs").Preload("Users").Where("uuid = ?", uuid).First(&league).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "League not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	return c.JSON(fiber.Map{
		"league": fiber.Map{
			"id":          league.ID,
			"uuid":        league.UUID.String(),
			"name":        league.Name,
			"country":     league.Country,
			"description": league.Description,
			"clubs":       league.Clubs,
			"users":       league.Users,
		},
	})
}

// ListLeagues retrieves all leagues with pagination and filtering
func ListLeagues(c *fiber.Ctx) error {
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
	query := database.DB.Model(&models.League{})

	// Apply filters
	if country := c.Query("country"); country != "" {
		query = query.Where("country = ?", country)
	}
	if name := c.Query("name"); name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to count leagues",
		})
	}

	// Get paginated results
	var leagues []models.League
	if err := query.Preload("Clubs").Preload("Users").
		Limit(perPage).Offset(offset).
		Order("created_at DESC").
		Find(&leagues).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list leagues",
		})
	}

	// Convert to response format
	leagueList := make([]fiber.Map, len(leagues))
	for i, league := range leagues {
		leagueList[i] = fiber.Map{
			"id":          league.ID,
			"uuid":        league.UUID.String(),
			"name":        league.Name,
			"country":     league.Country,
			"description": league.Description,
			"club_count":  len(league.Clubs),
			"user_count":  len(league.Users),
			"created_at":  league.CreatedAt,
			"updated_at":  league.UpdatedAt,
		}
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	return c.JSON(fiber.Map{
		"leagues":     leagueList,
		"total":       total,
		"page":        page,
		"per_page":    perPage,
		"total_pages": totalPages,
	})
}

// CreateLeague creates a new league
func CreateLeague(c *fiber.Ctx) error {
	var req struct {
		Name        string `json:"name"`
		Country     string `json:"country"`
		Description string `json:"description"`
		ClubIDs     []uint `json:"club_ids"` // Optional: Associate clubs during creation
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Name is required",
		})
	}

	// Check if league with same name already exists in the country
	var existingLeague models.League
	if err := database.DB.Where("name = ? AND country = ?", req.Name, req.Country).First(&existingLeague).Error; err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "League with this name already exists in this country",
		})
	}

	// Create league
	league := models.League{
		Name:        req.Name,
		Country:     req.Country,
		Description: req.Description,
	}

	if err := database.DB.Create(&league).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create league",
		})
	}

	// Associate with clubs if provided
	if len(req.ClubIDs) > 0 {
		var clubs []models.Club
		if err := database.DB.Find(&clubs, req.ClubIDs).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to find clubs",
			})
		}

		if len(clubs) != len(req.ClubIDs) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "One or more club IDs are invalid",
			})
		}

		if err := database.DB.Model(&league).Association("Clubs").Append(&clubs); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to associate league with clubs",
			})
		}
	}

	// Load relationships for response
	database.DB.Preload("Clubs").First(&league, league.ID)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "League created successfully",
		"league": fiber.Map{
			"id":          league.ID,
			"uuid":        league.UUID.String(),
			"name":        league.Name,
			"country":     league.Country,
			"description": league.Description,
			"clubs":       league.Clubs,
		},
	})
}

// UpdateLeague updates a league's information
func UpdateLeague(c *fiber.Ctx) error {
	id := c.Params("id")

	leagueID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid league ID",
		})
	}

	var req struct {
		Name        *string `json:"name,omitempty"`
		Country     *string `json:"country,omitempty"`
		Description *string `json:"description,omitempty"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Find league
	var league models.League
	if err := database.DB.First(&league, leagueID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "League not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	updates := make(map[string]interface{})

	// Update name if provided
	if req.Name != nil && *req.Name != league.Name {
		// Check if new name already exists in the same country
		country := league.Country
		if req.Country != nil {
			country = *req.Country
		}
		var existingLeague models.League
		if err := database.DB.Where("name = ? AND country = ? AND id != ?", *req.Name, country, leagueID).First(&existingLeague).Error; err == nil {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "League with this name already exists in this country",
			})
		}
		updates["name"] = *req.Name
	}

	// Update country if provided
	if req.Country != nil {
		updates["country"] = *req.Country
	}

	// Update description if provided
	if req.Description != nil {
		updates["description"] = *req.Description
	}

	// Perform update if there are changes
	if len(updates) > 0 {
		if err := database.DB.Model(&league).Updates(updates).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update league",
			})
		}
	}

	// Reload league with relationships
	database.DB.Preload("Clubs").First(&league, leagueID)

	return c.JSON(fiber.Map{
		"message": "League updated successfully",
		"league": fiber.Map{
			"id":          league.ID,
			"uuid":        league.UUID.String(),
			"name":        league.Name,
			"country":     league.Country,
			"description": league.Description,
			"clubs":       league.Clubs,
		},
	})
}

// DeleteLeague soft deletes a league
func DeleteLeague(c *fiber.Ctx) error {
	id := c.Params("id")

	leagueID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid league ID",
		})
	}

	result := database.DB.Delete(&models.League{}, leagueID)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete league",
		})
	}

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "League not found",
		})
	}

	return c.JSON(fiber.Map{
		"message": "League deleted successfully",
	})
}

// GetLeaguesByCountry retrieves all leagues for a specific country
func GetLeaguesByCountry(c *fiber.Ctx) error {
	country := c.Params("country")

	var leagues []models.League
	if err := database.DB.Preload("Clubs").Where("country = ?", country).Find(&leagues).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get leagues by country",
		})
	}

	leagueList := make([]fiber.Map, len(leagues))
	for i, league := range leagues {
		leagueList[i] = fiber.Map{
			"id":          league.ID,
			"uuid":        league.UUID.String(),
			"name":        league.Name,
			"country":     league.Country,
			"description": league.Description,
			"club_count":  len(league.Clubs),
			"created_at":  league.CreatedAt,
		}
	}

	return c.JSON(fiber.Map{
		"leagues": leagueList,
		"count":   len(leagueList),
	})
}

// GetLeagueClubs retrieves all clubs for a specific league
func GetLeagueClubs(c *fiber.Ctx) error {
	leagueID := c.Params("league_id")

	var league models.League
	if err := database.DB.Preload("Clubs").Preload("Clubs.Players").First(&league, leagueID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "League not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	clubList := make([]fiber.Map, len(league.Clubs))
	for i, club := range league.Clubs {
		clubList[i] = fiber.Map{
			"id":           club.ID,
			"uuid":         club.UUID,
			"name":         club.Name,
			"player_count": len(club.Players),
			"created_at":   club.CreatedAt,
		}
	}

	return c.JSON(fiber.Map{
		"clubs": clubList,
		"count": len(clubList),
	})
}

// GetLeagueUsers retrieves all users for a specific league
func GetLeagueUsers(c *fiber.Ctx) error {
	leagueID := c.Params("league_id")

	var users []models.User
	if err := database.DB.Preload("Club").Where("league_id = ?", leagueID).Find(&users).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get league users",
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
			"created_at": user.CreatedAt,
		}
	}

	return c.JSON(fiber.Map{
		"users": userList,
		"count": len(userList),
	})
}

// AddClubToLeague adds a club to a league
func AddClubToLeague(c *fiber.Ctx) error {
	var req struct {
		LeagueID uint `json:"league_id"`
		ClubID   uint `json:"club_id"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if req.LeagueID == 0 || req.ClubID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "League ID and Club ID are required",
		})
	}

	// Find league
	var league models.League
	if err := database.DB.First(&league, req.LeagueID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "League not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	// Find club
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

	// Add club to league (many-to-many)
	if err := database.DB.Model(&league).Association("Clubs").Append(&club); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to add club to league",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Club added to league successfully",
		"league": fiber.Map{
			"id":   league.ID,
			"uuid": league.UUID.String(),
			"name": league.Name,
		},
		"club": fiber.Map{
			"id":   club.ID,
			"uuid": club.UUID,
			"name": club.Name,
		},
	})
}

// RemoveClubFromLeague removes a club from a league
func RemoveClubFromLeague(c *fiber.Ctx) error {
	var req struct {
		LeagueID uint `json:"league_id"`
		ClubID   uint `json:"club_id"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if req.LeagueID == 0 || req.ClubID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "League ID and Club ID are required",
		})
	}

	// Find league
	var league models.League
	if err := database.DB.First(&league, req.LeagueID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "League not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	// Find club
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

	// Remove club from league
	if err := database.DB.Model(&league).Association("Clubs").Delete(&club); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to remove club from league",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Club removed from league successfully",
	})
}

// GetLeagueStatistics retrieves statistics for a league
func GetLeagueStatistics(c *fiber.Ctx) error {
	leagueID := c.Params("league_id")

	var league models.League
	if err := database.DB.Preload("Clubs").Preload("Users").First(&league, leagueID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "League not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	// Count total players across all clubs in the league
	var totalPlayers int64
	for _, club := range league.Clubs {
		var playerCount int64
		database.DB.Model(&models.Player{}).Where("club_id = ?", club.ID).Count(&playerCount)
		totalPlayers += playerCount
	}

	return c.JSON(fiber.Map{
		"league_id":    league.ID,
		"league_name":  league.Name,
		"country":      league.Country,
		"club_count":   len(league.Clubs),
		"user_count":   len(league.Users),
		"player_count": totalPlayers,
		"description":  league.Description,
	})
}
