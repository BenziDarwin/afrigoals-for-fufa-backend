package services

import (
	"encoding/json"
	"errors"
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

func canReadTeamReport(user *models.User, clubID uint) bool {
	return user != nil && (user.Role == models.AfrigoalsAdmin || user.Role == models.DataAnalyst || middleware.CanManageClub(user, clubID))
}

func teamReportContext(c *fiber.Ctx) (*models.User, *models.Match, uint, error) {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return nil, nil, 0, fiber.ErrUnauthorized
	}
	matchID, err := strconv.ParseUint(c.Params("match_id"), 10, 32)
	if err != nil || matchID == 0 {
		return nil, nil, 0, fiber.NewError(400, "Invalid match")
	}
	var match models.Match
	if err := database.DB.Preload("HomeClub").Preload("AwayClub").First(&match, matchID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, 0, fiber.ErrNotFound
		}
		return nil, nil, 0, err
	}
	clubID := uint(0)
	if c.Params("club_id") != "" {
		parsed, err := strconv.ParseUint(c.Params("club_id"), 10, 32)
		if err != nil || parsed == 0 {
			return nil, nil, 0, fiber.NewError(400, "Invalid report team")
		}
		clubID = uint(parsed)
		if clubID != match.HomeClubID && clubID != match.AwayClubID {
			return nil, nil, 0, fiber.NewError(400, "The report team must play in this match")
		}
		if !canReadTeamReport(user, clubID) {
			return nil, nil, 0, fiber.ErrForbidden
		}
	} else if !canViewMatchPerformance(user, &match) {
		return nil, nil, 0, fiber.ErrForbidden
	}
	c.Set("Cache-Control", "private, no-store")
	return user, &match, clubID, nil
}

func ListTeamMatchReports(c *fiber.Ctx) error {
	user, match, _, err := teamReportContext(c)
	if err != nil {
		return err
	}
	teams := []uint{}
	for _, id := range []uint{match.HomeClubID, match.AwayClubID} {
		if canReadTeamReport(user, id) {
			teams = append(teams, id)
		}
	}
	reports := []models.TeamMatchReport{}
	query := database.DB.Where("match_id=? AND club_id IN ?", match.ID, teams)
	if user.Role == models.ClubManager {
		query = query.Where("status=?", models.ReportReviewDistributed)
	}
	if err := query.Order("club_id").Find(&reports).Error; err != nil {
		return err
	}
	return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"reports": reports}})
}

func teamReportSnapshot(payload *teamPerformanceReportPayload, clubID uint) (models.JSONB, error) {
	for _, team := range payload.Teams {
		if team.ClubID != clubID {
			continue
		}
		ids := map[uint]bool{}
		for _, player := range team.PlayerBreakdown {
			ids[player.PlayerID] = true
		}
		reports := []models.PlayerAnalysisReport{}
		for _, report := range payload.Reports {
			if ids[report.PlayerID] {
				reports = append(reports, report)
			}
		}
		raw, err := json.Marshal(map[string]any{"team": team, "player_reports": reports, "generated_at": time.Now().UTC()})
		if err != nil {
			return nil, err
		}
		var snapshot models.JSONB
		if err := json.Unmarshal(raw, &snapshot); err != nil {
			return nil, err
		}
		return snapshot, nil
	}
	return nil, fiber.NewError(400, "No performance data exists for this team")
}

func SaveTeamMatchReport(c *fiber.Ctx) error {
	user, match, clubID, err := teamReportContext(c)
	if err != nil {
		return err
	}
	if user.Role != models.DataAnalyst && user.Role != models.AfrigoalsAdmin {
		return fiber.ErrForbidden
	}
	var req struct {
		ClubID          *uint  `json:"club_id"`
		Summary         string `json:"summary"`
		Strengths       string `json:"strengths"`
		Weaknesses      string `json:"weaknesses"`
		Recommendations string `json:"recommendations"`
	}
	if c.BodyParser(&req) != nil {
		return fiber.NewError(400, "Invalid report body")
	}
	if req.ClubID != nil && *req.ClubID != clubID {
		return fiber.NewError(400, "Report recipient does not match the selected team")
	}
	if strings.TrimSpace(req.Summary) == "" {
		return fiber.NewError(400, "Add a team performance summary")
	}
	payload, err := computeTeamPerformanceSummaries(match.ID)
	if err != nil {
		return err
	}
	snapshot, err := teamReportSnapshot(payload, clubID)
	if err != nil {
		return err
	}
	report := models.TeamMatchReport{MatchID: match.ID, ClubID: clubID, Status: models.ReportReviewDraft, Snapshot: snapshot, AuthoredBy: user.ID}
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "match_id"}, {Name: "club_id"}}, DoNothing: true}).Create(&report).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("match_id=? AND club_id=?", match.ID, clubID).First(&report).Error; err != nil {
			return err
		}
		if report.Status != models.ReportReviewDraft && report.Status != models.ReportReviewChangesRequested {
			return fiber.NewError(409, "Only drafts or reports returned for changes can be edited")
		}
		return tx.Model(&report).Updates(map[string]any{"summary": strings.TrimSpace(req.Summary), "strengths": strings.TrimSpace(req.Strengths), "weaknesses": strings.TrimSpace(req.Weaknesses), "recommendations": strings.TrimSpace(req.Recommendations), "snapshot": snapshot, "authored_by": user.ID}).Error
	})
	if err != nil {
		return err
	}
	if err := database.DB.First(&report, report.ID).Error; err != nil {
		return err
	}
	return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"report": report}})
}

func ChangeTeamMatchReportStatus(c *fiber.Ctx) error {
	user, match, clubID, err := teamReportContext(c)
	if err != nil {
		return err
	}
	var req struct {
		Action string `json:"action"`
		Notes  string `json:"notes"`
		ClubID *uint  `json:"club_id"`
	}
	if c.BodyParser(&req) != nil {
		return fiber.NewError(400, "Invalid report action")
	}
	if req.ClubID != nil && *req.ClubID != clubID {
		return fiber.NewError(400, "Report recipient does not match the selected team")
	}
	var report models.TeamMatchReport
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("match_id=? AND club_id=?", match.ID, clubID).First(&report).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fiber.NewError(404, "Save this team's report first")
			}
			return err
		}
		now := time.Now().UTC()
		updates := map[string]any{}
		switch req.Action {
		case "submit":
			if user.Role != models.DataAnalyst && user.Role != models.AfrigoalsAdmin {
				return fiber.ErrForbidden
			}
			if report.Status != models.ReportReviewDraft && report.Status != models.ReportReviewChangesRequested {
				return fiber.NewError(409, "This team report has already been submitted")
			}
			if strings.TrimSpace(report.Summary) == "" {
				return fiber.NewError(400, "Add a team performance summary")
			}
			updates["status"] = models.ReportReviewSubmitted
			updates["submitted_at"] = now
		case "approve", "request_changes":
			if user.Role != models.AfrigoalsAdmin && (user.Role != models.LeagueAdmin || !middleware.CanManageClub(user, clubID)) {
				return fiber.ErrForbidden
			}
			if report.Status != models.ReportReviewSubmitted {
				return fiber.NewError(409, "This team report is not awaiting review")
			}
			updates["status"] = models.ReportReviewApproved
			if req.Action == "request_changes" {
				if strings.TrimSpace(req.Notes) == "" {
					return fiber.NewError(400, "Explain the requested changes")
				}
				updates["status"] = models.ReportReviewChangesRequested
			}
			updates["reviewed_by"] = user.ID
			updates["reviewed_at"] = now
			updates["review_notes"] = strings.TrimSpace(req.Notes)
		case "distribute":
			if user.Role != models.AfrigoalsAdmin && (user.Role != models.LeagueAdmin || !middleware.CanManageClub(user, clubID)) {
				return fiber.ErrForbidden
			}
			if report.Status != models.ReportReviewApproved {
				return fiber.NewError(409, "Approve this team report before distributing it")
			}
			updates["status"] = models.ReportReviewDistributed
			updates["distributed_by"] = user.ID
			updates["distributed_at"] = now
		default:
			return fiber.NewError(400, "Invalid report action")
		}
		return tx.Model(&report).Updates(updates).Error
	})
	if err != nil {
		return err
	}
	if err := database.DB.First(&report, report.ID).Error; err != nil {
		return err
	}
	return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"report": report}})
}

// ListLeagueTeamReports keeps the recipient club attached throughout review.
func ListLeagueTeamReports(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	leagueID, err := strconv.ParseUint(c.Params("league_id"), 10, 32)
	if err != nil || leagueID == 0 {
		return fiber.ErrBadRequest
	}
	if !middleware.CanManageLeague(user, uint(leagueID)) {
		return fiber.ErrForbidden
	}
	type item struct {
		models.TeamMatchReport
		TeamName string `json:"team_name"`
		HomeName string `json:"home_name"`
		AwayName string `json:"away_name"`
	}
	reports := []item{}
	err = database.DB.Table("team_match_reports AS r").Select("r.*, c.name AS team_name, h.name AS home_name, a.name AS away_name").
		Joins("JOIN clubs c ON c.id=r.club_id").Joins("JOIN matches m ON m.id=r.match_id").
		Joins("JOIN clubs h ON h.id=m.home_club_id").Joins("JOIN clubs a ON a.id=m.away_club_id").
		Where("r.club_id IN (SELECT club_id FROM club_leagues WHERE league_id=?)", leagueID).
		Where("r.status IN ?", []string{"submitted", "approved", "changes_requested"}).Order("r.updated_at DESC").Scan(&reports).Error
	if err != nil {
		return err
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"reports": reports}})
}

func scopeTeamPerformancePayload(c *fiber.Ctx, payload *teamPerformanceReportPayload) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	teams := []teamPerformanceSummary{}
	reports := []models.PlayerAnalysisReport{}
	ids := map[uint]bool{}
	payload.TotalReports, payload.TotalEvents, payload.TotalScore = 0, 0, 0
	for _, team := range payload.Teams {
		if !canReadTeamReport(user, team.ClubID) {
			continue
		}
		teams = append(teams, team)
		payload.TotalReports += team.Reports
		payload.TotalEvents += team.Events
		payload.TotalScore += team.Score
		for _, player := range team.PlayerBreakdown {
			ids[player.PlayerID] = true
		}
	}
	if len(teams) == 0 {
		return fiber.ErrForbidden
	}
	for _, report := range payload.Reports {
		if ids[report.PlayerID] {
			reports = append(reports, report)
		}
	}
	payload.Teams, payload.Reports = teams, reports
	c.Set("Cache-Control", "private, no-store")
	return nil
}

func reportPlayerAccess(c *fiber.Ctx, matchID uint) (map[uint]bool, error) {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return nil, fiber.ErrUnauthorized
	}
	var match models.Match
	if err := database.DB.Preload("HomeClub").Preload("AwayClub").First(&match, matchID).Error; err != nil {
		return nil, err
	}
	if !canViewMatchPerformance(user, &match) {
		return nil, fiber.ErrForbidden
	}
	var players []models.Player
	if err := database.DB.Scopes(func(db *gorm.DB) *gorm.DB { return matchRosterPlayers(db, match) }).Find(&players).Error; err != nil {
		return nil, err
	}
	projectMatchSquad(players, match)
	ids := map[uint]bool{}
	for _, player := range players {
		if canReadTeamReport(user, player.ClubID) {
			ids[player.ID] = true
		}
	}
	c.Set("Cache-Control", "private, no-store")
	return ids, nil
}
