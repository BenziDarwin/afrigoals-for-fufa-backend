// services/analysis_events.go
package services

import (
	"context"
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
	q := database.DB.Where("match_id = ?", matchID)
	q = applyAnalysisEventFilters(c, q)

	if err := q.Preload("Stats").
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

func applyAnalysisEventFilters(c *fiber.Ctx, q *gorm.DB) *gorm.DB {
	if types := splitCSVQuery(c.Query("type")); len(types) > 0 {
		q = q.Where("type IN ?", types)
	}
	if eventTypeIDs := parseUintCSVQuery(c.Query("event_type_id")); len(eventTypeIDs) > 0 {
		q = q.Where("event_type_id IN ?", eventTypeIDs)
	}
	if teamIDs := parseUintCSVQuery(c.Query("team_id")); len(teamIDs) > 0 {
		q = q.Where("team_id IN ?", teamIDs)
	}
	if playerIDs := parseUintCSVQuery(c.Query("player_id")); len(playerIDs) > 0 {
		q = q.Where("player_id IN ?", playerIDs)
	}
	if videoIDs := parseUintCSVQuery(c.Query("video_id")); len(videoIDs) > 0 {
		q = q.Where("video_id IN ?", videoIDs)
	}
	if outcomes := splitCSVQuery(c.Query("outcome")); len(outcomes) > 0 {
		q = q.Where("outcome IN ?", outcomes)
	}
	if pitchZones := splitCSVQuery(c.Query("pitch_zone")); len(pitchZones) > 0 {
		q = q.Where("pitch_zone IN ?", pitchZones)
	}
	if minTimestamp, err := strconv.ParseFloat(strings.TrimSpace(c.Query("min_timestamp_seconds")), 64); err == nil {
		q = q.Where("timestamp_seconds >= ?", minTimestamp)
	}
	if maxTimestamp, err := strconv.ParseFloat(strings.TrimSpace(c.Query("max_timestamp_seconds")), 64); err == nil {
		q = q.Where("timestamp_seconds <= ?", maxTimestamp)
	}
	if hasClip := strings.TrimSpace(c.Query("has_clip")); hasClip != "" {
		if value, err := strconv.ParseBool(hasClip); err == nil {
			if value {
				q = q.Where("clip_url IS NOT NULL AND clip_url <> ''")
			} else {
				q = q.Where("clip_url IS NULL OR clip_url = ''")
			}
		}
	}
	return q
}

func splitCSVQuery(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseUintCSVQuery(value string) []uint {
	parts := splitCSVQuery(value)
	out := make([]uint, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.ParseUint(part, 10, 32)
		if err == nil && id > 0 {
			out = append(out, uint(id))
		}
	}
	return out
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
			"error":   "Failed to create analysis event",
			"details": err.Error(),
		})
	}

	eventManifestKey, eventManifestWarning := "", ""
	if key, err := saveAnalysisEventManifestToR2(context.Background(), event); err != nil {
		eventManifestWarning = err.Error()
	} else {
		eventManifestKey = key
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"event":                  event,
			"event_manifest_key":     eventManifestKey,
			"event_manifest_warning": eventManifestWarning,
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
				"error":   "Failed to update analysis event",
				"details": err.Error(),
			})
		}
	}

	_ = database.DB.Where("match_id = ? AND id = ?", matchID, eventID).First(&event).Error
	eventManifestKey, eventManifestWarning := "", ""
	if key, err := saveAnalysisEventManifestToR2(context.Background(), event); err != nil {
		eventManifestWarning = err.Error()
	} else {
		eventManifestKey = key
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"event":                  event,
			"event_manifest_key":     eventManifestKey,
			"event_manifest_warning": eventManifestWarning,
		},
	})
}

func DeleteAnalysisEvent(c *fiber.Ctx) error {
	matchID := c.Params("match_id")
	eventID := c.Params("id")

	var event models.AnalysisEvent
	_ = database.DB.Where("match_id = ? AND id = ?", matchID, eventID).First(&event).Error

	result := database.DB.Where("match_id = ? AND id = ?", matchID, eventID).Delete(&models.AnalysisEvent{})
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to delete analysis event",
			"details": result.Error.Error(),
		})
	}

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Analysis event not found",
		})
	}

	eventManifestWarning := ""
	if event.ID != 0 {
		if err := deleteAnalysisEventManifestFromR2(context.Background(), event.MatchID, event.ID); err != nil {
			eventManifestWarning = err.Error()
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Analysis event deleted",
		"data": fiber.Map{
			"event_manifest_warning": eventManifestWarning,
		},
	})
}
