package services

import (
	"errors"
	"strconv"

	"afrigoals.com/database"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// CreateMatchEvent creates a new match event
func CreateMatchEvent(c *fiber.Ctx) error {
	var req struct {
		MatchID      uint    `json:"match_id"`
		Minute       int     `json:"minute"`
		Type         string  `json:"type"` // e.g., "goal", "yellow_card", "red_card", "substitution"
		Team         *string `json:"team"`
		Description  string  `json:"description"`
		Player       *string `json:"player"`
		JerseyNumber *int    `json:"jersey_number"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate required fields
	if req.MatchID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Match ID is required",
		})
	}

	if req.Type == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Event type is required",
		})
	}

	// Verify match exists
	var match models.Match
	if err := database.DB.First(&match, req.MatchID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Match not found",
		})
	}

	// Create event
	event := models.MatchEvent{
		MatchID:      req.MatchID,
		Minute:       req.Minute,
		Type:         req.Type,
		Team:         req.Team,
		Description:  req.Description,
		Player:       req.Player,
		JerseyNumber: req.JerseyNumber,
	}

	if err := database.DB.Create(&event).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create match event",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Match event created successfully",
		"event": fiber.Map{
			"id":            event.ID,
			"uuid":          event.UUID,
			"match_id":      event.MatchID,
			"minute":        event.Minute,
			"type":          event.Type,
			"team":          event.Team,
			"description":   event.Description,
			"player":        event.Player,
			"jersey_number": event.JerseyNumber,
		},
	})
}

// GetMatchEvents retrieves all events for a match
func GetMatchEvents(c *fiber.Ctx) error {
	matchID := c.Params("match_id")

	var events []models.MatchEvent
	if err := database.DB.
		Where("match_id = ?", matchID).
		Order("minute ASC").
		Find(&events).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get match events",
		})
	}

	return c.JSON(fiber.Map{
		"events": events,
		"count":  len(events),
	})
}

// UpdateMatchEvent updates a match event
func UpdateMatchEvent(c *fiber.Ctx) error {
	id := c.Params("id")

	eventID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid event ID",
		})
	}

	var req struct {
		Minute       *int    `json:"minute,omitempty"`
		Type         *string `json:"type,omitempty"`
		Team         *string `json:"team,omitempty"`
		Description  *string `json:"description,omitempty"`
		Player       *string `json:"player,omitempty"`
		JerseyNumber *int    `json:"jersey_number,omitempty"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Find event
	var event models.MatchEvent
	if err := database.DB.First(&event, eventID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Match event not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	updates := make(map[string]interface{})

	if req.Minute != nil {
		updates["minute"] = *req.Minute
	}
	if req.Type != nil {
		updates["type"] = *req.Type
	}
	if req.Team != nil {
		updates["team"] = *req.Team
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Player != nil {
		updates["player"] = *req.Player
	}
	if req.JerseyNumber != nil {
		updates["jersey_number"] = *req.JerseyNumber
	}

	// Perform update if there are changes
	if len(updates) > 0 {
		if err := database.DB.Model(&event).Updates(updates).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update match event",
			})
		}
	}

	// Reload event
	database.DB.First(&event, eventID)

	return c.JSON(fiber.Map{
		"message": "Match event updated successfully",
		"event": fiber.Map{
			"id":            event.ID,
			"uuid":          event.UUID,
			"match_id":      event.MatchID,
			"minute":        event.Minute,
			"type":          event.Type,
			"team":          event.Team,
			"description":   event.Description,
			"player":        event.Player,
			"jersey_number": event.JerseyNumber,
		},
	})
}

// DeleteMatchEvent deletes a match event
func DeleteMatchEvent(c *fiber.Ctx) error {
	id := c.Params("id")

	eventID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid event ID",
		})
	}

	result := database.DB.Delete(&models.MatchEvent{}, eventID)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete match event",
		})
	}

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Match event not found",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Match event deleted successfully",
	})
}
