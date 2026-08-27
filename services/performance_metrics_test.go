package services

import (
	"testing"

	"afrigoals.com/models"
)

func uintPtr(v uint) *uint       { return &v }
func strPtr(v string) *string    { return &v }
func floatPtr(v float64) *float64 { return &v }

func TestResolveEventClub_PrefersTeamID(t *testing.T) {
	event := models.AnalysisEvent{TeamID: uintPtr(3), PlayerID: nil}
	clubID, ok := resolveEventClub(event, map[uint]uint{})
	if !ok || clubID != 3 {
		t.Fatalf("expected club 3 via TeamID, got %d ok=%v", clubID, ok)
	}
}

func TestResolveEventClub_FallsBackToPlayerClub(t *testing.T) {
	event := models.AnalysisEvent{TeamID: nil, PlayerID: uintPtr(10)}
	clubID, ok := resolveEventClub(event, map[uint]uint{10: 7})
	if !ok || clubID != 7 {
		t.Fatalf("expected club 7 via player fallback, got %d ok=%v", clubID, ok)
	}
}

func TestResolveEventClub_Unattributed(t *testing.T) {
	event := models.AnalysisEvent{TeamID: nil, PlayerID: nil}
	_, ok := resolveEventClub(event, map[uint]uint{})
	if ok {
		t.Fatalf("expected unattributed event to resolve to ok=false")
	}
}

func TestBucketEventsByClub_TeamLevelTransitionEventsAttributed(t *testing.T) {
	// Regression guard for the computeTeamPerformanceSummaries bug this
	// package deliberately does not repeat: team-level transition events
	// (requires_player=false, e.g. high_press) carry only TeamID.
	events := []models.AnalysisEvent{
		{ID: 1, Type: "high_press", TeamID: uintPtr(3)},
		{ID: 2, Type: "counter_attack", TeamID: uintPtr(7)},
	}
	home, away, unattributed := bucketEventsByClub(events, 3, 7, map[uint]uint{})
	if len(home) != 1 || home[0].ID != 1 {
		t.Fatalf("expected event 1 attributed to home club, got %+v", home)
	}
	if len(away) != 1 || away[0].ID != 2 {
		t.Fatalf("expected event 2 attributed to away club, got %+v", away)
	}
	if len(unattributed) != 0 {
		t.Fatalf("expected no unattributed events, got %+v", unattributed)
	}
}

func TestComputeAttackingMetrics_ShotAccuracy(t *testing.T) {
	events := []models.AnalysisEvent{
		{ID: 1, Type: "shot"},
		{ID: 2, Type: "shot"},
		{ID: 3, Type: "shot_on_target"},
		{ID: 4, Type: "goal"},
	}
	section := computeAttackingMetrics(events)

	shotsTotal, ok := metricByKey(section, "shots_total")
	if !ok || shotsTotal.Value == nil || *shotsTotal.Value != 4 {
		t.Fatalf("expected shots_total=4, got %+v", shotsTotal)
	}
	shotsOnTarget, ok := metricByKey(section, "shots_on_target")
	if !ok || shotsOnTarget.Value == nil || *shotsOnTarget.Value != 2 {
		t.Fatalf("expected shots_on_target=2, got %+v", shotsOnTarget)
	}
	accuracy, ok := metricByKey(section, "shot_accuracy_pct")
	if !ok || accuracy.Value == nil || *accuracy.Value != 50 {
		t.Fatalf("expected shot_accuracy_pct=50, got %+v", accuracy)
	}
	if accuracy.Status != StatusMeasured {
		t.Fatalf("expected shot_accuracy_pct to be measured, got %s", accuracy.Status)
	}
}

func TestComputeAttackingMetrics_UnavailableMetricsAlwaysNilValue(t *testing.T) {
	cases := [][]models.AnalysisEvent{
		nil,
		{{ID: 1, Type: "goal"}, {ID: 2, Type: "shot"}, {ID: 3, Type: "shot_on_target"}},
	}
	for _, events := range cases {
		section := computeAttackingMetrics(events)
		for _, key := range []string{"expected_goals_xg", "progressive_carry_distance", "shot_zone_map"} {
			m, ok := metricByKey(section, key)
			if !ok {
				t.Fatalf("expected metric %s to be present", key)
			}
			if m.Status != StatusUnavailable {
				t.Fatalf("expected %s to be unavailable, got %s", key, m.Status)
			}
			if m.Value != nil {
				t.Fatalf("expected %s value to be nil, got %v", key, *m.Value)
			}
		}
	}
}

func TestClassifyOutcome_RecognizedAndUnrecognizedStrings(t *testing.T) {
	cases := []struct {
		outcome           *string
		wantSuccess       bool
		wantClassified    bool
	}{
		{strPtr("Successful"), true, true},
		{strPtr("failed"), false, true},
		{strPtr("Won"), true, true},
		{strPtr("lost"), false, true},
		{strPtr("idk"), false, false},
		{strPtr(""), false, false},
		{nil, false, false},
	}
	for _, tc := range cases {
		success, classified := classifyOutcome(tc.outcome)
		if success != tc.wantSuccess || classified != tc.wantClassified {
			t.Fatalf("classifyOutcome(%v) = (%v,%v), want (%v,%v)", tc.outcome, success, classified, tc.wantSuccess, tc.wantClassified)
		}
	}
}

func TestEstimatedOutcomeRateMetric_StatusIsEstimatedNotMeasured(t *testing.T) {
	events := []models.AnalysisEvent{
		{ID: 1, Type: "tackle", Outcome: strPtr("successful")},
		{ID: 2, Type: "tackle", Outcome: strPtr("failed")},
	}
	m := estimatedOutcomeRateMetric("tackle_success_pct", "Tackle Success %", "defence", events)
	if m.Status != StatusEstimated {
		t.Fatalf("expected StatusEstimated, got %s", m.Status)
	}
	if m.Value == nil || *m.Value != 50 {
		t.Fatalf("expected 50%%, got %+v", m.Value)
	}
}

func TestEstimatedOutcomeRateMetric_UnclassifiedExcludedFromDenominator(t *testing.T) {
	events := []models.AnalysisEvent{
		{ID: 1, Type: "tackle", Outcome: strPtr("successful")},
		{ID: 2, Type: "tackle", Outcome: strPtr("not sure")},
		{ID: 3, Type: "tackle", Outcome: nil},
	}
	m := estimatedOutcomeRateMetric("tackle_success_pct", "Tackle Success %", "defence", events)
	if m.SampleSize != 1 {
		t.Fatalf("expected sample size 1 (only the classifiable event), got %d", m.SampleSize)
	}
	if m.Value == nil || *m.Value != 100 {
		t.Fatalf("expected 100%%, got %+v", m.Value)
	}
}

func TestPerNinety_NilAndZeroMinutesReturnNil(t *testing.T) {
	if v := perNinety(10, nil); v != nil {
		t.Fatalf("expected nil for nil minutesPlayed, got %v", *v)
	}
	zero := 0.0
	if v := perNinety(10, &zero); v != nil {
		t.Fatalf("expected nil for zero minutesPlayed, got %v", *v)
	}
}

func TestPerNinety_NormalValue(t *testing.T) {
	minutes := 45.0
	v := perNinety(9, &minutes)
	if v == nil || *v != 18 {
		t.Fatalf("expected 18 (9 events over 45 minutes -> 18 per 90), got %v", v)
	}
}

func TestZoneFrequency_TalliesNonEmptyZonesOnly(t *testing.T) {
	events := []models.AnalysisEvent{
		{ID: 1, Type: "shot", PitchZone: strPtr("att-left")},
		{ID: 2, Type: "shot", PitchZone: strPtr("att-left")},
		{ID: 3, Type: "shot", PitchZone: strPtr("  ")},
		{ID: 4, Type: "shot", PitchZone: nil},
		{ID: 5, Type: "shot", PitchZone: strPtr("mid-center")},
	}
	freq := zoneFrequency(events)
	if freq["att-left"] != 2 {
		t.Fatalf("expected att-left=2, got %d", freq["att-left"])
	}
	if freq["mid-center"] != 1 {
		t.Fatalf("expected mid-center=1, got %d", freq["mid-center"])
	}
	if len(freq) != 2 {
		t.Fatalf("expected only 2 zones tallied (blank/nil excluded), got %v", freq)
	}
}

func TestInjectConcededMetrics_CopiesOpponentAttackingNumbers(t *testing.T) {
	ownDefensive := computeDefensiveMetrics([]models.AnalysisEvent{{ID: 1, Type: "save"}})
	opponentAttacking := computeAttackingMetrics([]models.AnalysisEvent{
		{ID: 2, Type: "goal"}, {ID: 3, Type: "shot"}, {ID: 4, Type: "shot_on_target"},
	})

	result := injectConcededMetrics(ownDefensive, opponentAttacking)

	goalsConceded, ok := metricByKey(result, "goals_conceded")
	if !ok || goalsConceded.Value == nil || *goalsConceded.Value != 1 {
		t.Fatalf("expected goals_conceded=1, got %+v", goalsConceded)
	}
	shotsConceded, ok := metricByKey(result, "shots_conceded")
	if !ok || shotsConceded.Value == nil || *shotsConceded.Value != 3 {
		t.Fatalf("expected shots_conceded=3, got %+v", shotsConceded)
	}
	savePct, ok := metricByKey(result, "save_pct")
	if !ok || savePct.Value == nil {
		t.Fatalf("expected save_pct to be computed, got %+v", savePct)
	}
	if savePct.Status != StatusEstimated {
		t.Fatalf("expected save_pct to be estimated (match-level proxy), got %s", savePct.Status)
	}
}
