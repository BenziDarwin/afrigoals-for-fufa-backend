package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func exercisePlayerPortfolio(t *testing.T, tx *gorm.DB, app *fiber.App, actor *models.User, national models.Club, player models.Player, domestic models.Club) {
	t.Helper()
	if err := tx.Model(&player).Update("nationality", "Ugandan").Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.First(&player, player.ID).Error; err != nil {
		t.Fatal(err)
	}
	var auditor models.User
	if err := tx.Select("id").First(&auditor).Error; err != nil {
		t.Fatal(err)
	}
	*actor = models.User{ID: auditor.ID, Role: models.ClubManager, ClubID: &national.ID}
	app.Get("/players/:id/portfolio", GetPlayerPortfolio)
	app.Put("/players/:id", UpdatePlayer)
	app.Post("/players/:id/career", SavePlayerCareerEntry)
	app.Put("/players/:id/career/:entry_id", SavePlayerCareerEntry)
	app.Delete("/players/:id/career/:entry_id", DeletePlayerCareerEntry)
	app.Post("/players/:id/highlights", SavePlayerHighlight)
	app.Put("/players/:id/highlights/:highlight_id", SavePlayerHighlight)
	app.Post("/players/:id/highlights/upload", InitPlayerHighlightUpload)
	app.Post("/players/:id/highlights/:highlight_id/complete", CompletePlayerHighlightUpload)
	send := func(method, path string, body interface{}, want int) []byte {
		t.Helper()
		data, _ := json.Marshal(body)
		request := httptest.NewRequest(method, path, bytes.NewReader(data))
		request.Header.Set("Content-Type", "application/json")
		response, err := app.Test(request, 30000)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		result, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != want {
			t.Fatalf("%s %s returned %d, expected %d: %s", method, path, response.StatusCode, want, result)
		}
		return result
	}
	readID := func(data []byte) uint {
		t.Helper()
		var result struct {
			ID uint `json:"id"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatal(err)
		}
		if result.ID == 0 {
			t.Fatal("Response has no record ID")
		}
		return result.ID
	}
	base := fmt.Sprintf("/players/%d", player.ID)
	originalClub, originalUUID := player.ClubID, player.UUID
	send("PUT", base, map[string]interface{}{"name": "Updated overseas player " + uuid.NewString(), "photo_url": "https://example.com/player.jpg"}, 200)
	send("PUT", base, map[string]interface{}{"name": ""}, 400)
	payload := map[string]interface{}{"club_name": "Previous club " + uuid.NewString(), "club_country": "France", "season": "2023/24", "competition": "League outside Afrigoals", "start_date": "2023-07-01", "end_date": "2024-06-30", "appearances": 20, "goals": 0, "assists": nil, "source_url": "https://example.com/player-history"}
	entryID := readID(send("POST", base+"/career", payload, 200))
	var entry models.PlayerCareerEntry
	if err := tx.Preload("Club").First(&entry, entryID).Error; err != nil {
		t.Fatal(err)
	}
	if entry.Goals == nil || *entry.Goals != 0 || entry.Assists != nil || !entry.Club.IsExternal || entry.CreatedBy != auditor.ID {
		t.Fatal("Career record lost zero/unknown statistics, club link or author")
	}
	payload["goals"] = 8
	send("PUT", fmt.Sprintf("%s/career/%d", base, entryID), payload, 200)
	// The same named external club is reused on edits.
	if err := tx.First(&entry, entryID).Error; err != nil {
		t.Fatal(err)
	}
	if entry.Goals == nil || *entry.Goals != 8 {
		t.Fatal("Career edit did not persist")
	}
	if err := tx.First(&player, player.ID).Error; err != nil {
		t.Fatal(err)
	}
	if player.ClubID != originalClub || player.UUID != originalUUID || player.NationalTeamID == nil || *player.NationalTeamID != national.ID {
		t.Fatal("Adding past clubs changed the current registration or player identity")
	}
	send("POST", base+"/highlights", map[string]interface{}{"title": "Invalid link", "video_url": "javascript:alert(1)"}, 400)
	linkID := readID(send("POST", base+"/highlights", map[string]interface{}{"title": "Previous-club highlights", "video_url": "https://youtu.be/example", "career_entry_id": entryID}, 200))
	send("PUT", fmt.Sprintf("%s/highlights/%d", base, linkID), map[string]interface{}{"title": "Edited major highlight", "video_url": "https://example.com/highlight.mp4", "career_entry_id": entryID}, 200)
	var clip models.PlayerHighlight
	if err := tx.First(&clip, linkID).Error; err != nil {
		t.Fatal(err)
	}
	if clip.ClubID == nil || *clip.ClubID != entry.ClubID || clip.Title != "Edited major highlight" {
		t.Fatal("Highlight edit lost the linked club")
	}
	actor.ClubID = &domestic.ID
	send("POST", base+"/career", payload, 403)
	send("PUT", base, map[string]interface{}{"name": "Unauthorized change"}, 403)
	actor.ClubID = &national.ID

	var objects sync.Map
	type object struct {
		data        []byte
		contentType string
	}
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			body, _ := io.ReadAll(r.Body)
			objects.Store(r.URL.Path, object{data: body, contentType: r.Header.Get("Content-Type")})
			w.WriteHeader(200)
			return
		}
		value, ok := objects.Load(r.URL.Path)
		if !ok {
			w.WriteHeader(404)
			return
		}
		stored := value.(object)
		w.Header().Set("Content-Length", strconv.Itoa(len(stored.data)))
		w.Header().Set("Content-Type", stored.contentType)
		w.WriteHeader(200)
	}))
	defer storage.Close()
	for key, value := range map[string]string{"R2_ENDPOINT": storage.URL, "R2_BUCKET": "portfolio-test", "R2_ACCESS_KEY_ID": "test-access", "R2_SECRET_ACCESS_KEY": "test-secret", "R2_PUBLIC_BASE_URL": "https://media.example.test"} {
		t.Setenv(key, value)
	}
	video := []byte("test-video-data")
	uploadBody := map[string]interface{}{"title": "Uploaded major clip", "filename": "highlight.mp4", "content_type": "video/mp4", "size_bytes": len(video), "career_entry_id": entryID}
	uploadData := send("POST", base+"/highlights/upload", uploadBody, 201)
	var upload struct {
		ID  uint   `json:"id"`
		URL string `json:"upload_url"`
	}
	if err := json.Unmarshal(uploadData, &upload); err != nil {
		t.Fatal(err)
	}
	uploadBody["upload_id"] = upload.ID
	if id := readID(send("POST", base+"/highlights/upload", uploadBody, 201)); id != upload.ID {
		t.Fatal("Upload retry created a duplicate record")
	}
	complete := fmt.Sprintf("%s/highlights/%d/complete", base, upload.ID)
	send("POST", complete, nil, 409)
	put := func(body []byte) {
		request, _ := http.NewRequest(http.MethodPut, upload.URL, bytes.NewReader(body))
		request.Header.Set("Content-Type", "video/mp4")
		response, err := storage.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 200 {
			t.Fatal("Test video upload failed")
		}
	}
	put([]byte("partial"))
	send("POST", complete, nil, 400)
	put(video)
	send("POST", complete, nil, 200)
	send("POST", complete, nil, 200)
	var result struct {
		CanEdit        bool                       `json:"can_edit"`
		CanEditProfile bool                       `json:"can_edit_profile"`
		Highlights     []models.PlayerHighlight   `json:"highlights"`
		Entries        []models.PlayerCareerEntry `json:"career_entries"`
	}
	if err := json.Unmarshal(send("GET", base+"/portfolio", nil, 200), &result); err != nil {
		t.Fatal(err)
	}
	if !result.CanEdit || !result.CanEditProfile || len(result.Highlights) != 2 || len(result.Entries) != 1 {
		t.Fatal("Completed portfolio is missing records or overseas-manager editing access")
	}
	send("DELETE", fmt.Sprintf("%s/career/%d", base, entryID), nil, 200)
	if err := tx.First(&clip, linkID).Error; err != nil {
		t.Fatal(err)
	}
	if clip.CareerEntryID != nil || clip.ClubID == nil || *clip.ClubID != entry.ClubID {
		t.Fatal("Removing a career record lost its highlights or club attribution")
	}
}
