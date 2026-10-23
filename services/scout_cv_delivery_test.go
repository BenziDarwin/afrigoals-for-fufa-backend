package services

import (
	"afrigoals.com/models"
	"testing"
)

func TestScoutCVSenderAuthorization(t *testing.T) {
	club := uint(12)
	for _, tc := range []struct {
		role models.UserRole
		club *uint
		want bool
	}{
		{models.AfrigoalsAdmin, nil, true}, {models.DataAnalyst, nil, true},
		{models.ClubManager, &club, true}, {models.ClubManager, nil, false},
		{models.Scout, &club, false}, {models.LeagueAdmin, &club, false},
	} {
		if got := canSendScoutCV(&models.User{Role: tc.role, ClubID: tc.club}, models.Player{ClubID: 12}); got != tc.want {
			t.Errorf("role %s: got %v", tc.role, got)
		}
	}
	if canSendScoutCV(&models.User{Role: models.ClubManager, ClubID: &club}, models.Player{ClubID: 13}) {
		t.Fatal("another club must not send player CV")
	}
}

func TestScoutDeliveryScope(t *testing.T) {
	player := models.Player{ClubID: 12}
	match := models.Match{}
	sender := models.User{Role: models.DataAnalyst}
	scout := &models.User{Role: models.Scout}
	if !scoutDeliveryMatchVisible(scout, player, &match, []models.User{sender}) {
		t.Fatal("delivered pending evidence must be visible")
	}
	if scoutDeliveryMatchVisible(scout, player, &match, nil) {
		t.Fatal("unrelated scout must not see pending evidence")
	}
	sender.Role = models.Scout
	if scoutDeliveryMatchVisible(scout, player, &match, []models.User{sender}) {
		t.Fatal("sender no longer authorized")
	}
	if scoutDeliveryMatchVisible(nil, player, &match, []models.User{sender}) {
		t.Fatal("anonymous recipient")
	}
}
