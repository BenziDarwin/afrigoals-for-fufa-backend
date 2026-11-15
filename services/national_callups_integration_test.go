package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func exerciseNationalCallups(t *testing.T, tx *gorm.DB, app *fiber.App, actor *models.User, national models.Club, player models.Player, domestic models.Club) {
	t.Helper()
	var auditor models.User
	if err := tx.Select("id").First(&auditor).Error; err != nil {
		t.Fatal(err)
	}
	*actor = models.User{ID: auditor.ID, Role: models.ClubManager, ClubID: &national.ID}
	app.Put("/callup/:club_id/:player_id", SetNationalCallup)
	app.Post("/formations", CreateFormation)
	send := func(method, path string, body interface{}, want int) []byte {
		t.Helper()
		payload, _ := json.Marshal(body)
		request := httptest.NewRequest(method, path, strings.NewReader(string(payload)))
		request.Header.Set("Content-Type", "application/json")
		response, err := app.Test(request, 20000)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		data, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != want {
			t.Fatalf("%s %s returned %d, expected %d: %s", method, path, response.StatusCode, want, data)
		}
		return data
	}
	away := models.Club{Name: "Call-up test away " + uuid.NewString(), TeamType: "national", Country: "Kenya"}
	if err := tx.Create(&away).Error; err != nil {
		t.Fatal(err)
	}
	match := models.Match{HomeClubID: national.ID, AwayClubID: away.ID, Date: time.Now().Add(24 * time.Hour)}
	if err := tx.Omit("HomeClub", "AwayClub", "League", "Formations", "Events", "Analysts").Create(&match).Error; err != nil {
		t.Fatal(err)
	}
	selection := map[string]interface{}{"match_id": match.ID, "club_id": national.ID, "formation": "4-4-2", "lineup": []map[string]interface{}{{"player_id": player.ID, "name": player.Name, "position": player.Position, "jersey_number": 17, "x": 0.5, "y": 0.5}}}
	send("POST", "/formations", selection, 400)
	var count int64
	if err := matchRosterPlayers(tx.Model(&models.Player{}), match).Where("players.id=?", player.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("Eligibility alone added a player to an international match roster")
	}
	path := fmt.Sprintf("/callup/%d/%d", national.ID, player.ID)
	send("PUT", path, map[string]bool{"called_up": true}, 200)
	send("PUT", path, map[string]bool{"called_up": true}, 200)
	if err := tx.Model(&models.NationalCallup{}).Where("team_id=? AND player_id=? AND withdrawn_at IS NULL", national.ID, player.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("Repeated calls created duplicate active call-ups")
	}
	if err := matchRosterPlayers(tx.Model(&models.Player{}), match).Where("players.id=?", player.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("Called-up player is missing from the match roster")
	}
	roster := []models.Player{player}
	if err := attachNationalCallups(tx, national.ID, roster); err != nil {
		t.Fatal(err)
	}
	if !roster[0].CalledUp || roster[0].CalledUpAt == nil {
		t.Fatal("The player card has no call-up status")
	}
	send("POST", "/formations", selection, 201)
	var shirt int
	if err := tx.Raw("SELECT n FROM generate_series(1,99) n WHERE NOT EXISTS (SELECT 1 FROM players WHERE national_team_id=? AND national_jersey_number=n AND deleted_at IS NULL) LIMIT 1", national.ID).Scan(&shirt).Error; err != nil {
		t.Fatal(err)
	}
	if shirt == 0 {
		t.Fatal("No national shirt number is available for the test fixture")
	}
	if err := tx.Model(&player).Updates(map[string]interface{}{"jersey_number": 17, "national_jersey_number": shirt}).Error; err != nil {
		t.Fatal(err)
	}
	send("PUT", path, map[string]bool{"called_up": false}, 200)
	send("PUT", path, map[string]bool{"called_up": false}, 200)
	if err := tx.First(&player, player.ID).Error; err != nil {
		t.Fatal(err)
	}
	if player.NationalTeamID == nil || *player.NationalTeamID != national.ID || player.NationalJerseyNumber != nil || player.JerseyNumber != 17 {
		t.Fatal("Withdrawing a call-up changed eligibility or the club shirt")
	}
	var history models.NationalCallup
	if err := tx.Where("team_id=? AND player_id=?", national.ID, player.ID).First(&history).Error; err != nil {
		t.Fatal(err)
	}
	if history.WithdrawnAt == nil || history.JerseyNumber == nil || *history.JerseyNumber != shirt {
		t.Fatal("Withdrawal lost the call-up history or its national shirt")
	}
	if err := matchRosterPlayers(tx.Model(&models.Player{}), match).Where("players.id=?", player.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("Withdrawing a player removed their saved match lineup")
	}
	nextMatch := models.Match{HomeClubID: national.ID, AwayClubID: away.ID, Date: time.Now().Add(48 * time.Hour)}
	if err := tx.Omit("HomeClub", "AwayClub", "League", "Formations", "Events", "Analysts").Create(&nextMatch).Error; err != nil {
		t.Fatal(err)
	}
	if err := matchRosterPlayers(tx.Model(&models.Player{}), nextMatch).Where("players.id=?", player.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("A withdrawn player remains selectable for a new match")
	}
	actor.ClubID = &domestic.ID
	send("PUT", path, map[string]bool{"called_up": true}, 403)
	actor.ClubID = &national.ID
	send("PUT", path, map[string]bool{"called_up": true}, 200)
	if err := tx.Model(&player).Update("nationality", "Kenyan").Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Model(&models.NationalCallup{}).Where("team_id=? AND player_id=? AND withdrawn_at IS NULL", national.ID, player.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("A nationality change retained an ineligible active call-up")
	}
	send("PUT", path, map[string]bool{"called_up": true}, 400)
}
