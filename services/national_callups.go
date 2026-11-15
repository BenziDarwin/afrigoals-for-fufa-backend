package services

import (
	"errors"
	"strconv"
	"time"

	"afrigoals.com/database"
	"afrigoals.com/middleware"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func SetNationalCallup(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	teamID, err := strconv.ParseUint(c.Params("club_id"), 10, 32)
	if err != nil || teamID == 0 {
		return fiber.NewError(400, "Invalid national team")
	}
	playerID, err := strconv.ParseUint(c.Params("player_id"), 10, 32)
	if err != nil || playerID == 0 {
		return fiber.NewError(400, "Invalid player")
	}
	if !middleware.CanManageClub(user, uint(teamID)) {
		return fiber.ErrForbidden
	}
	var req struct {
		CalledUp *bool `json:"called_up"`
	}
	if c.BodyParser(&req) != nil || req.CalledUp == nil {
		return fiber.NewError(400, "Specify the call-up selection")
	}
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		var team models.Club
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&team, teamID).Error; err != nil {
			return err
		}
		if !nationalTeam(team) {
			return fiber.NewError(400, "Call-ups are available for national teams")
		}
		var player models.Player
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&player, playerID).Error; err != nil {
			return err
		}
		if player.NationalTeamID == nil || *player.NationalTeamID != team.ID {
			return fiber.NewError(400, "The player is not eligible for this national team")
		}
		var callup models.NationalCallup
		result := tx.Where("team_id=? AND player_id=? AND withdrawn_at IS NULL", team.ID, player.ID).First(&callup)
		if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return result.Error
		}
		if *req.CalledUp {
			if result.Error == nil {
				return nil
			}
			return tx.Create(&models.NationalCallup{TeamID: team.ID, PlayerID: player.ID, CalledBy: user.ID, CalledAt: time.Now().UTC(), JerseyNumber: player.NationalJerseyNumber}).Error
		}
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil
		}
		if err := tx.Model(&callup).Updates(map[string]interface{}{"withdrawn_at": time.Now().UTC(), "withdrawn_by": user.ID}).Error; err != nil {
			return err
		}
		return tx.Model(&player).Update("national_jersey_number", nil).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fiber.ErrNotFound
	}
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"success": true, "called_up": *req.CalledUp})
}

func attachNationalCallups(db *gorm.DB, teamID uint, players []models.Player) error {
	var calls []models.NationalCallup
	if err := db.Where("team_id=? AND withdrawn_at IS NULL", teamID).Find(&calls).Error; err != nil {
		return err
	}
	byPlayer := map[uint]time.Time{}
	for _, call := range calls {
		byPlayer[call.PlayerID] = call.CalledAt
	}
	for i := range players {
		if calledAt, ok := byPlayer[players[i].ID]; ok {
			players[i].CalledUp = true
			players[i].CalledUpAt = &calledAt
		}
	}
	return nil
}

// Saved lineups and analysis keep players in historical match reports after a
// withdrawal. New national match selections use the current call-up list.
func matchRosterPlayers(db *gorm.DB, match models.Match) *gorm.DB {
	teams := []uint{match.HomeClubID, match.AwayClubID}
	return db.Where(`(
  players.club_id IN ? OR
  (players.national_team_id IN ? AND EXISTS (SELECT 1 FROM national_callups nc WHERE nc.player_id=players.id AND nc.team_id=players.national_team_id AND nc.withdrawn_at IS NULL)) OR
  EXISTS (SELECT 1 FROM lineup_players lp JOIN formations f ON f.id=lp.formation_id WHERE f.match_id=? AND f.deleted_at IS NULL AND lp.deleted_at IS NULL AND lp.player_id=players.id) OR
  EXISTS (SELECT 1 FROM substitutes sp JOIN formations f ON f.id=sp.formation_id WHERE f.match_id=? AND f.deleted_at IS NULL AND sp.deleted_at IS NULL AND sp.player_id=players.id) OR
  EXISTS (SELECT 1 FROM analysis_events ae WHERE ae.match_id=? AND (ae.player_id=players.id OR ae.secondary_player_id=players.id)) OR
  EXISTS (SELECT 1 FROM player_analysis_reports pr WHERE pr.match_id=? AND pr.player_id=players.id)
 )`, teams, teams, match.ID, match.ID, match.ID, match.ID)
}

func validateNationalFormation(c *fiber.Ctx, tx *gorm.DB, clubID, matchID uint, ids []uint) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	if !middleware.CanManageClub(user, clubID) {
		return fiber.ErrForbidden
	}
	var club models.Club
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&club, clubID).Error; err != nil {
		return err
	}
	if !nationalTeam(club) {
		return nil
	}
	var match models.Match
	if err := tx.First(&match, matchID).Error; err != nil {
		return err
	}
	if match.HomeClubID != clubID && match.AwayClubID != clubID {
		return fiber.NewError(400, "The national team is not part of this match")
	}
	seen := map[uint]bool{}
	for _, id := range ids {
		if id == 0 || seen[id] {
			return fiber.NewError(400, "Select a different called-up player for each squad position")
		}
		seen[id] = true
	}
	if len(ids) == 0 {
		return nil
	}
	var count int64
	if err := tx.Model(&models.Player{}).Where("id IN ? AND national_team_id=? AND EXISTS (SELECT 1 FROM national_callups nc WHERE nc.player_id=players.id AND nc.team_id=? AND nc.withdrawn_at IS NULL)", ids, clubID, clubID).Count(&count).Error; err != nil {
		return err
	}
	if int(count) != len(ids) {
		return fiber.NewError(400, "National lineups can only include players currently called up by the coach")
	}
	return nil
}
