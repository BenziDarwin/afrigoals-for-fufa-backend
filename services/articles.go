package services

import (
	"afrigoals.com/database"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
)

// ListArticles exposes stored drafts and published articles to administrators.
func ListArticles(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	perPage := c.QueryInt("per_page", 100)
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 100
	}
	var total int64
	if err := database.DB.Model(&models.Article{}).Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not count articles"})
	}
	articles := make([]models.Article, 0)
	if err := database.DB.Order("created_at DESC").Offset((page - 1) * perPage).Limit(perPage).Find(&articles).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Could not load articles"})
	}
	return c.JSON(fiber.Map{"articles": articles, "total": total, "total_pages": (total + int64(perPage) - 1) / int64(perPage)})
}
