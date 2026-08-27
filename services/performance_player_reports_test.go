package services

import (
	"testing"

	"afrigoals.com/models"
	"gorm.io/gorm"
)

func TestNormalizePositionGroup_TableOfCommonStrings(t *testing.T) {
	cases := map[string]PositionGroup{
		"Goalkeeper":       PositionGK,
		"Sweeper-Keeper":   PositionGK,
		"Center Back":      PositionDEF,
		"Left Back":        PositionDEF,
		"CDM":              PositionMID,
		"CAM":              PositionMID,
		"Striker":          PositionFWD,
		"Right Winger":     PositionFWD,
		"Some Made Up Job": PositionUnknown,
		"":                 PositionUnknown,
	}
	for raw, want := range cases {
		got := normalizePositionGroup(raw)
		if got != want {
			t.Fatalf("normalizePositionGroup(%q) = %s, want %s", raw, got, want)
		}
	}
}

func TestPositionHeadlineMetricKeys_GKNeverGetsOutfieldAttackingKeys(t *testing.T) {
	forbidden := map[string]bool{"shot_accuracy_pct": true, "goals": true, "assists": true, "dribbles": true}
	for _, key := range positionHeadlineMetricKeys(PositionGK) {
		if forbidden[key] {
			t.Fatalf("GK headline metrics should never include %q", key)
		}
	}
}

func TestBuildPlayerPerformanceReports_OmitsZeroEventPlayers(t *testing.T) {
	players := []models.Player{
		{Model: gorm.Model{ID: 1}, ClubID: 5, Name: "Has Events", Position: "Striker"},
		{Model: gorm.Model{ID: 2}, ClubID: 5, Name: "No Events", Position: "Defender"},
	}
	events := []models.AnalysisEvent{
		{ID: 1, Type: "goal", PlayerID: uintPtr(1)},
	}

	reports := buildPlayerPerformanceReports(players, events, nil, map[uint]bool{})

	if len(reports) != 1 {
		t.Fatalf("expected exactly 1 report (zero-event player omitted), got %d", len(reports))
	}
	if reports[0].PlayerID != 1 {
		t.Fatalf("expected the report to be for player 1, got player %d", reports[0].PlayerID)
	}
}

func TestBuildPlayerPerformanceReports_PositionGroupAssigned(t *testing.T) {
	players := []models.Player{
		{Model: gorm.Model{ID: 1}, ClubID: 5, Name: "Keeper", Position: "Goalkeeper"},
	}
	events := []models.AnalysisEvent{
		{ID: 1, Type: "save", PlayerID: uintPtr(1)},
	}
	reports := buildPlayerPerformanceReports(players, events, nil, map[uint]bool{})
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
	if reports[0].PositionGroup != PositionGK {
		t.Fatalf("expected PositionGK, got %s", reports[0].PositionGroup)
	}
}

func TestBuildPlayerPerformanceReports_ZoneFrequencyScopedToPlayer(t *testing.T) {
	players := []models.Player{
		{Model: gorm.Model{ID: 1}, ClubID: 5, Name: "Player One", Position: "Striker"},
		{Model: gorm.Model{ID: 2}, ClubID: 5, Name: "Player Two", Position: "Defender"},
	}
	events := []models.AnalysisEvent{
		{ID: 1, Type: "shot", PlayerID: uintPtr(1), PitchZone: strPtr("att-left")},
		{ID: 2, Type: "shot", PlayerID: uintPtr(1), PitchZone: strPtr("att-left")},
		{ID: 3, Type: "tackle", PlayerID: uintPtr(2), PitchZone: strPtr("def-center")},
	}

	reports := buildPlayerPerformanceReports(players, events, nil, map[uint]bool{})
	if len(reports) != 2 {
		t.Fatalf("expected 2 reports, got %d", len(reports))
	}

	byID := map[uint]playerPerformanceReport{}
	for _, r := range reports {
		byID[r.PlayerID] = r
	}

	if byID[1].ZoneFrequency["att-left"] != 2 {
		t.Fatalf("expected player 1's zone_frequency to reflect only their own events, got %v", byID[1].ZoneFrequency)
	}
	if _, ok := byID[1].ZoneFrequency["def-center"]; ok {
		t.Fatalf("player 1's zone_frequency must not include player 2's zones, got %v", byID[1].ZoneFrequency)
	}
	if byID[2].ZoneFrequency["def-center"] != 1 {
		t.Fatalf("expected player 2's zone_frequency to reflect only their own events, got %v", byID[2].ZoneFrequency)
	}
}
