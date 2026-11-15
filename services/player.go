package services

import (
	"errors"
	"fmt"
	"gorm.io/gorm/clause"
	"strconv"
	"strings"
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
	if err := database.DB.Preload("Club").Preload("NationalTeam").Preload("Stats").First(&player, playerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Player not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	if !canReadPlayer(c, &player) {
		return fiber.ErrForbidden
	}
	return c.JSON(fiber.Map{
		"player": fiber.Map{
			"id":                     player.ID,
			"uuid":                   player.UUID,
			"name":                   player.Name,
			"jersey_number":          player.JerseyNumber,
			"position":               player.Position,
			"photo_url":              player.PhotoURL,
			"date_of_birth":          player.DateOfBirth,
			"nationality":            player.Nationality,
			"country_code":           player.CountryCode,
			"club_id":                player.ClubID,
			"national_team_id":       player.NationalTeamID,
			"national_team":          player.NationalTeam,
			"national_jersey_number": player.NationalJerseyNumber,
			"called_up":              player.CalledUp,
			"called_up_at":           player.CalledUpAt,
			"club":                   player.Club,
			"stats":                  player.Stats,
		},
	})
}

// GetPlayerByUUID retrieves a player by their UUID
func GetPlayerByUUID(c *fiber.Ctx) error {
	uuid := c.Params("uuid")

	var player models.Player
	if err := database.DB.Preload("Club").Preload("NationalTeam").Preload("Stats").Where("uuid = ?", uuid).First(&player).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Player not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	if !canReadPlayer(c, &player) {
		return fiber.ErrForbidden
	}
	return c.JSON(fiber.Map{
		"player": fiber.Map{
			"id":                     player.ID,
			"uuid":                   player.UUID,
			"name":                   player.Name,
			"jersey_number":          player.JerseyNumber,
			"position":               player.Position,
			"photo_url":              player.PhotoURL,
			"date_of_birth":          player.DateOfBirth,
			"nationality":            player.Nationality,
			"country_code":           player.CountryCode,
			"club_id":                player.ClubID,
			"national_team_id":       player.NationalTeamID,
			"national_team":          player.NationalTeam,
			"national_jersey_number": player.NationalJerseyNumber,
			"called_up":              player.CalledUp,
			"called_up_at":           player.CalledUpAt,
			"club":                   player.Club,
			"stats":                  player.Stats,
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
	user, _ := middleware.GetUserFromContext(c)
	query := visiblePlayers(database.DB.Model(&models.Player{}), user)

	// Apply filters
	if clubID := c.Query("club_id"); clubID != "" {
		query = teamPlayers(query, clubID)
	}
	if teamID := c.Query("national_team_id"); teamID != "" {
		query = query.Where("national_team_id = ?", teamID)
	}
	if position := c.Query("position"); position != "" {
		query = query.Where("position = ?", position)
	}
	if nationality := c.Query("nationality"); nationality != "" {
		query = query.Where("(country_code=public.afrigoals_country_code(?) OR LOWER(nationality)=LOWER(?))", nationality, nationality)
	}
	if name := c.Query("name"); name != "" {
		query = query.Where("LOWER(name) LIKE LOWER(?)", "%"+name+"%")
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
	if err := query.Preload("Club").Preload("NationalTeam").Preload("Stats").
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
			"id":                     player.ID,
			"uuid":                   player.UUID,
			"name":                   player.Name,
			"jersey_number":          player.JerseyNumber,
			"position":               player.Position,
			"nationality":            player.Nationality,
			"country_code":           player.CountryCode,
			"club_id":                player.ClubID,
			"national_team_id":       player.NationalTeamID,
			"national_team":          player.NationalTeam,
			"national_jersey_number": player.NationalJerseyNumber,
			"called_up":              player.CalledUp,
			"called_up_at":           player.CalledUpAt,
			"created_at":             player.CreatedAt,
			"updated_at":             player.UpdatedAt,
			"club":                   player.Club,
			"photo_url":              player.PhotoURL,
			"date_of_birth":          player.DateOfBirth,
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
		Name                 string  `json:"name"`
		JerseyNumber         int     `json:"jersey_number"`
		Position             string  `json:"position"`
		PhotoURL             *string `json:"photo_url"`
		DateOfBirth          string  `json:"date_of_birth"` // Format: YYYY-MM-DD
		Nationality          string  `json:"nationality"`
		NationalTeamID       *uint   `json:"national_team_id"`
		NationalJerseyNumber *int    `json:"national_jersey_number"`
		ClubID               *uint   `json:"club_id"` // Optional, can be assigned later
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

	if req.ClubID != nil {
		var primary models.Club
		if err := database.DB.First(&primary, *req.ClubID).Error; err != nil {
			return err
		}
		if nationalTeam(primary) {
			return fiber.NewError(400, "Primary club must be a club team")
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

	if err := checkNationalTeam(database.DB, req.NationalTeamID, req.NationalJerseyNumber, 0); err != nil {
		return fiber.NewError(400, err.Error())
	}
	// Create player
	player := models.Player{
		Name:           req.Name,
		JerseyNumber:   req.JerseyNumber,
		Position:       req.Position,
		PhotoURL:       req.PhotoURL,
		DateOfBirth:    dob,
		Nationality:    req.Nationality,
		NationalTeamID: req.NationalTeamID, NationalJerseyNumber: req.NationalJerseyNumber,
		ClubID: 0, // Will be set if req.ClubID is provided
	}

	if req.ClubID != nil {
		player.ClubID = *req.ClubID
	}

	create := database.DB.Omit("Club", "NationalTeam", "Stats")
	if player.ClubID == 0 {
		create = create.Omit("ClubID")
	}
	if err := create.Create(&player).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create player",
		})
	}

	// Load relationships for response
	database.DB.Preload("Club").Preload("NationalTeam").First(&player, player.ID)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Player created successfully",
		"player": fiber.Map{
			"id":                     player.ID,
			"uuid":                   player.UUID,
			"name":                   player.Name,
			"jersey_number":          player.JerseyNumber,
			"position":               player.Position,
			"photo_url":              player.PhotoURL,
			"date_of_birth":          player.DateOfBirth,
			"nationality":            player.Nationality,
			"country_code":           player.CountryCode,
			"club_id":                player.ClubID,
			"national_team_id":       player.NationalTeamID,
			"national_team":          player.NationalTeam,
			"national_jersey_number": player.NationalJerseyNumber,
			"called_up":              player.CalledUp,
			"called_up_at":           player.CalledUpAt,
			"club":                   player.Club,
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
		Name                 *string `json:"name,omitempty"`
		JerseyNumber         *int    `json:"jersey_number,omitempty"`
		Position             *string `json:"position,omitempty"`
		PhotoURL             *string `json:"photo_url,omitempty"`
		DateOfBirth          *string `json:"date_of_birth,omitempty"` // Format: YYYY-MM-DD
		Nationality          *string `json:"nationality,omitempty"`
		NationalTeamID       *uint   `json:"national_team_id,omitempty"`
		NationalJerseyNumber *int    `json:"national_jersey_number,omitempty"`
		ClubID               *uint   `json:"club_id,omitempty"`
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

	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	if !middleware.CanManagePlayer(user, &player) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You do not have permission to update this player",
		})
	}

	nextClubID := player.ClubID
	if req.ClubID != nil {
		nextClubID = *req.ClubID
		if !middleware.CanManageClub(user, nextClubID) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "You do not have permission to move this player to that club",
			})
		}
	}
	if req.JerseyNumber != nil {
		if err := ensureJerseyFree(nextClubID, *req.JerseyNumber, uint(playerID)); err != nil {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
	}

	nextNationalID := player.NationalTeamID
	nextNationalJersey := player.NationalJerseyNumber
	if req.NationalTeamID != nil {
		nextNationalID = req.NationalTeamID
		if *req.NationalTeamID == 0 {
			nextNationalID = nil
			nextNationalJersey = nil
		}
	}
	if req.NationalJerseyNumber != nil {
		nextNationalJersey = req.NationalJerseyNumber
	}
	if err := checkNationalTeam(database.DB, nextNationalID, nextNationalJersey, player.ID); err != nil {
		return fiber.NewError(409, err.Error())
	}
	updates := make(map[string]interface{})
	if req.NationalTeamID != nil || req.NationalJerseyNumber != nil {
		updates["national_team_id"] = nextNationalID
		updates["national_jersey_number"] = nextNationalJersey
	}

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
		if nationalTeam(club) {
			return fiber.NewError(400, "Primary club must be a club team")
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
	database.DB.Preload("Club").Preload("NationalTeam").Preload("Stats").First(&player, playerID)

	return c.JSON(fiber.Map{
		"message": "Player updated successfully",
		"player": fiber.Map{
			"id":                     player.ID,
			"uuid":                   player.UUID,
			"name":                   player.Name,
			"jersey_number":          player.JerseyNumber,
			"position":               player.Position,
			"photo_url":              player.PhotoURL,
			"date_of_birth":          player.DateOfBirth,
			"nationality":            player.Nationality,
			"country_code":           player.CountryCode,
			"club_id":                player.ClubID,
			"national_team_id":       player.NationalTeamID,
			"national_team":          player.NationalTeam,
			"national_jersey_number": player.NationalJerseyNumber,
			"called_up":              player.CalledUp,
			"called_up_at":           player.CalledUpAt,
			"club":                   player.Club,
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
	if err := teamPlayers(visiblePlayers(database.DB.Preload("Club").Preload("NationalTeam").Preload("Stats"), currentPlayerReader(c)), clubID).Find(&players).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get players by club",
		})
	}

	teamID, _ := strconv.ParseUint(clubID, 10, 32)
	projectSquad(players, uint(teamID))
	var squadTeam models.Club
	if err := database.DB.First(&squadTeam, uint(teamID)).Error; err != nil {
		return fiber.ErrNotFound
	}
	if nationalTeam(squadTeam) {
		if err := attachNationalCallups(database.DB, uint(teamID), players); err != nil {
			return fiber.NewError(500, "Could not load call-up selections")
		}
	}
	playerList := make([]fiber.Map, len(players))
	for i, player := range players {
		playerList[i] = fiber.Map{
			"id":                     player.ID,
			"uuid":                   player.UUID,
			"name":                   player.Name,
			"jersey_number":          player.JerseyNumber,
			"position":               player.Position,
			"nationality":            player.Nationality,
			"country_code":           player.CountryCode,
			"club_id":                player.ClubID,
			"national_team_id":       player.NationalTeamID,
			"national_team":          player.NationalTeam,
			"national_jersey_number": player.NationalJerseyNumber,
			"called_up":              player.CalledUp,
			"called_up_at":           player.CalledUpAt,
			"stats":                  player.Stats,
			"created_at":             player.CreatedAt,
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
	if err := visiblePlayers(database.DB.Preload("Club").Preload("NationalTeam").Preload("Stats"), currentPlayerReader(c)).Where("position = ?", position).Find(&players).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get players by position",
		})
	}

	playerList := make([]fiber.Map, len(players))
	for i, player := range players {
		playerList[i] = fiber.Map{
			"id":                     player.ID,
			"uuid":                   player.UUID,
			"name":                   player.Name,
			"jersey_number":          player.JerseyNumber,
			"position":               player.Position,
			"nationality":            player.Nationality,
			"country_code":           player.CountryCode,
			"club_id":                player.ClubID,
			"national_team_id":       player.NationalTeamID,
			"national_team":          player.NationalTeam,
			"national_jersey_number": player.NationalJerseyNumber,
			"called_up":              player.CalledUp,
			"called_up_at":           player.CalledUpAt,
			"club":                   player.Club,
			"created_at":             player.CreatedAt,
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
	if err := visiblePlayers(database.DB.Preload("Club").Preload("NationalTeam").Preload("Stats"), currentPlayerReader(c)).Where("(country_code=public.afrigoals_country_code(?) OR LOWER(nationality)=LOWER(?))", nationality, nationality).Find(&players).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get players by nationality",
		})
	}

	playerList := make([]fiber.Map, len(players))
	for i, player := range players {
		playerList[i] = fiber.Map{
			"id":                     player.ID,
			"uuid":                   player.UUID,
			"name":                   player.Name,
			"jersey_number":          player.JerseyNumber,
			"position":               player.Position,
			"nationality":            player.Nationality,
			"country_code":           player.CountryCode,
			"club_id":                player.ClubID,
			"national_team_id":       player.NationalTeamID,
			"national_team":          player.NationalTeam,
			"national_jersey_number": player.NationalJerseyNumber,
			"called_up":              player.CalledUp,
			"called_up_at":           player.CalledUpAt,
			"club":                   player.Club,
			"created_at":             player.CreatedAt,
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
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	var req struct {
		PlayerID       uint   `json:"player_id"`
		NewClubID      uint   `json:"new_club_id"`
		FromClubID     uint   `json:"from_club_id"`
		NewClubName    string `json:"new_club_name"`
		NewClubCountry string `json:"new_club_country"`
		JerseyNumber   *int   `json:"jersey_number"`
	}
	if c.BodyParser(&req) != nil || req.PlayerID == 0 {
		return fiber.NewError(400, "Player ID is required")
	}
	if req.NewClubID == 0 && (user.Role != models.AfrigoalsAdmin || strings.TrimSpace(req.NewClubName) == "" || strings.TrimSpace(req.NewClubCountry) == "") {
		return fiber.NewError(400, "Select a destination club or provide its name and country as an administrator")
	}
	if req.JerseyNumber != nil && (*req.JerseyNumber < 0 || *req.JerseyNumber > 99) {
		return fiber.NewError(400, "Shirt number must be between 1 and 99, or zero when unassigned")
	}
	var player models.Player
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&player, req.PlayerID).Error; err != nil {
			return err
		}
		if !middleware.CanManagePlayer(user, &player) {
			return fiber.ErrForbidden
		}
		if req.FromClubID > 0 && req.FromClubID != player.ClubID {
			return fiber.NewError(409, "The player has already moved from that club. Reload their profile")
		}
		var club models.Club
		if req.NewClubID > 0 {
			if err := tx.First(&club, req.NewClubID).Error; err != nil {
				return err
			}
		} else {
			result := tx.Where("LOWER(btrim(name))=LOWER(?) AND country_code=public.afrigoals_country_code(?) AND team_type='club'", strings.TrimSpace(req.NewClubName), req.NewClubCountry).First(&club)
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				club = models.Club{Name: strings.TrimSpace(req.NewClubName), Country: req.NewClubCountry, TeamType: "club", IsExternal: true}
				if err := tx.Create(&club).Error; err != nil {
					return err
				}
			} else if result.Error != nil {
				return result.Error
			}
		}
		if nationalTeam(club) {
			return fiber.NewError(400, "Select a club as the transfer destination")
		}
		if user.Role != models.AfrigoalsAdmin && !middleware.CanManageClub(user, club.ID) {
			return fiber.ErrForbidden
		}
		jersey := player.JerseyNumber
		if club.IsExternal {
			jersey = 0
		}
		if req.JerseyNumber != nil {
			jersey = *req.JerseyNumber
		}
		if jersey > 0 {
			var count int64
			if err := tx.Model(&models.Player{}).Where("club_id=? AND jersey_number=? AND id<>?", club.ID, jersey, player.ID).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return fiber.NewError(409, "That shirt number is already assigned at the destination club")
			}
		}
		if err := tx.Model(&player).Updates(map[string]interface{}{"club_id": club.ID, "jersey_number": jersey}).Error; err != nil {
			return err
		}
		return tx.Preload("Club").Preload("NationalTeam").First(&player, player.ID).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fiber.ErrNotFound
	}
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"success": true, "message": "Player transferred with their existing profile and history", "player": player})
}

// GetPlayerStatistics retrieves detailed statistics for a player
func GetPlayerStatistics(c *fiber.Ctx) error {
	playerID := c.Params("player_id")

	var player models.Player
	if err := database.DB.Preload("Club").Preload("NationalTeam").Preload("Stats").First(&player, playerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Player not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	if !canReadPlayer(c, &player) {
		return fiber.ErrForbidden
	}
	// Calculate age
	age := 0
	if !player.DateOfBirth.IsZero() {
		age = time.Now().Year() - player.DateOfBirth.Year()
	}

	return c.JSON(fiber.Map{
		"player_id":              player.ID,
		"player_name":            player.Name,
		"jersey_number":          player.JerseyNumber,
		"position":               player.Position,
		"age":                    age,
		"nationality":            player.Nationality,
		"country_code":           player.CountryCode,
		"club_id":                player.ClubID,
		"national_team_id":       player.NationalTeamID,
		"national_team":          player.NationalTeam,
		"national_jersey_number": player.NationalJerseyNumber,
		"called_up":              player.CalledUp,
		"called_up_at":           player.CalledUpAt,
		"club_name":              player.Club.Name,
		"stats":                  player.Stats,
	})
}

// ensureJerseyFree reports whether jersey is already taken at club by a player
// other than excludeID. Soft-deleted players are ignored, matching the partial
// unique index on players.
func ensureJerseyFree(clubID uint, jersey int, excludeID uint) error {
	if clubID == 0 || jersey == 0 {
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
