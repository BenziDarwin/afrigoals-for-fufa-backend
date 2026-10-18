package services

import (
	"afrigoals.com/models"
	"math"
	"testing"
)

func TestTrackingReviewRejectsCyclesAndConflictingPlayers(t *testing.T) {
	roles := map[string]string{"1": "player", "2": "player", "3": "referee"}
	roster := map[uint]models.Player{10: {}, 11: {}}
	doc := emptyTrackingReview()
	doc.Lineup = []uint{10, 11}
	doc.Tracks = map[string]TrackCorrection{"1": {CanonicalID: "2"}, "2": {CanonicalID: "1"}}
	if err := validateTrackingReview(&doc, roles, 4, roster, 1, 2); err == nil {
		t.Fatal("merge cycle accepted")
	}
	p1, p2 := uint(10), uint(11)
	doc.Tracks = map[string]TrackCorrection{"1": {CanonicalID: "2", PlayerID: &p1}, "2": {CanonicalID: "2", PlayerID: &p2}}
	if err := validateTrackingReview(&doc, roles, 4, roster, 1, 2); err == nil {
		t.Fatal("conflicting merged players accepted")
	}
	doc.Tracks = map[string]TrackCorrection{"1": {CanonicalID: "3"}}
	if err := validateTrackingReview(&doc, roles, 4, roster, 1, 2); err == nil {
		t.Fatal("player merged into referee")
	}
}

func TestTrackingReviewChecksRosterEventsAndExclusions(t *testing.T) {
	roles := map[string]string{"1": "player", "2": "player", "3": "referee"}
	roster := map[uint]models.Player{10: {}}
	doc := emptyTrackingReview()
	doc.Lineup = []uint{99}
	if err := validateTrackingReview(&doc, roles, 4, roster, 1, 2); err == nil {
		t.Fatal("foreign player accepted")
	}
	doc.Lineup = []uint{10}
	p := uint(10)
	doc.Tracks = map[string]TrackCorrection{"3": {CanonicalID: "3", PlayerID: &p}}
	if err := validateTrackingReview(&doc, roles, 4, roster, 1, 2); err == nil {
		t.Fatal("referee assigned to roster player")
	}
	doc.Tracks = map[string]TrackCorrection{"1": {CanonicalID: "2"}, "2": {CanonicalID: "2", PlayerID: &p}}
	if err := validateTrackingReview(&doc, roles, 4, roster, 1, 2); err != nil {
		t.Fatal(err)
	}
	doc.Events["9"] = EventCorrection{Status: "confirmed"}
	if err := validateTrackingReview(&doc, roles, 4, roster, 1, 2); err == nil {
		t.Fatal("unknown event accepted")
	}
	delete(doc.Events, "9")
	nan := math.NaN()
	doc.Events["0"] = EventCorrection{Status: "confirmed", Time: &nan}
	if err := validateTrackingReview(&doc, roles, 4, roster, 1, 2); err == nil {
		t.Fatal("NaN event time accepted")
	}
	delete(doc.Events, "0")
	doc.ExcludedSegments = []ExcludedVideoSegment{{Start: 1, End: 4, Reason: "replay"}, {Start: 3, End: 5, Reason: "closeup"}}
	if err := validateTrackingReview(&doc, roles, 4, roster, 1, 2); err == nil {
		t.Fatal("overlapping exclusions accepted")
	}
}
