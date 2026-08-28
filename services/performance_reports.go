package services

import (
	"context"
	"errors"
	"strconv"
	"time"

	"afrigoals.com/database"
	"afrigoals.com/middleware"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// matchPerformanceInputs centralizes every DB/R2 query the Phase 1 report
// endpoints need, so both handlers below (and any future caller) share one
// read path instead of duplicating queries.
type matchPerformanceInputs struct {
	Match             models.Match
	Players           []models.Player
	Events            []models.AnalysisEvent
	Stats             []models.AnalysisEventStats
	EventTypesByValue map[string]models.EventType
	ClipEventIDs      map[uint]bool // AnalysisEvent.ID -> has an R2 clip, from listMatchClipObjects
}

func loadMatchPerformanceInputs(ctx context.Context, matchID uint) (*matchPerformanceInputs, error) {
	var match models.Match
	if err := database.DB.
		Preload("HomeClub").
		Preload("AwayClub").
		First(&match, matchID).Error; err != nil {
		return nil, err
	}

	var players []models.Player
	if err := database.DB.
		Where("club_id IN ?", []uint{match.HomeClubID, match.AwayClubID}).
		Find(&players).Error; err != nil {
		return nil, err
	}

	var events []models.AnalysisEvent
	if err := database.DB.
		Where("match_id = ?", matchID).
		Order("timestamp_seconds ASC").
		Find(&events).Error; err != nil {
		return nil, err
	}

	var stats []models.AnalysisEventStats
	if err := database.DB.
		Joins("JOIN analysis_events ON analysis_events.id = analysis_event_stats.analysis_event_id").
		Where("analysis_events.match_id = ?", matchID).
		Find(&stats).Error; err != nil {
		return nil, err
	}

	var eventTypes []models.EventType
	if err := database.DB.Find(&eventTypes).Error; err != nil {
		return nil, err
	}
	eventTypesByValue := make(map[string]models.EventType, len(eventTypes))
	for _, et := range eventTypes {
		eventTypesByValue[et.Value] = et
	}

	// Clip listing is best-effort: R2 being unconfigured/unreachable must
	// not block the report - clip coverage/clip IDs simply come back empty.
	clipEventIDs := map[uint]bool{}
	if summaries, err := listMatchClipObjects(ctx, matchID); err == nil {
		for _, s := range summaries {
			clipEventIDs[s.EventID] = true
		}
	}

	return &matchPerformanceInputs{
		Match:             match,
		Players:           players,
		Events:            events,
		Stats:             stats,
		EventTypesByValue: eventTypesByValue,
		ClipEventIDs:      clipEventIDs,
	}, nil
}

// canViewMatchPerformance: AfrigoalsAdmin and DataAnalyst get the same
// blanket access every other analyst-matches endpoint already grants them
// (no per-match assignment check exists anywhere in this codebase).
// LeagueAdmin/ClubManager reuse canReviewMatch (match_report_reviews.go),
// which is already club/league scoped via middleware.CanManageClub.
func canViewMatchPerformance(user *models.User, match *models.Match) bool {
	if user.Role == models.AfrigoalsAdmin || user.Role == models.DataAnalyst {
		return true
	}
	return canReviewMatch(user, match)
}

type teamPerformanceIntelligence struct {
	ClubID        uint                 `json:"club_id"`
	TeamName      string               `json:"team_name"`
	IsHomeTeam    bool                 `json:"is_home_team"`
	Attacking     metricSection        `json:"attacking"`
	Defensive     metricSection        `json:"defensive"`
	Transition    metricSection        `json:"transition"`
	Scoring       scoringPrediction    `json:"scoring_prediction"`
	Possession    possessionLevels     `json:"possession_levels"`
	Diagnosis     performanceDiagnosis `json:"diagnosis"`
	ZoneFrequency map[string]int       `json:"zone_frequency"`
}

type performanceDiagnosis struct {
	Strengths  []performanceFinding `json:"strengths"`
	Weaknesses []performanceFinding `json:"weaknesses"`
}

func buildTeamBundle(clubID uint, events []models.AnalysisEvent) teamMetricsBundle {
	return teamMetricsBundle{
		ClubID:     clubID,
		Events:     events,
		Attacking:  computeAttackingMetrics(events),
		Defensive:  computeDefensiveMetrics(events),
		Transition: computeTransitionMetrics(events),
	}
}

// GetMatchPerformanceReport: GET /api/v1/analyst-matches/:match_id/performance-report
func GetMatchPerformanceReport(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "error": "Unauthorized"})
	}

	matchID, err := strconv.ParseUint(c.Params("match_id"), 10, 32)
	if err != nil || matchID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": "Invalid match ID"})
	}

	inputs, err := loadMatchPerformanceInputs(c.Context(), uint(matchID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "error": "Match not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": "Database error", "details": err.Error()})
	}

	if !canViewMatchPerformance(user, &inputs.Match) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "error": "You do not have access to this match's performance report"})
	}

	playerClub := make(map[uint]uint, len(inputs.Players))
	for _, p := range inputs.Players {
		playerClub[p.ID] = p.ClubID
	}

	homeEvents, awayEvents, _ := bucketEventsByClub(inputs.Events, inputs.Match.HomeClubID, inputs.Match.AwayClubID, playerClub)

	homeBundle := buildTeamBundle(inputs.Match.HomeClubID, homeEvents)
	awayBundle := buildTeamBundle(inputs.Match.AwayClubID, awayEvents)

	homeBundle.Defensive = injectConcededMetrics(homeBundle.Defensive, awayBundle.Attacking)
	awayBundle.Defensive = injectConcededMetrics(awayBundle.Defensive, homeBundle.Attacking)

	homeStrengths, homeWeaknesses := diagnoseTeam(homeBundle, awayBundle, inputs.ClipEventIDs)
	awayStrengths, awayWeaknesses := diagnoseTeam(awayBundle, homeBundle, inputs.ClipEventIDs)
	homeScoring, awayScoring := computeScoringPredictions(homeEvents, awayEvents)
	homePossession := computePossessionLevels(homeEvents, awayEvents)
	awayPossession := computePossessionLevels(awayEvents, homeEvents)

	teams := []teamPerformanceIntelligence{
		{
			ClubID: inputs.Match.HomeClubID, TeamName: inputs.Match.HomeClub.Name, IsHomeTeam: true,
			Attacking: homeBundle.Attacking, Defensive: homeBundle.Defensive, Transition: homeBundle.Transition,
			Scoring: homeScoring, Possession: homePossession,
			Diagnosis:     performanceDiagnosis{Strengths: homeStrengths, Weaknesses: homeWeaknesses},
			ZoneFrequency: zoneFrequency(homeEvents),
		},
		{
			ClubID: inputs.Match.AwayClubID, TeamName: inputs.Match.AwayClub.Name, IsHomeTeam: false,
			Attacking: awayBundle.Attacking, Defensive: awayBundle.Defensive, Transition: awayBundle.Transition,
			Scoring: awayScoring, Possession: awayPossession,
			Diagnosis:     performanceDiagnosis{Strengths: awayStrengths, Weaknesses: awayWeaknesses},
			ZoneFrequency: zoneFrequency(awayEvents),
		},
	}

	playerReports := buildPlayerPerformanceReports(inputs.Players, inputs.Events, inputs.Stats, inputs.ClipEventIDs)
	dataQuality := computeDataQuality(inputs.Events, inputs.Stats, inputs.ClipEventIDs, inputs.EventTypesByValue)

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"match": fiber.Map{
				"id":         inputs.Match.ID,
				"uuid":       inputs.Match.UUID,
				"date":       inputs.Match.Date,
				"home_club":  fiber.Map{"id": inputs.Match.HomeClubID, "name": inputs.Match.HomeClub.Name},
				"away_club":  fiber.Map{"id": inputs.Match.AwayClubID, "name": inputs.Match.AwayClub.Name},
				"score_home": inputs.Match.ScoreHome,
				"score_away": inputs.Match.ScoreAway,
			},
			"data_quality":   dataQuality,
			"teams":          teams,
			"player_reports": playerReports,
			"generated_at":   time.Now(),
		},
	})
}

// GetPlayerPerformanceReport: GET /api/v1/analyst-matches/:match_id/performance-report/players/:player_id
func GetPlayerPerformanceReport(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "error": "Unauthorized"})
	}

	matchID, err := strconv.ParseUint(c.Params("match_id"), 10, 32)
	if err != nil || matchID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": "Invalid match ID"})
	}
	playerID, err := strconv.ParseUint(c.Params("player_id"), 10, 32)
	if err != nil || playerID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "error": "Invalid player ID"})
	}

	inputs, err := loadMatchPerformanceInputs(c.Context(), uint(matchID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "error": "Match not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "error": "Database error", "details": err.Error()})
	}

	if !canViewMatchPerformance(user, &inputs.Match) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "error": "You do not have access to this match's performance report"})
	}

	reports := buildPlayerPerformanceReports(inputs.Players, inputs.Events, inputs.Stats, inputs.ClipEventIDs)
	for _, r := range reports {
		if r.PlayerID == uint(playerID) {
			return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"player_report": r}})
		}
	}

	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"success": false, "error": "No tagged events found for this player in this match"})
}
