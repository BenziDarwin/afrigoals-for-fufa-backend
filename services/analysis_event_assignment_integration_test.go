package services

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"afrigoals.com/database"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestAnalysisEventUnassignIntegration(t *testing.T) {
	dsn := os.Getenv("AFRIGOALS_COUNTRY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("AFRIGOALS_COUNTRY_TEST_DATABASE_URL is required")
	}
	for _, key := range []string{"R2_ACCESS_KEY_ID", "CLOUDFLARE_R2_ACCESS_KEY_ID"} {
		t.Setenv(key, "")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	prior := database.DB
	database.DB = tx
	defer func() { database.DB = prior }()
	var match models.Match
	var player models.Player
	var author models.User
	for _, model := range []any{&match, &player, &author} {
		if err := tx.First(model).Error; err != nil {
			t.Fatal(err)
		}
	}
	clip := "https://example.com/test-existing-clip.mp4"
	name := player.Name
	event := models.AnalysisEvent{MatchID: match.ID, Type: "shot", TimestampSeconds: 123, TeamID: &match.HomeClubID, PlayerID: &player.ID, PlayerName: &name, CreatedBy: author.ID, ClipURL: &clip}
	if err := tx.Create(&event).Error; err != nil {
		t.Fatal(err)
	}
	stats := models.AnalysisEventStats{ID: 1999999001, AnalysisEventID: event.ID, PlayerID: &player.ID, PlayerName: &name, Stats: models.JSONB{}}
	if err := tx.Create(&stats).Error; err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Put("/matches/:match_id/events/:id", UpdateAnalysisEvent)
	request := func(body string, status int) {
		t.Helper()
		req := httptest.NewRequest("PUT", fmt.Sprintf("/matches/%d/events/%d", match.ID, event.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		res, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if res.StatusCode != status {
			t.Fatalf("expected %d, got %d", status, res.StatusCode)
		}
	}
	request(`{"notes":"Keep the assignment"}`, 200)
	var loaded models.AnalysisEvent
	if err := tx.First(&loaded, event.ID).Error; err != nil {
		t.Fatal(err)
	}
	if loaded.PlayerID == nil || *loaded.PlayerID != player.ID {
		t.Fatal("omitted player_id changed the assignment")
	}
	request(`{"player_id":null,"player_name":"stale name"}`, 200)
	loaded = models.AnalysisEvent{}
	if err := tx.First(&loaded, event.ID).Error; err != nil {
		t.Fatal(err)
	}
	var loadedStats models.AnalysisEventStats
	if err := tx.First(&loadedStats, stats.ID).Error; err != nil {
		t.Fatal(err)
	}
	if loaded.PlayerID != nil || loaded.PlayerName != nil || loadedStats.PlayerID != nil || loadedStats.PlayerName != nil {
		t.Fatal("event or statistics retained the player assignment")
	}
	if loaded.TeamID == nil || *loaded.TeamID != match.HomeClubID || loaded.Type != "shot" || loaded.TimestampSeconds != 123 || loaded.ClipURL == nil || *loaded.ClipURL != clip {
		t.Fatal("unassigning changed event context")
	}
	request(`{"player_id":null}`, 200)
	payload, _ := json.Marshal(map[string]any{"player_id": player.ID, "player_name": player.Name})
	request(string(payload), 200)
	loaded = models.AnalysisEvent{}
	loadedStats = models.AnalysisEventStats{}
	if err := tx.First(&loaded, event.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.First(&loadedStats, stats.ID).Error; err != nil {
		t.Fatal(err)
	}
	if loaded.PlayerID == nil || *loaded.PlayerID != player.ID || loadedStats.PlayerID == nil || *loadedStats.PlayerID != player.ID {
		t.Fatal("reassignment failed")
	}
	request(`{"player_id":"invalid"}`, 400)
}
