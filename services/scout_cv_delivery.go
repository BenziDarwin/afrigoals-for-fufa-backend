package services

import (
	"afrigoals.com/database"
	"afrigoals.com/middleware"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm/clause"
	"strings"
	"time"
)

// Only staff and the player's own club may deliver a CV. A delivery grants
// its recipient access to this player's evidence visible to the sender.
func SendScoutCV(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	var body struct {
		PlayerID uint   `json:"player_id"`
		Email    string `json:"email"`
	}
	if c.BodyParser(&body) != nil || body.PlayerID == 0 || strings.TrimSpace(body.Email) == "" {
		return fiber.NewError(400, "Player and scout email are required")
	}
	db := database.DB.WithContext(c.UserContext())
	var player models.Player
	if err := db.First(&player, body.PlayerID).Error; err != nil {
		return fiber.NewError(404, "Player not found")
	}
	if !canSendScoutCV(user, player) {
		return fiber.ErrForbidden
	}
	var scout models.User
	if db.Where("LOWER(email) = ? AND role = ?", strings.ToLower(strings.TrimSpace(body.Email)), models.Scout).First(&scout).Error != nil {
		return fiber.NewError(404, "No scout account found for that email")
	}
	delivery := models.ScoutCVDelivery{ScoutID: scout.ID, PlayerID: player.ID, SenderID: user.ID, CreatedAt: time.Now()}
	if db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "scout_id"}, {Name: "player_id"}, {Name: "sender_id"}}, DoUpdates: clause.AssignmentColumns([]string{"created_at"})}).Create(&delivery).Error != nil {
		return fiber.NewError(500, "Could not send player CV")
	}
	return c.Status(201).JSON(fiber.Map{"message": "CV sent to scout dashboard"})
}

func canSendScoutCV(user *models.User, player models.Player) bool {
	return user.Role == models.AfrigoalsAdmin || user.Role == models.DataAnalyst || user.Role == models.ClubManager && user.ClubID != nil && *user.ClubID == player.ClubID
}

func ScoutCVInbox(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	c.Set("Cache-Control", "private, no-store")
	type item struct {
		ID        uint      `json:"id"`
		PlayerID  uint      `json:"player_id"`
		Name      string    `json:"name"`
		Position  string    `json:"position"`
		Sender    string    `json:"sender"`
		CreatedAt time.Time `json:"created_at"`
	}
	items := []item{}
	err = database.DB.WithContext(c.UserContext()).Table("scout_cv_deliveries AS d").Select("d.id, d.player_id, p.name, p.position, u.full_name AS sender, d.created_at").Joins("JOIN players p ON p.id = d.player_id AND p.deleted_at IS NULL").Joins("JOIN users u ON u.id = d.sender_id AND u.deleted_at IS NULL").Where("d.scout_id = ?", user.ID).Order("d.created_at DESC").Scan(&items).Error
	if err != nil {
		return fiber.NewError(500, "Could not load received CVs")
	}
	return c.JSON(items)
}

func scoutDeliveryMatchVisible(recipient *models.User, player models.Player, match *models.Match, senders []models.User) bool {
	if recipient == nil || recipient.Role != models.Scout {
		return false
	}
	for _, sender := range senders {
		if canSendScoutCV(&sender, player) && profileMatchVisible(&sender, match, "") {
			return true
		}
	}
	return false
}
