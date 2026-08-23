package services

import (
	"errors"
	"strconv"

	"afrigoals.com/database"
	"afrigoals.com/middleware"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// GetLeagueAnalysts retrieves all data analysts for a specific league
func GetLeagueAnalysts(c *fiber.Ctx) error {
	leagueID := c.Params("league_id")

	var analysts []models.User
	if err := database.DB.
		Where("league_id = ? AND role = ?", leagueID, models.DataAnalyst).
		Find(&analysts).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get league analysts",
		})
	}

	analystList := make([]fiber.Map, len(analysts))
	for i, analyst := range analysts {
		analystList[i] = fiber.Map{
			"id":         analyst.ID,
			"uuid":       analyst.UUID.String(),
			"email":      analyst.Email,
			"role":       analyst.Role,
			"league_id":  analyst.LeagueID,
			"created_at": analyst.CreatedAt,
		}
	}

	return c.JSON(fiber.Map{
		"success":  true,
		"analysts": analystList,
		"count":    len(analystList),
	})
}

// AssignAnalystToMatch assigns a data analyst to a match
func AssignAnalystToMatch(c *fiber.Ctx) error {
	// Get match_id from URL params
	matchIDParam := c.Params("match_id")
	if matchIDParam == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Match ID is required in URL",
		})
	}

	matchIDInt, err := strconv.Atoi(matchIDParam)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid match ID format",
		})
	}
	matchID := uint(matchIDInt)

	var req struct {
		AnalystID uint   `json:"analyst_id"`
		Notes     string `json:"notes"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.AnalystID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Analyst ID is required",
		})
	}

	// Get current user (league manager)
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	// Verify match exists
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

	// Verify analyst exists
	var analyst models.User
	if err := database.DB.First(&analyst, req.AnalystID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Analyst not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	// Verify user is a data analyst
	if analyst.Role != models.DataAnalyst {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User is not a data analyst",
		})
	}

	// Check if analyst is already assigned
	var existingAssignment models.MatchAnalyst
	err = database.DB.
		Where("match_id = ? AND user_id = ?", matchID, req.AnalystID).
		First(&existingAssignment).Error

	if err == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error": "Analyst is already assigned to this match",
		})
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	// Create assignment
	assignment := models.MatchAnalyst{
		MatchID:    matchID,
		UserID:     req.AnalystID,
		AssignedBy: user.ID,
		Notes:      req.Notes,
	}

	if err := database.DB.Create(&assignment).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to assign analyst to match",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Analyst assigned to match successfully",
		"assignment": fiber.Map{
			"match_id":    assignment.MatchID,
			"analyst_id":  assignment.UserID,
			"assigned_by": assignment.AssignedBy,
			"assigned_at": assignment.AssignedAt,
			"notes":       assignment.Notes,
		},
	})
}

// RemoveAnalystFromMatch removes a data analyst from a match
func RemoveAnalystFromMatch(c *fiber.Ctx) error {
	// Get match_id from URL params
	matchIDParam := c.Params("match_id")
	if matchIDParam == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Match ID is required in URL",
		})
	}

	matchIDInt, err := strconv.Atoi(matchIDParam)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid match ID format",
		})
	}
	matchID := uint(matchIDInt)

	var req struct {
		AnalystID uint `json:"analyst_id"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.AnalystID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Analyst ID is required",
		})
	}

	// Verify user is authenticated
	_, err = middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	// Verify match exists
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

	// Remove assignment
	result := database.DB.
		Where("match_id = ? AND user_id = ?", matchID, req.AnalystID).
		Delete(&models.MatchAnalyst{})

	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to remove analyst from match",
		})
	}

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Analyst assignment not found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Analyst removed from match successfully",
	})
}

// GetAllAnalysts retrieves all data analysts in the system
func GetAllAnalysts(c *fiber.Ctx) error {
	var analysts []models.User

	if err := database.DB.
		Where("role = ?", models.DataAnalyst).
		Order("email ASC").
		Find(&analysts).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to get analysts",
		})
	}

	analystList := make([]fiber.Map, len(analysts))
	for i, analyst := range analysts {
		analystList[i] = fiber.Map{
			"id":         analyst.ID,
			"uuid":       analyst.UUID.String(),
			"email":      analyst.Email,
			"role":       analyst.Role,
			"league_id":  analyst.LeagueID,
			"created_at": analyst.CreatedAt,
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    analystList,
		"count":   len(analystList),
	})
}

// GetMatchAnalysts retrieves all analysts assigned to a specific match
func GetMatchAnalysts(c *fiber.Ctx) error {
	matchID := c.Params("match_id")

	var match models.Match
	if err := database.DB.First(&match, matchID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"error":   "Match not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Database error",
		})
	}

	// Join table fetch (guaranteed correct) + role filter
	var analysts []models.User
	if err := database.DB.
		Joins("JOIN match_analysts ma ON ma.user_id = users.id").
		Where("ma.match_id = ?", match.ID).
		Where("users.role = ?", models.DataAnalyst).
		Find(&analysts).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to get match analysts",
		})
	}

	analystList := make([]fiber.Map, len(analysts))
	for i, a := range analysts {
		analystList[i] = fiber.Map{
			"id":    a.ID,
			"uuid":  a.UUID.String(),
			"email": a.Email,
		}
	}

	return c.JSON(fiber.Map{
		"success":  true,
		"analysts": analystList,
		"count":    len(analystList),
	})
}

// GetAvailableAnalysts retrieves all available data analysts
func GetAvailableAnalysts(c *fiber.Ctx) error {
	var analysts []models.User

	if err := database.DB.
		Where("role = ?", models.DataAnalyst).
		Order("created_at DESC").
		Find(&analysts).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to get analysts",
		})
	}

	analystList := make([]fiber.Map, len(analysts))
	for i, a := range analysts {
		analystList[i] = fiber.Map{
			"id":        a.ID,
			"uuid":      a.UUID.String(),
			"email":     a.Email,
			"league_id": a.LeagueID,
		}
	}

	return c.JSON(fiber.Map{
		"success":  true,
		"analysts": analystList,
		"count":    len(analystList),
	})
}

// GetAnalystMatches - Fixed for models that store player data directly
func GetAnalystMatches(c *fiber.Ctx) error {
	analystID := c.Params("analyst_id")

	// Pagination
	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "50"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	offset := (page - 1) * perPage

	var analyst models.User
	if err := database.DB.First(&analyst, analystID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"error":   "Analyst not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Database error",
		})
	}

	// Get total count
	var total int64
	countQuery := database.DB.Model(&models.Match{}).
		Joins("JOIN match_analysts ON match_analysts.match_id = matches.id").
		Where("match_analysts.user_id = ?", analystID)

	if err := countQuery.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to count matches",
		})
	}

	// ✅ FIXED: No .Player preloads since models store player data directly
	var matches []models.Match
	matchQuery := database.DB.
		Joins("JOIN match_analysts ON match_analysts.match_id = matches.id").
		Where("match_analysts.user_id = ?", analystID).
		Preload("HomeClub").
		Preload("AwayClub").
		Preload("League").
		Preload("Formations").
		Preload("Formations.Lineup").      // ✅ This works - has Name, JerseyNumber fields
		Preload("Formations.Substitutes"). // ✅ This works
		Preload("Formations.Unavailable"). // ✅ This works
		Preload("Events").
		Order("matches.date DESC").
		Limit(perPage).
		Offset(offset)

	if err := matchQuery.Find(&matches).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to get matches",
		})
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	matchList := make([]fiber.Map, len(matches))
	for i, match := range matches {
		matchList[i] = buildMatchResponse(match)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"matches":     matchList,
			"total":       total,
			"page":        page,
			"per_page":    perPage,
			"total_pages": totalPages,
		},
	})
}
