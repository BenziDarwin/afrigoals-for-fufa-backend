package services

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"afrigoals.com/database"
	"afrigoals.com/models"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/gofiber/fiber/v2"
)

// Cap the raw event list sent to the model - aggregate counts already cover
// most questions; the chronological list is for "when"/"which half" style
// questions and doesn't need to include every single tagged event to be
// useful, even for a very heavily-tagged match.
const maxAssistantContextEvents = 300

// AskMatchAssistant answers an analyst's question about a match using only
// the data already tagged for it (events, player reports, team performance).
// It does not look at the video/clips themselves - this is Q&A over
// structured data, not vision analysis of a clip.
func AskMatchAssistant(c *fiber.Ctx) error {
	matchID, err := strconv.ParseUint(c.Params("match_id"), 10, 32)
	if err != nil || matchID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid match ID"})
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

	matchContext, err := buildMatchAssistantContext(uint(matchID))
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

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"answer": answer.String(),
		},
	})
}

func matchAssistantSystemPrompt(matchContext string) string {
	return "You are a football (soccer) match analysis assistant helping a data analyst " +
		"understand a specific match they have been tagging events for.\n\n" +
		"Answer using ONLY the match data below. If the data does not contain the answer, " +
		"say so plainly instead of guessing or inventing statistics. Keep answers concise " +
		"and specific (cite player names, event types, and timestamps where relevant).\n\n" +
		matchContext
}

// buildMatchAssistantContext reuses the same aggregation the team-performance
// report/PDF is built from, plus the raw chronological event list, so the
// assistant's answers stay consistent with what the analyst sees elsewhere
// in the product.
func buildMatchAssistantContext(matchID uint) (string, error) {
	payload, err := computeTeamPerformanceSummaries(matchID)
	if err != nil {
		return "", err
	}

	var events []models.AnalysisEvent
	if err := database.DB.
		Where("match_id = ?", matchID).
		Order("timestamp_seconds ASC").
		Find(&events).Error; err != nil {
		return "", err
	}

	var b strings.Builder

	fmt.Fprintf(&b, "Match: %s vs %s\n", payload.Match.HomeClub.Name, payload.Match.AwayClub.Name)
	if payload.Match.ScoreHome != nil && payload.Match.ScoreAway != nil {
		fmt.Fprintf(&b, "Score: %d - %d\n", *payload.Match.ScoreHome, *payload.Match.ScoreAway)
	}
	fmt.Fprintf(&b, "Date: %s\n\n", payload.Match.Date.Format("2006-01-02"))

	for _, team := range payload.Teams {
		side := "Away"
		if team.IsHomeTeam {
			side = "Home"
		}
		fmt.Fprintf(&b, "Team: %s (%s)\n", team.TeamName, side)
		fmt.Fprintf(&b, "  Players with saved reports: %d, total performance score: %d\n", team.Reports, team.Score)
		if team.TopPlayer != "" && team.TopPlayer != "No data" {
			fmt.Fprintf(&b, "  Top performer: %s (score %d)\n", team.TopPlayer, team.TopPlayerScore)
		}
		if len(team.EventTypeCounts) > 0 {
			b.WriteString("  Event type counts: ")
			first := true
			for eventType, count := range team.EventTypeCounts {
				if !first {
					b.WriteString(", ")
				}
				fmt.Fprintf(&b, "%s=%d", eventType, count)
				first = false
			}
			b.WriteString("\n")
		}
		for _, pb := range team.PlayerBreakdown {
			fmt.Fprintf(&b, "  Player %s: score %d, %d tagged events\n", pb.PlayerName, pb.Score, pb.EventCount)
		}
		b.WriteString("\n")
	}

	if len(events) > 0 {
		b.WriteString("Tagged events, chronological (minute:second - type - player):\n")
		truncated := false
		list := events
		if len(list) > maxAssistantContextEvents {
			list = list[:maxAssistantContextEvents]
			truncated = true
		}
		for _, ev := range list {
			player := "unassigned"
			if ev.PlayerName != nil && strings.TrimSpace(*ev.PlayerName) != "" {
				player = *ev.PlayerName
			}
			minutes := int(ev.TimestampSeconds) / 60
			seconds := int(ev.TimestampSeconds) % 60
			fmt.Fprintf(&b, "  %02d:%02d - %s - %s", minutes, seconds, ev.Type, player)
			if ev.Outcome != nil && strings.TrimSpace(*ev.Outcome) != "" {
				fmt.Fprintf(&b, " (%s)", *ev.Outcome)
			}
			b.WriteString("\n")
		}
		if truncated {
			fmt.Fprintf(&b, "  ... (%d more events not shown)\n", len(events)-maxAssistantContextEvents)
		}
	}

	return b.String(), nil
}
