package services

import (
	"afrigoals.com/models"
	"testing"
	"time"
)

func TestScoutRoleCanRegisterWithoutManagementPrivileges(t *testing.T) {
	if !models.IsSelfRegisterable(models.Scout) || !models.IsValidRole(models.Scout) {
		t.Fatal("scout role missing")
	}
	if models.IsSelfRegisterable(models.AfrigoalsAdmin) {
		t.Fatal("admin self-registration opened")
	}
	match := models.Match{HomeClubID: 1, AwayClubID: 2}
	scout := models.User{Role: models.Scout}
	if !profileMatchVisible(&scout, &match, models.ReportReviewDistributed) {
		t.Fatal("distributed player evidence hidden")
	}
	for _, status := range []models.MatchReportReviewStatus{models.ReportReviewApproved, models.ReportReviewDistributed} {
		if !profileMatchVisible(&scout, &match, status) {
			t.Fatalf("released scout evidence hidden: %s", status)
		}
	}
	for _, status := range []models.MatchReportReviewStatus{models.ReportReviewSubmitted, models.ReportReviewChangesRequested, ""} {
		if profileMatchVisible(&scout, &match, status) {
			t.Fatalf("unapproved scout evidence exposed: %s", status)
		}
	}
	if profileMatchVisible(&scout, &match, models.ReportReviewDraft) {
		t.Fatal("draft evidence exposed")
	}
}
func TestScoutSearchFilterValidation(t *testing.T) {
	low, high := 30, 20
	for _, f := range []scoutFilters{{Position: "admin"}, {MinAge: &low, MaxAge: &high}, {Query: string(make([]byte, 101))}} {
		if validateScoutFilters(&f) == nil {
			t.Fatal("invalid filters accepted", f)
		}
	}
	f := scoutFilters{Position: " mid ", Query: " name "}
	if validateScoutFilters(&f) != nil || f.Position != "MID" || f.Query != "name" {
		t.Fatal(f)
	}
	birth := time.Date(2000, 10, 24, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	if age := scoutAge(birth, now); age == nil || *age != 25 {
		t.Fatal("age birthday boundary")
	}
	if scoutAge(time.Time{}, now) != nil {
		t.Fatal("missing birth date invented an age")
	}
}
func TestScoutRecommendationsRejectInventedIDsAndDedupe(t *testing.T) {
	players := []scoutCandidate{{ID: 7, Name: "Actual Player"}}
	answer, ids, err := parseScoutAnswer("```json\n{\"answer\":\"Recorded evidence fits\",\"player_ids\":[7,7]}\n```", players)
	if err != nil || answer == "" || len(ids) != 1 || ids[0] != 7 {
		t.Fatal(answer, ids, err)
	}
	if _, _, err = parseScoutAnswer(`{"answer":"Invented player","player_ids":[999]}`, players); err == nil {
		t.Fatal("unknown player accepted")
	}
	if _, _, err = parseScoutAnswer(`{"answer":"","player_ids":[7]}`, players); err == nil {
		t.Fatal("empty answer accepted")
	}
	if _, ids, err = parseScoutAnswer(`{"answer":"No fit in this evidence","player_ids":[]}`, players); err != nil || len(ids) != 0 {
		t.Fatal("honest no-match answer rejected")
	}
}

func TestScoutQuestionRetrievesTheRequestedPosition(t *testing.T) {
	for question, want := range map[string]string{"Find a creative midfielder": "MID", "Find a goalkeeper with good distribution": "GK", "Need a striker": "FWD", "Show me a left back": "DEF", "Find a midfielder or defender": "", "Find a fast player": ""} {
		if got := scoutQuestionPosition(question); got != want {
			t.Fatalf("%s: %s, want %s", question, got, want)
		}
	}
}
