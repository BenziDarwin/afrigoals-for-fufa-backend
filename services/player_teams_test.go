package services

import (
	"testing"

	"afrigoals.com/models"
	"gorm.io/gorm"
)

func TestDualTeamSquadNumbers(t *testing.T) {
	nationalID, nationalShirt := uint(20), 7
	player := models.Player{Model: gorm.Model{ID: 3}, ClubID: 10, JerseyNumber: 17, NationalTeamID: &nationalID, NationalJerseyNumber: &nationalShirt}
	clubSquad := []models.Player{player}
	projectSquad(clubSquad, 10)
	if clubSquad[0].JerseyNumber != 17 {
		t.Fatal("club roster lost its club shirt number")
	}
	nationalSquad := []models.Player{player}
	projectSquad(nationalSquad, nationalID)
	if nationalSquad[0].ID != player.ID || nationalSquad[0].ClubID != 10 || nationalSquad[0].JerseyNumber != 7 || nationalSquad[0].ClubJerseyNumber != 17 {
		t.Fatalf("national roster must retain the player's identity and club registration: %+v", nationalSquad[0])
	}
}

func TestInternationalMatchUsesNationalRegistration(t *testing.T) {
	nationalID, nationalShirt := uint(20), 7
	club := models.Club{Model: gorm.Model{ID: 10}, Name: "Vipers", TeamType: "club"}
	national := models.Club{Model: gorm.Model{ID: nationalID}, Name: "Uganda Cranes", TeamType: "national"}
	player := models.Player{Model: gorm.Model{ID: 3}, ClubID: club.ID, Club: club, JerseyNumber: 17, NationalTeamID: &nationalID, NationalTeam: &national, NationalJerseyNumber: &nationalShirt}
	roster := []models.Player{player}
	projectMatchSquad(roster, models.Match{HomeClubID: nationalID, AwayClubID: 30})
	if roster[0].ClubID != nationalID || roster[0].Club.Name != national.Name || roster[0].JerseyNumber != 7 || roster[0].ClubTeam == nil || roster[0].ClubTeam.ID != club.ID {
		t.Fatalf("international match used an incorrect team registration: %+v", roster[0])
	}
	if player.ClubID != club.ID || player.JerseyNumber != 17 {
		t.Fatal("match projection changed the original club registration")
	}
	clubRoster := []models.Player{player}
	projectMatchSquad(clubRoster, models.Match{HomeClubID: club.ID, AwayClubID: 40})
	if clubRoster[0].ClubID != club.ID || clubRoster[0].JerseyNumber != 17 {
		t.Fatal("club match used national registration")
	}
}

func TestUnknownNationalShirtDoesNotUseClubShirt(t *testing.T) {
	nationalID := uint(20)
	roster := []models.Player{{ClubID: 10, JerseyNumber: 17, NationalTeamID: &nationalID}}
	projectSquad(roster, nationalID)
	if roster[0].JerseyNumber != 0 || roster[0].ClubJerseyNumber != 17 {
		t.Fatal("unknown national shirt must remain unassigned")
	}
}
