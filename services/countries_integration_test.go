package services

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"afrigoals.com/database"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Opt-in tests run inside one rolled-back transaction, including migration 20
// when testing a database at version 19. Fixtures never persist.
func TestCountryDirectoryAndOverseasPlayerLifecycle(t *testing.T) {
	dsn := os.Getenv("AFRIGOALS_COUNTRY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("AFRIGOALS_COUNTRY_TEST_DATABASE_URL is required")
	}
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("Could not connect to the opt-in test database")
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
	original := database.DB
	database.DB = tx
	defer func() { database.DB = original }()
	if err := tx.Exec("SET LOCAL lock_timeout='10s'; SET LOCAL statement_timeout='60s'").Error; err != nil {
		t.Fatal(err)
	}
	if !tx.Migrator().HasTable(&models.Country{}) {
		migration, err := os.ReadFile("../migrations/000020_countries_and_player_tracking.up.sql")
		if err != nil {
			t.Fatal(err)
		}
		if err := tx.Exec(string(migration)).Error; err != nil {
			t.Fatal(err)
		}
	}
	national := models.Club{}
	if err := tx.Where("team_type='national' AND country_code='UG'").Order("id").First(&national).Error; err != nil {
		national = models.Club{Name: "Country test national " + uuid.NewString(), TeamType: "national", Country: "Uganda"}
		if err := tx.Create(&national).Error; err != nil {
			t.Fatal(err)
		}
	}
	domestic := models.Club{Name: "Country test club " + uuid.NewString(), TeamType: "club", Country: "Uganda"}
	if err := tx.Create(&domestic).Error; err != nil {
		t.Fatal(err)
	}
	name := "Country test player " + uuid.NewString()
	player := models.Player{Name: name, ClubID: domestic.ID, JerseyNumber: 17, Nationality: "Ugandan", Position: "Midfielder", DateOfBirth: time.Date(2000, 1, 2, 0, 0, 0, 0, time.UTC)}
	if err := tx.Omit("Club", "NationalTeam", "Stats").Create(&player).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.First(&player, player.ID).Error; err != nil {
		t.Fatal(err)
	}
	if player.CountryCode == nil || *player.CountryCode != "UG" || player.NationalTeamID == nil || *player.NationalTeamID != national.ID || player.NationalJerseyNumber != nil {
		t.Fatalf("Player was not automatically listed with an unassigned national shirt: %+v", player)
	}
	originalUUID := player.UUID
	actor := models.User{Role: models.AfrigoalsAdmin}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("user", actor); return c.Next() })
	app.Get("/countries", ListCountries)
	app.Get("/players", ListPlayers)
	app.Get("/club/:club_id/players", GetClubPlayers)
	app.Post("/transfer", TransferPlayer)
	response, err := app.Test(httptest.NewRequest("GET", "/countries", nil), 20000)
	if err != nil {
		t.Fatal(err)
	}
	var directory struct {
		Countries []models.Country `json:"countries"`
	}
	if err := json.NewDecoder(response.Body).Decode(&directory); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 || len(directory.Countries) != 249 {
		t.Fatalf("Invalid country directory: %d entries", len(directory.Countries))
	}
	destination := "Country test overseas " + uuid.NewString()
	body, _ := json.Marshal(map[string]interface{}{"player_id": player.ID, "from_club_id": domestic.ID, "new_club_name": destination, "new_club_country": "Germany", "jersey_number": 0})
	request := httptest.NewRequest("POST", "/transfer", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	response, err = app.Test(request, 20000)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 {
		t.Fatalf("Transfer failed with HTTP %d", response.StatusCode)
	}
	response.Body.Close()
	if err := tx.Preload("Club").First(&player, player.ID).Error; err != nil {
		t.Fatal(err)
	}
	if player.UUID != originalUUID || !player.Club.IsExternal || player.NationalTeamID == nil || *player.NationalTeamID != national.ID {
		t.Fatal("Transfer lost the player's identity, external club flag or national listing")
	}
	var count int64
	if err := tx.Model(&models.PlayerClubHistory{}).Where("player_id=? AND from_club_id=? AND to_club_id=?", player.ID, domestic.ID, player.ClubID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("Transfer history was not recorded")
	}
	for _, check := range []struct {
		role     models.UserRole
		clubID   uint
		expected int
	}{
		{models.AfrigoalsAdmin, 0, 1}, {models.ClubManager, domestic.ID, 0}, {models.ClubManager, national.ID, 1},
	} {
		actor = models.User{Role: check.role, ClubID: &check.clubID}
		request = httptest.NewRequest("GET", fmt.Sprintf("/players?name=%s", strings.ReplaceAll(name, " ", "%20")), nil)
		response, err = app.Test(request, 20000)
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			Players []json.RawMessage `json:"players"`
		}
		if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 200 || len(result.Players) != check.expected {
			t.Fatalf("Incorrect external-player visibility for %s: HTTP %d, %d players", check.role, response.StatusCode, len(result.Players))
		}
	}
	actor = models.User{Role: models.ClubManager, ClubID: &national.ID}
	response, err = app.Test(httptest.NewRequest("GET", fmt.Sprintf("/club/%d/players", national.ID), nil), 20000)
	if err != nil {
		t.Fatal(err)
	}
	var squad struct {
		Players []models.Player `json:"players"`
	}
	if err := json.NewDecoder(response.Body).Decode(&squad); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	found := false
	for _, member := range squad.Players {
		if member.ID == player.ID {
			found = true
		}
	}
	if response.StatusCode != 200 || !found {
		t.Fatal("Overseas player disappeared from the national manager's roster")
	}
}
