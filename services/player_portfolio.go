package services

import (
	"errors"
	"net/url"
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

func canManagePlayerPortfolio(user *models.User, player *models.Player) bool {
	return middleware.CanManagePlayer(user, player) || (player.NationalTeamID != nil && middleware.CanManageClub(user, *player.NationalTeamID))
}

func canEditPlayerProfile(user *models.User, player *models.Player) bool {
	if middleware.CanManagePlayer(user, player) {
		return true
	}
	if player.NationalTeamID == nil || !middleware.CanManageClub(user, *player.NationalTeamID) {
		return false
	}
	if player.ClubID == 0 {
		return true
	}
	var club models.Club
	return database.DB.First(&club, player.ClubID).Error == nil && club.IsExternal
}

func portfolioPlayer(c *fiber.Ctx, write bool) (models.Player, *models.User, error) {
	var player models.Player
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return player, nil, fiber.ErrUnauthorized
	}
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil || id == 0 {
		return player, user, fiber.NewError(400, "Invalid player ID")
	}
	if err := database.DB.First(&player, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return player, user, fiber.ErrNotFound
		}
		return player, user, err
	}
	if write && !canManagePlayerPortfolio(user, &player) {
		return player, user, fiber.ErrForbidden
	}
	if !write && !canReadPlayer(c, &player) {
		return player, user, fiber.ErrForbidden
	}
	return player, user, nil
}

func lockPortfolioPlayer(tx *gorm.DB, player *models.Player, user *models.User) error {
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(player, player.ID).Error; err != nil {
		return err
	}
	if !canManagePlayerPortfolio(user, player) {
		return fiber.ErrForbidden
	}
	return nil
}

func GetPlayerPortfolio(c *fiber.Ctx) error {
	player, user, err := portfolioPlayer(c, false)
	if err != nil {
		return err
	}
	var entries []models.PlayerCareerEntry
	var highlights []models.PlayerHighlight
	if err := database.DB.Preload("Club", func(db *gorm.DB) *gorm.DB { return db.Unscoped() }).Where("player_id=?", player.ID).Order("start_date DESC NULLS LAST,id DESC").Find(&entries).Error; err != nil {
		return err
	}
	if err := database.DB.Preload("Club", func(db *gorm.DB) *gorm.DB { return db.Unscoped() }).Where("player_id=? AND status='ready'", player.ID).Order("created_at DESC,id DESC").Find(&highlights).Error; err != nil {
		return err
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"career_entries": entries, "highlights": highlights, "can_edit": canManagePlayerPortfolio(user, &player), "can_edit_profile": canEditPlayerProfile(user, &player)})
}

type careerEntryRequest struct {
	ClubID      uint   `json:"club_id"`
	ClubName    string `json:"club_name"`
	ClubCountry string `json:"club_country"`
	Season      string `json:"season"`
	Competition string `json:"competition"`
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	Appearances *int   `json:"appearances"`
	Goals       *int   `json:"goals"`
	Assists     *int   `json:"assists"`
	Minutes     *int   `json:"minutes"`
	SourceURL   string `json:"source_url"`
	Notes       string `json:"notes"`
}

func optionalPortfolioDate(value string) (*time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, fiber.NewError(400, "Dates must use YYYY-MM-DD")
	}
	return &date, nil
}

func validatePortfolioURL(value string, required bool) error {
	if value == "" && !required {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || len(value) > 2048 || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.User != nil {
		return fiber.NewError(400, "Enter a valid HTTP or HTTPS link")
	}
	return nil
}

func resolvePlayerClub(tx *gorm.DB, id uint, name, country string) (models.Club, error) {
	var club models.Club
	if id > 0 {
		if err := tx.Unscoped().First(&club, id).Error; err != nil {
			return club, fiber.NewError(400, "Club not found")
		}
	} else {
		name = strings.TrimSpace(name)
		if name == "" || len(name) > 150 || strings.TrimSpace(country) == "" {
			return club, fiber.NewError(400, "Select a club or enter its name and country")
		}
		var code string
		if err := tx.Raw("SELECT COALESCE(public.afrigoals_country_code(?),'')", country).Scan(&code).Error; err != nil {
			return club, err
		}
		if code == "" {
			return club, fiber.NewError(400, "Select the club country from the directory")
		}
		// Serialize creation of a matching unregistered club across requests.
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(LOWER(?) || ':' || ?,0))", name, code).Error; err != nil {
			return club, err
		}
		result := tx.Where("LOWER(btrim(name))=LOWER(?) AND country_code=? AND team_type='club'", name, code).First(&club)
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			club = models.Club{Name: name, Country: country, TeamType: "club", IsExternal: true}
			if err := tx.Create(&club).Error; err != nil {
				return club, err
			}
		} else if result.Error != nil {
			return club, result.Error
		}
	}
	if nationalTeam(club) {
		return club, fiber.NewError(400, "Select a club team for the player's club history")
	}
	return club, nil
}

func SavePlayerCareerEntry(c *fiber.Ctx) error {
	player, user, err := portfolioPlayer(c, true)
	if err != nil {
		return err
	}
	var req careerEntryRequest
	if c.BodyParser(&req) != nil {
		return fiber.NewError(400, "Invalid career entry")
	}
	for _, value := range []*int{req.Appearances, req.Goals, req.Assists, req.Minutes} {
		if value != nil && (*value < 0 || *value > 1000000) {
			return fiber.NewError(400, "Career statistics must be between zero and 1,000,000")
		}
	}
	if len(req.Season) > 100 || len(req.Competition) > 200 || len(req.Notes) > 10000 {
		return fiber.NewError(400, "Career entry text is too long")
	}
	req.SourceURL = strings.TrimSpace(req.SourceURL)
	if err := validatePortfolioURL(req.SourceURL, false); err != nil {
		return err
	}
	start, err := optionalPortfolioDate(req.StartDate)
	if err != nil {
		return err
	}
	end, err := optionalPortfolioDate(req.EndDate)
	if err != nil {
		return err
	}
	if start != nil && end != nil && end.Before(*start) {
		return fiber.NewError(400, "End date must follow the start date")
	}
	entryID := uint(0)
	if c.Params("entry_id") != "" {
		parsed, err := strconv.ParseUint(c.Params("entry_id"), 10, 32)
		if err != nil || parsed == 0 {
			return fiber.NewError(400, "Invalid career entry ID")
		}
		entryID = uint(parsed)
	}
	var entry models.PlayerCareerEntry
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockPortfolioPlayer(tx, &player, user); err != nil {
			return err
		}
		if entryID > 0 {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND player_id=?", entryID, player.ID).First(&entry).Error; err != nil {
				return err
			}
		}
		club, err := resolvePlayerClub(tx, req.ClubID, req.ClubName, req.ClubCountry)
		if err != nil {
			return err
		}
		if entryID == 0 {
			entry = models.PlayerCareerEntry{PlayerID: player.ID, ClubID: club.ID, Season: strings.TrimSpace(req.Season), Competition: strings.TrimSpace(req.Competition), StartDate: start, EndDate: end, Appearances: req.Appearances, Goals: req.Goals, Assists: req.Assists, Minutes: req.Minutes, SourceURL: req.SourceURL, Notes: strings.TrimSpace(req.Notes), CreatedBy: user.ID, UpdatedBy: user.ID}
			return tx.Omit("Club").Create(&entry).Error
		}
		updates := map[string]interface{}{"club_id": club.ID, "season": strings.TrimSpace(req.Season), "competition": strings.TrimSpace(req.Competition), "start_date": start, "end_date": end, "appearances": req.Appearances, "goals": req.Goals, "assists": req.Assists, "minutes": req.Minutes, "source_url": req.SourceURL, "notes": strings.TrimSpace(req.Notes), "updated_by": user.ID}
		if err := tx.Model(&entry).Updates(updates).Error; err != nil {
			return err
		}
		return tx.Model(&models.PlayerHighlight{}).Where("career_entry_id=? AND player_id=?", entry.ID, player.ID).Update("club_id", club.ID).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fiber.ErrNotFound
	}
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"success": true, "id": entry.ID})
}

func DeletePlayerCareerEntry(c *fiber.Ctx) error {
	player, user, err := portfolioPlayer(c, true)
	if err != nil {
		return err
	}
	id, err := strconv.ParseUint(c.Params("entry_id"), 10, 32)
	if err != nil || id == 0 {
		return fiber.NewError(400, "Invalid career entry ID")
	}
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockPortfolioPlayer(tx, &player, user); err != nil {
			return err
		}
		result := tx.Model(&models.PlayerCareerEntry{}).Where("id=? AND player_id=?", id, player.ID).Updates(map[string]interface{}{"deleted_at": time.Now().UTC(), "updated_by": user.ID})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fiber.ErrNotFound
		}
		return tx.Model(&models.PlayerHighlight{}).Where("career_entry_id=? AND player_id=?", id, player.ID).Update("career_entry_id", nil).Error
	})
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"success": true})
}

type highlightRequest struct {
	UploadID      *uint  `json:"upload_id"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	CareerEntryID *uint  `json:"career_entry_id"`
	VideoURL      string `json:"video_url"`
	Filename      string `json:"filename"`
	ContentType   string `json:"content_type"`
	SizeBytes     int64  `json:"size_bytes"`
}

func applyHighlightDetails(tx *gorm.DB, playerID uint, req highlightRequest, highlight *models.PlayerHighlight) error {
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" || len(req.Title) > 200 || len(req.Description) > 10000 {
		return fiber.NewError(400, "Provide a title of at most 200 characters and a description of at most 10,000 characters")
	}
	highlight.Title = req.Title
	highlight.Description = strings.TrimSpace(req.Description)
	highlight.CareerEntryID = nil
	highlight.ClubID = nil
	if req.CareerEntryID != nil && *req.CareerEntryID > 0 {
		var entry models.PlayerCareerEntry
		if err := tx.Where("id=? AND player_id=?", *req.CareerEntryID, playerID).First(&entry).Error; err != nil {
			return fiber.NewError(400, "Select a career entry belonging to this player")
		}
		highlight.CareerEntryID = &entry.ID
		highlight.ClubID = &entry.ClubID
	}
	return nil
}

func SavePlayerHighlight(c *fiber.Ctx) error {
	player, user, err := portfolioPlayer(c, true)
	if err != nil {
		return err
	}
	var req highlightRequest
	if c.BodyParser(&req) != nil {
		return fiber.NewError(400, "Invalid highlight details")
	}
	var highlight models.PlayerHighlight
	isUpdate := c.Params("highlight_id") != ""
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockPortfolioPlayer(tx, &player, user); err != nil {
			return err
		}
		if isUpdate {
			id, err := strconv.ParseUint(c.Params("highlight_id"), 10, 32)
			if err != nil || id == 0 {
				return fiber.NewError(400, "Invalid highlight ID")
			}
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND player_id=? AND status='ready'", id, player.ID).First(&highlight).Error; err != nil {
				return err
			}
		} else {
			highlight = models.PlayerHighlight{PlayerID: player.ID, Kind: "link", Status: "ready", CreatedBy: user.ID}
		}
		if err := applyHighlightDetails(tx, player.ID, req, &highlight); err != nil {
			return err
		}
		if highlight.Kind == "link" {
			req.VideoURL = strings.TrimSpace(req.VideoURL)
			if err := validatePortfolioURL(req.VideoURL, true); err != nil {
				return err
			}
			highlight.VideoURL = req.VideoURL
		}
		highlight.UpdatedBy = user.ID
		return tx.Omit("Club").Save(&highlight).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fiber.ErrNotFound
	}
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"success": true, "id": highlight.ID})
}

func DeletePlayerHighlight(c *fiber.Ctx) error {
	player, user, err := portfolioPlayer(c, true)
	if err != nil {
		return err
	}
	id, err := strconv.ParseUint(c.Params("highlight_id"), 10, 32)
	if err != nil || id == 0 {
		return fiber.NewError(400, "Invalid highlight ID")
	}
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockPortfolioPlayer(tx, &player, user); err != nil {
			return err
		}
		result := tx.Model(&models.PlayerHighlight{}).Where("id=? AND player_id=?", id, player.ID).Updates(map[string]interface{}{"deleted_at": time.Now().UTC(), "updated_by": user.ID})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fiber.ErrNotFound
		}
		return nil
	})
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"success": true})
}
