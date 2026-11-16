package services

import (
	"afrigoals.com/database"
	"afrigoals.com/models"
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestTeamReportSnapshotKeepsOnlyRecipient(t *testing.T) {
	payload := &teamPerformanceReportPayload{Teams: []teamPerformanceSummary{
		{ClubID: 1, TeamName: "Home", PlayerBreakdown: []playerPerformanceBreakdown{{PlayerID: 10}}},
		{ClubID: 2, TeamName: "Away", PlayerBreakdown: []playerPerformanceBreakdown{{PlayerID: 20}}},
	}, Reports: []models.PlayerAnalysisReport{{PlayerID: 10, PlayerName: "Home player"}, {PlayerID: 20, PlayerName: "Opponent confidential"}}}
	snapshot, err := teamReportSnapshot(payload, 1)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(snapshot)
	if strings.Contains(string(raw), "Opponent confidential") || !strings.Contains(string(raw), "Home player") {
		t.Fatalf("snapshot has wrong team data: %s", raw)
	}
	if _, err := teamReportSnapshot(payload, 999); err == nil {
		t.Fatal("unrelated club was accepted")
	}
}

func TestTeamReportWorkflowIntegration(t *testing.T) {
	dsn := os.Getenv("AFRIGOALS_COUNTRY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("AFRIGOALS_COUNTRY_TEST_DATABASE_URL is required")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := db.DB()
	defer pool.Close()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	prior := database.DB
	database.DB = tx
	defer func() { database.DB = prior }()
	// DDL and all report fixtures remain inside this rolled-back transaction.
	var exists bool
	if err := tx.Raw("SELECT to_regclass('public.team_match_reports') IS NOT NULL").Scan(&exists).Error; err != nil {
		t.Fatal(err)
	}
	if !exists {
		ddl, err := os.ReadFile("../migrations/000023_team_match_reports.up.sql")
		if err != nil {
			t.Fatal(err)
		}
		if err := tx.Exec(string(ddl)).Error; err != nil {
			t.Fatal(err)
		}
	}
	var match models.Match
	if err := tx.Preload("HomeClub").Preload("AwayClub").Where("home_club_id <> away_club_id").First(&match).Error; err != nil {
		t.Fatal(err)
	}
	var user models.User
	if err := tx.First(&user).Error; err != nil {
		t.Fatal(err)
	}
	// Isolate existing reports for this fixture, with rollback restoring them.
	if err := tx.Where("match_id=?", match.ID).Delete(&models.TeamMatchReport{}).Error; err != nil {
		t.Fatal(err)
	}
	user.Role = models.DataAnalyst
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("user", user); return c.Next() })
	app.Get("/matches/:match_id/team-reports", ListTeamMatchReports)
	app.Get("/matches/:match_id/team-reports/performance", GetMatchPerformanceReport)
	app.Put("/matches/:match_id/team-reports/:club_id", SaveTeamMatchReport)
	app.Post("/matches/:match_id/team-reports/:club_id/status", ChangeTeamMatchReportStatus)
	request := func(method, path, body string, status int) map[string]any {
		t.Helper()
		req := httptest.NewRequest(method, fmt.Sprintf("/matches/%d/team-reports%s", match.ID, path), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		res, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if res.StatusCode != status {
			t.Fatalf("%s %s: expected %d, got %d", method, path, status, res.StatusCode)
		}
		data := map[string]any{}
		if status == 200 {
			if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
				t.Fatal(err)
			}
		}
		return data
	}
	home := fmt.Sprintf("/%d", match.HomeClubID)
	away := fmt.Sprintf("/%d", match.AwayClubID)
	request("PUT", home, fmt.Sprintf(`{"club_id":%d,"summary":"Wrong recipient"}`, match.AwayClubID), 400)
	request("PUT", home, `{"summary":"Home assessment","strengths":"Controlled midfield"}`, 200)
	request("PUT", away, `{"summary":"Away confidential assessment"}`, 200)
	request("POST", home+"/status", `{"action":"submit"}`, 200)
	request("PUT", home, `{"summary":"Cannot overwrite submitted"}`, 409)
	user.Role = models.ClubManager
	user.ClubID = &match.HomeClubID
	performance := request("GET", "/performance", "", 200)
	visibleTeams := performance["data"].(map[string]any)["teams"].([]any)
	if len(visibleTeams) != 1 || visibleTeams[0].(map[string]any)["club_id"] != float64(match.HomeClubID) {
		t.Fatalf("opponent performance leaked: %+v", visibleTeams)
	}

	data := request("GET", "", "", 200)
	if reports := data["data"].(map[string]any)["reports"].([]any); len(reports) != 0 {
		t.Fatal("draft report exposed to club")
	}
	request("POST", away+"/status", `{"action":"approve"}`, 403)
	request("POST", home+"/status", `{"action":"approve"}`, 403)
	user.Role = models.AfrigoalsAdmin
	request("POST", home+"/status", `{"action":"distribute"}`, 409)
	request("POST", home+"/status", `{"action":"request_changes"}`, 400)
	request("POST", home+"/status", `{"action":"request_changes","notes":"Explain defensive transitions"}`, 200)
	user.Role = models.DataAnalyst
	request("PUT", home, `{"summary":"Revised home assessment"}`, 200)
	request("POST", home+"/status", `{"action":"submit"}`, 200)
	user.Role = models.AfrigoalsAdmin
	request("POST", home+"/status", `{"action":"approve"}`, 200)
	request("POST", home+"/status", `{"action":"distribute"}`, 200)
	user.Role = models.ClubManager
	data = request("GET", "", "", 200)
	reports := data["data"].(map[string]any)["reports"].([]any)
	if len(reports) != 1 || reports[0].(map[string]any)["club_id"] != float64(match.HomeClubID) {
		t.Fatalf("wrong recipient report visible: %+v", reports)
	}
}
