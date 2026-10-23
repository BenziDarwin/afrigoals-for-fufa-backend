package services

import (
	"afrigoals.com/database"
	"afrigoals.com/middleware"
	"afrigoals.com/models"
	"errors"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"strconv"
)

type squadPlayerSummary struct {
	Goals   int      `json:"goals"`
	Assists int      `json:"assists"`
	Matches int      `json:"matches"`
	Events  int      `json:"tagged_events"`
	Score   *float64 `json:"performance_score"`
}

func squadAnalysisSummaries(players []models.Player, matches []models.Match, events []models.AnalysisEvent, reports []models.PlayerAnalysisReport) map[uint]squadPlayerSummary {
	summaries := map[uint]squadPlayerSummary{}
	for _, player := range players {
		linked := map[uint]bool{}
		for _, event := range events {
			if event.PlayerID != nil && *event.PlayerID == player.ID || event.SecondaryPlayerID != nil && *event.SecondaryPlayerID == player.ID {
				linked[event.MatchID] = true
			}
		}
		for _, report := range reports {
			if report.PlayerID == player.ID {
				linked[report.MatchID] = true
			}
		}
		ownMatches := []models.Match{}
		for _, match := range matches {
			if linked[match.ID] {
				ownMatches = append(ownMatches, match)
			}
		}
		profile := buildPlayerProfileAnalysis(player, ownMatches, events, reports, nil)
		summaries[player.ID] = squadPlayerSummary{Goals: profile.EventTypes["goal"], Assists: profile.EventTypes["assist"], Matches: profile.MatchCount, Events: profile.EventCount, Score: profile.Score.Value}
	}
	return summaries
}

// A single batch read powers the squad cards without N video/R2-backed CV calls.
func GetClubPlayerAnalysis(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	id, err := strconv.ParseUint(c.Params("club_id"), 10, 32)
	if err != nil || id == 0 {
		return fiber.NewError(400, "Invalid club ID")
	}
	db := database.DB.WithContext(c.UserContext())
	var club models.Club
	if err = db.First(&club, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return fiber.NewError(404, "Club not found")
	} else if err != nil {
		return fiber.NewError(500, "Could not load club")
	}
	players := []models.Player{}
	if db.Where("club_id = ?", id).Find(&players).Error != nil {
		return fiber.NewError(500, "Could not load squad")
	}
	ids := []uint{}
	for _, player := range players {
		ids = append(ids, player.ID)
	}
	events := []models.AnalysisEvent{}
	reports := []models.PlayerAnalysisReport{}
	if len(ids) > 0 {
		if db.Where("player_id IN ? OR secondary_player_id IN ?", ids, ids).Find(&events).Error != nil {
			return fiber.NewError(500, "Could not load squad events")
		}
		if db.Where("player_id IN ?", ids).Find(&reports).Error != nil {
			return fiber.NewError(500, "Could not load squad reports")
		}
	}
	linked := map[uint]bool{}
	for _, event := range events {
		linked[event.MatchID] = true
	}
	for _, report := range reports {
		linked[report.MatchID] = true
	}
	matchIDs := []uint{}
	for id := range linked {
		matchIDs = append(matchIDs, id)
	}
	matches := []models.Match{}
	reviews := []models.MatchReportReview{}
	if len(matchIDs) > 0 {
		if db.Where("id IN ?", matchIDs).Find(&matches).Error != nil {
			return fiber.NewError(500, "Could not load squad matches")
		}
		if db.Where("match_id IN ?", matchIDs).Find(&reviews).Error != nil {
			return fiber.NewError(500, "Could not load report visibility")
		}
	}
	statuses := map[uint]models.MatchReportReviewStatus{}
	for _, review := range reviews {
		statuses[review.MatchID] = review.Status
	}
	allowed := []models.Match{}
	for _, match := range matches {
		if profileMatchVisible(user, &match, statuses[match.ID]) {
			allowed = append(allowed, match)
		}
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(fiber.Map{"players": squadAnalysisSummaries(players, allowed, events, reports)})
}
