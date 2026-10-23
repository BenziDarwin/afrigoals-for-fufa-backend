package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"

	"afrigoals.com/database"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

// Uses actual configured PostgreSQL data in an explicitly read-only transaction.
func TestPlayerProfileReadIntegration(t *testing.T) {
	if os.Getenv("PLAYER_PROFILE_INTEGRATION") != "1" {
		t.Skip("set PLAYER_PROFILE_INTEGRATION=1 for a read-only database check")
	}
	if err := godotenv.Load("../.env"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AUTO_MIGRATE", "false")
	previous := database.DB
	database.ConnectDB()
	tx := database.DB.Begin(&sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	database.DB = tx
	defer func() { tx.Rollback(); database.DB = previous }()
	var player models.Player
	err := tx.Where("id IN (SELECT player_id FROM analysis_events WHERE player_id IS NOT NULL)").First(&player).Error
	if err == gorm.ErrRecordNotFound {
		t.Skip("no analyst-linked players in configured database")
	}
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("user", models.User{Role: models.DataAnalyst}); return c.Next() })
	app.Get("/:id/profile-analysis", GetPlayerProfileAnalysis)
	response, err := app.Test(httptest.NewRequest("GET", fmt.Sprintf("/%d/profile-analysis", player.ID), nil), 30000)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("profile projection returned %d", response.StatusCode)
	}
	var result playerProfileAnalysis
	if err = json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.PlayerID != player.ID || result.Scope != "analyst_working_data" {
		t.Fatal("incorrect profile identity or scope")
	}
	if response.Header.Get("Cache-Control") != "private, no-store" {
		t.Fatal("profile evidence is cacheable")
	}
	app.Get("/clubs/:club_id/player-analysis", GetClubPlayerAnalysis)
	squadResponse, err := app.Test(httptest.NewRequest("GET", fmt.Sprintf("/clubs/%d/player-analysis", player.ClubID), nil), 30000)
	if err != nil {
		t.Fatal(err)
	}
	defer squadResponse.Body.Close()
	if squadResponse.StatusCode != 200 {
		t.Fatalf("squad summary returned %d", squadResponse.StatusCode)
	}
	var squad struct {
		Players map[uint]squadPlayerSummary `json:"players"`
	}
	if err = json.NewDecoder(squadResponse.Body).Decode(&squad); err != nil {
		t.Fatal(err)
	}
	if squad.Players[player.ID].Events != result.EventCount || squad.Players[player.ID].Matches != result.MatchCount {
		t.Fatal("real squad card totals differ from player CV")
	}
	t.Logf("Read-only projection: %d linked events, %d matches, %d playable clips", result.EventCount, result.MatchCount, len(result.Clips))
}
