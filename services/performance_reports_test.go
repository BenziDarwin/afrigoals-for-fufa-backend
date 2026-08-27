package services

import (
	"testing"

	"afrigoals.com/models"
)

func TestCanViewMatchPerformance_ClubManagerRejectedForOtherClub(t *testing.T) {
	user := &models.User{Role: models.ClubManager, ClubID: uintPtr(99)}
	match := &models.Match{HomeClubID: 1, AwayClubID: 2}
	if canViewMatchPerformance(user, match) {
		t.Fatalf("expected a club manager unrelated to either club to be rejected")
	}
}

func TestCanViewMatchPerformance_ClubManagerAllowedForOwnClub(t *testing.T) {
	user := &models.User{Role: models.ClubManager, ClubID: uintPtr(1)}
	match := &models.Match{HomeClubID: 1, AwayClubID: 2}
	if !canViewMatchPerformance(user, match) {
		t.Fatalf("expected a club manager of the home club to be allowed")
	}

	awayManager := &models.User{Role: models.ClubManager, ClubID: uintPtr(2)}
	if !canViewMatchPerformance(awayManager, match) {
		t.Fatalf("expected a club manager of the away club to be allowed")
	}
}

func TestCanViewMatchPerformance_DataAnalystAlwaysAllowed(t *testing.T) {
	user := &models.User{Role: models.DataAnalyst}
	match := &models.Match{HomeClubID: 1, AwayClubID: 2}
	if !canViewMatchPerformance(user, match) {
		t.Fatalf("expected a data analyst to always be allowed, matching every other analyst-matches endpoint")
	}
}

func TestCanViewMatchPerformance_AfrigoalsAdminAlwaysAllowed(t *testing.T) {
	user := &models.User{Role: models.AfrigoalsAdmin}
	match := &models.Match{HomeClubID: 1, AwayClubID: 2}
	if !canViewMatchPerformance(user, match) {
		t.Fatalf("expected a platform admin to always be allowed")
	}
}

// The LeagueAdmin path additionally needs CanManageClub's DB-backed
// club_leagues lookup - this repo has no DB-backed test harness today, so
// that case is an integration-test gap rather than something faked here.
