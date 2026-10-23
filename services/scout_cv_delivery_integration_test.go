package services

import (
	"afrigoals.com/database"
	"afrigoals.com/models"
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestScoutCVDeliveryIntegration(t *testing.T) {
	if os.Getenv("SCOUT_CV_INTEGRATION") != "1" {
		t.Skip("set SCOUT_CV_INTEGRATION=1")
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
	var player models.Player
	if err := tx.Where("id IN (SELECT player_id FROM analysis_events WHERE player_id IS NOT NULL)").First(&player).Error; err != nil {
		t.Fatal(err)
	}
	users := []models.User{{Email: uuid.NewString() + "@example.test", Password: "test", Role: models.AfrigoalsAdmin}, {Email: uuid.NewString() + "@example.test", Password: "test", Role: models.Scout}, {Email: uuid.NewString() + "@example.test", Password: "test", Role: models.Scout}}
	for i := range users {
		if err := tx.Create(&users[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	current := users[0]
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("user", current); return c.Next() })
	app.Post("/send", SendScoutCV)
	app.Get("/inbox", ScoutCVInbox)
	app.Get("/players/:id", GetPlayerProfileAnalysis)
	app.Post("/requests", RequestScoutCV)
	app.Get("/requests", ListScoutCVRequests)
	app.Post("/requests/:id/respond", RespondScoutCVRequest)
	current = users[1]
	requestID := uint(0)
	for n := 0; n < 2; n++ {
		req := httptest.NewRequest("POST", "/requests", strings.NewReader(fmt.Sprintf(`{"player_id":%d}`, player.ID)))
		req.Header.Set("Content-Type", "application/json")
		res, err := app.Test(req, 30000)
		if err != nil {
			t.Fatal(err)
		}
		var requested models.ScoutCVRequest
		err = json.NewDecoder(res.Body).Decode(&requested)
		res.Body.Close()
		if err != nil || res.StatusCode != 201 || requested.ClubID != player.ClubID {
			t.Fatalf("request failed: %v status=%d", err, res.StatusCode)
		}
		if n == 1 && requestID != requested.ID {
			t.Fatal("duplicate request created")
		}
		requestID = requested.ID
	}
	var grants int64
	tx.Model(&models.ScoutCVDelivery{}).Where("scout_id=?", users[1].ID).Count(&grants)
	if grants != 0 {
		t.Fatal("request must not grant CV access")
	}
	// A scout cannot approve their own request.
	req := httptest.NewRequest("POST", fmt.Sprintf("/requests/%d/respond", requestID), strings.NewReader(`{"action":"share"}`))
	req.Header.Set("Content-Type", "application/json")
	response, err := app.Test(req, 30000)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatal("scout self-approved CV")
	}
	wrongClub := player.ClubID + 1
	current = models.User{ID: users[0].ID, Role: models.ClubManager, ClubID: &wrongClub}
	wrongReq := httptest.NewRequest("POST", fmt.Sprintf("/requests/%d/respond", requestID), strings.NewReader(`{"action":"share"}`))
	wrongReq.Header.Set("Content-Type", "application/json")
	denied, err := app.Test(wrongReq, 30000)
	if err != nil {
		t.Fatal(err)
	}
	denied.Body.Close()
	if denied.StatusCode != 403 {
		t.Fatal("wrong club shared CV")
	}
	listed, err := app.Test(httptest.NewRequest("GET", "/requests", nil), 30000)
	if err != nil {
		t.Fatal(err)
	}
	var queue []models.ScoutCVRequest
	err = json.NewDecoder(listed.Body).Decode(&queue)
	listed.Body.Close()
	if err != nil || len(queue) != 0 {
		t.Fatal("another club can see request")
	}
	current = users[0]
	req = httptest.NewRequest("POST", fmt.Sprintf("/requests/%d/respond", requestID), strings.NewReader(`{"action":"share"}`))
	req.Header.Set("Content-Type", "application/json")
	response, err = app.Test(req, 30000)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("team approval failed: %d", response.StatusCode)
	}
	// Discovery can see database evidence before any delivery to another scout.
	discovered, _, err := loadScoutDatabaseCandidates(t.Context(), scoutFilters{Query: player.Name}, 1, true, &users[2])
	if err != nil {
		t.Fatal(err)
	}
	foundEvidence := false
	for _, candidate := range discovered {
		if candidate.ID == player.ID && candidate.Events > 0 {
			foundEvidence = true
		}
	}
	if !foundEvidence {
		t.Fatal("database discovery lost analyst evidence")
	}
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/send", strings.NewReader(fmt.Sprintf(`{"player_id":%d,"email":%q}`, player.ID, users[1].Email)))
		req.Header.Set("Content-Type", "application/json")
		res, err := app.Test(req, 30000)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 201 {
			t.Fatalf("send: %d", res.StatusCode)
		}
	}
	current = users[1]
	res, err := app.Test(httptest.NewRequest("GET", fmt.Sprintf("/players/%d", player.ID), nil), 30000)
	if err != nil {
		t.Fatal(err)
	}
	var profile playerProfileAnalysis
	err = json.NewDecoder(res.Body).Decode(&profile)
	res.Body.Close()
	if err != nil || res.StatusCode != 200 || profile.Scope != "directly_shared_data" || profile.EventCount == 0 {
		t.Fatalf("delivered CV missing stats: status=%d scope=%s events=%d err=%v", res.StatusCode, profile.Scope, profile.EventCount, err)
	}
	candidates, _, err := loadScoutCandidates(t.Context(), scoutFilters{Query: player.Name}, 1, &users[1])
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, candidate := range candidates {
		if candidate.ID == player.ID {
			found = true
			if candidate.Events != profile.EventCount {
				t.Fatalf("AI search/CV disagree: %d vs %d", candidate.Events, profile.EventCount)
			}
		}
	}
	if !found {
		t.Fatal("shared player missing from AI candidate pool")
	}
	for i := 1; i <= 2; i++ {
		current = users[i]
		res, err := app.Test(httptest.NewRequest("GET", "/inbox", nil), 30000)
		if err != nil {
			t.Fatal(err)
		}
		var rows []struct {
			PlayerID uint `json:"player_id"`
		}
		err = json.NewDecoder(res.Body).Decode(&rows)
		res.Body.Close()
		if err != nil || res.StatusCode != 200 {
			t.Fatalf("inbox failed: %v status %d", err, res.StatusCode)
		}
		want := 0
		if i == 1 {
			want = 1
		}
		if len(rows) != want {
			t.Fatalf("scout %d got %d rows, want %d", i, len(rows), want)
		}
		if i == 1 && rows[0].PlayerID != player.ID {
			t.Fatal("wrong player delivered")
		}
	}
}
