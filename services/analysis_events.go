// services/analysis_events.go
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

func ListAnalysisEvents(c *fiber.Ctx) error {
	matchID := c.Params("match_id")

	var events []models.AnalysisEvent
	if err := database.DB.
		Where("match_id = ?", matchID).
		Preload("Stats").
		Order("timestamp_seconds ASC").
		Find(&events).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to list analysis events",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"events": events,
			"count":  len(events),
		},
	})
}

func GetAnalysisEventByID(c *fiber.Ctx) error {
	matchID := c.Params("match_id")
	eventID := c.Params("id")

	var event models.AnalysisEvent
	if err := database.DB.
		Where("match_id = ? AND id = ?", matchID, eventID).
		Preload("Stats").
		First(&event).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"error":   "Analysis event not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Database error",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"event": event,
		},
	})
}

func CreateAnalysisEvent(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	matchID := c.Params("match_id")
	matchIDUint, err := strconv.ParseUint(matchID, 10, 32)
	if err != nil || matchIDUint == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid match_id",
		})
	}

	var req struct {
		VideoID          uint    `json:"video_id"`
		Type             string  `json:"type"`
		TimestampSeconds float64 `json:"timestamp_seconds"`
		PlayerID         *uint   `json:"player_id"`
		PlayerName       *string `json:"player_name"`
		Notes            *string `json:"notes"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.Type == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Event type is required",
		})
	}
	if req.VideoID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "video_id is required",
		})
	}

	if req.PlayerID != nil && *req.PlayerID == 0 {
		req.PlayerID = nil
	}

	// Set default player name if not provided
	playerName := "Team Event"
	if req.PlayerName != nil && *req.PlayerName != "" {
		playerName = *req.PlayerName
	}

	// VideoID is a *uint on the model because an event may be tagged before a
	// video is attached; this handler still requires one, checked above.
	videoID := req.VideoID

	event := models.AnalysisEvent{
		MatchID:          uint(matchIDUint),
		VideoID:          &videoID,
		Type:             req.Type,
		TimestampSeconds: req.TimestampSeconds,
		PlayerID:         req.PlayerID,
		PlayerName:       &playerName,
		Notes:            req.Notes,
		CreatedBy:        user.ID,
	}

	if err := database.DB.Create(&event).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create analysis event",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"event": event,
		},
	})
}

func UpdateAnalysisEvent(c *fiber.Ctx) error {
	matchID := c.Params("match_id")
	eventID := c.Params("id")

	var event models.AnalysisEvent
	if err := database.DB.
		Where("match_id = ? AND id = ?", matchID, eventID).
		First(&event).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Analysis event not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	var req struct {
		Type             *string  `json:"type"`
		TimestampSeconds *float64 `json:"timestamp_seconds"`
		PlayerID         *uint    `json:"player_id"`
		PlayerName       *string  `json:"player_name"`
		ClipURL          *string  `json:"clip_url"`
		Notes            *string  `json:"notes"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Update only the fields the caller supplied. Save would write every column,
	// including uuid and created_by, which on rows predating this model would
	// overwrite a NULL uuid with the zero UUID and collide on its unique index.
	updates := map[string]any{}

	if req.Type != nil {
		event.Type = *req.Type
		updates["type"] = event.Type
	}
	if req.TimestampSeconds != nil {
		event.TimestampSeconds = *req.TimestampSeconds
		updates["timestamp_seconds"] = event.TimestampSeconds
	}
	if req.PlayerID != nil {
		event.PlayerID = req.PlayerID
	}
	if req.PlayerName != nil {
		event.PlayerName = req.PlayerName
		updates["player_name"] = event.PlayerName
	}
	if req.ClipURL != nil {
		event.ClipURL = req.ClipURL
		updates["clip_url"] = event.ClipURL
	}
	if req.Notes != nil {
		event.Notes = req.Notes
	}

	if len(updates) > 0 {
		if err := database.DB.Model(&event).Updates(updates).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update analysis event",
			})
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"event": event,
		},
	})
}

func DeleteAnalysisEvent(c *fiber.Ctx) error {
	matchID := c.Params("match_id")
	eventID := c.Params("id")

	result := database.DB.Where("match_id = ? AND id = ?", matchID, eventID).Delete(&models.AnalysisEvent{})
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete analysis event",
		})
	}

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Analysis event not found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Analysis event deleted",
	})
}
