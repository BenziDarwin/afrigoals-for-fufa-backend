package services

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"testing"

	"afrigoals.com/database"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

// Opt-in local integration check. All rows, reviews and audit revisions are
// created inside an outer transaction and rolled back, including failed saves.
func TestReviewWorkflowIntegration(t *testing.T) {
	if os.Getenv("REVIEW_INTEGRATION") != "1" {
		t.Skip("set REVIEW_INTEGRATION=1 to test the configured PostgreSQL database")
	}
	if err := godotenv.Load("../.env"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AUTO_MIGRATE", "false")
	prior := database.DB
	database.ConnectDB()
	connection := database.DB
	tx := connection.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	database.DB = tx
	defer func() { tx.Rollback(); database.DB = prior }()
	var match models.Match
	if err := tx.First(&match, 2).Error; err != nil {
		t.Fatal("match 2 fixture required", err)
	}
	var user models.User
	if err := tx.First(&user).Error; err != nil {
		t.Fatal(err)
	}
	var player models.Player
	if err := tx.Where("club_id IN ?", []uint{match.HomeClubID, match.AwayClubID}).First(&player).Error; err != nil {
		t.Fatal(err)
	}
	job := models.VideoAnalysisJob{JobID: uuid.NewString(), MatchID: match.ID, Status: models.JobCompleted, ResultJSON: `{"stats":{"role_of_track":{"1":"player","2":"player","3":"referee"},"player_stats":{}},"events":[{"type":"pass","t":1,"from_tid":1},{"type":"shot","t":2},{"type":"tackle","t":4},{"type":"goal","t":6}]}`}
	if err := tx.Create(&job).Error; err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("user", user); return c.Next() })
	app.Get("/:match_id/:job_id", GetVideoAnalysisReview)
	app.Put("/:match_id/:job_id", SaveVideoAnalysisReview)
	app.Get("/:match_id/:job_id/export", ExportVideoAnalysisReview)
	path := "/2/" + job.JobID
	request := func(method, url string, body any) (int, map[string]any) {
		t.Helper()
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(method, url, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		res, err := app.Test(req, 30000)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		var data map[string]any
		json.Unmarshal(b, &data)
		return res.StatusCode, data
	}
	if status, _ := request("GET", path, nil); status != 200 {
		t.Fatal("get review", status)
	}
	doc := emptyTrackingReview()
	doc.Lineup = []uint{player.ID}
	doc.Tracks = map[string]TrackCorrection{"1": {CanonicalID: "2"}, "2": {CanonicalID: "2", PlayerID: &player.ID}}
	doc.Events = map[string]EventCorrection{"0": {Status: "confirmed"}, "1": {Status: "rejected"}, "2": {Status: "confirmed"}}
	doc.ExcludedSegments = []ExcludedVideoSegment{{Start: 3.5, End: 4.5, Reason: "replay"}}
	payload := fiber.Map{"version": 0, "document": doc}
	if status, data := request("PUT", path, payload); status != 200 {
		t.Fatal("save review", status, data)
	}
	if status, _ := request("PUT", path, payload); status != 409 {
		t.Fatal("stale update must conflict", status)
	}
	status, data := request("GET", path+"/export", nil)
	if status != 200 {
		t.Fatal("export", status, data)
	}
	examples, ok := data["examples"].([]any)
	if !ok || len(examples) != 2 {
		t.Fatal("unreviewed or excluded example leaked into export", data)
	}
	positive := examples[0].(map[string]any)
	negative := examples[1].(map[string]any)
	if positive["label"] != float64(1) || positive["from_tid"] != "2" || positive["from_player_id"] != float64(player.ID) || negative["label"] != float64(0) {
		t.Fatal("incorrect reviewed labels", examples)
	}
	var revisions int64
	tx.Model(&models.VideoAnalysisReviewRevision{}).Where("job_id = ?", job.JobID).Count(&revisions)
	if revisions != 1 {
		t.Fatal("conflict wrote an audit revision", revisions)
	}
}
