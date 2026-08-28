package services

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"afrigoals.com/models"
)

type scoringPrediction struct {
	ThreatScore     float64              `json:"threat_score"`
	LikelihoodPct   float64              `json:"likelihood_pct"`
	LikelyToScore   bool                 `json:"likely_to_score"`
	Confidence      string               `json:"confidence"`
	Evidence        string               `json:"evidence"`
	SampleSize      int                  `json:"sample_size"`
	ContributingIDs []uint               `json:"event_ids"`
	Timeline        []scoringThreatPoint `json:"timeline"`
	ThreatByLevel   []scoringThreatLevel `json:"threat_by_level"`
}

type scoringThreatPoint struct {
	Minute          int     `json:"minute"`
	EventType       string  `json:"event_type"`
	EventID         uint    `json:"event_id"`
	ThreatAdded     float64 `json:"threat_added"`
	CumulativeScore float64 `json:"cumulative_score"`
}

type scoringThreatLevel struct {
	Level       string  `json:"level"`
	ThreatScore float64 `json:"threat_score"`
	SharePct    float64 `json:"share_pct"`
}

type possessionLevel struct {
	Level       string  `json:"level"`
	EventCount  int     `json:"event_count"`
	SharePct    float64 `json:"share_pct"`
	Description string  `json:"description"`
}

type possessionLevels struct {
	DefensiveThird possessionLevel `json:"defensive_third"`
	MiddleThird    possessionLevel `json:"middle_third"`
	AttackingThird possessionLevel `json:"attacking_third"`
	OverallPct     float64         `json:"overall_pct"`
	SampleSize     int             `json:"sample_size"`
	Notes          string          `json:"notes"`
}

var scoringThreatWeights = map[string]float64{
	"goal":                        6,
	"goal_disallowed":             3,
	"shot_on_target":              4,
	"big_chance":                  4,
	"shot":                        2,
	"assist":                      2.5,
	"key_pass":                    2,
	"through_ball":                1.5,
	"pass_into_penalty_area":      1.5,
	"set_piece_chance_created":    1.5,
	"set_piece_goal":              3,
	"cross":                       1,
	"cutback":                     1.25,
	"counter_attack":              1,
	"fast_break":                  1,
	"attacking_transition":        0.75,
	"progressive_pass":            0.75,
	"passes_into_final_third":     0.75,
	"carry_into_final_third":      0.75,
	"dribble":                     0.5,
	"one_v_one_attack":            0.5,
	"numerical_advantage_created": 0.75,
}

func computeScoringPredictions(homeEvents, awayEvents []models.AnalysisEvent) (scoringPrediction, scoringPrediction) {
	homeScore, homeIDs, homeSample := scoringThreatScore(homeEvents)
	awayScore, awayIDs, awaySample := scoringThreatScore(awayEvents)
	homeTimeline := scoringThreatTimeline(homeEvents)
	awayTimeline := scoringThreatTimeline(awayEvents)
	homeThreatByLevel := scoringThreatByLevel(homeEvents, homeScore)
	awayThreatByLevel := scoringThreatByLevel(awayEvents, awayScore)
	total := homeScore + awayScore

	homePct, awayPct := 0.0, 0.0
	if total > 0 {
		homePct = 100 * homeScore / total
		awayPct = 100 * awayScore / total
	}

	home := buildScoringPrediction(homeScore, homePct, homePct > awayPct, homeSample, homeIDs, awayScore, homeTimeline, homeThreatByLevel)
	away := buildScoringPrediction(awayScore, awayPct, awayPct > homePct, awaySample, awayIDs, homeScore, awayTimeline, awayThreatByLevel)
	return home, away
}

func scoringThreatScore(events []models.AnalysisEvent) (float64, []uint, int) {
	var score float64
	var ids []uint
	var sample int
	for _, e := range events {
		weight, ok := scoringThreatWeights[e.Type]
		if !ok {
			continue
		}
		score += weight
		ids = append(ids, e.ID)
		sample++
	}
	return math.Round(score*10) / 10, ids, sample
}

func scoringThreatTimeline(events []models.AnalysisEvent) []scoringThreatPoint {
	sorted := append([]models.AnalysisEvent{}, events...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].TimestampSeconds < sorted[j].TimestampSeconds
	})

	points := []scoringThreatPoint{}
	var cumulative float64
	for _, e := range sorted {
		weight, ok := scoringThreatWeights[e.Type]
		if !ok {
			continue
		}
		cumulative += weight
		points = append(points, scoringThreatPoint{
			Minute:          int(math.Floor(e.TimestampSeconds / 60)),
			EventType:       e.Type,
			EventID:         e.ID,
			ThreatAdded:     weight,
			CumulativeScore: math.Round(cumulative*10) / 10,
		})
	}
	return points
}

func scoringThreatByLevel(events []models.AnalysisEvent, totalScore float64) []scoringThreatLevel {
	levels := []struct {
		key   string
		label string
	}{
		{key: "defensive_third", label: "Defensive third"},
		{key: "middle_third", label: "Middle third"},
		{key: "attacking_third", label: "Attacking third"},
	}
	scoreByLevel := map[string]float64{}
	for _, e := range events {
		weight, ok := scoringThreatWeights[e.Type]
		if !ok {
			continue
		}
		level, ok := possessionLevelFromZone(e.PitchZone)
		if !ok {
			continue
		}
		scoreByLevel[level] += weight
	}
	out := make([]scoringThreatLevel, 0, len(levels))
	for _, level := range levels {
		score := math.Round(scoreByLevel[level.key]*10) / 10
		share := 0.0
		if totalScore > 0 {
			share = math.Round(100 * score / totalScore)
		}
		out = append(out, scoringThreatLevel{Level: level.label, ThreatScore: score, SharePct: share})
	}
	return out
}

func buildScoringPrediction(score, pct float64, likely bool, sample int, ids []uint, opponentScore float64, timeline []scoringThreatPoint, threatByLevel []scoringThreatLevel) scoringPrediction {
	confidence := "low"
	if sample >= 8 && math.Abs(score-opponentScore) >= 4 {
		confidence = "high"
	} else if sample >= 4 && math.Abs(score-opponentScore) >= 2 {
		confidence = "medium"
	}
	return scoringPrediction{
		ThreatScore:     score,
		LikelihoodPct:   math.Round(pct),
		LikelyToScore:   likely,
		Confidence:      confidence,
		Evidence:        fmt.Sprintf("Estimated from %d attacking threat events using weighted shots, big chances, final-third entries and chance-creation tags.", sample),
		SampleSize:      sample,
		ContributingIDs: ids,
		Timeline:        timeline,
		ThreatByLevel:   threatByLevel,
	}
}

func computePossessionLevels(ownEvents, opponentEvents []models.AnalysisEvent) possessionLevels {
	ownCounts := possessionZoneCounts(ownEvents)
	opponentCounts := possessionZoneCounts(opponentEvents)
	totalOwn := ownCounts["defensive_third"] + ownCounts["middle_third"] + ownCounts["attacking_third"]
	totalOpponent := opponentCounts["defensive_third"] + opponentCounts["middle_third"] + opponentCounts["attacking_third"]
	total := totalOwn + totalOpponent

	return possessionLevels{
		DefensiveThird: possessionLevelFor("defensive_third", "Defensive third", ownCounts, opponentCounts),
		MiddleThird:    possessionLevelFor("middle_third", "Middle third", ownCounts, opponentCounts),
		AttackingThird: possessionLevelFor("attacking_third", "Attacking third", ownCounts, opponentCounts),
		OverallPct:     sharePct(totalOwn, total),
		SampleSize:     totalOwn,
		Notes:          "Event-based possession proxy from attributed, zone-tagged events; not clock-time possession.",
	}
}

func possessionZoneCounts(events []models.AnalysisEvent) map[string]int {
	counts := map[string]int{"defensive_third": 0, "middle_third": 0, "attacking_third": 0}
	for _, e := range events {
		level, ok := possessionLevelFromZone(e.PitchZone)
		if !ok {
			continue
		}
		counts[level]++
	}
	return counts
}

func possessionLevelFromZone(zone *string) (string, bool) {
	if zone == nil {
		return "", false
	}
	z := strings.ToLower(strings.TrimSpace(*zone))
	switch {
	case strings.HasPrefix(z, "def-"), strings.Contains(z, "defensive"):
		return "defensive_third", true
	case strings.HasPrefix(z, "mid-"), strings.Contains(z, "middle"):
		return "middle_third", true
	case strings.HasPrefix(z, "att-"), strings.Contains(z, "attack"), strings.Contains(z, "final"):
		return "attacking_third", true
	default:
		return "", false
	}
}

func possessionLevelFor(key, label string, ownCounts, opponentCounts map[string]int) possessionLevel {
	own := ownCounts[key]
	opponent := opponentCounts[key]
	return possessionLevel{
		Level:       label,
		EventCount:  own,
		SharePct:    sharePct(own, own+opponent),
		Description: fmt.Sprintf("%d zone-tagged events in this level.", own),
	}
}

func sharePct(own, total int) float64 {
	if total == 0 {
		return 0
	}
	return math.Round(100 * float64(own) / float64(total))
}
