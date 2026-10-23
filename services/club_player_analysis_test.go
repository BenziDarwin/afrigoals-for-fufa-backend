package services

import (
	"afrigoals.com/models"
	"gorm.io/gorm"
	"testing"
)

func TestSquadSummariesMatchCVAndKeepPlayerIDsSeparate(t *testing.T) {
	players := []models.Player{{Model: gorm.Model{ID: 7}, Name: "Same Name", Position: "Midfielder"}, {Model: gorm.Model{ID: 8}, Name: "Same Name", Position: "Forward"}}
	matches := []models.Match{{Model: gorm.Model{ID: 1}}, {Model: gorm.Model{ID: 2}}}
	goal := profileTestEvent(1, 1, 7, "goal")
	assist := profileTestEvent(2, 1, 8, "assist")
	hidden := profileTestEvent(3, 3, 7, "goal")
	reports := []models.PlayerAnalysisReport{{PlayerID: 7, MatchID: 2}}
	events := []models.AnalysisEvent{goal, goal, assist, hidden}
	summary := squadAnalysisSummaries(players, matches, events, reports)
	if summary[7].Goals != 1 || summary[7].Assists != 0 || summary[7].Matches != 2 || summary[7].Events != 1 {
		t.Fatal("wrong player totals", summary[7])
	}
	if summary[8].Goals != 0 || summary[8].Assists != 1 || summary[8].Matches != 1 {
		t.Fatal("same-name players mixed", summary[8])
	}
	cv := buildPlayerProfileAnalysis(players[0], matches, events, reports, nil)
	if summary[7].Matches != cv.MatchCount || summary[7].Events != cv.EventCount {
		t.Fatal("cards disagree with CV")
	}
	empty := squadAnalysisSummaries(players, nil, events, reports)
	if empty[7].Goals != 0 || empty[7].Matches != 0 {
		t.Fatal("hidden evidence counted")
	}
}
