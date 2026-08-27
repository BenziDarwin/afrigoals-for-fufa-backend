package services

import (
	"strings"

	"afrigoals.com/models"
)

type PositionGroup string

const (
	PositionGK      PositionGroup = "GK"
	PositionDEF     PositionGroup = "DEF"
	PositionMID     PositionGroup = "MID"
	PositionFWD     PositionGroup = "FWD"
	PositionUnknown PositionGroup = "UNKNOWN"
)

// Keyword vocabularies for normalizePositionGroup. Player.Position has no
// schema enum (models/player.go: a free-text string), so this is a
// best-effort heuristic - checked in GK -> DEF -> FWD -> MID order so that
// e.g. "Sweeper-Keeper" resolves to GK (via "keeper") rather than DEF (via
// "sweeper"), and "Striker" resolves to FWD before any MID keyword could
// match a substring.
var (
	gkKeywords  = []string{"goalkeeper", "keeper", "gk"}
	defKeywords = []string{"centre-back", "center-back", "fullback", "full-back", "wing-back", "wingback", "back", "defender", "sweeper", "cb"}
	fwdKeywords = []string{"forward", "striker", "winger", "attacker", "fwd", "cf", "st"}
	midKeywords = []string{"midfield", "cdm", "cam", "cm", "dm", "am", "mid"}
)

// normalizePositionGroup keyword-matches a free-text Player.Position
// against GK/DEF/MID/FWD vocabularies. An unrecognized string returns
// PositionUnknown rather than being silently misassigned.
func normalizePositionGroup(rawPosition string) PositionGroup {
	p := strings.ToLower(strings.TrimSpace(rawPosition))
	if p == "" {
		return PositionUnknown
	}
	switch {
	case containsAny(p, gkKeywords):
		return PositionGK
	case containsAny(p, defKeywords):
		return PositionDEF
	case containsAny(p, fwdKeywords):
		return PositionFWD
	case containsAny(p, midKeywords):
		return PositionMID
	default:
		return PositionUnknown
	}
}

func containsAny(haystack string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}

// positionHeadlineMetricKeys returns which already-computed metric keys are
// shown as this position group's headline cards, so e.g. a goalkeeper is
// never headlined on shot_accuracy_pct and a defender is never headlined
// primarily on goals.
func positionHeadlineMetricKeys(group PositionGroup) []string {
	switch group {
	case PositionGK:
		return []string{"saves", "save_pct", "gk_claims_catches", "gk_distribution", "sweeper_actions", "goalkeeper_errors", "goals_conceded"}
	case PositionDEF:
		return []string{"tackles", "tackle_success_pct", "interceptions", "clearances", "aerial_duels", "recoveries", "defensive_errors_general", "fouls_committed"}
	case PositionMID:
		return []string{"pass_completion_pct", "key_passes", "progressive_passes", "recoveries", "tackles", "possession_lost", "dangerous_turnovers"}
	case PositionFWD:
		return []string{"goals", "assists", "shots_total", "shots_on_target", "shot_accuracy_pct", "big_chances", "dribbles", "key_passes"}
	default:
		return []string{"goals", "assists", "tackles", "interceptions", "pass_completion_pct"}
	}
}

type playerPerformanceReport struct {
	PlayerID        uint                  `json:"player_id"`
	PlayerName      string                `json:"player_name"`
	JerseyNumber    int                   `json:"jersey_number"`
	ClubID          uint                  `json:"club_id"`
	PositionRaw     string                `json:"position_raw"`
	PositionGroup   PositionGroup         `json:"position_group"`
	EventCount      int                   `json:"event_count"`
	HeadlineMetrics []performanceMetric   `json:"headline_metrics"`
	EventTypeCounts map[string]int        `json:"event_type_counts"`
	ZoneFrequency   map[string]int        `json:"zone_frequency"`
	PhysicalStats   *physicalStatsAverage `json:"physical_stats"`
	EventIDs        []uint                `json:"event_ids"`
	ClipIDs         []uint                `json:"clip_ids"`
}

// buildPlayerPerformanceReports emits one report per player with at least
// one tagged event in this match. Players with zero events are omitted
// entirely, never shown with fabricated zeros. Headline metrics are always
// selected from within the player's own PositionGroup - callers must not
// rank or compare players across groups (e.g. no cross-position "MVP").
func buildPlayerPerformanceReports(players []models.Player, events []models.AnalysisEvent, stats []models.AnalysisEventStats, clipEventIDs map[uint]bool) []playerPerformanceReport {
	eventsByPlayer := map[uint][]models.AnalysisEvent{}
	eventPlayerByID := map[uint]uint{}
	for _, e := range events {
		if e.PlayerID == nil {
			continue
		}
		eventsByPlayer[*e.PlayerID] = append(eventsByPlayer[*e.PlayerID], e)
		eventPlayerByID[e.ID] = *e.PlayerID
	}

	statsByPlayer := map[uint][]models.AnalysisEventStats{}
	for _, s := range stats {
		pid := uintOrZero(s.PlayerID)
		if pid == 0 {
			pid = eventPlayerByID[s.AnalysisEventID]
		}
		if pid == 0 {
			continue
		}
		statsByPlayer[pid] = append(statsByPlayer[pid], s)
	}

	reports := []playerPerformanceReport{}
	for _, p := range players {
		playerEvents := eventsByPlayer[p.ID]
		if len(playerEvents) == 0 {
			continue
		}

		group := normalizePositionGroup(p.Position)
		combined := combinedMetricSections(playerEvents)
		headline := selectMetrics(combined, positionHeadlineMetricKeys(group))

		typeCounts := map[string]int{}
		for _, e := range playerEvents {
			typeCounts[e.Type]++
		}

		ids := eventIDs(playerEvents)
		reports = append(reports, playerPerformanceReport{
			PlayerID:        p.ID,
			PlayerName:      strings.TrimSpace(p.Name),
			JerseyNumber:    p.JerseyNumber,
			ClubID:          p.ClubID,
			PositionRaw:     p.Position,
			PositionGroup:   group,
			EventCount:      len(playerEvents),
			HeadlineMetrics: headline,
			EventTypeCounts: typeCounts,
			ZoneFrequency:   zoneFrequency(playerEvents),
			PhysicalStats:   averagePhysicalStats(statsByPlayer[p.ID]),
			EventIDs:        ids,
			ClipIDs:         clipIDsFor(ids, clipEventIDs),
		})
	}
	return reports
}

// combinedMetricSections computes all three metric sections for one
// player's events and flattens them into a single lookup, so
// positionHeadlineMetricKeys can pull a key from whichever section it lives in.
func combinedMetricSections(events []models.AnalysisEvent) metricSection {
	var all []performanceMetric
	all = append(all, computeAttackingMetrics(events).Metrics...)
	all = append(all, computeDefensiveMetrics(events).Metrics...)
	all = append(all, computeTransitionMetrics(events).Metrics...)
	return metricSection{Metrics: all}
}

func selectMetrics(section metricSection, keys []string) []performanceMetric {
	out := []performanceMetric{}
	for _, key := range keys {
		if m, ok := metricByKey(section, key); ok {
			out = append(out, m)
		}
	}
	return out
}

// averagePhysicalStats mirrors physicalStatsByClub's nil-safe averaging
// (player_analysis_reports.go) but scoped to one player's stats rows.
// Returns nil when the player has no AnalysisEventStats rows at all - never
// a fabricated zero.
func averagePhysicalStats(stats []models.AnalysisEventStats) *physicalStatsAverage {
	if len(stats) == 0 {
		return nil
	}
	var distanceSum, speedSum, maxSpeedSum, sprintsSum, touchesSum float64
	var distanceN, speedN, maxSpeedN, sprintsN, touchesN int
	for _, s := range stats {
		if s.DistanceCoveredM != nil {
			distanceSum += *s.DistanceCoveredM
			distanceN++
		}
		if s.AverageSpeedKmh != nil {
			speedSum += *s.AverageSpeedKmh
			speedN++
		}
		if s.MaxSpeedKmh != nil {
			maxSpeedSum += *s.MaxSpeedKmh
			maxSpeedN++
		}
		if s.SprintsCount != nil {
			sprintsSum += float64(*s.SprintsCount)
			sprintsN++
		}
		if s.TouchesCount != nil {
			touchesSum += float64(*s.TouchesCount)
			touchesN++
		}
	}
	avg := func(sum float64, n int) *float64 {
		if n == 0 {
			return nil
		}
		v := sum / float64(n)
		return &v
	}
	return &physicalStatsAverage{
		SampleSize:          len(stats),
		AvgDistanceCoveredM: avg(distanceSum, distanceN),
		AvgSpeedKmh:         avg(speedSum, speedN),
		AvgMaxSpeedKmh:      avg(maxSpeedSum, maxSpeedN),
		AvgSprints:          avg(sprintsSum, sprintsN),
		AvgTouches:          avg(touchesSum, touchesN),
	}
}
