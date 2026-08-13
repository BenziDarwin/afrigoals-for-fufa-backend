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

	var match models.Match
	if err := database.DB.
		Preload("HomeClub").
		Preload("AwayClub").
		First(&match, uint(matchID)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Match not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	var reports []models.PlayerAnalysisReport
	if err := database.DB.
		Where("match_id = ?", uint(matchID)).
		Order("score DESC, event_count DESC, player_name ASC").
		Find(&reports).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to list player analysis reports"})
	}
	if len(reports) == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "No player analysis reports found for this match"})
	}

	var players []models.Player
	if err := database.DB.
		Preload("Club").
		Where("club_id IN ?", []uint{match.HomeClubID, match.AwayClubID}).
		Find(&players).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to list match players"})
	}

	pdf := buildTeamPerformancePDF(match, reports, players)
	filename := fmt.Sprintf("team_performance_match_%d.pdf", match.ID)

	c.Set("Content-Type", "application/pdf")
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Set("Content-Length", strconv.Itoa(len(pdf)))
	c.Set("Cache-Control", "no-store")
	return c.Send(pdf)
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

type teamPerformanceSummary struct {
	TeamName       string
	Players        int
	Reports        int
	Events         int
	Score          int
	TopPlayer      string
	TopPlayerScore int
}

func buildTeamPerformancePDF(match models.Match, reports []models.PlayerAnalysisReport, players []models.Player) []byte {
	playerTeams := make(map[uint]string)
	teams := make(map[string]*teamPerformanceSummary)

	ensureTeam := func(name string) *teamPerformanceSummary {
		name = strings.TrimSpace(name)
		if name == "" {
			name = "Unknown team"
		}
		if _, ok := teams[name]; !ok {
			teams[name] = &teamPerformanceSummary{
				TeamName:  name,
				TopPlayer: "No data",
			}
		}
		return teams[name]
	}

	for _, player := range players {
		teamName := strings.TrimSpace(player.Club.Name)
		if teamName == "" {
			switch player.ClubID {
			case match.HomeClubID:
				teamName = match.HomeClub.Name
			case match.AwayClubID:
				teamName = match.AwayClub.Name
			default:
				teamName = "Unknown team"
			}
		}
		playerTeams[player.ID] = teamName
		ensureTeam(teamName).Players++
	}

	for _, report := range reports {
		teamName := playerTeams[report.PlayerID]
		team := ensureTeam(teamName)
		team.Reports++
		team.Events += report.EventCount
		team.Score += report.Score
		if report.Score > team.TopPlayerScore {
			team.TopPlayer = report.PlayerName
			team.TopPlayerScore = report.Score
		}
	}

	summaries := make([]teamPerformanceSummary, 0, len(teams))
	for _, team := range teams {
		summaries = append(summaries, *team)
	}
	sortTeamPerformanceSummaries(summaries)

	totalEvents := 0
	totalScore := 0
	for _, report := range reports {
		totalEvents += report.EventCount
		totalScore += report.Score
	}

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
		fmt.Sprintf("Saved player reports: %d", len(reports)),
		fmt.Sprintf("Tagged player events: %d", totalEvents),
		fmt.Sprintf("Total performance score: %d", totalScore),
		"",
		"Team summary:",
	}

	for _, team := range summaries {
		lines = append(lines,
			fmt.Sprintf("- %s", team.TeamName),
			fmt.Sprintf("  Players: %d", team.Players),
			fmt.Sprintf("  Reports: %d", team.Reports),
			fmt.Sprintf("  Events: %d", team.Events),
			fmt.Sprintf("  Score: %d", team.Score),
			fmt.Sprintf("  Top player: %s (%d)", team.TopPlayer, team.TopPlayerScore),
			"",
		)
	}

	lines = append(lines, "Top player reports:")
	for i, report := range reports {
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
