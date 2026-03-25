package services

import (
	"errors"
	"strconv"

	"afrigoals.com/database"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// AttachStatsToAnalysisEvent attaches AI model statistics to an analysis event
func AttachStatsToAnalysisEvent(c *fiber.Ctx) error {
	matchIDStr := c.Params("match_id")
	eventIDStr := c.Params("event_id")

	matchID, err := strconv.ParseUint(matchIDStr, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid match ID",
		})
	}

	eventID, err := strconv.ParseUint(eventIDStr, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid event ID",
		})
	}

	var req struct {
		PlayerID         *uint                  `json:"player_id"`
		PlayerName       *string                `json:"player_name"`
		Stats            map[string]interface{} `json:"stats"`
		DistanceCoveredM *float64               `json:"distance_covered_m"`
		AverageSpeedKmh  *float64               `json:"average_speed_kmh"`
		MaxSpeedKmh      *float64               `json:"max_speed_kmh"`
		SprintsCount     *int                   `json:"sprints_count"`
		TouchesCount     *int                   `json:"touches_count"`
		Source           string                 `json:"source"`
		ModelVersion     *string                `json:"model_version"`
		JobID            *string                `json:"job_id"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Verify analysis event exists and belongs to this match
	var event models.AnalysisEvent
	if err := database.DB.Where("id = ? AND match_id = ?", eventID, matchID).
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

	// Default source
	if req.Source == "" {
		req.Source = "ai_model"
	}

	// Create stats record
	stats := models.AnalysisEventStats{
		AnalysisEventID:  uint(eventID),
		PlayerID:         req.PlayerID,
		PlayerName:       req.PlayerName,
		Stats:            req.Stats,
		DistanceCoveredM: req.DistanceCoveredM,
		AverageSpeedKmh:  req.AverageSpeedKmh,
		MaxSpeedKmh:      req.MaxSpeedKmh,
		SprintsCount:     req.SprintsCount,
		TouchesCount:     req.TouchesCount,
		Source:           req.Source,
		ModelVersion:     req.ModelVersion,
		JobID:            req.JobID,
	}

	if err := database.DB.Create(&stats).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to attach statistics",
		})
	}

	// Update event's stats_id reference
	database.DB.Model(&event).Update("stats_id", stats.ID)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Statistics attached successfully",
		"data": fiber.Map{
			"stats": stats,
		},
	})
}

// GetAnalysisEventStats retrieves statistics for an analysis event
func GetAnalysisEventStats(c *fiber.Ctx) error {
	matchIDStr := c.Params("match_id")
	eventIDStr := c.Params("event_id")

	matchID, err := strconv.ParseUint(matchIDStr, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid match ID",
		})
	}

	eventID, err := strconv.ParseUint(eventIDStr, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid event ID",
		})
	}

	// Verify event exists
	var event models.AnalysisEvent
	if err := database.DB.Where("id = ? AND match_id = ?", eventID, matchID).
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

	// Get all stats for this event (there might be multiple versions)
	var stats []models.AnalysisEventStats
	if err := database.DB.Where("analysis_event_id = ?", eventID).
		Order("created_at DESC").
		Find(&stats).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve statistics",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"stats": stats,
			"count": len(stats),
		},
	})
}

// UpdateAnalysisEventStats updates statistics for an event
func UpdateAnalysisEventStats(c *fiber.Ctx) error {
	matchIDStr := c.Params("match_id")
	eventIDStr := c.Params("event_id")
	statsIDStr := c.Params("stats_id")

	matchID, err := strconv.ParseUint(matchIDStr, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid match ID",
		})
	}

	eventID, err := strconv.ParseUint(eventIDStr, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid event ID",
		})
	}

	statsID, err := strconv.ParseUint(statsIDStr, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid stats ID",
		})
	}

	var req struct {
		Stats            map[string]interface{} `json:"stats"`
		DistanceCoveredM *float64               `json:"distance_covered_m"`
		AverageSpeedKmh  *float64               `json:"average_speed_kmh"`
		MaxSpeedKmh      *float64               `json:"max_speed_kmh"`
		SprintsCount     *int                   `json:"sprints_count"`
		TouchesCount     *int                   `json:"touches_count"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Verify stats exist and belong to the event
	var stats models.AnalysisEventStats
	if err := database.DB.
		Joins("JOIN analysis_events ON analysis_events.id = analysis_event_stats.analysis_event_id").
		Where("analysis_event_stats.id = ? AND analysis_events.id = ? AND analysis_events.match_id = ?",
			statsID, eventID, matchID).
		First(&stats).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "Statistics not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database error",
		})
	}

	// Update fields
	updates := make(map[string]interface{})
	if req.Stats != nil {
		updates["stats"] = req.Stats
	}
	if req.DistanceCoveredM != nil {
		updates["distance_covered_m"] = *req.DistanceCoveredM
	}
	if req.AverageSpeedKmh != nil {
		updates["average_speed_kmh"] = *req.AverageSpeedKmh
	}
	if req.MaxSpeedKmh != nil {
		updates["max_speed_kmh"] = *req.MaxSpeedKmh
	}
	if req.SprintsCount != nil {
		updates["sprints_count"] = *req.SprintsCount
	}
	if req.TouchesCount != nil {
		updates["touches_count"] = *req.TouchesCount
	}

	if len(updates) > 0 {
		if err := database.DB.Model(&stats).Updates(updates).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to update statistics",
			})
		}
	}

	// Reload
	database.DB.First(&stats, statsID)

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Statistics updated successfully",
		"data": fiber.Map{
			"stats": stats,
		},
	})
}

// DeleteAnalysisEventStats removes statistics from an event
func DeleteAnalysisEventStats(c *fiber.Ctx) error {
	matchIDStr := c.Params("match_id")
	eventIDStr := c.Params("event_id")
	statsIDStr := c.Params("stats_id")

	matchID, err := strconv.ParseUint(matchIDStr, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid match ID",
		})
	}

	eventID, err := strconv.ParseUint(eventIDStr, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid event ID",
		})
	}

	statsID, err := strconv.ParseUint(statsIDStr, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid stats ID",
		})
	}

	// Verify and delete
	result := database.DB.
		Joins("JOIN analysis_events ON analysis_events.id = analysis_event_stats.analysis_event_id").
		Where("analysis_event_stats.id = ? AND analysis_events.id = ? AND analysis_events.match_id = ?",
			statsID, eventID, matchID).
		Delete(&models.AnalysisEventStats{})

	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete statistics",
		})
	}

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Statistics not found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Statistics deleted successfully",
	})
}

// BatchAttachStatsFromJobResult processes AI job results and attaches stats to multiple events
func BatchAttachStatsFromJobResult(c *fiber.Ctx) error {
	matchIDStr := c.Params("match_id")

	matchID, err := strconv.ParseUint(matchIDStr, 10, 32)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid match ID",
		})
	}

	var req struct {
		JobID          string                 `json:"job_id"`
		PlayerStatsMap map[string]interface{} `json:"player_stats"` // player_id -> stats object
		ModelVersion   *string                `json:"model_version"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.JobID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "job_id is required",
		})
	}

	// Get all analysis events for this match
	var events []models.AnalysisEvent
	if err := database.DB.Where("match_id = ?", matchID).Find(&events).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve analysis events",
		})
	}

	attached := 0
	errors := []string{}

	// Attach stats to each event that has a matching player
	for _, event := range events {
		if event.PlayerID == nil {
			continue
		}

		playerIDStr := strconv.FormatUint(uint64(*event.PlayerID), 10)
		playerStats, exists := req.PlayerStatsMap[playerIDStr]
		if !exists {
			continue
		}

		// Convert playerStats to proper format
		statsMap, ok := playerStats.(map[string]interface{})
		if !ok {
			errors = append(errors, "Invalid stats format for player "+playerIDStr)
			continue
		}

		// Extract common stats
		var distanceM *float64
		if dist, ok := statsMap["distance_covered_m"].(float64); ok {
			distanceM = &dist
		}

		// Create stats record
		stats := models.AnalysisEventStats{
			AnalysisEventID:  event.ID,
			PlayerID:         event.PlayerID,
			PlayerName:       event.PlayerName,
			Stats:            statsMap,
			DistanceCoveredM: distanceM,
			Source:           "ai_model",
			ModelVersion:     req.ModelVersion,
			JobID:            &req.JobID,
		}

		if err := database.DB.Create(&stats).Error; err != nil {
			errors = append(errors, "Failed to attach stats for event "+strconv.FormatUint(uint64(event.ID), 10))
			continue
		}

		// Update event reference
		database.DB.Model(&event).Update("stats_id", stats.ID)
		attached++
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Batch stats attachment completed",
		"data": fiber.Map{
			"attached_count": attached,
			"total_events":   len(events),
			"errors":         errors,
		},
	})
}
