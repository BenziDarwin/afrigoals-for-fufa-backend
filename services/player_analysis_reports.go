package services

import (
	"bytes"
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
)

func GetMatchRosterPlayers(c *fiber.Ctx) error {
	matchID, err := strconv.ParseUint(c.Params("match_id"), 10, 32)
	if err != nil || matchID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid match ID"})
	}

	var match models.Match
	if err := database.DB.First(&match, uint(matchID)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Match not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	var players []models.Player
	if err := database.DB.
		Preload("Club").
		Where("club_id IN ?", []uint{match.HomeClubID, match.AwayClubID}).
		Order("club_id ASC, jersey_number ASC, name ASC").
		Find(&players).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to list match roster players"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"players": players,
			"count":   len(players),
		},
	})
}

func ListPlayerAnalysisReports(c *fiber.Ctx) error {
	matchID, err := strconv.ParseUint(c.Params("match_id"), 10, 32)
	if err != nil || matchID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid match ID"})
	}

	var reports []models.PlayerAnalysisReport
	if err := database.DB.
		Where("match_id = ?", uint(matchID)).
		Order("updated_at DESC").
		Find(&reports).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to list player analysis reports"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"reports": reports,
			"count":   len(reports),
		},
	})
}

func SavePlayerAnalysisReport(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	matchID, err := strconv.ParseUint(c.Params("match_id"), 10, 32)
	if err != nil || matchID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid match ID"})
	}

	var req struct {
		PlayerID             uint                   `json:"player_id"`
		PlayerName           string                 `json:"player_name"`
		EventCount           int                    `json:"event_count"`
		EventTypes           []string               `json:"event_types"`
		Score                int                    `json:"score"`
		LastEventTimeSeconds *float64               `json:"last_event_time_seconds"`
		AnalystComment       *string                `json:"analyst_comment"`
		ReportText           string                 `json:"report_text"`
		EventIDs             []uint                 `json:"event_ids"`
		AIStatsKeys          []string               `json:"ai_stats_keys"`
		Metadata             map[string]interface{} `json:"metadata"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.PlayerID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "player_id is required"})
	}
	if req.PlayerName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "player_name is required"})
	}
	if req.ReportText == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "report_text is required"})
	}
	if req.EventCount < 0 {
		req.EventCount = 0
	}
	if req.Score < 0 {
		req.Score = 0
	}
	if req.Metadata == nil {
		req.Metadata = map[string]interface{}{}
	}

	report := models.PlayerAnalysisReport{
		MatchID:              uint(matchID),
		PlayerID:             req.PlayerID,
		PlayerName:           req.PlayerName,
		EventCount:           req.EventCount,
		EventTypes:           models.StringSlice(req.EventTypes),
		Score:                req.Score,
		LastEventTimeSeconds: req.LastEventTimeSeconds,
		AnalystComment:       req.AnalystComment,
		ReportText:           req.ReportText,
		EventIDs:             models.UintSlice(req.EventIDs),
		AIStatsKeys:          models.StringSlice(req.AIStatsKeys),
		Metadata:             models.JSONB(req.Metadata),
		CreatedBy:            user.ID,
	}

	var existing models.PlayerAnalysisReport
	err = database.DB.Where("match_id = ? AND player_id = ?", report.MatchID, report.PlayerID).First(&existing).Error
	if err == nil {
		existing.PlayerName = report.PlayerName
		existing.EventCount = report.EventCount
		existing.EventTypes = report.EventTypes
		existing.Score = report.Score
		existing.LastEventTimeSeconds = report.LastEventTimeSeconds
		existing.AnalystComment = report.AnalystComment
		existing.ReportText = report.ReportText
		existing.EventIDs = report.EventIDs
		existing.AIStatsKeys = report.AIStatsKeys
		existing.Metadata = report.Metadata
		if existing.CreatedBy == 0 {
			existing.CreatedBy = user.ID
		}

		if err := database.DB.Save(&existing).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update player analysis report"})
		}
		return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"report": existing}})
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	if err := database.DB.Create(&report).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save player analysis report"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": fiber.Map{"report": report}})
}

func DeletePlayerAnalysisReport(c *fiber.Ctx) error {
	matchID, err := strconv.ParseUint(c.Params("match_id"), 10, 32)
	if err != nil || matchID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid match ID"})
	}

	reportID, err := strconv.ParseUint(c.Params("report_id"), 10, 32)
	if err != nil || reportID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid report ID"})
	}

	result := database.DB.
		Where("match_id = ? AND id = ?", uint(matchID), uint(reportID)).
		Delete(&models.PlayerAnalysisReport{})
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete player analysis report"})
	}
	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Player analysis report not found"})
	}

	return c.JSON(fiber.Map{"success": true, "message": "Player analysis report deleted"})
}

func DownloadPlayerAnalysisReportPDF(c *fiber.Ctx) error {
	matchID, err := strconv.ParseUint(c.Params("match_id"), 10, 32)
	if err != nil || matchID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid match ID"})
	}

	reportID, err := strconv.ParseUint(c.Params("report_id"), 10, 32)
	if err != nil || reportID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid report ID"})
	}

	var report models.PlayerAnalysisReport
	if err := database.DB.
		Where("match_id = ? AND id = ?", uint(matchID), uint(reportID)).
		First(&report).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Player analysis report not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	pdf := buildPlayerReportPDF(report)
	filename := fmt.Sprintf("player_report_match_%d_player_%d.pdf", report.MatchID, report.PlayerID)

	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Set("Content-Length", strconv.Itoa(len(pdf)))
	c.Set("Cache-Control", "no-store")
	return c.Send(pdf)
}

func DownloadTeamPerformanceReportPDF(c *fiber.Ctx) error {
	matchID, err := strconv.ParseUint(c.Params("match_id"), 10, 32)
	if err != nil || matchID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid match ID"})
	}

	payload, err := computeTeamPerformanceSummaries(uint(matchID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Match not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}
	if payload.TotalReports == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "No player analysis reports found for this match"})
	}

	pdf := buildTeamPerformancePDF(payload)
	filename := fmt.Sprintf("team_performance_match_%d.pdf", payload.Match.ID)

	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Set("Content-Length", strconv.Itoa(len(pdf)))
	c.Set("Cache-Control", "no-store")
	return c.Send(pdf)
}

// teamPerformanceReportPayload is the single source of truth for "what does
// this match's team performance look like" - both the PDF download above and
// GetMatchReportReview's JSON response are built from it.
type teamPerformanceReportPayload struct {
	Match        models.Match                  `json:"match"`
	Teams        []teamPerformanceSummary      `json:"teams"`
	Reports      []models.PlayerAnalysisReport `json:"-"`
	TotalReports int                           `json:"total_reports"`
	TotalEvents  int                           `json:"total_events"`
	TotalScore   int                           `json:"total_score"`
}

type playerPerformanceBreakdown struct {
	PlayerID        uint           `json:"player_id"`
	PlayerName      string         `json:"player_name"`
	Score           int            `json:"score"`
	EventCount      int            `json:"event_count"`
	EventTypeCounts map[string]int `json:"event_type_counts"`
}

// physicalStatsAverage is nil on a team's summary when it has no
// AnalysisEventStats rows at all (no AI job has run yet) - never a
// fabricated zero. Even when present, individual averages may themselves be
// nil if that specific field was nil on every contributing row.
type physicalStatsAverage struct {
	SampleSize          int      `json:"sample_size"`
	AvgDistanceCoveredM *float64 `json:"avg_distance_covered_m"`
	AvgSpeedKmh         *float64 `json:"avg_speed_kmh"`
	AvgMaxSpeedKmh      *float64 `json:"avg_max_speed_kmh"`
	AvgSprints          *float64 `json:"avg_sprints"`
	AvgTouches          *float64 `json:"avg_touches"`
}

type teamPerformanceSummary struct {
	ClubID          uint                         `json:"club_id"`
	TeamName        string                       `json:"team_name"`
	IsHomeTeam      bool                         `json:"is_home_team"`
	Players         int                          `json:"players"`
	Reports         int                          `json:"reports"`
	Events          int                          `json:"events"`
	Score           int                          `json:"score"`
	TopPlayer       string                       `json:"top_player"`
	TopPlayerScore  int                          `json:"top_player_score"`
	PlayerBreakdown []playerPerformanceBreakdown `json:"player_breakdown"`
	EventTypeCounts map[string]int               `json:"event_type_counts"`
	ZoneFrequency   map[string]int               `json:"zone_frequency"`
	PhysicalStats   *physicalStatsAverage        `json:"physical_stats"`
}

// computeTeamPerformanceSummaries aggregates a match's PlayerAnalysisReport,
// AnalysisEvent, and AnalysisEventStats rows into a per-club summary. Teams
// are keyed by club_id (not name - two blank/unknown names must not merge).
func computeTeamPerformanceSummaries(matchID uint) (*teamPerformanceReportPayload, error) {
	var match models.Match
	if err := database.DB.
		Preload("HomeClub").
		Preload("AwayClub").
		First(&match, matchID).Error; err != nil {
		return nil, err
	}

	var reports []models.PlayerAnalysisReport
	if err := database.DB.
		Where("match_id = ?", matchID).
		Order("score DESC, event_count DESC, player_name ASC").
		Find(&reports).Error; err != nil {
		return nil, err
	}

	var players []models.Player
	if err := database.DB.
		Preload("Club").
		Where("club_id IN ?", []uint{match.HomeClubID, match.AwayClubID}).
		Find(&players).Error; err != nil {
		return nil, err
	}

	var events []models.AnalysisEvent
	if err := database.DB.
		Where("match_id = ?", matchID).
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

	playerClub := make(map[uint]uint, len(players))
	for _, p := range players {
		playerClub[p.ID] = p.ClubID
	}
	eventPlayer := make(map[uint]uint, len(events))
	eventClub := make(map[uint]uint, len(events))
	for _, e := range events {
		pid := uintOrZero(e.PlayerID)
		eventPlayer[e.ID] = pid
		if clubID, ok := playerClub[pid]; ok {
			eventClub[e.ID] = clubID
		}
	}

	teams := map[uint]*teamPerformanceSummary{}
	ensureTeam := func(clubID uint, name string, isHome bool) *teamPerformanceSummary {
		if t, ok := teams[clubID]; ok {
			return t
		}
		name = strings.TrimSpace(name)
		if name == "" {
			name = "Unknown team"
		}
		t := &teamPerformanceSummary{
			ClubID:          clubID,
			TeamName:        name,
			IsHomeTeam:      isHome,
			TopPlayer:       "No data",
			EventTypeCounts: map[string]int{},
			ZoneFrequency:   map[string]int{},
		}
		teams[clubID] = t
		return t
	}
	ensureTeam(match.HomeClubID, match.HomeClub.Name, true)
	ensureTeam(match.AwayClubID, match.AwayClub.Name, false)

	for _, p := range players {
		isHome := p.ClubID == match.HomeClubID
		name := strings.TrimSpace(p.Club.Name)
		if name == "" {
			if isHome {
				name = match.HomeClub.Name
			} else {
				name = match.AwayClub.Name
			}
		}
		ensureTeam(p.ClubID, name, isHome).Players++
	}

	playerBreakdown := map[uint]*playerPerformanceBreakdown{}
	for _, report := range reports {
		clubID, ok := playerClub[report.PlayerID]
		if !ok {
			continue // player no longer on either roster, skip
		}
		team := ensureTeam(clubID, "Unknown team", clubID == match.HomeClubID)
		team.Reports++
		team.Events += report.EventCount
		team.Score += report.Score
		if report.Score > team.TopPlayerScore {
			team.TopPlayer = report.PlayerName
			team.TopPlayerScore = report.Score
		}
		playerBreakdown[report.PlayerID] = &playerPerformanceBreakdown{
			PlayerID:        report.PlayerID,
			PlayerName:      report.PlayerName,
			Score:           report.Score,
			EventCount:      report.EventCount,
			EventTypeCounts: map[string]int{},
		}
	}

	for _, e := range events {
		clubID, ok := eventClub[e.ID]
		if !ok {
			continue // event has no assigned player/club yet
		}
		team := teams[clubID]
		if team == nil {
			continue
		}
		team.EventTypeCounts[e.Type]++
		if e.PitchZone != nil && strings.TrimSpace(*e.PitchZone) != "" {
			team.ZoneFrequency[strings.TrimSpace(*e.PitchZone)]++
		}
		if bd, ok := playerBreakdown[eventPlayer[e.ID]]; ok {
			bd.EventTypeCounts[e.Type]++
		}
	}

	for _, bd := range playerBreakdown {
		clubID, ok := playerClub[bd.PlayerID]
		if !ok {
			continue
		}
		if team := teams[clubID]; team != nil {
			team.PlayerBreakdown = append(team.PlayerBreakdown, *bd)
		}
	}

	physicalStatsByClub(stats, playerClub, eventPlayer, teams)

	summaries := make([]teamPerformanceSummary, 0, len(teams))
	for _, t := range teams {
		summaries = append(summaries, *t)
	}
	sortTeamPerformanceSummaries(summaries)

	totalEvents, totalScore := 0, 0
	for _, r := range reports {
		totalEvents += r.EventCount
		totalScore += r.Score
	}

	return &teamPerformanceReportPayload{
		Match:        match,
		Teams:        summaries,
		Reports:      reports,
		TotalReports: len(reports),
		TotalEvents:  totalEvents,
		TotalScore:   totalScore,
	}, nil
}

// physicalStatsByClub averages AnalysisEventStats per club, over only the
// non-nil values contributing to each specific field, and attaches the
// result to the matching team in teams. A team with zero stats rows is left
// with PhysicalStats == nil.
func physicalStatsByClub(stats []models.AnalysisEventStats, playerClub map[uint]uint, eventPlayer map[uint]uint, teams map[uint]*teamPerformanceSummary) {
	type accumulator struct {
		sampleSize                         int
		distanceSum, speedSum, maxSpeedSum float64
		distanceN, speedN, maxSpeedN       int
		sprintsSum, touchesSum             float64
		sprintsN, touchesN                 int
	}
	byClub := map[uint]*accumulator{}

	for _, s := range stats {
		pid := uintOrZero(s.PlayerID)
		if pid == 0 {
			pid = eventPlayer[s.AnalysisEventID]
		}
		clubID, ok := playerClub[pid]
		if !ok {
			continue
		}
		acc := byClub[clubID]
		if acc == nil {
			acc = &accumulator{}
			byClub[clubID] = acc
		}
		acc.sampleSize++
		if s.DistanceCoveredM != nil {
			acc.distanceSum += *s.DistanceCoveredM
			acc.distanceN++
		}
		if s.AverageSpeedKmh != nil {
			acc.speedSum += *s.AverageSpeedKmh
			acc.speedN++
		}
		if s.MaxSpeedKmh != nil {
			acc.maxSpeedSum += *s.MaxSpeedKmh
			acc.maxSpeedN++
		}
		if s.SprintsCount != nil {
			acc.sprintsSum += float64(*s.SprintsCount)
			acc.sprintsN++
		}
		if s.TouchesCount != nil {
			acc.touchesSum += float64(*s.TouchesCount)
			acc.touchesN++
		}
	}

	avgPtr := func(sum float64, n int) *float64 {
		if n == 0 {
			return nil
		}
		v := sum / float64(n)
		return &v
	}

	for clubID, acc := range byClub {
		team := teams[clubID]
		if team == nil {
			continue
		}
		team.PhysicalStats = &physicalStatsAverage{
			SampleSize:          acc.sampleSize,
			AvgDistanceCoveredM: avgPtr(acc.distanceSum, acc.distanceN),
			AvgSpeedKmh:         avgPtr(acc.speedSum, acc.speedN),
			AvgMaxSpeedKmh:      avgPtr(acc.maxSpeedSum, acc.maxSpeedN),
			AvgSprints:          avgPtr(acc.sprintsSum, acc.sprintsN),
			AvgTouches:          avgPtr(acc.touchesSum, acc.touchesN),
		}
	}
}

func uintOrZero(p *uint) uint {
	if p == nil {
		return 0
	}
	return *p
}

func buildPlayerReportPDF(report models.PlayerAnalysisReport) []byte {
	lines := []string{
		"Afrigoals Player Analysis Report",
		"",
		fmt.Sprintf("Player: %s", report.PlayerName),
		fmt.Sprintf("Match ID: %d", report.MatchID),
		fmt.Sprintf("Player ID: %d", report.PlayerID),
		fmt.Sprintf("Event count: %d", report.EventCount),
		fmt.Sprintf("Event score: %d", report.Score),
		fmt.Sprintf("Event types: %s", strings.Join([]string(report.EventTypes), ", ")),
		fmt.Sprintf("AI stat keys: %s", strings.Join([]string(report.AIStatsKeys), ", ")),
		fmt.Sprintf("Updated: %s", report.UpdatedAt.Format(time.RFC3339)),
		"",
		"Analyst comment:",
		emptyIfNil(report.AnalystComment),
		"",
		"Report:",
	}

	for _, line := range wrapPDFText(report.ReportText, 92) {
		lines = append(lines, line)
	}

	pages := paginatePDFLines(lines, 44)
	return writeSimplePDF(pages)
}

func buildTeamPerformancePDF(payload *teamPerformanceReportPayload) []byte {
	match := payload.Match

	scoreLine := "Score: not recorded"
	if match.ScoreHome != nil && match.ScoreAway != nil {
		scoreLine = fmt.Sprintf("Score: %s %d - %d %s", match.HomeClub.Name, *match.ScoreHome, *match.ScoreAway, match.AwayClub.Name)
	}

	lines := []string{
		"Afrigoals Overall Team Performance Report",
		"",
		fmt.Sprintf("Match ID: %d", match.ID),
		fmt.Sprintf("Fixture: %s vs %s", match.HomeClub.Name, match.AwayClub.Name),
		scoreLine,
		fmt.Sprintf("Match date: %s", match.Date.Format("2006-01-02 15:04")),
		fmt.Sprintf("Generated: %s", time.Now().Format(time.RFC3339)),
		"",
		fmt.Sprintf("Saved player reports: %d", payload.TotalReports),
		fmt.Sprintf("Tagged player events: %d", payload.TotalEvents),
		fmt.Sprintf("Total performance score: %d", payload.TotalScore),
		"",
		"Team summary:",
	}

	for _, team := range payload.Teams {
		lines = append(lines,
			fmt.Sprintf("- %s", team.TeamName),
			fmt.Sprintf("  Players: %d", team.Players),
			fmt.Sprintf("  Reports: %d", team.Reports),
			fmt.Sprintf("  Events: %d", team.Events),
			fmt.Sprintf("  Score: %d", team.Score),
			fmt.Sprintf("  Top player: %s (%d)", team.TopPlayer, team.TopPlayerScore),
		)
		if team.PhysicalStats != nil {
			lines = append(lines, fmt.Sprintf("  Physical stats (n=%d, avg speed %.1f km/h, avg distance %.0f m)",
				team.PhysicalStats.SampleSize,
				floatOrZero(team.PhysicalStats.AvgSpeedKmh),
				floatOrZero(team.PhysicalStats.AvgDistanceCoveredM),
			))
		}
		lines = append(lines, "")
	}

	lines = append(lines, "Top player reports:")
	for i, report := range payload.Reports {
		if i >= 10 {
			break
		}
		lines = append(lines,
			fmt.Sprintf("- %s: score %d, %d events, types: %s",
				report.PlayerName,
				report.Score,
				report.EventCount,
				joinOrNone([]string(report.EventTypes)),
			),
		)
	}

	pages := paginatePDFLines(lines, 44)
	return writeSimplePDF(pages)
}

func floatOrZero(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func sortTeamPerformanceSummaries(summaries []teamPerformanceSummary) {
	for i := 0; i < len(summaries); i++ {
		for j := i + 1; j < len(summaries); j++ {
			if summaries[j].Score > summaries[i].Score ||
				(summaries[j].Score == summaries[i].Score && summaries[j].TeamName < summaries[i].TeamName) {
				summaries[i], summaries[j] = summaries[j], summaries[i]
			}
		}
	}
}

func joinOrNone(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}

func emptyIfNil(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func wrapPDFText(text string, width int) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}

		current := words[0]
		for _, word := range words[1:] {
			if len(current)+1+len(word) > width {
				lines = append(lines, current)
				current = word
				continue
			}
			current += " " + word
		}
		lines = append(lines, current)
	}
	return lines
}

func paginatePDFLines(lines []string, perPage int) [][]string {
	if len(lines) == 0 {
		return [][]string{{""}}
	}

	var pages [][]string
	for start := 0; start < len(lines); start += perPage {
		end := start + perPage
		if end > len(lines) {
			end = len(lines)
		}
		pages = append(pages, lines[start:end])
	}
	return pages
}

func writeSimplePDF(pages [][]string) []byte {
	var buf bytes.Buffer
	offsets := []int{0}
	writeObj := func(id int, body string) {
		offsets = append(offsets, buf.Len())
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", id, body)
	}

	buf.WriteString("%PDF-1.4\n")

	pageCount := len(pages)
	firstPageObjectID := 4
	firstContentObjectID := firstPageObjectID + pageCount

	pageRefs := make([]string, pageCount)
	for i := range pages {
		pageRefs[i] = fmt.Sprintf("%d 0 R", firstPageObjectID+i)
	}

	writeObj(1, "<< /Type /Catalog /Pages 2 0 R >>")
	writeObj(2, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(pageRefs, " "), pageCount))
	writeObj(3, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")

	for i := range pages {
		pageID := firstPageObjectID + i
		contentID := firstContentObjectID + i
		body := fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", contentID)
		writeObj(pageID, body)
	}

	for i, lines := range pages {
		content := buildPDFContentStream(lines)
		body := fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content)
		writeObj(firstContentObjectID+i, body)
	}

	xrefStart := buf.Len()
	totalObjects := firstContentObjectID + pageCount - 1
	fmt.Fprintf(&buf, "xref\n0 %d\n", totalObjects+1)
	buf.WriteString("0000000000 65535 f \n")
	for i := 1; i <= totalObjects; i++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", totalObjects+1, xrefStart)

	return buf.Bytes()
}

func buildPDFContentStream(lines []string) string {
	var b strings.Builder
	b.WriteString("BT\n/F1 11 Tf\n50 760 Td\n14 TL\n")
	for _, line := range lines {
		fmt.Fprintf(&b, "(%s) Tj\nT*\n", escapePDFText(line))
	}
	b.WriteString("ET")
	return b.String()
}

func escapePDFText(text string) string {
	text = strings.ReplaceAll(text, "\\", "\\\\")
	text = strings.ReplaceAll(text, "(", "\\(")
	text = strings.ReplaceAll(text, ")", "\\)")
	return text
}
