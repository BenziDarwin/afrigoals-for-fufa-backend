package services

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"afrigoals.com/database"
	"afrigoals.com/middleware"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func nationalTeam(team models.Club) bool { return team.TeamType == "national" }

func teamPlayers(db *gorm.DB, teamID interface{}) *gorm.DB {
	return db.Where("(club_id = ? OR national_team_id = ?)", teamID, teamID)
}

func projectSquad(players []models.Player, teamID uint) {
	for i := range players {
		player := &players[i]
		player.ClubJerseyNumber = player.JerseyNumber
		if player.NationalTeamID != nil && *player.NationalTeamID == teamID {
			player.JerseyNumber = 0
			if player.NationalJerseyNumber != nil {
				player.JerseyNumber = *player.NationalJerseyNumber
			}
		}
	}
}

func projectMatchSquad(players []models.Player, match models.Match) {
	for i := range players {
		player := &players[i]
		player.ClubJerseyNumber = player.JerseyNumber
		if player.ClubID == match.HomeClubID || player.ClubID == match.AwayClubID || player.NationalTeamID == nil {
			continue
		}
		if *player.NationalTeamID != match.HomeClubID && *player.NationalTeamID != match.AwayClubID {
			continue
		}
		club := player.Club
		player.ClubTeam = &club
		player.ClubID = *player.NationalTeamID
		if player.NationalTeam != nil {
			player.Club = *player.NationalTeam
		}
		player.JerseyNumber = 0
		if player.NationalJerseyNumber != nil {
			player.JerseyNumber = *player.NationalJerseyNumber
		}
	}
}

func checkNationalTeam(db *gorm.DB, id *uint, jersey *int, exclude uint) error {
	if id == nil || *id == 0 {
		return nil
	}
	var team models.Club
	if err := db.First(&team, *id).Error; err != nil {
		return fmt.Errorf("national team not found")
	}
	if !nationalTeam(team) {
		return fmt.Errorf("select a team marked as a national team")
	}
	if jersey == nil || *jersey == 0 {
		return nil
	}
	if *jersey < 1 || *jersey > 99 {
		return fmt.Errorf("national shirt number must be between 1 and 99")
	}
	var count int64
	if err := db.Model(&models.Player{}).Where("national_team_id = ? AND national_jersey_number = ? AND id <> ?", *id, *jersey, exclude).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("that shirt number is already assigned in the national team")
	}
	return nil
}

// A national-team manager changes the national registration through this route.
// Primary-club ownership continues to govern global profile edits and deletion.
func SetPlayerNationalTeam(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil || id == 0 {
		return fiber.NewError(400, "Invalid player ID")
	}
	var req struct {
		NationalTeamID uint `json:"national_team_id"`
		JerseyNumber   *int `json:"jersey_number"`
	}
	if c.BodyParser(&req) != nil {
		return fiber.NewError(400, "Invalid national team registration")
	}
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		var player models.Player
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&player, id).Error; err != nil {
			return err
		}
		canManage := middleware.CanManagePlayer(user, &player)
		if req.NationalTeamID > 0 {
			canManage = canManage || middleware.CanManageClub(user, req.NationalTeamID)
		}
		if req.NationalTeamID == 0 && player.NationalTeamID != nil {
			canManage = canManage || middleware.CanManageClub(user, *player.NationalTeamID)
		}
		if !canManage {
			return fiber.ErrForbidden
		}
		var teamID *uint
		if req.NationalTeamID > 0 {
			teamID = &req.NationalTeamID
		}
		if err := checkNationalTeam(tx, teamID, req.JerseyNumber, player.ID); err != nil {
			return fiber.NewError(400, err.Error())
		}
		jersey := req.JerseyNumber
		if teamID == nil {
			jersey = nil
		}
		return tx.Model(&player).Updates(map[string]interface{}{"national_team_id": teamID, "national_jersey_number": jersey}).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fiber.ErrNotFound
	}
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"success": true, "message": "National team registration updated"})
}

func createSquadPlayer(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	var req struct {
		ClubID               uint   `json:"club_id"`
		PrimaryClubID        uint   `json:"primary_club_id"`
		PrimaryClubName      string `json:"primary_club_name"`
		PrimaryClubCountry   string `json:"primary_club_country"`
		ClubJerseyNumber     int    `json:"club_jersey_number"`
		NationalTeamID       *uint  `json:"national_team_id"`
		NationalJerseyNumber *int   `json:"national_jersey_number"`
		Name                 string `json:"name"`
		JerseyNumber         int    `json:"jersey_number"`
		Position             string `json:"position"`
		PhotoURL             string `json:"photo_url"`
		DateOfBirth          string `json:"date_of_birth"`
		Nationality          string `json:"nationality"`
	}
	if c.BodyParser(&req) != nil {
		return fiber.NewError(400, "Invalid player details")
	}
	if !middleware.CanManageClub(user, req.ClubID) {
		return fiber.ErrForbidden
	}
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Position) == "" || strings.TrimSpace(req.Nationality) == "" {
		return fiber.NewError(400, "Name, position and nationality are required")
	}
	dob, err := time.Parse("2006-01-02", req.DateOfBirth)
	if err != nil {
		return fiber.NewError(400, "Date of birth must use YYYY-MM-DD")
	}
	if req.JerseyNumber < 0 || req.JerseyNumber > 99 || req.ClubJerseyNumber < 0 || req.ClubJerseyNumber > 99 {
		return fiber.NewError(400, "Shirt numbers must be between 1 and 99")
	}
	player := models.Player{Name: strings.TrimSpace(req.Name), Position: req.Position, DateOfBirth: dob, Nationality: req.Nationality, ClubID: req.ClubID, JerseyNumber: req.JerseyNumber, NationalTeamID: req.NationalTeamID, NationalJerseyNumber: req.NationalJerseyNumber}
	if req.PhotoURL != "" {
		player.PhotoURL = &req.PhotoURL
	}
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		var team models.Club
		if err := tx.First(&team, req.ClubID).Error; err != nil {
			return err
		}
		if !nationalTeam(team) && req.JerseyNumber == 0 {
			return fiber.NewError(400, "Club shirt number is required")
		}
		if nationalTeam(team) {
			var existing models.Player
			match := tx.Where("LOWER(btrim(name))=LOWER(?) AND date_of_birth::date=?::date AND country_code=public.afrigoals_country_code(?)", strings.TrimSpace(req.Name), req.DateOfBirth, req.Nationality).First(&existing)
			if match.Error == nil {
				return fiber.NewError(409, "This player already has an Afrigoals profile and is listed automatically for their national team")
			}
			if !errors.Is(match.Error, gorm.ErrRecordNotFound) {
				return match.Error
			}
			player.NationalTeamID = &team.ID
			player.NationalJerseyNumber = &req.JerseyNumber
			player.JerseyNumber = req.ClubJerseyNumber
			player.ClubID = req.PrimaryClubID
			if player.ClubID == 0 {
				name := strings.TrimSpace(req.PrimaryClubName)
				if name == "" || len(name) > 150 || strings.TrimSpace(req.PrimaryClubCountry) == "" || len(req.PrimaryClubCountry) > 100 {
					return fiber.NewError(400, "Select the player's club or enter their overseas club name")
				}
				var club models.Club
				result := tx.Where("LOWER(name) = LOWER(?) AND LOWER(COALESCE(country, '')) = LOWER(?) AND team_type = ?", name, strings.TrimSpace(req.PrimaryClubCountry), "club").First(&club)
				if errors.Is(result.Error, gorm.ErrRecordNotFound) {
					club = models.Club{Name: name, TeamType: "club", Country: strings.TrimSpace(req.PrimaryClubCountry), IsExternal: true}
					if err := tx.Create(&club).Error; err != nil {
						return err
					}
				} else if result.Error != nil {
					return result.Error
				}
				player.ClubID = club.ID
			}
		}
		var primary models.Club
		if err := tx.First(&primary, player.ClubID).Error; err != nil {
			return err
		}
		if nationalTeam(team) && !primary.IsExternal && user.Role != models.AfrigoalsAdmin {
			return fiber.NewError(400, "Register domestic players through their club. Matching players appear in the national team automatically")
		}
		if nationalTeam(primary) {
			return fiber.NewError(400, "Primary club must be a club team")
		}
		if err := checkNationalTeam(tx, player.NationalTeamID, player.NationalJerseyNumber, 0); err != nil {
			return fiber.NewError(409, err.Error())
		}
		if err := tx.Omit(clause.Associations).Create(&player).Error; err != nil {
			return err
		}
		return tx.Preload("Club").Preload("NationalTeam").First(&player, player.ID).Error
	})
	if err != nil {
		return fiber.NewError(400, "Could not add player: "+err.Error())
	}
	return c.Status(201).JSON(fiber.Map{"success": true, "message": "Player added", "data": player})
}

func loadNationalRoster(db *gorm.DB, club *models.Club, user *models.User) error {
	if !nationalTeam(*club) {
		projectSquad(club.Players, club.ID)
		return nil
	}
	if err := teamPlayers(visiblePlayers(db.Preload("Club").Preload("NationalTeam").Preload("Stats"), user), club.ID).Find(&club.Players).Error; err != nil {
		return err
	}
	projectSquad(club.Players, club.ID)
	return attachNationalCallups(db, club.ID, club.Players)
}
