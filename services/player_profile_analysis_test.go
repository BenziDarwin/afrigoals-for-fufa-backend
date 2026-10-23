package services

import (
	"math"
	"net/http/httptest"
	"testing"
	"time"

	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func profileTestEvent(id, match, player uint, kind string) models.AnalysisEvent {
	return models.AnalysisEvent{ID: id, MatchID: match, PlayerID: &player, Type: kind, TimestampSeconds: float64(id), UpdatedAt: time.Unix(int64(id), 0)}
}

func TestPlayerProfileStableIdentityClipsAndReportRevisions(t *testing.T) {
	player := models.Player{Model: gorm.Model{ID: 7}, Name: "Same Name", Position: "Midfielder"}
	match := models.Match{Model: gorm.Model{ID: 1}, Date: time.Now()}
	event := profileTestEvent(10, 1, 7, "goal")
	raw := "https://media.example/goal.mp4"
	event.ClipURL = &raw
	other := profileTestEvent(11, 1, 8, "goal")
	other.PlayerName = &player.Name
	secondary := profileTestEvent(12, 1, 8, "goal")
	secondary.SecondaryPlayerID = &player.ID
	secondary.ClipURL = &raw
	hidden := profileTestEvent(13, 2, 7, "goal")
	hidden.ClipURL = &raw
	eventID := uint(10)
	clip := models.Clip{ID: 1, MatchID: 1, EventID: &eventID, Status: "completed", ClipURL: &raw}
	reports := []models.PlayerAnalysisReport{
		{ID: 1, MatchID: 1, PlayerID: 7, ReportText: "Old", UpdatedAt: time.Unix(1, 0)},
		{ID: 2, MatchID: 1, PlayerID: 7, ReportText: "Current", UpdatedAt: time.Unix(2, 0)},
		{ID: 3, MatchID: 2, PlayerID: 7, ReportText: "Hidden", UpdatedAt: time.Unix(3, 0)},
	}
	result := buildPlayerProfileAnalysis(player, []models.Match{match}, []models.AnalysisEvent{event, event, other, secondary, hidden}, reports, []models.Clip{clip, clip})
	if result.EventCount != 1 || result.EventTypes["goal"] != 1 {
		t.Fatal("other players, secondary actions or duplicate events inflated totals", result.EventTypes)
	}
	if len(result.Reports) != 1 || result.Reports[0].ReportText != "Current" {
		t.Fatal("report revisions or hidden reports leaked", result.Reports)
	}
	if len(result.Clips) != 2 || result.Clips[1].Participation != "secondary" {
		t.Fatal("clips were duplicated or secondary evidence disappeared", result.Clips)
	}
	if result.MatchCount != 1 {
		t.Fatal("incorrect match count")
	}
	// Reassignment/deletion is reflected on the next read with no stored aggregate.
	empty := buildPlayerProfileAnalysis(player, nil, []models.AnalysisEvent{other}, nil, nil)
	if empty.EventCount != 0 || len(empty.Clips) != 0 || empty.Score.Value != nil {
		t.Fatal("stale evidence survived")
	}
}

func TestPlayerProfileFailedUnlinkedAndUnsafeClipsAreExcluded(t *testing.T) {
	player := models.Player{Model: gorm.Model{ID: 7}}
	event := profileTestEvent(1, 1, 7, "shot")
	eventID := event.ID
	unsafe := "javascript:alert(1)"
	event.ClipURL = &unsafe
	valid := "https://media.example/clip.mp4"
	result := buildPlayerProfileAnalysis(player, []models.Match{{Model: gorm.Model{ID: 1}}}, []models.AnalysisEvent{event}, nil, []models.Clip{
		{MatchID: 1, EventID: &eventID, Status: "failed", ClipURL: &valid},
		{MatchID: 2, EventID: &eventID, Status: "completed", ClipURL: &valid},
		{MatchID: 1, Status: "completed", ClipURL: &valid},
	})
	if len(result.Clips) != 0 {
		t.Fatal("unsafe, failed or unlinked clip exposed")
	}
	for _, raw := range []string{"javascript:bad", "//untrusted.example/x", "/uploads/clips/old.mp4", "https://", "data:video/mp4;base64,abc"} {
		if safeProfileClip(raw) {
			t.Fatal("unsafe URL", raw)
		}
	}
	if !safeProfileClip(valid) {
		t.Fatal("valid clip rejected")
	}
}

func TestPlayerProfileScoreUsesOutcomesPositionAndSampleCoverage(t *testing.T) {
	events := []models.AnalysisEvent{}
	success := "successful"
	for i := uint(1); i <= 30; i++ {
		kind := "pass_completed"
		if i%2 == 0 {
			kind = "tackle"
		}
		event := profileTestEvent(i, 1+i%3, 7, kind)
		event.Outcome = &success
		events = append(events, event)
	}
	defender := profilePerformance("Defender", events, 3)
	if defender.Status != "available" || defender.Value == nil || defender.Coverage != .55 || defender.OutcomeSamples != 30 {
		t.Fatal("unexpected supported score", defender)
	}
	if profilePerformance("Striker", events, 3).Status != "provisional" {
		t.Fatal("cross-position coverage treated as complete")
	}
	if profilePerformance("unknown", events, 3).Value != nil {
		t.Fatal("unknown position scored")
	}
	if profilePerformance("Defender", events, 1).Status != "provisional" {
		t.Fatal("one match treated as established")
	}
	// Twenty unrelated tags cannot turn two classified attempts into enough evidence.
	sparse := []models.AnalysisEvent{events[0], events[1]}
	for i := uint(40); i < 65; i++ {
		sparse = append(sparse, profileTestEvent(i, 1, 7, "corner_kick"))
	}
	if profilePerformance("Defender", sparse, 3).Status != "provisional" {
		t.Fatal("unrelated event volume inflated confidence")
	}
}

func TestPlayerProfileMonetaryEstimateRequiresCalibrationAndEvidence(t *testing.T) {
	scoreValue := 75.0
	score := profileScore{Value: &scoreValue, Status: "available", Matches: 10, Coverage: 1}
	for _, baseline := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if profileMarketValue(score, baseline).Estimate != nil {
			t.Fatal("invalid baseline yielded money")
		}
	}
	value := profileMarketValue(score, 10000)
	if value.Currency != "USD" || value.Estimate == nil || *value.Estimate != 12500 || *value.Low >= *value.Estimate || *value.High <= *value.Estimate {
		t.Fatal("wrong scenario estimate", value)
	}
	score.Status = "provisional"
	if profileMarketValue(score, 10000).Estimate != nil {
		t.Fatal("sparse evidence yielded money")
	}
	t.Setenv("PLAYER_VALUE_BASELINE_USD", "10000")
	t.Setenv("PLAYER_VALUE_BASELINE_USD_DEF", "20000")
	if playerValueBaseline(PositionDEF) != 20000 || playerValueBaseline(PositionMID) != 10000 {
		t.Fatal("position baseline precedence failed")
	}
}

func TestPlayerProfileRespectsReportDistributionAndClubScope(t *testing.T) {
	club, other := uint(1), uint(3)
	match := models.Match{HomeClubID: club, AwayClubID: 2}
	manager := models.User{Role: models.ClubManager, ClubID: &club}
	if !profileMatchVisible(&manager, &match, models.ReportReviewDraft) {
		t.Fatal("own-club working evidence hidden from team CV")
	}
	if !profileMatchVisible(&manager, &match, models.ReportReviewDistributed) {
		t.Fatal("own distributed evidence hidden")
	}
	manager.ClubID = &other
	if profileMatchVisible(&manager, &match, models.ReportReviewDistributed) {
		t.Fatal("other club evidence exposed")
	}
	if !profileMatchVisible(&models.User{Role: models.DataAnalyst}, &match, models.ReportReviewDraft) {
		t.Fatal("analyst preview hidden")
	}
	if profileMatchVisible(nil, &match, models.ReportReviewDistributed) {
		t.Fatal("unauthenticated access")
	}
}

func TestPlayerProfileEndpointRequiresUserAndValidID(t *testing.T) {
	app := fiber.New()
	app.Get("/:id/profile-analysis", GetPlayerProfileAnalysis)
	response, err := app.Test(httptest.NewRequest("GET", "/7/profile-analysis", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 401 {
		t.Fatal(response.StatusCode)
	}
	authorized := fiber.New()
	authorized.Use(func(c *fiber.Ctx) error { c.Locals("user", models.User{Role: models.DataAnalyst}); return c.Next() })
	authorized.Get("/:id/profile-analysis", GetPlayerProfileAnalysis)
	response, err = authorized.Test(httptest.NewRequest("GET", "/invalid/profile-analysis", nil))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 400 {
		t.Fatal(response.StatusCode)
	}
}
