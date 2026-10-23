package services

import (
	"context"
	"errors"
	"math"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"afrigoals.com/database"
	"afrigoals.com/middleware"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type profileClip struct {
	EventID       uint      `json:"event_id"`
	MatchID       uint      `json:"match_id"`
	Title         string    `json:"title"`
	URL           string    `json:"url"`
	Date          time.Time `json:"date"`
	Timestamp     float64   `json:"timestamp_seconds"`
	Participation string    `json:"participation"`
}

type profileMatch struct {
	ID          uint                `json:"id"`
	Date        time.Time           `json:"date"`
	Home        string              `json:"home"`
	Away        string              `json:"away"`
	Competition string              `json:"competition"`
	ScoreHome   *int                `json:"score_home"`
	ScoreAway   *int                `json:"score_away"`
	EventCount  int                 `json:"event_count"`
	Metrics     []performanceMetric `json:"metrics"`
	Score       *float64            `json:"performance_score"`
}

type profileScore struct {
	Value          *float64            `json:"value"`
	Status         string              `json:"status"`
	Model          string              `json:"model"`
	Position       PositionGroup       `json:"position_group"`
	Matches        int                 `json:"matches"`
	Events         int                 `json:"events"`
	Coverage       float64             `json:"metric_coverage"`
	OutcomeSamples int                 `json:"outcome_samples"`
	Weights        map[string]float64  `json:"weights"`
	Components     []performanceMetric `json:"components"`
	Notes          string              `json:"notes"`
}

type profileValuation struct {
	Currency string   `json:"currency"`
	Status   string   `json:"status"`
	Estimate *float64 `json:"estimate"`
	Low      *float64 `json:"low"`
	High     *float64 `json:"high"`
	Baseline *float64 `json:"baseline"`
	Model    string   `json:"model"`
	Notes    string   `json:"notes"`
}

type playerProfileAnalysis struct {
	PlayerID   uint                          `json:"player_id"`
	Scope      string                        `json:"scope"`
	MatchCount int                           `json:"match_count"`
	EventCount int                           `json:"event_count"`
	Metrics    []performanceMetric           `json:"metrics"`
	EventTypes map[string]int                `json:"event_type_counts"`
	Zones      map[string]int                `json:"zone_frequency"`
	Matches    []profileMatch                `json:"matches"`
	Reports    []models.PlayerAnalysisReport `json:"reports"`
	Clips      []profileClip                 `json:"clips"`
	Physical   *physicalStatsAverage         `json:"physical_stats"`
	Score      profileScore                  `json:"performance_score"`
	Valuation  profileValuation              `json:"valuation"`
	UpdatedAt  *time.Time                    `json:"updated_at"`
	Warnings   []string                      `json:"warnings"`
}

// Rates describe the recorded sample, not inferred full-match actions. Existing
// metric functions retain their sample-size adjustment and provenance notes.
func profilePerformance(position string, events []models.AnalysisEvent, matchCount int) profileScore {
	group := normalizePositionGroup(position)
	score := profileScore{Status: "insufficient_data", Model: "analyst-outcomes-v1", Position: group, Matches: matchCount, Events: len(events), Components: []performanceMetric{}, Notes: "Position-specific weighted outcome index (0–100) from tagged actions, adjusted for sample size. Not a peer percentile or a full-match rating. Missing outcomes are excluded; selective tagging can bias the score."}
	score.Weights = map[string]float64{}
	metrics := combinedMetricSections(events)
	keys := []string{}
	weights := []float64{}
	switch group {
	case PositionGK:
		metrics.Metrics = append(metrics.Metrics,
			estimatedOutcomeRateMetric("distribution_success_pct", "Distribution outcomes", "goalkeeper", filterByType(events, "distribution_short", "distribution_long", "throw_distribution")),
			estimatedOutcomeRateMetric("claim_success_pct", "Claim outcomes", "goalkeeper", filterByType(events, "catch_cross", "punch", "claim_high_ball")),
			estimatedOutcomeRateMetric("save_outcome_pct", "Save outcomes", "goalkeeper", filterByType(events, "save", "one_vs_one_save")))
		keys, weights = []string{"distribution_success_pct", "claim_success_pct", "save_outcome_pct"}, []float64{.4, .3, .3}
	case PositionDEF:
		keys, weights = []string{"tackle_success_pct", "aerial_duel_success_pct", "ground_duel_success_pct", "pass_completion_pct"}, []float64{.35, .25, .2, .2}
	case PositionMID:
		keys, weights = []string{"pass_completion_pct", "dribble_success_pct", "ground_duel_success_pct", "cross_success_pct"}, []float64{.45, .25, .2, .1}
	case PositionFWD:
		keys, weights = []string{"shot_accuracy_pct", "dribble_success_pct", "cross_success_pct", "pass_completion_pct"}, []float64{.4, .3, .15, .15}
	default:
		return score
	}
	total, available := 0.0, 0.0
	for i, key := range keys {
		score.Weights[key] = weights[i]
		m, ok := metricByKey(metrics, key)
		if !ok || m.Value == nil || m.SampleSize == 0 || !finiteSeconds(*m.Value) {
			continue
		}
		total += math.Max(0, math.Min(100, *m.Value)) * weights[i]
		available += weights[i]
		score.Components = append(score.Components, m)
		score.OutcomeSamples += m.SampleSize
	}
	score.Coverage = math.Round(available*100) / 100
	if available == 0 {
		return score
	}
	value := math.Round(total/available*10) / 10
	score.Value = &value
	score.Status = "provisional"
	if matchCount >= 3 && score.OutcomeSamples >= 20 && len(score.Components) >= 2 && available >= .5 {
		score.Status = "available"
	}
	return score
}

func profileMarketValue(score profileScore, baseline float64) profileValuation {
	value := profileValuation{Currency: "USD", Status: "awaiting_calibration", Model: "baseline-performance-v1", Notes: "Indicative scenario estimate, not a predicted transfer fee. Baseline is the configured league/position value at a performance index of 50. Adjustment = 0.5 + score/100. Range reflects chosen evidence scenarios, not a statistical confidence interval. Age, contract duration, league economics and market demand are not modelled."}
	if !finiteSeconds(baseline) || baseline <= 0 {
		return value
	}
	value.Baseline = &baseline
	value.Status = "insufficient_data"
	if score.Value == nil || score.Status != "available" {
		return value
	}
	estimate := math.Round(baseline * (.5 + *score.Value/100))
	spread := .25 + .25*(1-score.Coverage) + .25*(1-math.Min(float64(score.Matches)/10, 1))
	low, high := math.Round(estimate*(1-spread)), math.Round(estimate*(1+spread))
	if !finiteSeconds(estimate) || !finiteSeconds(low) || !finiteSeconds(high) {
		value.Status = "awaiting_calibration"
		return value
	}
	value.Status = "indicative"
	value.Estimate, value.Low, value.High = &estimate, &low, &high
	return value
}

func playerValueBaseline(group PositionGroup) float64 {
	for _, key := range []string{"PLAYER_VALUE_BASELINE_USD_" + string(group), "PLAYER_VALUE_BASELINE_USD"} {
		if value, err := strconv.ParseFloat(strings.TrimSpace(os.Getenv(key)), 64); err == nil && finiteSeconds(value) && value > 0 {
			return value
		}
	}
	return 0
}

func profileMatchVisible(user *models.User, match *models.Match, status models.MatchReportReviewStatus) bool {
	if user == nil {
		return false
	}
	if user.Role == models.AfrigoalsAdmin || user.Role == models.DataAnalyst {
		return true
	}
	if user.Role == models.Scout {
		return status == models.ReportReviewApproved || status == models.ReportReviewDistributed
	}
	// Club managers already have access to their own match performance reports,
	// including work in progress. Do not hide that same evidence on their CVs.
	if user.Role == models.ClubManager {
		return canViewMatchPerformance(user, match)
	}
	return status == models.ReportReviewDistributed && canViewMatchPerformance(user, match)
}

func safeProfileClip(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	if parsed.Scheme == "http" || parsed.Scheme == "https" {
		return parsed.Host != ""
	}
	return parsed.Scheme == "" && parsed.Host == "" && strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") && !strings.HasPrefix(raw, "/uploads/clips/")
}

// Stable player IDs, never names or track numbers, join analyst evidence. This
// projection is calculated on read so edits/deletions/reassignments need no sync job.
func buildPlayerProfileAnalysis(player models.Player, matches []models.Match, events []models.AnalysisEvent, reports []models.PlayerAnalysisReport, clips []models.Clip) playerProfileAnalysis {
	result := playerProfileAnalysis{PlayerID: player.ID, Scope: "distributed_matches", Metrics: []performanceMetric{}, EventTypes: map[string]int{}, Zones: map[string]int{}, Matches: []profileMatch{}, Reports: []models.PlayerAnalysisReport{}, Clips: []profileClip{}, Warnings: []string{}}
	matchesByID := map[uint]models.Match{}
	for _, match := range matches {
		matchesByID[match.ID] = match
	}
	primary := []models.AnalysisEvent{}
	byMatch := map[uint][]models.AnalysisEvent{}
	linked := map[uint]models.AnalysisEvent{}
	touch := func(t time.Time) {
		if !t.IsZero() && (result.UpdatedAt == nil || t.After(*result.UpdatedAt)) {
			copy := t
			result.UpdatedAt = &copy
		}
	}
	for _, event := range events {
		if _, ok := matchesByID[event.MatchID]; !ok {
			continue
		}
		own := event.PlayerID != nil && *event.PlayerID == player.ID
		secondary := event.SecondaryPlayerID != nil && *event.SecondaryPlayerID == player.ID
		if !own && !secondary {
			continue
		}
		if _, duplicate := linked[event.ID]; duplicate {
			continue
		}
		linked[event.ID] = event
		if own {
			primary = append(primary, event)
			byMatch[event.MatchID] = append(byMatch[event.MatchID], event)
			result.EventTypes[event.Type]++
		}
		touch(event.UpdatedAt)
	}
	newest := map[uint]models.PlayerAnalysisReport{}
	for _, report := range reports {
		if report.PlayerID != player.ID {
			continue
		}
		if _, ok := matchesByID[report.MatchID]; !ok {
			continue
		}
		old, ok := newest[report.MatchID]
		if !ok || report.UpdatedAt.After(old.UpdatedAt) || (report.UpdatedAt.Equal(old.UpdatedAt) && report.ID > old.ID) {
			newest[report.MatchID] = report
		}
	}
	for _, report := range newest {
		result.Reports = append(result.Reports, report)
		touch(report.UpdatedAt)
	}
	sort.Slice(result.Reports, func(i, j int) bool { return result.Reports[i].UpdatedAt.After(result.Reports[j].UpdatedAt) })
	scoreMatches := 0
	for _, match := range matches {
		own := byMatch[match.ID]
		if len(own) > 0 {
			scoreMatches++
		}
		competition := "Not recorded"
		if match.League != nil {
			competition = match.League.Name
		}
		row := profileMatch{ID: match.ID, Date: match.Date, Home: match.HomeClub.Name, Away: match.AwayClub.Name, Competition: competition, ScoreHome: match.ScoreHome, ScoreAway: match.ScoreAway, EventCount: len(own), Metrics: combinedMetricSections(own).Metrics, Score: profilePerformance(player.Position, own, 1).Value}
		result.Matches = append(result.Matches, row)
	}
	sort.Slice(result.Matches, func(i, j int) bool { return result.Matches[i].Date.After(result.Matches[j].Date) })
	// Prefer completed clip records; retain event URLs for R2-only clip workflows.
	chosen := map[uint]string{}
	for _, clip := range clips {
		event, ok := linked[uintOrZero(clip.EventID)]
		if !ok || event.MatchID != clip.MatchID || clip.Status != "completed" || clip.ClipURL == nil || !safeProfileClip(*clip.ClipURL) {
			continue
		}
		chosen[event.ID] = *clip.ClipURL
		touch(clip.UpdatedAt)
	}
	for id, event := range linked {
		clipURL := chosen[id]
		if clipURL == "" && event.ClipURL != nil && safeProfileClip(*event.ClipURL) {
			clipURL = *event.ClipURL
		}
		if clipURL == "" {
			continue
		}
		participation := "primary"
		if event.PlayerID == nil || *event.PlayerID != player.ID {
			participation = "secondary"
		}
		result.Clips = append(result.Clips, profileClip{EventID: id, MatchID: event.MatchID, Title: strings.ReplaceAll(event.Type, "_", " "), URL: clipURL, Date: matchesByID[event.MatchID].Date, Timestamp: event.TimestampSeconds, Participation: participation})
	}
	sort.Slice(result.Clips, func(i, j int) bool {
		a, b := result.Clips[i], result.Clips[j]
		if !a.Date.Equal(b.Date) {
			return a.Date.After(b.Date)
		}
		if a.Timestamp != b.Timestamp {
			return a.Timestamp < b.Timestamp
		}
		return a.EventID < b.EventID
	})
	result.MatchCount, result.EventCount = len(result.Matches), len(primary)
	result.Metrics = combinedMetricSections(primary).Metrics
	result.Zones = zoneFrequency(primary)
	result.Score = profilePerformance(player.Position, primary, scoreMatches)
	result.Valuation = profileMarketValue(result.Score, playerValueBaseline(result.Score.Position))
	return result
}

func GetPlayerProfileAnalysis(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	id, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil || id == 0 {
		return fiber.NewError(400, "Invalid player ID")
	}
	db := database.DB.WithContext(c.UserContext())
	var player models.Player
	if err = db.First(&player, id).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return fiber.NewError(404, "Player not found")
	} else if err != nil {
		return fiber.NewError(500, "Could not load player")
	}
	var events []models.AnalysisEvent
	if db.Where("player_id = ? OR secondary_player_id = ?", id, id).Order("timestamp_seconds ASC, id ASC").Find(&events).Error != nil {
		return fiber.NewError(500, "Could not load player events")
	}
	var reports []models.PlayerAnalysisReport
	if db.Where("player_id = ?", id).Find(&reports).Error != nil {
		return fiber.NewError(500, "Could not load player reports")
	}
	matchIDs := map[uint]bool{}
	for _, event := range events {
		matchIDs[event.MatchID] = true
	}
	for _, report := range reports {
		matchIDs[report.MatchID] = true
	}
	ids := []uint{}
	for id := range matchIDs {
		ids = append(ids, id)
	}
	matches := []models.Match{}
	reviews := []models.MatchReportReview{}
	if len(ids) > 0 {
		if db.Preload("HomeClub").Preload("AwayClub").Preload("League").Where("id IN ?", ids).Find(&matches).Error != nil {
			return fiber.NewError(500, "Could not load match evidence")
		}
		if db.Where("match_id IN ?", ids).Find(&reviews).Error != nil {
			return fiber.NewError(500, "Could not load report visibility")
		}
	}
	statuses := map[uint]models.MatchReportReviewStatus{}
	for _, review := range reviews {
		statuses[review.MatchID] = review.Status
	}
	// A delivery grants only this recipient access to this player's working
	// evidence, bounded by the sender's current authorization.
	senders := []models.User{}
	if user.Role == models.Scout {
		if db.Where("id IN (SELECT sender_id FROM scout_cv_deliveries WHERE scout_id = ? AND player_id = ?)", user.ID, player.ID).Find(&senders).Error != nil {
			return fiber.NewError(500, "Could not load CV sharing permissions")
		}
	}
	sharedWorkingData := false
	allowed := []models.Match{}
	allowedIDs := []uint{}
	for _, match := range matches {
		shared := scoutDeliveryMatchVisible(user, player, &match, senders)
		if shared {
			sharedWorkingData = true
		}
		if shared || profileMatchVisible(user, &match, statuses[match.ID]) {
			allowed = append(allowed, match)
			allowedIDs = append(allowedIDs, match.ID)
		}
	}
	filtered := []models.AnalysisEvent{}
	permitted := map[uint]bool{}
	for _, id := range allowedIDs {
		permitted[id] = true
	}
	for _, event := range events {
		if permitted[event.MatchID] {
			filtered = append(filtered, event)
		}
	}
	eventIDs := []uint{}
	for _, event := range filtered {
		eventIDs = append(eventIDs, event.ID)
	}
	clips := []models.Clip{}
	stats := []models.AnalysisEventStats{}
	if len(eventIDs) > 0 {
		if db.Where("event_id IN ? AND match_id IN ?", eventIDs, allowedIDs).Order("updated_at ASC, id ASC").Find(&clips).Error != nil {
			return fiber.NewError(500, "Could not load player clips")
		}
		normalizeClipResponses(clips)
		if db.Where("analysis_event_id IN ?", eventIDs).Find(&stats).Error != nil {
			return fiber.NewError(500, "Could not load physical observations")
		}
	}
	warnings := []string{}
	// Analyst clip cutting stores objects directly in R2, without Clip DB rows.
	if len(filtered) > 0 {
		if _, err := loadR2Config(); err == nil {
			ctx, cancel := context.WithTimeout(c.UserContext(), 10*time.Second)
			defer cancel()
			objects := map[uint]string{}
			for _, match := range allowed {
				rows, err := listMatchClipObjects(ctx, match.ID)
				if err != nil {
					warnings = append(warnings, "Some stored clips could not be refreshed; showing available linked clips.")
					break
				}
				for _, row := range rows {
					objects[row.EventID] = row.ClipURL
				}
			}
			for i := range filtered {
				if raw, ok := objects[filtered[i].ID]; ok {
					copy := raw
					filtered[i].ClipURL = &copy
					// Current R2 objects supersede legacy clip records for this event.
					eventID := filtered[i].ID
					clips = append(clips, models.Clip{MatchID: filtered[i].MatchID, EventID: &eventID, Status: "completed", ClipURL: &copy})
				}
			}
		} else {
			warnings = append(warnings, "Cloud clip storage is not configured; showing available linked clips.")
		}
	}
	payload := buildPlayerProfileAnalysis(player, allowed, filtered, reports, clips)
	ownEvents := map[uint]bool{}
	for _, event := range filtered {
		if event.PlayerID != nil && *event.PlayerID == player.ID {
			ownEvents[event.ID] = true
		}
	}
	ownStats := []models.AnalysisEventStats{}
	for _, row := range stats {
		if ownEvents[row.AnalysisEventID] && (row.PlayerID == nil || *row.PlayerID == player.ID) {
			ownStats = append(ownStats, row)
		}
	}
	payload.Physical = averagePhysicalStats(ownStats)
	for _, row := range ownStats {
		if payload.UpdatedAt == nil || row.UpdatedAt.After(*payload.UpdatedAt) {
			stamp := row.UpdatedAt
			payload.UpdatedAt = &stamp
		}
	}
	payload.Warnings = warnings
	if user.Role == models.Scout {
		payload.Scope = "approved_matches"
		if sharedWorkingData {
			payload.Scope = "directly_shared_data"
		}
	}
	if user.Role == models.AfrigoalsAdmin || user.Role == models.DataAnalyst {
		payload.Scope = "analyst_working_data"
	} else if user.Role == models.ClubManager {
		payload.Scope = "club_working_data"
	}
	c.Set("Cache-Control", "private, no-store")
	return c.JSON(payload)
}
