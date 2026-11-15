package services

import (
	"afrigoals.com/database"
	"afrigoals.com/middleware"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func ListCountries(c *fiber.Ctx) error {
	var countries []models.Country
	if err := database.DB.Order("name").Find(&countries).Error; err != nil {
		return fiber.NewError(500, "Could not load countries")
	}
	return c.JSON(fiber.Map{"countries": countries})
}

// External profiles remain available to administrators and their national manager.
func visiblePlayers(db *gorm.DB, user *models.User) *gorm.DB {
	if user != nil && user.Role == models.AfrigoalsAdmin {
		return db
	}
	teamID := uint(0)
	if user != nil && user.Role == models.ClubManager && user.ClubID != nil {
		teamID = *user.ClubID
	}
	return db.Where("(NOT EXISTS (SELECT 1 FROM clubs WHERE clubs.id=players.club_id AND clubs.is_external=true) OR players.club_id=? OR players.national_team_id=?)", teamID, teamID)
}

func canReadPlayer(c *fiber.Ctx, player *models.Player) bool {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return false
	}
	var count int64
	return visiblePlayers(database.DB.Model(&models.Player{}), user).Where("players.id=?", player.ID).Count(&count).Error == nil && count > 0
}

func GetPlayerClubHistory(c *fiber.Ctx) error {
	var player models.Player
	if err := database.DB.First(&player, c.Params("id")).Error; err != nil {
		return fiber.ErrNotFound
	}
	if !canReadPlayer(c, &player) {
		return fiber.ErrForbidden
	}
	var history []models.PlayerClubHistory
	if err := database.DB.Preload("FromClub").Preload("ToClub").Where("player_id=?", player.ID).Order("transferred_at DESC,id DESC").Find(&history).Error; err != nil {
		return fiber.NewError(500, "Could not load club history")
	}
	return c.JSON(fiber.Map{"history": history})
}

func currentPlayerReader(c *fiber.Ctx) *models.User {
	user, _ := middleware.GetUserFromContext(c)
	return user
}
