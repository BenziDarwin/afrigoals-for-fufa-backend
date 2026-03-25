package services

import (
	"errors"
	"strconv"
	"time"

	"afrigoals.com/database"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// GetClubByID retrieves a club by their ID
func GetClubByID(c *fiber.Ctx) error {
	id := c.Params("id")

	clubID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid club ID",
		})
	}

	var club models.Club
	if err := database.DB.Preload("Leagues").Preload("Players").First(&club, clubID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Club not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	return c.JSON(fiber.Map{
		"club": fiber.Map{
			"id":      club.ID,
			"uuid":    club.UUID,
			"name":    club.Name,
			"leagues": club.Leagues,
			"players": club.Players,
		},
	})
}

// GetClubByUUID retrieves a club by their UUID
func GetClubByUUID(c *fiber.Ctx) error {
	uuid := c.Params("uuid")

	var club models.Club
	if err := database.DB.Preload("Leagues").Preload("Players").Where("uuid = ?", uuid).First(&club).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Club not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	return c.JSON(fiber.Map{
		"club": fiber.Map{
			"id":      club.ID,
			"uuid":    club.UUID,
			"name":    club.Name,
			"leagues": club.Leagues,
			"players": club.Players,
		},
	})
}

// ListClubs retrieves all clubs with pagination and filtering
func ListClubs(c *fiber.Ctx) error {
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
	query := database.DB.Model(&models.Club{})

	// Apply filters
	if leagueID := c.Query("league_id"); leagueID != "" {
		// Filter clubs by league through the many-to-many relationship
		query = query.Joins("JOIN club_leagues ON club_leagues.club_id = clubs.id").
			Where("club_leagues.league_id = ?", leagueID)
	}
	if name := c.Query("name"); name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to count clubs",
		})
	}

	// Get paginated results
	var clubs []models.Club
	if err := query.Preload("Leagues").Preload("Players").
		Limit(perPage).Offset(offset).
		Order("created_at DESC").
		Find(&clubs).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list clubs",
		})
	}

	// Convert to response format
	clubList := make([]fiber.Map, len(clubs))
	for i, club := range clubs {
		clubList[i] = fiber.Map{
			"id":           club.ID,
			"uuid":         club.UUID,
			"name":         club.Name,
			"league_count": len(club.Leagues),
			"player_count": len(club.Players),
			"created_at":   club.CreatedAt,
			"updated_at":   club.UpdatedAt,
		}
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	return c.JSON(fiber.Map{
		"clubs":       clubList,
		"total":       total,
		"page":        page,
		"per_page":    perPage,
		"total_pages": totalPages,
	})
}

// CreateClub creates a new club
func CreateClub(c *fiber.Ctx) error {
	var req struct {
		Name      string `json:"name"`
		LeagueIDs []uint `json:"league_ids"` // Array of league IDs
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

	// Check if club with same name already exists
	var existingClub models.Club
	if err := database.DB.Where("name = ?", req.Name).First(&existingClub).Error; err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Club with this name already exists",
		})
	}

	// Create club
	club := models.Club{
		Name: req.Name,
	}

	if err := database.DB.Create(&club).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create club",
		})
	}

	// Associate with leagues if provided
	if len(req.LeagueIDs) > 0 {
		var leagues []models.League
		if err := database.DB.Find(&leagues, req.LeagueIDs).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to find leagues",
			})
		}

		if len(leagues) != len(req.LeagueIDs) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "One or more league IDs are invalid",
			})
		}

		if err := database.DB.Model(&club).Association("Leagues").Append(&leagues); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to associate club with leagues",
			})
		}
	}

	// Load relationships for response
	database.DB.Preload("Leagues").First(&club, club.ID)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Club created successfully",
		"club": fiber.Map{
			"id":      club.ID,
			"uuid":    club.UUID,
			"name":    club.Name,
			"leagues": club.Leagues,
		},
	})
}

// UpdateClub updates a club's information
func UpdateClub(c *fiber.Ctx) error {
	id := c.Params("id")

	clubID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid club ID",
		})
	}

	var req struct {
		Name *string `json:"name,omitempty"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Find club
	var club models.Club
	if err := database.DB.First(&club, clubID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Club not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	updates := make(map[string]interface{})

	// Update name if provided
	if req.Name != nil && *req.Name != club.Name {
		// Check if new name already exists
		var existingClub models.Club
		if err := database.DB.Where("name = ? AND id != ?", *req.Name, clubID).First(&existingClub).Error; err == nil {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "Club with this name already exists",
			})
		}
		updates["name"] = *req.Name
	}

	// Perform update if there are changes
	if len(updates) > 0 {
		if err := database.DB.Model(&club).Updates(updates).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update club",
			})
		}
	}

	// Reload club with relationships
	database.DB.Preload("Leagues").First(&club, clubID)

	return c.JSON(fiber.Map{
		"message": "Club updated successfully",
		"club": fiber.Map{
			"id":      club.ID,
			"uuid":    club.UUID,
			"name":    club.Name,
			"leagues": club.Leagues,
		},
	})
}

// DeleteClub soft deletes a club
func DeleteClub(c *fiber.Ctx) error {
	id := c.Params("id")

	clubID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid club ID",
		})
	}

	result := database.DB.Delete(&models.Club{}, clubID)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete club",
		})
	}

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Club not found",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Club deleted successfully",
	})
}

// GetClubsByLeague retrieves all clubs for a specific league
func GetClubsByLeague(c *fiber.Ctx) error {
	leagueID := c.Params("league_id")

	// Find league and preload clubs
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

func CreateAndAssignPlayerToClub(c *fiber.Ctx) error {
	// Get authenticated user from context
	user, ok := c.Locals("user").(models.User)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"error":   "Unauthorized",
		})
	}

	// ✅ FIXED: Use snake_case JSON tags to match frontend and REST conventions
	var req struct {
		ClubID       uint   `json:"club_id"`
		Name         string `json:"name"`
		JerseyNumber int    `json:"jersey_number"`
		Position     string `json:"position"`
		PhotoURL     string `json:"photo_url"`
		DateOfBirth  string `json:"date_of_birth"`
		Nationality  string `json:"nationality"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid request body",
			"details": err.Error(),
		})
	}

	// Validate required fields
	if req.ClubID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Club ID is required",
		})
	}

	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Player name is required",
		})
	}

	if req.Position == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Position is required",
		})
	}

	if req.DateOfBirth == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Date of birth is required",
		})
	}

	if req.Nationality == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Nationality is required",
		})
	}

	// ✅ CRITICAL: If user is club_manager, they can ONLY add players to their own club
	if user.Role == models.ClubManager {
		if user.ClubID == nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"error":   "Your account is not associated with any club",
			})
		}
		if *user.ClubID != req.ClubID {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"error":   "You can only add players to your own club",
			})
		}
	}

	// Verify club exists
	var club models.Club
	if err := database.DB.First(&club, req.ClubID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"error":   "Club not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Database error",
		})
	}

	// Parse date of birth
	dob, err := time.Parse("2006-01-02", req.DateOfBirth)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid date format. Use YYYY-MM-DD",
			"details": err.Error(),
		})
	}

	// Create player with club assignment
	var photoURL *string
	if req.PhotoURL != "" {
		photoURL = &req.PhotoURL
	}

	player := models.Player{
		ClubID:       req.ClubID,
		Name:         req.Name,
		JerseyNumber: req.JerseyNumber,
		Position:     req.Position,
		PhotoURL:     photoURL,
		DateOfBirth:  dob,
		Nationality:  req.Nationality,
	}

	if err := database.DB.Create(&player).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to create player",
			"details": err.Error(),
		})
	}

	// Load relationships for response
	database.DB.Preload("Club").First(&player, player.ID)

	// ✅ Return response matching frontend expectations (snake_case)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Player created and assigned to club successfully",
		"data": fiber.Map{
			"id":            player.ID,
			"uuid":          player.UUID,
			"name":          player.Name,
			"jersey_number": player.JerseyNumber,
			"position":      player.Position,
			"photo_url":     player.PhotoURL,
			"date_of_birth": player.DateOfBirth.Format("2006-01-02"),
			"nationality":   player.Nationality,
			"club_id":       player.ClubID,
			"club":          club,
			"created_at":    player.CreatedAt,
			"updated_at":    player.UpdatedAt,
		},
	})
}

// GetClubLeagues retrieves all leagues for a specific club
func GetClubLeagues(c *fiber.Ctx) error {
	clubID := c.Params("club_id")

	var club models.Club
	if err := database.DB.Preload("Leagues").First(&club, clubID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Club not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	leagueList := make([]fiber.Map, len(club.Leagues))
	for i, league := range club.Leagues {
		leagueList[i] = fiber.Map{
			"id":          league.ID,
			"uuid":        league.UUID.String(),
			"name":        league.Name,
			"country":     league.Country,
			"description": league.Description,
			"created_at":  league.CreatedAt,
		}
	}

	return c.JSON(fiber.Map{
		"leagues": leagueList,
		"count":   len(leagueList),
	})
}

// GetClubPlayers retrieves all players for a specific club
func GetClubPlayers(c *fiber.Ctx) error {
	clubID := c.Params("club_id")

	var players []models.Player
	if err := database.DB.Where("club_id = ?", clubID).Find(&players).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get club players",
		})
	}

	playerList := make([]fiber.Map, len(players))
	for i, player := range players {
		playerList[i] = fiber.Map{
			"id":         player.ID,
			"uuid":       player.UUID,
			"name":       player.Name,
			"club_id":    player.ClubID,
			"created_at": player.CreatedAt,
		}
	}

	return c.JSON(fiber.Map{
		"players": playerList,
		"count":   len(playerList),
	})
}

// AssignPlayerToClub assigns a player to a club
func AssignPlayerToClub(c *fiber.Ctx) error {
	var req struct {
		PlayerID uint `json:"player_id"`
		ClubID   uint `json:"club_id"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if req.PlayerID == 0 || req.ClubID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Player ID and Club ID are required",
		})
	}

	// Find player
	var player models.Player
	if err := database.DB.First(&player, req.PlayerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Player not found",
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

	// Update player's club
	if err := database.DB.Model(&player).Update("club_id", req.ClubID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to assign player to club",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Player assigned to club successfully",
		"player": fiber.Map{
			"id":      player.ID,
			"uuid":    player.UUID,
			"name":    player.Name,
			"club_id": req.ClubID,
		},
	})
}

// RemovePlayerFromClub removes a player from their club
func RemovePlayerFromClub(c *fiber.Ctx) error {
	id := c.Params("id")

	playerID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid player ID",
		})
	}

	// Find player
	var player models.Player
	if err := database.DB.First(&player, playerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Player not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	// Update player's club_id to nil
	if err := database.DB.Model(&player).Update("club_id", nil).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to remove player from club",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Player removed from club successfully",
		"player": fiber.Map{
			"id":      player.ID,
			"uuid":    player.UUID,
			"name":    player.Name,
			"club_id": nil,
		},
	})
}

// GetClubStatistics retrieves statistics for a club
func GetClubStatistics(c *fiber.Ctx) error {
	clubID := c.Params("club_id")

	var club models.Club
	if err := database.DB.Preload("Players").Preload("Leagues").First(&club, clubID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Club not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	return c.JSON(fiber.Map{
		"club_id":      club.ID,
		"club_name":    club.Name,
		"player_count": len(club.Players),
		"league_count": len(club.Leagues),
	})
}
