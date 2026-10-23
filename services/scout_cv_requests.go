package services

import (
	"afrigoals.com/database"
	"afrigoals.com/middleware"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strconv"
	"time"
)

func RequestScoutCV(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	if user.Role != models.Scout {
		return fiber.ErrForbidden
	}
	var body struct {
		PlayerID uint `json:"player_id"`
	}
	if c.BodyParser(&body) != nil || body.PlayerID == 0 {
		return fiber.NewError(400, "Player is required")
	}
	var player models.Player
	if database.DB.First(&player, body.PlayerID).Error != nil {
		return fiber.NewError(404, "Player not found")
	}
	if player.ClubID == 0 {
		return fiber.NewError(409, "Player has no team to receive the request")
	}
	request := models.ScoutCVRequest{ScoutID: user.ID, PlayerID: player.ID, ClubID: player.ClubID, Status: "pending", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	err = database.DB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "scout_id"}, {Name: "player_id"}, {Name: "club_id"}}, DoNothing: true}).Create(&request).Error
	if err != nil {
		return fiber.NewError(500, "Could not request CV")
	}
	database.DB.Where("scout_id=? AND player_id=? AND club_id=?", user.ID, player.ID, player.ClubID).First(&request)
	return c.Status(201).JSON(request)
}
func ListScoutCVRequests(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	q := database.DB.Table("scout_cv_requests r").Select("r.*, p.name AS player_name, u.full_name AS scout_name, clubs.name AS club_name").Joins("JOIN players p ON p.id=r.player_id AND p.deleted_at IS NULL").Joins("JOIN users u ON u.id=r.scout_id AND u.deleted_at IS NULL").Joins("JOIN clubs ON clubs.id=r.club_id AND clubs.deleted_at IS NULL")
	switch user.Role {
	case models.Scout:
		q = q.Where("r.scout_id=?", user.ID)
	case models.ClubManager:
		if user.ClubID == nil {
			return fiber.ErrForbidden
		}
		q = q.Where("r.club_id=? AND p.club_id=r.club_id", *user.ClubID)
	case models.AfrigoalsAdmin:
	default:
		return fiber.ErrForbidden
	}
	type row struct {
		models.ScoutCVRequest
		PlayerName string `json:"player_name"`
		ScoutName  string `json:"scout_name"`
		ClubName   string `json:"club_name"`
	}
	rows := []row{}
	if q.Order("r.created_at DESC").Scan(&rows).Error != nil {
		return fiber.NewError(500, "Could not load CV requests")
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(rows)
}
func RespondScoutCVRequest(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	if user.Role != models.ClubManager && user.Role != models.AfrigoalsAdmin {
		return fiber.ErrForbidden
	}
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil || id == 0 {
		return fiber.ErrBadRequest
	}
	var body struct {
		Action string `json:"action"`
	}
	if c.BodyParser(&body) != nil || (body.Action != "share" && body.Action != "decline") {
		return fiber.ErrBadRequest
	}
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		var request models.ScoutCVRequest
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&request, id).Error != nil {
			return fiber.ErrNotFound
		}
		var player models.Player
		if tx.First(&player, request.PlayerID).Error != nil {
			return fiber.ErrNotFound
		}
		if player.ClubID != request.ClubID || !canSendScoutCV(user, player) {
			return fiber.ErrForbidden
		}
		if request.Status != "pending" {
			return fiber.NewError(409, "Request already handled")
		}
		status := "declined"
		if body.Action == "share" {
			var scout models.User
			if tx.Where("id=? AND role=?", request.ScoutID, models.Scout).First(&scout).Error != nil {
				return fiber.ErrNotFound
			}
			delivery := models.ScoutCVDelivery{ScoutID: request.ScoutID, PlayerID: player.ID, SenderID: user.ID, CreatedAt: time.Now()}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "scout_id"}, {Name: "player_id"}, {Name: "sender_id"}}, DoUpdates: clause.AssignmentColumns([]string{"created_at"})}).Create(&delivery).Error; err != nil {
				return err
			}
			status = "shared"
		}
		return tx.Model(&request).Updates(map[string]interface{}{"status": status, "updated_at": time.Now()}).Error
	})
	if err != nil {
		if _, ok := err.(*fiber.Error); ok {
			return err
		}
		return fiber.NewError(500, "Could not respond to CV request")
	}
	return c.JSON(fiber.Map{"message": "Request updated"})
}
