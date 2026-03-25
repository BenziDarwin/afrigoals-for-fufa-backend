package services

import (
	"errors"
	"strconv"

	"afrigoals.com/database"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func GetFormationByID(c *fiber.Ctx) error {
	id := c.Params("id")

	formationID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid formation ID",
		})
	}

	var formation models.Formation
	if err := database.DB.
		Preload("Lineup").
		First(&formation, formationID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Formation not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	return c.JSON(fiber.Map{
		"formation": fiber.Map{
			"id":         formation.ID,
			"uuid":       formation.UUID,
			"match_id":   formation.MatchID,
			"club_id":    formation.ClubID,
			"formation":  formation.Formation,
			"lineup":     formation.Lineup,
			"created_at": formation.CreatedAt,
		},
	})
}

// GetFormationByUUID retrieves a formation by UUID
func GetFormationByUUID(c *fiber.Ctx) error {
	uuid := c.Params("uuid")

	var formation models.Formation
	if err := database.DB.
		Preload("Lineup").
		Where("uuid = ?", uuid).
		First(&formation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Formation not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	return c.JSON(fiber.Map{
		"formation": fiber.Map{
			"id":        formation.ID,
			"uuid":      formation.UUID,
			"match_id":  formation.MatchID,
			"club_id":   formation.ClubID,
			"formation": formation.Formation,
			"lineup":    formation.Lineup,
		},
	})
}

// ListFormations retrieves all formations with pagination and filtering
func ListFormations(c *fiber.Ctx) error {
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
	query := database.DB.Model(&models.Formation{})

	// Apply filters
	if matchID := c.Query("match_id"); matchID != "" {
		query = query.Where("match_id = ?", matchID)
	}
	if clubID := c.Query("club_id"); clubID != "" {
		query = query.Where("club_id = ?", clubID)
	}
	if formationType := c.Query("formation"); formationType != "" {
		query = query.Where("formation = ?", formationType)
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to count formations",
		})
	}

	// Get paginated results
	var formations []models.Formation
	if err := query.
		Preload("Lineup").
		Limit(perPage).Offset(offset).
		Order("created_at DESC").
		Find(&formations).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list formations",
		})
	}

	// Convert to response format
	formationList := make([]fiber.Map, len(formations))
	for i, formation := range formations {
		formationList[i] = fiber.Map{
			"id":           formation.ID,
			"uuid":         formation.UUID,
			"match_id":     formation.MatchID,
			"club_id":      formation.ClubID,
			"formation":    formation.Formation,
			"lineup_count": len(formation.Lineup),
			"created_at":   formation.CreatedAt,
		}
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	return c.JSON(fiber.Map{
		"formations":  formationList,
		"total":       total,
		"page":        page,
		"per_page":    perPage,
		"total_pages": totalPages,
	})
}

// CreateFormation creates a new formation with lineup + substitutes + unavailable
func CreateFormation(c *fiber.Ctx) error {
	var req struct {
		MatchID      uint   `json:"match_id"`
		ClubID       uint   `json:"club_id"`
		Formation    string `json:"formation"`

		Lineup []struct {
			PlayerID     uint    `json:"player_id"`
			Name         string  `json:"name"`
			JerseyNumber int     `json:"jersey_number"`
			Position     string  `json:"position"`
			X            float64 `json:"x"`
			Y            float64 `json:"y"`
		} `json:"lineup"`

		Substitutes []struct {
			PlayerID     uint   `json:"player_id"`
			Name         string `json:"name"`
			JerseyNumber int    `json:"jersey_number"`
			Position     string `json:"position"`
		} `json:"substitutes"`

		Unavailable []struct {
			PlayerID     uint   `json:"player_id"`
			Name         string `json:"name"`
			JerseyNumber int    `json:"jersey_number"`
			Position     string `json:"position"`
			Reason       string `json:"reason"`
			Details      string `json:"details"`
		} `json:"unavailable"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// (keep your existing validation + match/club checks unchanged)

	formation := models.Formation{
		MatchID:   req.MatchID,
		ClubID:    req.ClubID,
		Formation: req.Formation,
	}

	// Use a transaction so partial writes don't happen
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&formation).Error; err != nil {
			return err
		}

		// lineup
		for _, p := range req.Lineup {
			lp := models.LineupPlayer{
				FormationID:  formation.ID,
				PlayerID:     p.PlayerID,
				Name:         p.Name,
				JerseyNumber: p.JerseyNumber,
				Position:     p.Position,
				X:            p.X,
				Y:            p.Y,
			}
			if err := tx.Create(&lp).Error; err != nil {
				return err
			}
		}

		// substitutes
		for _, s := range req.Substitutes {
			sub := models.Substitute{
				FormationID:  formation.ID,
				PlayerID:     s.PlayerID,
				Name:         s.Name,
				JerseyNumber: s.JerseyNumber,
				Position:     s.Position,
			}
			if err := tx.Create(&sub).Error; err != nil {
				return err
			}
		}

		// unavailable
		for _, u := range req.Unavailable {
			up := models.UnavailablePlayer{
				FormationID:  formation.ID,
				PlayerID:     u.PlayerID,
				Name:         u.Name,
				JerseyNumber: u.JerseyNumber,
				Position:     u.Position,
				Reason:       u.Reason,
				Details:      u.Details,
			}
			if err := tx.Create(&up).Error; err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create formation"})
	}

	// Reload with relations
	database.DB.
		Preload("Lineup").
		Preload("Substitutes").
		Preload("Unavailable").
		First(&formation, formation.ID)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Formation created successfully",
		"formation": formation, // includes lineup/substitutes/unavailable via json tags
	})
}


// UpdateFormation updates a formation's information
func UpdateFormation(c *fiber.Ctx) error {
	id := c.Params("id")
	formationID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid formation ID"})
	}

	var req struct {
		Formation *string `json:"formation,omitempty"`

		Lineup *[]struct {
			PlayerID     uint    `json:"player_id"`
			Name         string  `json:"name"`
			JerseyNumber int     `json:"jersey_number"`
			Position     string  `json:"position"`
			X            float64 `json:"x"`
			Y            float64 `json:"y"`
		} `json:"lineup,omitempty"`

		Substitutes *[]struct {
			PlayerID     uint   `json:"player_id"`
			Name         string `json:"name"`
			JerseyNumber int    `json:"jersey_number"`
			Position     string `json:"position"`
		} `json:"substitutes,omitempty"`

		Unavailable *[]struct {
			PlayerID     uint   `json:"player_id"`
			Name         string `json:"name"`
			JerseyNumber int    `json:"jersey_number"`
			Position     string `json:"position"`
			Reason       string `json:"reason"`
			Details      string `json:"details"`
		} `json:"unavailable,omitempty"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	var formation models.Formation
	if err := database.DB.First(&formation, uint(formationID)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Formation not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		// formation type
		if req.Formation != nil {
			if err := tx.Model(&formation).Updates(map[string]interface{}{
				"formation": *req.Formation,
			}).Error; err != nil {
				return err
			}
		}

		// lineup
		if req.Lineup != nil {
			if err := tx.Where("formation_id = ?", formation.ID).Delete(&models.LineupPlayer{}).Error; err != nil {
				return err
			}
			for _, p := range *req.Lineup {
				lp := models.LineupPlayer{
					FormationID:  formation.ID,
					PlayerID:     p.PlayerID,
					Name:         p.Name,
					JerseyNumber: p.JerseyNumber,
					Position:     p.Position,
					X:            p.X,
					Y:            p.Y,
				}
				if err := tx.Create(&lp).Error; err != nil {
					return err
				}
			}
		}

		// substitutes
		if req.Substitutes != nil {
			if err := tx.Where("formation_id = ?", formation.ID).Delete(&models.Substitute{}).Error; err != nil {
				return err
			}
			for _, s := range *req.Substitutes {
				sub := models.Substitute{
					FormationID:  formation.ID,
					PlayerID:     s.PlayerID,
					Name:         s.Name,
					JerseyNumber: s.JerseyNumber,
					Position:     s.Position,
				}
				if err := tx.Create(&sub).Error; err != nil {
					return err
				}
			}
		}

		// unavailable
		if req.Unavailable != nil {
			if err := tx.Where("formation_id = ?", formation.ID).Delete(&models.UnavailablePlayer{}).Error; err != nil {
				return err
			}
			for _, u := range *req.Unavailable {
				up := models.UnavailablePlayer{
					FormationID:  formation.ID,
					PlayerID:     u.PlayerID,
					Name:         u.Name,
					JerseyNumber: u.JerseyNumber,
					Position:     u.Position,
					Reason:       u.Reason,
					Details:      u.Details,
				}
				if err := tx.Create(&up).Error; err != nil {
					return err
				}
			}
		}

		return nil
	}); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update formation"})
	}

	database.DB.
		Preload("Lineup").
		Preload("Substitutes").
		Preload("Unavailable").
		First(&formation, uint(formationID))

	return c.JSON(fiber.Map{
		"message":   "Formation updated successfully",
		"formation": formation,
	})
}


// DeleteFormation soft deletes a formation and its lineup
func DeleteFormation(c *fiber.Ctx) error {
	id := c.Params("id")

	formationID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid formation ID",
		})
	}

	// Delete lineup players first (cascade delete)
	if err := database.DB.Where("formation_id = ?", formationID).Delete(&models.LineupPlayer{}).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete lineup players",
		})
	}

	// Delete formation
	result := database.DB.Delete(&models.Formation{}, formationID)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete formation",
		})
	}

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Formation not found",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Formation deleted successfully",
	})
}

// ========================================
// FORMATION QUERIES
// ========================================

// GetFormationsByMatch retrieves all formations for a specific match
func GetFormationsByMatch(c *fiber.Ctx) error {
	matchID := c.Params("match_id")

	var formations []models.Formation
	if err := database.DB.
		Preload("Lineup").
		Where("match_id = ?", matchID).
		Find(&formations).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get formations",
		})
	}

	formationList := make([]fiber.Map, len(formations))
	for i, formation := range formations {
		formationList[i] = fiber.Map{
			"id":        formation.ID,
			"uuid":      formation.UUID,
			"match_id":  formation.MatchID,
			"club_id":   formation.ClubID,
			"formation": formation.Formation,
			"lineup":    formation.Lineup,
		}
	}

	return c.JSON(fiber.Map{
		"formations": formationList,
		"count":      len(formationList),
	})
}

// GetFormationsByClub retrieves all formations for a specific club
func GetFormationsByClub(c *fiber.Ctx) error {
	clubID := c.Params("club_id")

	var formations []models.Formation
	if err := database.DB.
		Preload("Lineup").
		Where("club_id = ?", clubID).
		Order("created_at DESC").
		Find(&formations).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get formations",
		})
	}

	formationList := make([]fiber.Map, len(formations))
	for i, formation := range formations {
		formationList[i] = fiber.Map{
			"id":        formation.ID,
			"uuid":      formation.UUID,
			"match_id":  formation.MatchID,
			"club_id":   formation.ClubID,
			"formation": formation.Formation,
			"lineup":    formation.Lineup,
		}
	}

	return c.JSON(fiber.Map{
		"formations": formationList,
		"count":      len(formationList),
	})
}

// GetFormationByMatchAndClub retrieves a formation for a specific match and club
func GetFormationByMatchAndClub(c *fiber.Ctx) error {
	matchID := c.Params("match_id")
	clubID := c.Params("club_id")

	var formation models.Formation
	if err := database.DB.
		Preload("Lineup").
		Where("match_id = ? AND club_id = ?", matchID, clubID).
		First(&formation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Formation not found for this match and club",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	return c.JSON(fiber.Map{
		"formation": fiber.Map{
			"id":        formation.ID,
			"uuid":      formation.UUID,
			"match_id":  formation.MatchID,
			"club_id":   formation.ClubID,
			"formation": formation.Formation,
			"lineup":    formation.Lineup,
		},
	})
}

// ========================================
// LINEUP PLAYER OPERATIONS
// ========================================

// GetLineupPlayer retrieves a specific lineup player
func GetLineupPlayer(c *fiber.Ctx) error {
	id := c.Params("id")

	lineupPlayerID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid lineup player ID",
		})
	}

	var lineupPlayer models.LineupPlayer
	if err := database.DB.First(&lineupPlayer, lineupPlayerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Lineup player not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	return c.JSON(fiber.Map{
		"lineup_player": fiber.Map{
			"id":            lineupPlayer.ID,
			"uuid":          lineupPlayer.UUID,
			"formation_id":  lineupPlayer.FormationID,
			"player_id":     lineupPlayer.PlayerID,
			"name":          lineupPlayer.Name,
			"jersey_number": lineupPlayer.JerseyNumber,
			"position":      lineupPlayer.Position,
			"x":             lineupPlayer.X,
			"y":             lineupPlayer.Y,
		},
	})
}

// UpdateLineupPlayer updates a specific lineup player position
func UpdateLineupPlayer(c *fiber.Ctx) error {
	id := c.Params("id")

	lineupPlayerID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid lineup player ID",
		})
	}

	var req struct {
		Position *string  `json:"position,omitempty"`
		X        *float64 `json:"x,omitempty"`
		Y        *float64 `json:"y,omitempty"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Find lineup player
	var lineupPlayer models.LineupPlayer
	if err := database.DB.First(&lineupPlayer, lineupPlayerID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Lineup player not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	updates := make(map[string]interface{})

	if req.Position != nil {
		updates["position"] = *req.Position
	}
	if req.X != nil {
		updates["x"] = *req.X
	}
	if req.Y != nil {
		updates["y"] = *req.Y
	}

	// Perform update if there are changes
	if len(updates) > 0 {
		if err := database.DB.Model(&lineupPlayer).Updates(updates).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update lineup player",
			})
		}
	}

	// Reload lineup player
	database.DB.First(&lineupPlayer, lineupPlayerID)

	return c.JSON(fiber.Map{
		"message": "Lineup player updated successfully",
		"lineup_player": fiber.Map{
			"id":            lineupPlayer.ID,
			"uuid":          lineupPlayer.UUID,
			"formation_id":  lineupPlayer.FormationID,
			"player_id":     lineupPlayer.PlayerID,
			"name":          lineupPlayer.Name,
			"jersey_number": lineupPlayer.JerseyNumber,
			"position":      lineupPlayer.Position,
			"x":             lineupPlayer.X,
			"y":             lineupPlayer.Y,
		},
	})
}

// DeleteLineupPlayer removes a player from a lineup
func DeleteLineupPlayer(c *fiber.Ctx) error {
	id := c.Params("id")

	lineupPlayerID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid lineup player ID",
		})
	}

	result := database.DB.Delete(&models.LineupPlayer{}, lineupPlayerID)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete lineup player",
		})
	}

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Lineup player not found",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Lineup player deleted successfully",
	})
}
