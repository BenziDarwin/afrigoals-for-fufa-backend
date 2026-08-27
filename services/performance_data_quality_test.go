package services

import (
	"testing"

	"afrigoals.com/models"
)

func TestComputeDataQuality_ComponentsSumToOverallAverage(t *testing.T) {
	events := []models.AnalysisEvent{
		{ID: 1, Type: "tackle", EventTypeID: uintPtr(1), PlayerID: uintPtr(1), PitchZone: strPtr("def-left")},
		{ID: 2, Type: "pass_completed", EventTypeID: uintPtr(2), PlayerID: nil},
	}
	eventTypesByValue := map[string]models.EventType{
		"tackle":         {Value: "tackle", RequiresPlayer: true, Priority: models.EventPriorityHigh},
		"pass_completed": {Value: "pass_completed", RequiresPlayer: true, RequiresSecondaryPlayer: true, Priority: models.EventPriorityMedium},
	}

	report := computeDataQuality(events, nil, map[uint]bool{}, eventTypesByValue)

	var sum float64
	for _, c := range report.Components {
		sum += c.Pct
	}
	want := sum / float64(len(report.Components))
	if report.OverallCompletenessPct != want {
		t.Fatalf("expected overall %.4f to equal the mean of components %.4f", report.OverallCompletenessPct, want)
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
