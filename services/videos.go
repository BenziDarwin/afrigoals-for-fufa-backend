// services/videos.go
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

func ListVideos(c *fiber.Ctx) error {
	// Filters
	matchID := c.Query("match_id")
	leagueID := c.Query("league_id")

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

	q := database.DB.Model(&models.Video{}).Order("created_at DESC")

	if matchID != "" {
		q = q.Where("match_id = ?", matchID)
	}
	if leagueID != "" {
		q = q.Where("league_id = ?", leagueID)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": "Failed to count videos"})
	}

	var videos []models.Video
	if err := q.Limit(perPage).Offset(offset).Find(&videos).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": "Failed to list videos"})
	}

	totalPages := int(total) / perPage
	if int(total)%perPage > 0 {
		totalPages++
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"videos":      videos,
			"total":       total,
			"page":        page,
			"per_page":    perPage,
			"total_pages": totalPages,
		},
	})
}

func GetVideoByID(c *fiber.Ctx) error {
	id := c.Params("id")
	var video models.Video
	if err := database.DB.First(&video, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "error": "Video not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": "Database error"})
	}
	return c.JSON(fiber.Map{"success": true, "video": video})
}

func CreateVideo(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var req struct {
		MatchID      uint   `json:"match_id"`
		LeagueID     *uint  `json:"league_id"`
		ClubID       *uint  `json:"club_id"`
		Title        string `json:"title"`
		Provider     string `json:"provider"`
		URL          string `json:"url"`
		ThumbnailURL string `json:"thumbnail_url"`
		DurationSec  *int   `json:"duration_sec"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if req.MatchID == 0 || req.Title == "" || req.Provider == "" || req.URL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "match_id, title, provider and url are required"})
	}

	video := models.Video{
		MatchID:      req.MatchID,
		LeagueID:     req.LeagueID,
		ClubID:       req.ClubID,
		Title:        req.Title,
		Provider:     models.VideoProvider(req.Provider),
		URL:          req.URL,
		ThumbnailURL: req.ThumbnailURL,
		DurationSec:  req.DurationSec,
		CreatedBy:    user.ID,
	}

	if err := database.DB.Create(&video).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create video"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "video": video})
}

func UpdateVideo(c *fiber.Ctx) error {
	id := c.Params("id")

	var video models.Video
	if err := database.DB.First(&video, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Video not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	var req struct {
		Title        *string `json:"title"`
		Provider     *string `json:"provider"`
		URL          *string `json:"url"`
		ThumbnailURL *string `json:"thumbnail_url"`
		DurationSec  *int    `json:"duration_sec"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Title != nil {
		video.Title = *req.Title
	}
	if req.Provider != nil {
		video.Provider = models.VideoProvider(*req.Provider)
	}
	if req.URL != nil {
		video.URL = *req.URL
	}
	if req.ThumbnailURL != nil {
		video.ThumbnailURL = *req.ThumbnailURL
	}
	if req.DurationSec != nil {
		video.DurationSec = req.DurationSec
	}

	if err := database.DB.Save(&video).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update video"})
	}

	return c.JSON(fiber.Map{"success": true, "video": video})
}

func DeleteVideo(c *fiber.Ctx) error {
	id := c.Params("id")
	res := database.DB.Delete(&models.Video{}, id)
	if res.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete video"})
	}
	if res.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Video not found"})
	}
	return c.JSON(fiber.Map{"success": true, "message": "Video deleted"})
}
	
func ListMatchVideos(c *fiber.Ctx) error {
	matchID := c.Params("match_id")

	var videos []models.Video
	if err := database.DB.
		Where("match_id = ?", matchID).
		Order("created_at DESC").
		Find(&videos).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "Failed to list videos",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"videos": videos,
			"count":  len(videos),
		},
	})
}