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

// ========================================
// MATCH CRUD OPERATIONS
// ========================================

// GetMatchByID retrieves a match by ID with all relationships
func GetMatchByID(c *fiber.Ctx) error {
	id := c.Params("id")

	matchID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid match ID",
		})
	}

	var match models.Match
	if err := database.DB.
		Preload("League").
		Preload("HomeClub").
		Preload("AwayClub").
		Preload("Formations").
		Preload("Formations.Lineup").
		Preload("Formations.Substitutes").
		Preload("Formations.Unavailable").
		Preload("Events").
		First(&match, matchID).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Match not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	return c.JSON(fiber.Map{
		"match": fiber.Map{
			"id":           match.ID,
			"uuid":         match.UUID,
			"league_id":    match.LeagueID,
			"home_club_id": match.HomeClubID,
			"away_club_id": match.AwayClubID,
			"date":         match.Date,
			"score_home":   match.ScoreHome,
			"score_away":   match.ScoreAway,
			"formations":   match.Formations,
			"events":       match.Events,
			"created_at":   match.CreatedAt,
		},
	})
}

// GetMatchByUUID retrieves a match by UUID
func GetMatchByUUID(c *fiber.Ctx) error {
	uuid := c.Params("uuid")

	var match models.Match
	if err := database.DB.
		Preload("League").
		Preload("HomeClub").
		Preload("AwayClub").
		Preload("Formations").
		Preload("Formations.Lineup").
		Preload("Formations.Substitutes").
		Preload("Formations.Unavailable").
		Preload("Events").
		Where("uuid = ?", uuid).
		First(&match).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Match not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	return c.JSON(fiber.Map{
		"match": fiber.Map{
			"id":           match.ID,
			"uuid":         match.UUID,
			"league_id":    match.LeagueID,
			"home_club_id": match.HomeClubID,
			"away_club_id": match.AwayClubID,
			"date":         match.Date,
			"score_home":   match.ScoreHome,
			"score_away":   match.ScoreAway,
			"formations":   match.Formations,
			"events":       match.Events,
		},
	})
}

// ListMatches retrieves all matches with pagination and filtering
func ListMatches(c *fiber.Ctx) error {
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
	query := database.DB.Model(&models.Match{})

	// Apply league filter if provided
	if leagueID := c.Query("league_id"); leagueID != "" {
		query = query.Where("league_id = ?", leagueID)
	}

	// Apply other filters
	if clubID := c.Query("club_id"); clubID != "" {
		query = query.Where("home_club_id = ? OR away_club_id = ?", clubID, clubID)
	}
	if homeClubID := c.Query("home_club_id"); homeClubID != "" {
		query = query.Where("home_club_id = ?", homeClubID)
	}
	if awayClubID := c.Query("away_club_id"); awayClubID != "" {
		query = query.Where("away_club_id = ?", awayClubID)
	}
	if dateFrom := c.Query("date_from"); dateFrom != "" {
		query = query.Where("date >= ?", dateFrom)
	}
	if dateTo := c.Query("date_to"); dateTo != "" {
		query = query.Where("date <= ?", dateTo)
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to count matches",
		})
	}

	// Get paginated results with ALL relationships preloaded
	var matches []models.Match
	if err := query.
		Preload("League").
		Preload("HomeClub").
		Preload("AwayClub").
		Preload("Formations").
		Preload("Formations.Lineup").
		Preload("Formations.Substitutes").
		Preload("Formations.Unavailable").
		Preload("Events").
		Limit(perPage).Offset(offset).
		Order("date DESC").
		Find(&matches).Error; err != nil {

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list matches",
		})
	}

	// Convert to response format WITH formations
	matchList := make([]fiber.Map, len(matches))
	for i, match := range matches {
		formationsList := make([]fiber.Map, len(match.Formations))
		for j, formation := range match.Formations {
			lineupList := make([]fiber.Map, len(formation.Lineup))
			for k, lp := range formation.Lineup {
				lineupList[k] = fiber.Map{
					"id":            lp.ID,
					"player_id":     lp.PlayerID,
					"name":          lp.Name,
					"jersey_number": lp.JerseyNumber,
					"position":      lp.Position,
					"x":             lp.X,
					"y":             lp.Y,
				}
			}

			subsList := make([]fiber.Map, len(formation.Substitutes))
			for k, s := range formation.Substitutes {
				subsList[k] = fiber.Map{
					"id":            s.ID,
					"player_id":     s.PlayerID,
					"name":          s.Name,
					"jersey_number": s.JerseyNumber,
					"position":      s.Position,
				}
			}

			unavailableList := make([]fiber.Map, len(formation.Unavailable))
			for k, u := range formation.Unavailable {
				unavailableList[k] = fiber.Map{
					"id":            u.ID,
					"player_id":     u.PlayerID,
					"name":          u.Name,
					"jersey_number": u.JerseyNumber,
					"position":      u.Position,
					"reason":        u.Reason,
					"details":       u.Details,
				}
			}

			formationsList[j] = fiber.Map{
				"id":          formation.ID,
				"uuid":        formation.UUID,
				"match_id":    formation.MatchID,
				"club_id":     formation.ClubID,
				"formation":   formation.Formation,
				"lineup":      lineupList,
				"substitutes": subsList,
				"unavailable": unavailableList,
			}
		}

		matchList[i] = fiber.Map{
			"id":           match.ID,
			"uuid":         match.UUID,
			"league_id":    match.LeagueID,
			"home_club_id": match.HomeClubID,
			"away_club_id": match.AwayClubID,
			"date":         match.Date,
			"score_home":   match.ScoreHome,
			"score_away":   match.ScoreAway,
			"event_count":  len(match.Events),
			"formations":   formationsList,
			"created_at":   match.CreatedAt,
		}
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	return c.JSON(fiber.Map{
		"matches":     matchList,
		"total":       total,
		"page":        page,
		"per_page":    perPage,
		"total_pages": totalPages,
	})
}

// CreateMatch creates a new match
func CreateMatch(c *fiber.Ctx) error {
	var req struct {
		LeagueID   uint   `json:"league_id"`
		HomeClubID uint   `json:"home_club_id"`
		AwayClubID uint   `json:"away_club_id"`
		Date       string `json:"date"` // RFC3339 or YYYY-MM-DD
		ScoreHome  *int   `json:"score_home"`
		ScoreAway  *int   `json:"score_away"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if req.LeagueID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "league_id is required",
		})
	}
	if req.HomeClubID == 0 || req.AwayClubID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Home club ID and away club ID are required",
		})
	}
	if req.HomeClubID == req.AwayClubID {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Home club and away club must be different",
		})
	}

	// Parse date
	matchDate, err := time.Parse(time.RFC3339, req.Date)
	if err != nil {
		matchDate, err = time.Parse("2006-01-02", req.Date)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid date format. Use YYYY-MM-DD or RFC3339",
			})
		}
	}

	// Verify league exists
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

	// Verify clubs exist
	var homeClub, awayClub models.Club
	if err := database.DB.First(&homeClub, req.HomeClubID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Home club not found",
		})
	}
	if err := database.DB.First(&awayClub, req.AwayClubID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Away club not found",
		})
	}

	// Create match with league_id properly set
	leagueID := req.LeagueID // Create a copy for the pointer
	match := models.Match{
		HomeClubID: req.HomeClubID,
		AwayClubID: req.AwayClubID,
		LeagueID:   &leagueID, // Use pointer to the copied value
		Date:       matchDate,
		ScoreHome:  req.ScoreHome,
		ScoreAway:  req.ScoreAway,
	}

	if err := database.DB.Create(&match).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create match",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Match created successfully",
		"match": fiber.Map{
			"id":           match.ID,
			"uuid":         match.UUID,
			"league_id":    match.LeagueID,
			"home_club_id": match.HomeClubID,
			"away_club_id": match.AwayClubID,
			"date":         match.Date,
			"score_home":   match.ScoreHome,
			"score_away":   match.ScoreAway,
		},
	})
}

// UpdateMatch updates a match's information
func UpdateMatch(c *fiber.Ctx) error {
	id := c.Params("id")

	matchID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid match ID",
		})
	}

	var req struct {
		LeagueID   *uint   `json:"league_id,omitempty"` // ✅ allow updating league (optional)
		HomeClubID *uint   `json:"home_club_id,omitempty"`
		AwayClubID *uint   `json:"away_club_id,omitempty"`
		Date       *string `json:"date,omitempty"`
		ScoreHome  *int    `json:"score_home,omitempty"`
		ScoreAway  *int    `json:"score_away,omitempty"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Find match
	var match models.Match
	if err := database.DB.First(&match, matchID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Match not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	updates := make(map[string]interface{})

	// Update league_id if provided (optional)
	if req.LeagueID != nil {
		if *req.LeagueID == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "league_id must be a valid number",
			})
		}
		var league models.League
		if err := database.DB.First(&league, *req.LeagueID).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "League not found",
			})
		}
		updates["league_id"] = *req.LeagueID
	}

	// Update home_club_id if provided
	if req.HomeClubID != nil {
		var club models.Club
		if err := database.DB.First(&club, *req.HomeClubID).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Home club not found",
			})
		}
		updates["home_club_id"] = *req.HomeClubID
	}

	// Update away_club_id if provided
	if req.AwayClubID != nil {
		var club models.Club
		if err := database.DB.First(&club, *req.AwayClubID).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Away club not found",
			})
		}
		updates["away_club_id"] = *req.AwayClubID
	}

	// Validate home and away are different
	homeID := match.HomeClubID
	awayID := match.AwayClubID
	if req.HomeClubID != nil {
		homeID = *req.HomeClubID
	}
	if req.AwayClubID != nil {
		awayID = *req.AwayClubID
	}
	if homeID == awayID {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Home club and away club must be different",
		})
	}

	// Update date if provided
	if req.Date != nil {
		matchDate, err := time.Parse(time.RFC3339, *req.Date)
		if err != nil {
			matchDate, err = time.Parse("2006-01-02", *req.Date)
			if err != nil {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "Invalid date format",
				})
			}
		}
		updates["date"] = matchDate
	}

	// Update scores if provided
	if req.ScoreHome != nil {
		updates["score_home"] = *req.ScoreHome
	}
	if req.ScoreAway != nil {
		updates["score_away"] = *req.ScoreAway
	}

	// Perform update if there are changes
	if len(updates) > 0 {
		if err := database.DB.Model(&match).Updates(updates).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update match",
			})
		}
	}

	// Reload match
	database.DB.First(&match, matchID)

	return c.JSON(fiber.Map{
		"message": "Match updated successfully",
		"match": fiber.Map{
			"id":           match.ID,
			"uuid":         match.UUID,
			"league_id":    match.LeagueID,
			"home_club_id": match.HomeClubID,
			"away_club_id": match.AwayClubID,
			"date":         match.Date,
			"score_home":   match.ScoreHome,
			"score_away":   match.ScoreAway,
		},
	})
}

// DeleteMatch soft deletes a match
func DeleteMatch(c *fiber.Ctx) error {
	id := c.Params("id")

	matchID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid match ID",
		})
	}

	result := database.DB.Delete(&models.Match{}, matchID)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete match",
		})
	}

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Match not found",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Match deleted successfully",
	})
}

// GetMatchesByClub retrieves all matches for a specific club
func GetMatchesByClub(c *fiber.Ctx) error {
	clubID := c.Params("club_id")

	var matches []models.Match
	if err := database.DB.
		Preload("League").
		Preload("Formations").
		Preload("Formations.Lineup").
		Preload("Formations.Substitutes").
		Preload("Formations.Unavailable").
		Where("home_club_id = ? OR away_club_id = ?", clubID, clubID).
		Order("date DESC").
		Find(&matches).Error; err != nil {

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get matches",
		})
	}

	matchList := make([]fiber.Map, len(matches))
	for i, match := range matches {
		formationsList := make([]fiber.Map, len(match.Formations))

		for j, formation := range match.Formations {
			lineupList := make([]fiber.Map, len(formation.Lineup))
			for k, lp := range formation.Lineup {
				lineupList[k] = fiber.Map{
					"id":            lp.ID,
					"player_id":     lp.PlayerID,
					"name":          lp.Name,
					"jersey_number": lp.JerseyNumber,
					"position":      lp.Position,
					"x":             lp.X,
					"y":             lp.Y,
				}
			}

			subsList := make([]fiber.Map, len(formation.Substitutes))
			for k, s := range formation.Substitutes {
				subsList[k] = fiber.Map{
					"id":            s.ID,
					"player_id":     s.PlayerID,
					"name":          s.Name,
					"jersey_number": s.JerseyNumber,
					"position":      s.Position,
				}
			}

			unavailableList := make([]fiber.Map, len(formation.Unavailable))
			for k, u := range formation.Unavailable {
				unavailableList[k] = fiber.Map{
					"id":            u.ID,
					"player_id":     u.PlayerID,
					"name":          u.Name,
					"jersey_number": u.JerseyNumber,
					"position":      u.Position,
					"reason":        u.Reason,
					"details":       u.Details,
				}
			}

			formationsList[j] = fiber.Map{
				"id":          formation.ID,
				"uuid":        formation.UUID,
				"match_id":    formation.MatchID,
				"club_id":     formation.ClubID,
				"formation":   formation.Formation,
				"lineup":      lineupList,
				"substitutes": subsList,
				"unavailable": unavailableList,
			}
		}

		matchList[i] = fiber.Map{
			"id":           match.ID,
			"uuid":         match.UUID,
			"league_id":    match.LeagueID,
			"home_club_id": match.HomeClubID,
			"away_club_id": match.AwayClubID,
			"formations":   formationsList,
			"date":         match.Date,
			"score_home":   match.ScoreHome,
			"score_away":   match.ScoreAway,
		}
	}

	return c.JSON(fiber.Map{
		"matches": matchList,
		"count":   len(matchList),
	})
}

// GetUpcomingMatches retrieves upcoming matches
func GetUpcomingMatches(c *fiber.Ctx) error {
	var matches []models.Match
	if err := database.DB.
		Where("date > ?", time.Now()).
		Order("date ASC").
		Limit(10).
		Find(&matches).Error; err != nil {

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get upcoming matches",
		})
	}

	matchList := make([]fiber.Map, len(matches))
	for i, match := range matches {
		matchList[i] = fiber.Map{
			"id":           match.ID,
			"uuid":         match.UUID,
			"league_id":    match.LeagueID,
			"home_club_id": match.HomeClubID,
			"away_club_id": match.AwayClubID,
			"date":         match.Date,
		}
	}

	return c.JSON(fiber.Map{
		"matches": matchList,
		"count":   len(matchList),
	})
}

// GetMatchSummary retrieves a complete match summary
func GetMatchSummary(c *fiber.Ctx) error {
	matchID := c.Params("match_id")

	var match models.Match
	if err := database.DB.
		Preload("League").
		Preload("Formations").
		Preload("Formations.Lineup").
		Preload("Events").
		First(&match, matchID).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Match not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	return c.JSON(fiber.Map{
		"match": fiber.Map{
			"id":           match.ID,
			"uuid":         match.UUID,
			"league_id":    match.LeagueID,
			"home_club_id": match.HomeClubID,
			"away_club_id": match.AwayClubID,
			"date":         match.Date,
			"score_home":   match.ScoreHome,
			"score_away":   match.ScoreAway,
		},
		"formations": match.Formations,
		"events":     match.Events,
	})
}
