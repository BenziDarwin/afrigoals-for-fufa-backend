package services

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"afrigoals.com/database"
	"afrigoals.com/middleware"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// GetPlayerByID retrieves a player by their ID
func GetPlayerByID(c *fiber.Ctx) error {
	id := c.Params("id")

	playerID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid player ID",
		})
	}

	var player models.Player
	if err := database.DB.Preload("Club").Preload("Stats").First(&player, playerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Player not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	return c.JSON(fiber.Map{
		"player": fiber.Map{
			"id":            player.ID,
			"uuid":          player.UUID,
			"name":          player.Name,
			"jersey_number": player.JerseyNumber,
			"position":      player.Position,
			"photo_url":     player.PhotoURL,
			"date_of_birth": player.DateOfBirth,
			"nationality":   player.Nationality,
			"club_id":       player.ClubID,
			"club":          player.Club,
			"stats":         player.Stats,
		},
	})
}

// GetPlayerByUUID retrieves a player by their UUID
func GetPlayerByUUID(c *fiber.Ctx) error {
	uuid := c.Params("uuid")

	var player models.Player
	if err := database.DB.Preload("Club").Preload("Stats").Where("uuid = ?", uuid).First(&player).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Player not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	return c.JSON(fiber.Map{
		"player": fiber.Map{
			"id":            player.ID,
			"uuid":          player.UUID,
			"name":          player.Name,
			"jersey_number": player.JerseyNumber,
			"position":      player.Position,
			"photo_url":     player.PhotoURL,
			"date_of_birth": player.DateOfBirth,
			"nationality":   player.Nationality,
			"club_id":       player.ClubID,
			"club":          player.Club,
			"stats":         player.Stats,
		},
	})
}

// ListPlayers retrieves all players with pagination and filtering
func ListPlayers(c *fiber.Ctx) error {
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
	query := database.DB.Model(&models.Player{})

	// Apply filters
	if clubID := c.Query("club_id"); clubID != "" {
		query = query.Where("club_id = ?", clubID)
	}
	if position := c.Query("position"); position != "" {
		query = query.Where("position = ?", position)
	}
	if nationality := c.Query("nationality"); nationality != "" {
		query = query.Where("nationality = ?", nationality)
	}
	if name := c.Query("name"); name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to count players",
		})
	}

	// Get paginated results
	var players []models.Player
	if err := query.Preload("Club").Preload("Stats").
		Limit(perPage).Offset(offset).
		Order("created_at DESC").
		Find(&players).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list players",
		})
	}

	// Convert to response format
	playerList := make([]fiber.Map, len(players))
	for i, player := range players {
		playerList[i] = fiber.Map{
			"id":            player.ID,
			"uuid":          player.UUID,
			"name":          player.Name,
			"jersey_number": player.JerseyNumber,
			"position":      player.Position,
			"nationality":   player.Nationality,
			"club_id":       player.ClubID,
			"created_at":    player.CreatedAt,
			"updated_at":    player.UpdatedAt,
		}
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	return c.JSON(fiber.Map{
		"players":     playerList,
		"total":       total,
		"page":        page,
		"per_page":    perPage,
		"total_pages": totalPages,
	})
}

// CreatePlayer creates a new player (Note: AssignPlayerToClub in club.go also creates)
func CreatePlayer(c *fiber.Ctx) error {
	var req struct {
		Name         string  `json:"name"`
		JerseyNumber int     `json:"jersey_number"`
		Position     string  `json:"position"`
		PhotoURL     *string `json:"photo_url"`
		DateOfBirth  string  `json:"date_of_birth"` // Format: YYYY-MM-DD
		Nationality  string  `json:"nationality"`
		ClubID       *uint   `json:"club_id"` // Optional, can be assigned later
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

	if req.Position == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Position is required",
		})
	}

	// Parse date of birth
	var dob time.Time
	var err error
	if req.DateOfBirth != "" {
		dob, err = time.Parse("2006-01-02", req.DateOfBirth)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid date format. Use YYYY-MM-DD",
			})
		}
	}

	// If club_id is provided, verify club exists
	if req.ClubID != nil {
		var club models.Club
		if err := database.DB.First(&club, *req.ClubID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"error": "Club not found",
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Database error",
			})
		}
	}

	// One shirt number per club (see models.Player).
	if req.ClubID != nil {
		if err := ensureJerseyFree(*req.ClubID, req.JerseyNumber, 0); err != nil {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
	}

	// Create player
	player := models.Player{
		Name:         req.Name,
		JerseyNumber: req.JerseyNumber,
		Position:     req.Position,
		PhotoURL:     req.PhotoURL,
		DateOfBirth:  dob,
		Nationality:  req.Nationality,
		ClubID:       0, // Will be set if req.ClubID is provided
	}

	if req.ClubID != nil {
		player.ClubID = *req.ClubID
	}

	if err := database.DB.Create(&player).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create player",
		})
	}

	// Load relationships for response
	database.DB.Preload("Club").First(&player, player.ID)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Player created successfully",
		"player": fiber.Map{
			"id":            player.ID,
			"uuid":          player.UUID,
			"name":          player.Name,
			"jersey_number": player.JerseyNumber,
			"position":      player.Position,
			"photo_url":     player.PhotoURL,
			"date_of_birth": player.DateOfBirth,
			"nationality":   player.Nationality,
			"club_id":       player.ClubID,
			"club":          player.Club,
		},
	})
}

// UpdatePlayer updates a player's information
func UpdatePlayer(c *fiber.Ctx) error {
	id := c.Params("id")

	playerID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid player ID",
		})
	}

	var req struct {
		Name         *string `json:"name,omitempty"`
		JerseyNumber *int    `json:"jersey_number,omitempty"`
		Position     *string `json:"position,omitempty"`
		PhotoURL     *string `json:"photo_url,omitempty"`
		DateOfBirth  *string `json:"date_of_birth,omitempty"` // Format: YYYY-MM-DD
		Nationality  *string `json:"nationality,omitempty"`
		ClubID       *uint   `json:"club_id,omitempty"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
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

	updates := make(map[string]interface{})

	// Update name if provided
	if req.Name != nil {
		updates["name"] = *req.Name
	}

	// Update jersey number if provided
	if req.JerseyNumber != nil {
		updates["jersey_number"] = *req.JerseyNumber
	}

	// Update position if provided
	if req.Position != nil {
		updates["position"] = *req.Position
	}

	// Update photo URL if provided
	if req.PhotoURL != nil {
		updates["photo_url"] = *req.PhotoURL
	}

	// Update date of birth if provided
	if req.DateOfBirth != nil {
		dob, err := time.Parse("2006-01-02", *req.DateOfBirth)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid date format. Use YYYY-MM-DD",
			})
		}
		updates["date_of_birth"] = dob
	}

	// Update nationality if provided
	if req.Nationality != nil {
		updates["nationality"] = *req.Nationality
	}

	// Update club_id if provided
	if req.ClubID != nil {
		// Verify club exists
		var club models.Club
		if err := database.DB.First(&club, *req.ClubID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
					"error": "Club not found",
				})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Database error",
			})
		}
		updates["club_id"] = *req.ClubID
	}

	// Perform update if there are changes
	if len(updates) > 0 {
		if err := database.DB.Model(&player).Updates(updates).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update player",
			})
		}
	}

	// Reload player with relationships
	database.DB.Preload("Club").Preload("Stats").First(&player, playerID)

	return c.JSON(fiber.Map{
		"message": "Player updated successfully",
		"player": fiber.Map{
			"id":            player.ID,
			"uuid":          player.UUID,
			"name":          player.Name,
			"jersey_number": player.JerseyNumber,
			"position":      player.Position,
			"photo_url":     player.PhotoURL,
			"date_of_birth": player.DateOfBirth,
			"nationality":   player.Nationality,
			"club_id":       player.ClubID,
			"club":          player.Club,
		},
	})
}

// DeletePlayer soft deletes a player
func DeletePlayer(c *fiber.Ctx) error {
	id := c.Params("id")

	playerID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid player ID",
		})
	}

	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	// Load the player first: this route is open to club managers, so the
	// player's club decides whether this user may delete it.
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

	if !middleware.CanManagePlayer(user, &player) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You do not have permission to delete this player",
		})
	}

	result := database.DB.Delete(&models.Player{}, playerID)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete player",
		})
	}

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Player not found",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Player deleted successfully",
	})
}

// GetPlayersByClub retrieves all players for a specific club
func GetPlayersByClub(c *fiber.Ctx) error {
	clubID := c.Params("club_id")

	var players []models.Player
	if err := database.DB.Preload("Stats").Where("club_id = ?", clubID).Find(&players).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get players by club",
		})
	}

	playerList := make([]fiber.Map, len(players))
	for i, player := range players {
		playerList[i] = fiber.Map{
			"id":            player.ID,
			"uuid":          player.UUID,
			"name":          player.Name,
			"jersey_number": player.JerseyNumber,
			"position":      player.Position,
			"nationality":   player.Nationality,
			"club_id":       player.ClubID,
			"stats":         player.Stats,
			"created_at":    player.CreatedAt,
		}
	}

	return c.JSON(fiber.Map{
		"players": playerList,
		"count":   len(playerList),
	})
}

// GetPlayersByPosition retrieves all players with a specific position
func GetPlayersByPosition(c *fiber.Ctx) error {
	position := c.Params("position")

	var players []models.Player
	if err := database.DB.Preload("Club").Preload("Stats").Where("position = ?", position).Find(&players).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get players by position",
		})
	}

	playerList := make([]fiber.Map, len(players))
	for i, player := range players {
		playerList[i] = fiber.Map{
			"id":            player.ID,
			"uuid":          player.UUID,
			"name":          player.Name,
			"jersey_number": player.JerseyNumber,
			"position":      player.Position,
			"nationality":   player.Nationality,
			"club_id":       player.ClubID,
			"club":          player.Club,
			"created_at":    player.CreatedAt,
		}
	}

	return c.JSON(fiber.Map{
		"players": playerList,
		"count":   len(playerList),
	})
}

// GetPlayersByNationality retrieves all players with a specific nationality
func GetPlayersByNationality(c *fiber.Ctx) error {
	nationality := c.Params("nationality")

	var players []models.Player
	if err := database.DB.Preload("Club").Preload("Stats").Where("nationality = ?", nationality).Find(&players).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get players by nationality",
		})
	}

	playerList := make([]fiber.Map, len(players))
	for i, player := range players {
		playerList[i] = fiber.Map{
			"id":            player.ID,
			"uuid":          player.UUID,
			"name":          player.Name,
			"jersey_number": player.JerseyNumber,
			"position":      player.Position,
			"nationality":   player.Nationality,
			"club_id":       player.ClubID,
			"club":          player.Club,
			"created_at":    player.CreatedAt,
		}
	}

	return c.JSON(fiber.Map{
		"players": playerList,
		"count":   len(playerList),
	})
}

// GetFreePlayers retrieves all players not assigned to any club
func GetFreePlayers(c *fiber.Ctx) error {
	var players []models.Player
	if err := database.DB.Preload("Stats").Where("club_id = 0 OR club_id IS NULL").Find(&players).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get free players",
		})
	}

	playerList := make([]fiber.Map, len(players))
	for i, player := range players {
		playerList[i] = fiber.Map{
			"id":            player.ID,
			"uuid":          player.UUID,
			"name":          player.Name,
			"jersey_number": player.JerseyNumber,
			"position":      player.Position,
			"nationality":   player.Nationality,
			"created_at":    player.CreatedAt,
		}
	}

	return c.JSON(fiber.Map{
		"players": playerList,
		"count":   len(playerList),
	})
}

// TransferPlayer transfers a player from one club to another
func TransferPlayer(c *fiber.Ctx) error {
	var req struct {
		PlayerID   uint `json:"player_id"`
		NewClubID  uint `json:"new_club_id"`
		FromClubID uint `json:"from_club_id"` // Optional validation
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if req.PlayerID == 0 || req.NewClubID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Player ID and New Club ID are required",
		})
	}

	// Find player
	var player models.Player
	if err := database.DB.Preload("Club").First(&player, req.PlayerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Player not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	// Validate from_club_id if provided
	if req.FromClubID != 0 && player.ClubID != req.FromClubID {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Player is not in the specified source club",
		})
	}

	// Verify new club exists
	var newClub models.Club
	if err := database.DB.First(&newClub, req.NewClubID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "New club not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	oldClubID := player.ClubID

	// Transfer player to new club
	if err := database.DB.Model(&player).Update("club_id", req.NewClubID).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to transfer player",
		})
	}

	// Reload player
	database.DB.Preload("Club").First(&player, req.PlayerID)

	return c.JSON(fiber.Map{
		"message": "Player transferred successfully",
		"player": fiber.Map{
			"id":          player.ID,
			"uuid":        player.UUID,
			"name":        player.Name,
			"old_club_id": oldClubID,
			"new_club_id": player.ClubID,
			"club":        player.Club,
		},
	})
}

// GetPlayerStatistics retrieves detailed statistics for a player
func GetPlayerStatistics(c *fiber.Ctx) error {
	playerID := c.Params("player_id")

	var player models.Player
	if err := database.DB.Preload("Club").Preload("Stats").First(&player, playerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Player not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	// Calculate age
	age := 0
	if !player.DateOfBirth.IsZero() {
		age = time.Now().Year() - player.DateOfBirth.Year()
	}

	return c.JSON(fiber.Map{
		"player_id":     player.ID,
		"player_name":   player.Name,
		"jersey_number": player.JerseyNumber,
		"position":      player.Position,
		"age":           age,
		"nationality":   player.Nationality,
		"club_id":       player.ClubID,
		"club_name":     player.Club.Name,
		"stats":         player.Stats,
	})
}

// ensureJerseyFree reports whether jersey is already taken at club by a player
// other than excludeID. Soft-deleted players are ignored, matching the partial
// unique index on players.
func ensureJerseyFree(clubID uint, jersey int, excludeID uint) error {
	if clubID == 0 {
		return nil
	}

	q := database.DB.Model(&models.Player{}).
		Where("club_id = ? AND jersey_number = ?", clubID, jersey)
	if excludeID != 0 {
		q = q.Where("id <> ?", excludeID)
	}

	var count int64
	if err := q.Count(&count).Error; err != nil {
		return fmt.Errorf("could not verify jersey number")
	}
	if count > 0 {
		return fmt.Errorf("jersey number %d is already taken at this club", jersey)
	}
	return nil
}
