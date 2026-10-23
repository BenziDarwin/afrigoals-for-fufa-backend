package services

import (
	"afrigoals.com/database"
	"afrigoals.com/models"
	"fmt"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
	"os"
	"strings"
	"testing"
	"time"
)

// DDL and fixtures live in one rolled-back transaction; no real AI is called.
func TestScoutQuotaIntegration(t *testing.T) {
	if os.Getenv("SCOUT_INTEGRATION") != "1" {
		t.Skip("set SCOUT_INTEGRATION=1 for transactional PostgreSQL test")
	}
	if err := godotenv.Load("../.env"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AUTO_MIGRATE", "false")
	previous := database.DB
	database.ConnectDB()
	tx := database.DB.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	database.DB = tx
	defer func() { tx.Rollback(); database.DB = previous }()
	schema := "scout_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := tx.Exec(fmt.Sprintf("CREATE SCHEMA %q", schema)).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Exec(fmt.Sprintf("SET LOCAL search_path TO %q, public", schema)).Error; err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../migrations/000016_scout_ai_insights.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Exec(string(migration)).Error; err != nil {
		t.Fatal(err)
	}
	user := models.User{Email: uuid.NewString() + "@example.test", Password: "test-only", Role: models.Scout}
	if err = tx.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	// JSONB arrays and objects must survive the actual simple-protocol driver.
	var fixture models.AnalysisEvent
	if err := tx.Where("player_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM player_analysis_reports WHERE player_analysis_reports.player_id = analysis_events.player_id AND player_analysis_reports.match_id = analysis_events.match_id)").First(&fixture).Error; err == nil {
		report := models.PlayerAnalysisReport{MatchID: fixture.MatchID, PlayerID: *fixture.PlayerID, PlayerName: "Test fixture", ReportText: "JSON persistence test", CreatedBy: user.ID, EventTypes: models.StringSlice{"pass"}, EventIDs: models.UintSlice{fixture.ID}, AIStatsKeys: models.StringSlice{}, Metadata: models.JSONB{"test": true}}
		if err := tx.Create(&report).Error; err != nil {
			t.Fatal(err)
		}
		var read models.PlayerAnalysisReport
		if err := tx.First(&read, report.ID).Error; err != nil {
			t.Fatal(err)
		}
		if len(read.EventIDs) != 1 || len(read.EventTypes) != 1 || read.Metadata["test"] != true {
			t.Fatal("analyst report JSON did not round-trip")
		}
	}
	var first models.ScoutAIInsight
	for i := 0; i < 3; i++ {
		row, err := reserveScoutQuestion(tx, user.ID, fmt.Sprintf("question-%d", i), "Find a midfielder", "criteria")
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = row
		}
		if err = tx.Model(&row).Updates(map[string]any{"status": "completed", "answer": "Stored answer", "player_ids": models.UintSlice{}}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if _, err = reserveScoutQuestion(tx, user.ID, "question-fourth", "Find a midfielder", "criteria"); err == nil {
		t.Fatal("fourth trial question accepted")
	}
	retry, err := reserveScoutQuestion(tx, user.ID, first.RequestKey, first.Question, "criteria")
	if err != nil || retry.Status != "completed" {
		t.Fatal("completed retry charged or blocked", err)
	}
	if _, err = reserveScoutQuestion(tx, user.ID, first.RequestKey, first.Question, "other-filters"); err == nil {
		t.Fatal("request ID reused for different criteria")
	}
	// Provider failure releases a slot, but deleting browser storage cannot.
	if err = tx.Model(&models.ScoutAIInsight{}).Where("id = ?", first.ID).Update("status", "failed").Error; err != nil {
		t.Fatal(err)
	}
	pending, err := reserveScoutQuestion(tx, user.ID, "question-retry", "Another question", "criteria")
	if err != nil {
		t.Fatal(err)
	}
	used, err := scoutQuotaCount(tx, user.ID, time.Now())
	if err != nil || used != 3 {
		t.Fatal("pending reservation not counted", used, err)
	}
	if err = tx.Model(&pending).Update("created_at", time.Now().Add(-11*time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	used, err = scoutQuotaCount(tx, user.ID, time.Now())
	if err != nil || used != 2 {
		t.Fatal("expired reservation not released", used, err)
	}
	// A separate account has its own allowance and cannot access this history.
	other := models.User{Email: uuid.NewString() + "@example.test", Password: "test-only", Role: models.Scout}
	if err = tx.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	used, err = scoutQuotaCount(tx, other.ID, time.Now())
	if err != nil || used != 0 {
		t.Fatal("allowance leaked across accounts")
	}
	_, _, err = loadScoutCandidates(t.Context(), scoutFilters{Position: "MID"}, 1)
	if err != nil && !isMissingRecord(err) {
		t.Fatal(err)
	}
	down, err := os.ReadFile("../migrations/000016_scout_ai_insights.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Exec(string(down)).Error; err != nil {
		t.Fatal(err)
	}
	if err = tx.Exec(string(migration)).Error; err != nil {
		t.Fatal(err)
	}
}
func isMissingRecord(err error) bool { return err == gorm.ErrRecordNotFound }
