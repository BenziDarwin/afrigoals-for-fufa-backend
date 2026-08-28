package services

import (
	"testing"

	"afrigoals.com/models"
)

func TestComputeScoringPredictions_FavorsTeamWithStrongerThreatEvents(t *testing.T) {
	homeEvents := []models.AnalysisEvent{
		{ID: 1, Type: "shot_on_target"},
	}
	awayEvents := []models.AnalysisEvent{
		{ID: 2, Type: "shot_on_target"},
		{ID: 3, Type: "shot_on_target"},
		{ID: 4, Type: "big_chance"},
		{ID: 5, Type: "key_pass"},
	}

	home, away := computeScoringPredictions(homeEvents, awayEvents)

	if home.LikelyToScore {
		t.Fatalf("expected home not to be more likely to score, got %+v", home)
	}
	if !away.LikelyToScore {
		t.Fatalf("expected away to be more likely to score, got %+v", away)
	}
	if away.LikelihoodPct <= home.LikelihoodPct {
		t.Fatalf("expected away likelihood to exceed home, got home=%v away=%v", home.LikelihoodPct, away.LikelihoodPct)
	}
}

func TestComputePossessionLevels_ComputesEventBasedSharesByThird(t *testing.T) {
	ownEvents := []models.AnalysisEvent{
		{ID: 1, Type: "pass_completed", PitchZone: strPtr("def-left")},
		{ID: 2, Type: "pass_completed", PitchZone: strPtr("mid-center")},
		{ID: 3, Type: "pass_completed", PitchZone: strPtr("att-right")},
	}
	opponentEvents := []models.AnalysisEvent{
		{ID: 4, Type: "pass_completed", PitchZone: strPtr("att-left")},
	}

	possession := computePossessionLevels(ownEvents, opponentEvents)

	if possession.OverallPct != 75 {
		t.Fatalf("expected overall event possession proxy 75%%, got %v", possession.OverallPct)
	}
	if possession.AttackingThird.SharePct != 50 {
		t.Fatalf("expected attacking-third share 50%%, got %v", possession.AttackingThird.SharePct)
	}
	if possession.SampleSize != 3 {
		t.Fatalf("expected sample size 3, got %d", possession.SampleSize)
	}
}
