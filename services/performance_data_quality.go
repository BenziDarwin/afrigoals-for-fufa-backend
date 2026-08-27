package services

import "afrigoals.com/models"

type dataQualityComponent struct {
	Key     string  `json:"key"`
	Label   string  `json:"label"`
	Present int     `json:"present"`
	Total   int     `json:"total"`
	Pct     float64 `json:"pct"`
}

type dataQualityReport struct {
	OverallCompletenessPct float64                `json:"overall_completeness_pct"`
	TotalEvents            int                    `json:"total_events"`
	Components             []dataQualityComponent `json:"components"`
}

// computeDataQuality scores six independent, auditable completeness signals
// and averages them (simple arithmetic mean, reproducible from Components -
// never an opaque single number), so the report never presents weakly
// tagged data as high-confidence analysis:
//
//  1. event_type_linked: EventTypeID != nil
//  2. player_attribution: PlayerID != nil, of events whose matched
//     EventType.RequiresPlayer == true
//  3. secondary_player_attribution: SecondaryPlayerID != nil, of events
//     whose matched EventType.RequiresSecondaryPlayer == true
//  4. pitch_zone_tagged: PitchZone non-empty
//  5. clip_coverage: has an R2 clip, of events whose matched
//     EventType.Priority is critical|high
//  6. physical_stats_coverage: has an attached AnalysisEventStats row
func computeDataQuality(events []models.AnalysisEvent, stats []models.AnalysisEventStats, clipEventIDs map[uint]bool, eventTypesByValue map[string]models.EventType) dataQualityReport {
	total := len(events)

	statsByEvent := map[uint]bool{}
	for _, s := range stats {
		statsByEvent[s.AnalysisEventID] = true
	}

	var eventTypeLinked, playerAttribRequired, playerAttribPresent int
	var secondaryRequired, secondaryPresent int
	var zoneTagged int
	var clipRequired, clipPresent int
	var physicalStatsPresent int

	for _, e := range events {
		if e.EventTypeID != nil {
			eventTypeLinked++
		}
		if e.PitchZone != nil && *e.PitchZone != "" {
			zoneTagged++
		}
		if statsByEvent[e.ID] {
			physicalStatsPresent++
		}

		et, hasType := eventTypesByValue[e.Type]
		if hasType && et.RequiresPlayer {
			playerAttribRequired++
			if e.PlayerID != nil {
				playerAttribPresent++
			}
		}
		if hasType && et.RequiresSecondaryPlayer {
			secondaryRequired++
			if e.SecondaryPlayerID != nil {
				secondaryPresent++
			}
		}
		if hasType && (et.Priority == models.EventPriorityCritical || et.Priority == models.EventPriorityHigh) {
			clipRequired++
			if clipEventIDs[e.ID] {
				clipPresent++
			}
		}
	}

	components := []dataQualityComponent{
		componentPct("event_type_linked", "Events linked to a known type", eventTypeLinked, total),
		componentPct("player_attribution", "Player attribution (where required)", playerAttribPresent, playerAttribRequired),
		componentPct("secondary_player_attribution", "Secondary player attribution (where required)", secondaryPresent, secondaryRequired),
		componentPct("pitch_zone_tagged", "Pitch zone tagged", zoneTagged, total),
		componentPct("clip_coverage", "Clip coverage (critical/high priority events)", clipPresent, clipRequired),
		componentPct("physical_stats_coverage", "Physical stats attached", physicalStatsPresent, total),
	}

	var sum float64
	for _, c := range components {
		sum += c.Pct
	}
	overall := 0.0
	if len(components) > 0 {
		overall = sum / float64(len(components))
	}

	return dataQualityReport{
		OverallCompletenessPct: overall,
		TotalEvents:            total,
		Components:             components,
	}
}

// componentPct returns a zero-value component (0/0, 0%) rather than
// dividing by zero when a signal has no applicable denominator in this
// match (e.g. no events require a secondary player).
func componentPct(key, label string, present, total int) dataQualityComponent {
	if total == 0 {
		return dataQualityComponent{Key: key, Label: label, Present: 0, Total: 0, Pct: 0}
	}
	return dataQualityComponent{Key: key, Label: label, Present: present, Total: total, Pct: 100 * float64(present) / float64(total)}
}
