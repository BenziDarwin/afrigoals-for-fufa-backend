package services

import (
	"afrigoals.com/models"
	"testing"
)

func TestSuggestionDecisionsUseNormalMergeValidation(t *testing.T) {
	source := map[string]any{"stats": map[string]any{"track_merge_suggestions": []any{
		map[string]any{"from_track": float64(1), "to_track": float64(2)},
		map[string]any{"from_track": float64(2), "to_track": float64(3)},
	}}}
	doc := emptyTrackingReview()
	doc.SuggestionDecisions = map[string]string{"1:2": "accepted", "2:3": "accepted"}
	if err := applySuggestionDecisions(&doc, source); err != nil {
		t.Fatal(err)
	}
	root, err := canonicalTrack("3", doc.Tracks)
	if err != nil || root != "1" {
		t.Fatal("accepted chain did not collapse", root, err)
	}
	p1, p2 := uint(7), uint(8)
	doc.Lineup = []uint{7, 8}
	doc.Tracks["1"] = TrackCorrection{CanonicalID: "1", PlayerID: &p1}
	doc.Tracks["2"] = TrackCorrection{CanonicalID: "1", PlayerID: &p2}
	if validateTrackingReview(&doc, map[string]string{"1": "player", "2": "player", "3": "player"}, 0, map[uint]models.Player{7: {}, 8: {}}, 1, 2) == nil {
		t.Fatal("accepted merge allowed conflicting players")
	}
	doc = emptyTrackingReview()
	doc.SuggestionDecisions = map[string]string{"1:2": "rejected"}
	if err := applySuggestionDecisions(&doc, source); err != nil {
		t.Fatal(err)
	}
	if len(doc.Tracks) != 0 {
		t.Fatal("rejection changed the identity map")
	}
	doc.SuggestionDecisions["4:5"] = "accepted"
	if applySuggestionDecisions(&doc, source) == nil {
		t.Fatal("unknown suggestion accepted")
	}
}

func TestSuggestionRejectsExistingCycle(t *testing.T) {
	doc := emptyTrackingReview()
	doc.Tracks = map[string]TrackCorrection{"1": {CanonicalID: "2"}, "2": {CanonicalID: "1"}}
	doc.SuggestionDecisions = map[string]string{"1:2": "accepted"}
	source := map[string]any{"stats": map[string]any{"track_merge_suggestions": []any{map[string]any{"from_track": float64(1), "to_track": float64(2)}}}}
	if applySuggestionDecisions(&doc, source) == nil {
		t.Fatal("cycle accepted")
	}
}

func TestSuggestionRejectionsSurviveOlderClients(t *testing.T) {
	previous := emptyTrackingReview()
	previous.SuggestionDecisions = map[string]string{"1:2": "rejected"}
	current := emptyTrackingReview()
	if err := preserveRejectedSuggestions(&current, previous); err != nil {
		t.Fatal(err)
	}
	if current.SuggestionDecisions["1:2"] != "rejected" {
		t.Fatal("old client erased rejected pair")
	}
	current.SuggestionDecisions["1:2"] = "accepted"
	if preserveRejectedSuggestions(&current, previous) == nil {
		t.Fatal("rejected suggestion accepted")
	}
}
