package services

import (
	"errors"
	"sort"
	"strconv"
	"strings"

	"afrigoals.com/database"
	"afrigoals.com/middleware"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func ListClips(c *fiber.Ctx) error {
	if err := ensureClipStatusColumns(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": "Failed to prepare clip storage", "details": err.Error()})
	}

	matchID := c.Query("match_id")
	videoID := c.Query("video_id")
	eventID := c.Query("event_id")

	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "50"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}
	offset := (page - 1) * perPage

	q := database.DB.Model(&models.Clip{}).Order("created_at DESC")

	if matchID != "" {
		q = q.Where("match_id = ?", matchID)
	}
	if videoID != "" {
		q = q.Where("video_id = ?", videoID)
	}
	if eventID != "" {
		q = q.Where("event_id = ?", eventID)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": "Failed to count clips"})
	}

	var clips []models.Clip
	if err := q.Limit(perPage).Offset(offset).Find(&clips).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": "Failed to list clips"})
	}
	normalizeClipResponses(clips)

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"clips":       clips,
			"total":       total,
			"page":        page,
			"per_page":    perPage,
			"total_pages": totalPages,
		},
	})
}

func GetClipByID(c *fiber.Ctx) error {
	if err := ensureClipStatusColumns(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": "Failed to prepare clip storage", "details": err.Error()})
	}

	id := c.Params("id")
	var clip models.Clip
	if err := database.DB.First(&clip, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "error": "Clip not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": "Database error"})
	}
	normalizeClipResponse(&clip)
	return c.JSON(fiber.Map{"success": true, "clip": clip})
}

func CreateClip(c *fiber.Ctx) error {
	if err := ensureClipStatusColumns(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to prepare clip storage", "details": err.Error()})
	}

	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var req struct {
		MatchID   uint    `json:"match_id"`
		VideoID   *uint   `json:"video_id"`
		EventID   *uint   `json:"event_id"`
		Title     string  `json:"title"`
		StartSec  int     `json:"start_sec"`
		EndSec    *int    `json:"end_sec"`
		ClipURL   *string `json:"clip_url"`
		ObjectKey *string `json:"object_key"`
		Tags      *string `json:"tags"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.MatchID == 0 || req.Title == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "match_id and title are required"})
	}
	if req.StartSec < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "start_sec must be >= 0"})
	}
	if req.EndSec != nil && *req.EndSec < req.StartSec {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "end_sec must be >= start_sec"})
	}

	// Optional integrity checks (recommended):
	// - confirm Match exists
	// - confirm Video exists if VideoID != nil
	// - confirm Event exists if EventID != nil
	// Keep minimal here for brevity.

	clip := models.Clip{

		MatchID: req.MatchID,

		VideoID: req.VideoID,

		EventID: req.EventID,

		Title: req.Title,

		StartSec: req.StartSec,

		EndSec: req.EndSec,

		ClipURL: req.ClipURL,

		ObjectKey: req.ObjectKey,

		Tags: req.Tags,

		Status: "pending",

		CreatedBy: user.ID,
	}

	if err := database.DB.Create(&clip).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create clip"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "clip": clip})
}

func UpdateClip(c *fiber.Ctx) error {
	if err := ensureClipStatusColumns(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to prepare clip storage", "details": err.Error()})
	}

	id := c.Params("id")

	var clip models.Clip
	if err := database.DB.First(&clip, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Clip not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	var req struct {
		Title     *string `json:"title"`
		StartSec  *int    `json:"start_sec"`
		EndSec    *int    `json:"end_sec"`
		VideoID   *uint   `json:"video_id"`
		EventID   *uint   `json:"event_id"`
		ClipURL   *string `json:"clip_url"`
		ObjectKey *string `json:"object_key"`
		Tags      *string `json:"tags"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Title != nil {
		clip.Title = *req.Title
	}
	if req.StartSec != nil {
		if *req.StartSec < 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "start_sec must be >= 0"})
		}
		clip.StartSec = *req.StartSec
	}
	if req.EndSec != nil {
		if *req.EndSec < clip.StartSec {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "end_sec must be >= start_sec"})
		}
		clip.EndSec = req.EndSec
	}
	if req.VideoID != nil {
		clip.VideoID = req.VideoID
	}
	if req.EventID != nil {
		clip.EventID = req.EventID
	}
	if req.ClipURL != nil {
		clip.ClipURL = req.ClipURL
	}
	if req.ObjectKey != nil {
		clip.ObjectKey = req.ObjectKey
	}
	if req.Tags != nil {
		clip.Tags = req.Tags
	}

	if err := database.DB.Save(&clip).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update clip"})
	}

	return c.JSON(fiber.Map{"success": true, "clip": clip})
}

func DeleteClip(c *fiber.Ctx) error {
	id := c.Params("id")
	res := database.DB.Delete(&models.Clip{}, id)
	if res.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete clip"})
	}
	if res.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Clip not found"})
	}
	return c.JSON(fiber.Map{"success": true, "message": "Clip deleted"})
}

// ListMatchClips lists the analyst-generated clips for a match. Unlike the
// generic ListClips above, these clips have no DB row - the R2 bucket under
// matches/<match_id>/clips/ is the source of truth, so this walks that prefix
// directly and joins against AnalysisEvent for display fields (type, player
// name). The response envelope and pagination params match ListClips so the
// frontend's existing listMatchClips() client needs no changes.
func ListMatchClips(c *fiber.Ctx) error {
	matchIDUint := mustParseUint(c.Params("match_id"))
	if matchIDUint == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "Invalid match_id",
		})
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "50"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 50
	}

	objects, err := listMatchClipObjects(c.Context(), matchIDUint)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to list clips",
			"details": err.Error(),
		})
	}

	sort.Slice(objects, func(i, j int) bool {
		return objects[i].LastModified.After(objects[j].LastModified)
	})

	eventIDs := make([]uint, 0, len(objects))
	for _, obj := range objects {
		eventIDs = append(eventIDs, obj.EventID)
	}

	eventsByID := map[uint]models.AnalysisEvent{}
	if len(eventIDs) > 0 {
		var events []models.AnalysisEvent
		if err := database.DB.
			Where("match_id = ? AND id IN ?", matchIDUint, eventIDs).
			Find(&events).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   "Failed to load events for clips",
			})
		}
		for _, ev := range events {
			eventsByID[ev.ID] = ev
		}
	}

	clips := make([]fiber.Map, 0, len(objects))
	for _, obj := range objects {
		event, ok := eventsByID[obj.EventID]
		if !ok {
			event = models.AnalysisEvent{ID: obj.EventID, Type: "Clip"}
		}
		clips = append(clips, fiber.Map{
			"id":         obj.EventID,
			"match_id":   matchIDUint,
			"event_id":   obj.EventID,
			"clip_url":   obj.ClipURL,
			"object_key": obj.ObjectKey,
			"status":     "completed",
			"created_at": obj.LastModified,
			"event": fiber.Map{
				"id":          event.ID,
				"type":        event.Type,
				"player_name": event.PlayerName,
			},
		})
	}

	total := len(clips)
	totalPages := total / perPage
	if total%perPage > 0 {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1
	}

	offset := (page - 1) * perPage
	if offset > total {
		offset = total
	}
	end := offset + perPage
	if end > total {
		end = total
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"clips":       clips[offset:end],
			"total":       total,
			"page":        page,
			"per_page":    perPage,
			"total_pages": totalPages,
		},
	})
}

func normalizeClipResponse(clip *models.Clip) {
	cfg, cfgErr := loadR2Config()
	if clip.ObjectKey != nil && strings.TrimSpace(*clip.ObjectKey) != "" && cfgErr == nil {
		url := publicR2ObjectURL(cfg, *clip.ObjectKey)
		clip.ClipURL = &url
	}

	if clip.ClipURL == nil {
		return
	}
	clipURL := strings.TrimSpace(*clip.ClipURL)
	if strings.HasPrefix(clipURL, "/uploads/clips/") {
		clip.ClipURL = nil
		clip.Status = "pending"
	}
}

func normalizeClipResponses(clips []models.Clip) {
	for index := range clips {
		normalizeClipResponse(&clips[index])
	}
}
