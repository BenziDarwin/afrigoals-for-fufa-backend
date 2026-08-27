package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"afrigoals.com/database"
	"afrigoals.com/middleware"
	"afrigoals.com/models"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// loadMatchForAssistantAccess fetches just enough of the match to run the
// same canViewMatchPerformance() check the report page itself uses, so
// access to the assistant always matches access to the report it's
// grounded in - a club manager who can see the report can ask it and read
// its saved answers; an analyst assigned to the other club's match cannot.
func loadMatchForAssistantAccess(c *fiber.Ctx, matchID uint) (*models.User, *models.Match, error) {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return nil, nil, c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var match models.Match
	if err := database.DB.First(&match, matchID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Match not found"})
		}
		return nil, nil, c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}
	if !canViewMatchPerformance(user, &match) {
		return nil, nil, c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "You do not have access to this match's performance report",
		})
	}

	return user, &match, nil
}

// AskMatchAssistant answers a question about a match using only the data
// already tagged for it (events, player reports, team performance), and
// saves the question/answer pair to match_ai_insights so it persists as
// part of the match's report - visible to anyone who later opens it, always
// clearly an AI-generated answer since that table holds nothing else. It
// does not look at the video/clips themselves - this is Q&A over structured
// data, not vision analysis of a clip.
func AskMatchAssistant(c *fiber.Ctx) error {
	matchID, err := strconv.ParseUint(c.Params("match_id"), 10, 32)
	if err != nil || matchID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid match ID"})
	}

	user, _, accessErr := loadMatchForAssistantAccess(c, uint(matchID))
	if accessErr != nil {
		return accessErr
	}

	var req struct {
		Question string `json:"question"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	req.Question = strings.TrimSpace(req.Question)
	if req.Question == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "question is required"})
	}

	apiKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	if apiKey == "" {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "The AI assistant is not configured on this server (missing ANTHROPIC_API_KEY).",
		})
	}

	matchContext, err := buildCoachAssistantContext(c.Context(), uint(matchID))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to load match data",
			"details": err.Error(),
		})
	}

	client := anthropic.NewClient(option.WithAPIKey(apiKey))

	resp, err := client.Messages.New(c.Context(), anthropic.MessageNewParams{
		Model:     "claude-opus-5",
		MaxTokens: 2048,
		System: []anthropic.TextBlockParam{{
			Text: matchAssistantSystemPrompt(matchContext),
		}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(req.Question)),
		},
	})
	if err != nil {
		log.Printf("ai-assistant: anthropic request failed for match %d: %v", matchID, err)
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   "The AI assistant could not answer right now.",
			"details": err.Error(),
		})
	}

	var answer strings.Builder
	for _, block := range resp.Content {
		if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
			answer.WriteString(tb.Text)
		}
	}

	insight := models.MatchAIInsight{
		MatchID:  uint(matchID),
		Question: req.Question,
		Answer:   answer.String(),
		AskedBy:  &user.ID,
	}
	if err := database.DB.Create(&insight).Error; err != nil {
		// The analyst still gets their answer even if persistence fails -
		// don't fail a working Q&A over a save error.
		log.Printf("ai-assistant: failed to save insight for match %d: %v", matchID, err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"answer":  answer.String(),
			"insight": insight,
		},
	})
}

// ListMatchAIInsights: GET /api/v1/analyst-matches/:match_id/ai-assistant/insights
// Returns every AI-answered question saved for this match, oldest first, so
// the report page can show the assistant's saved analysis to anyone who
// later views the report - not just the browser session that asked it.
func ListMatchAIInsights(c *fiber.Ctx) error {
	matchID, err := strconv.ParseUint(c.Params("match_id"), 10, 32)
	if err != nil || matchID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid match ID"})
	}

	if _, _, accessErr := loadMatchForAssistantAccess(c, uint(matchID)); accessErr != nil {
		return accessErr
	}

	var insights []models.MatchAIInsight
	if err := database.DB.
		Where("match_id = ?", matchID).
		Order("created_at ASC").
		Find(&insights).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    fiber.Map{"insights": insights},
	})
}

func matchAssistantSystemPrompt(matchContext string) string {
	return "You are a football (soccer) coaching-insights assistant. You are given the same " +
		"Phase 1 performance report (measured/estimated metrics, evidence-based strengths and " +
		"weaknesses, and per-player headline metrics) a data analyst and coach already see for " +
		"this match.\n\n" +
		"When asked which team or player performed better, ground your judgment in the metrics " +
		"and diagnosis findings below - cite the specific numbers or findings you're using, and " +
		"say so plainly if the tagged data doesn't clearly support a verdict either way. Never " +
		"compare players across different position groups (their headline metrics aren't " +
		"comparable). When asked how to create more goal-scoring chances, base suggestions on " +
		"this match's actual weaknesses (e.g. a low shot-accuracy or big-chance-conversion " +
		"reading, a specific weakness finding) rather than generic coaching advice - if nothing " +
		"in the data points to a specific fix, say so rather than inventing one. Answer using " +
		"ONLY the data below. Keep answers concise and specific.\n\n" +
		matchContext
}

// buildCoachAssistantContext reuses the exact Phase 1 performance-report
// pipeline (metrics, diagnosis, player reports) the report page renders -
// see GetMatchPerformanceReport in performance_reports.go - so the
// assistant's answers about who performed better and how to create more
// goal-scoring chances stay grounded in the same numbers the coach/analyst
// already sees there, not a separate, looser summary.
func buildCoachAssistantContext(ctx context.Context, matchID uint) (string, error) {
	inputs, err := loadMatchPerformanceInputs(ctx, matchID)
	if err != nil {
		return "", err
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

	playerReports := buildPlayerPerformanceReports(inputs.Players, inputs.Events, inputs.Stats, inputs.ClipEventIDs)

	var b strings.Builder

	fmt.Fprintf(&b, "Match: %s vs %s\n", inputs.Match.HomeClub.Name, inputs.Match.AwayClub.Name)
	if inputs.Match.ScoreHome != nil && inputs.Match.ScoreAway != nil {
		fmt.Fprintf(&b, "Score: %d - %d\n", *inputs.Match.ScoreHome, *inputs.Match.ScoreAway)
	}
	fmt.Fprintf(&b, "Date: %s\n\n", inputs.Match.Date.Format("2006-01-02"))

	writeMetrics := func(label string, section metricSection) {
		var parts []string
		for _, m := range section.Metrics {
			if m.Status == StatusUnavailable || m.Value == nil {
				continue
			}
			if m.Unit == "percent" {
				parts = append(parts, fmt.Sprintf("%s: %.0f%%", m.Label, *m.Value))
			} else {
				parts = append(parts, fmt.Sprintf("%s: %.0f", m.Label, *m.Value))
			}
		}
		if len(parts) > 0 {
			fmt.Fprintf(&b, "%s - %s\n", label, strings.Join(parts, ", "))
		}
	}

	writeFindings := func(label string, findings []performanceFinding) {
		if len(findings) == 0 {
			return
		}
		fmt.Fprintf(&b, "%s:\n", label)
		for _, f := range findings {
			fmt.Fprintf(&b, "  - %s (%s confidence): %s\n", f.Title, f.Confidence, f.Evidence)
		}
	}

	writeTeam := func(name, side string, bundle teamMetricsBundle, strengths, weaknesses []performanceFinding) {
		fmt.Fprintf(&b, "== %s (%s) ==\n", name, side)
		writeMetrics("Attacking", bundle.Attacking)
		writeMetrics("Defensive", bundle.Defensive)
		writeMetrics("Transition", bundle.Transition)
		writeFindings("Strengths", strengths)
		writeFindings("Weaknesses", weaknesses)
		b.WriteString("\n")
	}

	writeTeam(inputs.Match.HomeClub.Name, "Home", homeBundle, homeStrengths, homeWeaknesses)
	writeTeam(inputs.Match.AwayClub.Name, "Away", awayBundle, awayStrengths, awayWeaknesses)

	if len(playerReports) > 0 {
		b.WriteString("Player performance (headline metrics are only comparable within the same position group):\n")
		for _, p := range playerReports {
			var parts []string
			for _, m := range p.HeadlineMetrics {
				if m.Value == nil {
					continue
				}
				if m.Unit == "percent" {
					parts = append(parts, fmt.Sprintf("%s: %.0f%%", m.Label, *m.Value))
				} else {
					parts = append(parts, fmt.Sprintf("%s: %.0f", m.Label, *m.Value))
				}
			}
			fmt.Fprintf(&b, "  %s (#%d, %s, %d tagged events): %s\n",
				p.PlayerName, p.JerseyNumber, p.PositionGroup, p.EventCount, strings.Join(parts, ", "))
		}
	}

	return b.String(), nil
}
