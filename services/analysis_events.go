// services/analysis_events.go
package services

import (
	"errors"
	"strconv"
	"strings"

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

func ListEventTypes(c *fiber.Ctx) error {
	category := strings.TrimSpace(c.Query("category"))

	q := database.DB.Model(&models.EventType{}).Order("category ASC, priority ASC, name ASC")
	if category != "" {
		q = q.Where("category = ?", category)
	}

	var eventTypes []models.EventType
	if err := q.Find(&eventTypes).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to list event types",
			"details": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"event_types": eventTypes,
			"count":       len(eventTypes),
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
		VideoID           uint     `json:"video_id"`
		Type              string   `json:"type"`
		EventTypeID       *uint    `json:"event_type_id"`
		TimestampSeconds  float64  `json:"timestamp_seconds"`
		TeamID            *uint    `json:"team_id"`
		PlayerID          *uint    `json:"player_id"`
		SecondaryPlayerID *uint    `json:"secondary_player_id"`
		PlayerName        *string  `json:"player_name"`
		PitchZone         *string  `json:"pitch_zone"`
		Outcome           *string  `json:"outcome"`
		Notes             *string  `json:"notes"`
		ConfidenceScore   *float64 `json:"confidence_score"`
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
	if req.TeamID != nil && *req.TeamID == 0 {
		req.TeamID = nil
	}
	if req.SecondaryPlayerID != nil && *req.SecondaryPlayerID == 0 {
		req.SecondaryPlayerID = nil
	}
	if req.ConfidenceScore != nil {
		if *req.ConfidenceScore < 0 {
			*req.ConfidenceScore = 0
		}
		if *req.ConfidenceScore > 1 {
			*req.ConfidenceScore = 1
		}
	}

	var eventType models.EventType
	if req.EventTypeID != nil && *req.EventTypeID != 0 {
		if err := database.DB.First(&eventType, *req.EventTypeID).Error; err == nil {
			req.Type = eventType.Value
		}
	} else if strings.TrimSpace(req.Type) != "" {
		if err := database.DB.Where("value = ?", req.Type).First(&eventType).Error; err == nil {
			req.EventTypeID = &eventType.ID
		}
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
		MatchID:           uint(matchIDUint),
		VideoID:           &videoID,
		Type:              req.Type,
		EventTypeID:       req.EventTypeID,
		TimestampSeconds:  req.TimestampSeconds,
		TeamID:            req.TeamID,
		PlayerID:          req.PlayerID,
		SecondaryPlayerID: req.SecondaryPlayerID,
		PlayerName:        &playerName,
		PitchZone:         req.PitchZone,
		Outcome:           req.Outcome,
		Notes:             req.Notes,
		ConfidenceScore:   req.ConfidenceScore,
		CreatedBy:         user.ID,
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
		Type              *string  `json:"type"`
		EventTypeID       *uint    `json:"event_type_id"`
		TimestampSeconds  *float64 `json:"timestamp_seconds"`
		TeamID            *uint    `json:"team_id"`
		PlayerID          *uint    `json:"player_id"`
		SecondaryPlayerID *uint    `json:"secondary_player_id"`
		PlayerName        *string  `json:"player_name"`
		PitchZone         *string  `json:"pitch_zone"`
		Outcome           *string  `json:"outcome"`
		ClipURL           *string  `json:"clip_url"`
		Notes             *string  `json:"notes"`
		ConfidenceScore   *float64 `json:"confidence_score"`
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
	if req.EventTypeID != nil {
		event.EventTypeID = req.EventTypeID
		updates["event_type_id"] = event.EventTypeID
	}
	if req.TimestampSeconds != nil {
		event.TimestampSeconds = *req.TimestampSeconds
		updates["timestamp_seconds"] = event.TimestampSeconds
	}
	if req.TeamID != nil {
		event.TeamID = req.TeamID
		updates["team_id"] = event.TeamID
	}
	if req.PlayerID != nil {
		event.PlayerID = req.PlayerID
		updates["player_id"] = event.PlayerID
	}
	if req.SecondaryPlayerID != nil {
		event.SecondaryPlayerID = req.SecondaryPlayerID
		updates["secondary_player_id"] = event.SecondaryPlayerID
	}
	if req.PlayerName != nil {
		event.PlayerName = req.PlayerName
		updates["player_name"] = event.PlayerName
	}
	if req.PitchZone != nil {
		event.PitchZone = req.PitchZone
		updates["pitch_zone"] = event.PitchZone
	}
	if req.Outcome != nil {
		event.Outcome = req.Outcome
		updates["outcome"] = event.Outcome
	}
	if req.ClipURL != nil {
		event.ClipURL = req.ClipURL
		updates["clip_url"] = event.ClipURL
	}
	if req.Notes != nil {
		event.Notes = req.Notes
		updates["notes"] = event.Notes
	}
	if req.ConfidenceScore != nil {
		if *req.ConfidenceScore < 0 {
			*req.ConfidenceScore = 0
		}
		if *req.ConfidenceScore > 1 {
			*req.ConfidenceScore = 1
		}
		event.ConfidenceScore = req.ConfidenceScore
		updates["confidence_score"] = event.ConfidenceScore
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
