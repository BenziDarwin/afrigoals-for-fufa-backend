package services

import (
	"testing"

	"afrigoals.com/models"
)

func TestRuleShotAccuracyGap_NoFindingBelowSampleFloor(t *testing.T) {
	own := buildBundle(1, "shot", "shot", "goal")             // 2/3 on target-ish, but only 3 shots
	opponent := buildBundle(2, "shot", "shot", "shot", "shot", "shot")
	finding := ruleShotAccuracyGap(own, opponent, map[uint]bool{})
	if finding != nil {
		t.Fatalf("expected no finding below the min-5-shots sample floor, got %+v", finding)
	}
}

func TestRuleShotAccuracyGap_NoFindingWithoutCorroboration(t *testing.T) {
	// Own has a big accuracy gap below the opponent, but MORE big chances
	// than the opponent - corroboration disagrees, so no finding should fire.
	own := buildBundle(1, "shot", "shot", "shot", "shot", "shot", "big_chance", "big_chance", "big_chance")
	opponent := buildBundle(2, "shot", "shot", "shot", "shot", "shot_on_target", "goal")
	finding := ruleShotAccuracyGap(own, opponent, map[uint]bool{})
	if finding != nil {
		t.Fatalf("expected no finding when big_chances corroboration disagrees, got %+v", finding)
	}
}

func TestRuleShotAccuracyGap_HighConfidenceWithCorroboration(t *testing.T) {
	own := buildBundle(1, "shot", "shot", "shot", "shot", "shot") // 0% accuracy, 0 big chances
	opponent := buildBundle(2, "shot", "shot", "shot", "shot", "shot_on_target", "goal", "big_chance", "big_chance")
	finding := ruleShotAccuracyGap(own, opponent, map[uint]bool{})
	if finding == nil {
		t.Fatalf("expected a weakness finding")
	}
	if finding.Type != "weakness" {
		t.Fatalf("expected type=weakness, got %s", finding.Type)
	}
	if finding.Confidence != "high" {
		t.Fatalf("expected high confidence (large gap + corroboration), got %s", finding.Confidence)
	}
}

func TestRuleDisciplineRisk_RequiresACard(t *testing.T) {
	own := buildBundle(1, "foul_committed", "foul_committed", "foul_committed", "foul_committed", "foul_committed")
	opponent := buildBundle(2, "foul_committed")
	finding := ruleDisciplineRisk(own, opponent, map[uint]bool{})
	if finding != nil {
		t.Fatalf("expected no finding without any card, got %+v", finding)
	}
}

func TestRuleDisciplineRisk_FiresWithCard(t *testing.T) {
	own := buildBundle(1, "foul_committed", "foul_committed", "foul_committed", "foul_committed", "foul_committed", "yellow_card")
	opponent := buildBundle(2, "foul_committed")
	finding := ruleDisciplineRisk(own, opponent, map[uint]bool{})
	if finding == nil {
		t.Fatalf("expected a weakness finding when fouls lead and a card was shown")
	}
	if finding.Confidence != "medium" {
		t.Fatalf("expected medium confidence with exactly one card, got %s", finding.Confidence)
	}
}

func TestDiagnoseTeam_ClipIDsOnlySetWhenClipExists(t *testing.T) {
	own := buildBundle(1, "foul_committed", "foul_committed", "foul_committed", "foul_committed", "foul_committed", "yellow_card")
	opponent := buildBundle(2, "foul_committed")

	// Only the yellow_card event (id 6, 1-indexed by buildBundle) has a clip.
	clipEventIDs := map[uint]bool{6: true}
	_, weaknesses := diagnoseTeam(own, opponent, clipEventIDs)

	found := false
	for _, w := range weaknesses {
		if w.MetricKey != "fouls_committed" {
			continue
		}
		found = true
		for _, id := range w.ClipIDs {
			if id != 6 {
				t.Fatalf("expected only event 6 in ClipIDs, got %v", w.ClipIDs)
			}
		}
		if len(w.ClipIDs) != 1 {
			t.Fatalf("expected exactly one clip id, got %v", w.ClipIDs)
		}
	}
	if !found {
		t.Fatalf("expected the discipline-risk finding to be present")
	}
}

// buildBundle constructs a teamMetricsBundle from a sequence of EventType
// values, assigning sequential IDs starting at 1, for use as diagnosis-rule
// test fixtures.
func buildBundle(clubID uint, types ...string) teamMetricsBundle {
	events := make([]models.AnalysisEvent, 0, len(types))
	for i, t := range types {
		events = append(events, models.AnalysisEvent{ID: uint(i + 1), Type: t})
	}
	return teamMetricsBundle{
		ClubID:     clubID,
		Events:     events,
		Attacking:  computeAttackingMetrics(events),
		Defensive:  computeDefensiveMetrics(events),
		Transition: computeTransitionMetrics(events),
	}
}
