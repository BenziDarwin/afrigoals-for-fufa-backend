package services

import (
	"testing"

	"afrigoals.com/models"
)

func TestComputeDataQuality_OverallUsesTaggingComponentsOnly(t *testing.T) {
	events := []models.AnalysisEvent{
		{ID: 1, Type: "tackle", EventTypeID: uintPtr(1), PlayerID: uintPtr(1), PitchZone: strPtr("def-left")},
		{ID: 2, Type: "pass_completed", EventTypeID: uintPtr(2), PlayerID: uintPtr(2), SecondaryPlayerID: uintPtr(3), PitchZone: strPtr("mid-center")},
	}
	eventTypesByValue := map[string]models.EventType{
		"tackle":         {Value: "tackle", RequiresPlayer: true, Priority: models.EventPriorityHigh},
		"pass_completed": {Value: "pass_completed", RequiresPlayer: true, RequiresSecondaryPlayer: true, Priority: models.EventPriorityMedium},
	}

	report := computeDataQuality(events, nil, map[uint]bool{}, eventTypesByValue)

	if report.OverallCompletenessPct != 100 {
		t.Fatalf("expected complete tagging to show 100%% overall, got %.4f", report.OverallCompletenessPct)
	}
	physicalStats, ok := dataQualityComponentByKey(report.Components, "physical_stats_coverage")
	if !ok {
		t.Fatalf("expected physical_stats_coverage component")
	}
	if physicalStats.Pct != 0 {
		t.Fatalf("expected physical stats enrichment to remain separate at 0%%, got %.4f", physicalStats.Pct)
	}
}

func TestComputeDataQuality_EmptyMatchIsZeroNotDivideByZero(t *testing.T) {
	report := computeDataQuality(nil, nil, map[uint]bool{}, map[string]models.EventType{})
	if report.OverallCompletenessPct != 0 {
		t.Fatalf("expected 0%% completeness for an empty match, got %v", report.OverallCompletenessPct)
	}
	if report.TotalEvents != 0 {
		t.Fatalf("expected 0 total events, got %d", report.TotalEvents)
	}
	for _, c := range report.Components {
		if c.Pct != 0 {
			t.Fatalf("expected every component to be 0%% on an empty match, got %+v", c)
		}
	}
}

func dataQualityComponentByKey(components []dataQualityComponent, key string) (dataQualityComponent, bool) {
	for _, c := range components {
		if c.Key == key {
			return c, true
		}
	}
	return dataQualityComponent{}, false
}
