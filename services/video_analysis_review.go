package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"afrigoals.com/database"
	"afrigoals.com/middleware"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TrackCorrection struct {
	CanonicalID string `json:"canonical_id"`
	PlayerID    *uint  `json:"player_id,omitempty"`
	Role        string `json:"role,omitempty"`
}
type EventCorrection struct {
	Status string   `json:"status"`
	Type   *string  `json:"type,omitempty"`
	Time   *float64 `json:"t,omitempty"`
}
type ExcludedVideoSegment struct {
	Start  float64 `json:"start"`
	End    float64 `json:"end"`
	Reason string  `json:"reason"`
}
type TrackingReviewDocument struct {
	Lineup           []uint                     `json:"lineup"`
	TeamClubs        map[string]uint            `json:"team_clubs"`
	Tracks           map[string]TrackCorrection `json:"tracks"`
	Events           map[string]EventCorrection `json:"events"`
	ExcludedSegments []ExcludedVideoSegment     `json:"excluded_segments"`
}

func emptyTrackingReview() TrackingReviewDocument {
	return TrackingReviewDocument{Lineup: []uint{}, TeamClubs: map[string]uint{}, Tracks: map[string]TrackCorrection{}, Events: map[string]EventCorrection{}, ExcludedSegments: []ExcludedVideoSegment{}}
}

func trackingSource(raw string) (map[string]any, map[string]string, []any, error) {
	var result map[string]any
	if err := json.Unmarshal([]byte(raw), &result); err != nil || result == nil {
		return nil, nil, nil, fmt.Errorf("analysis results are not available yet")
	}
	stats, _ := result["stats"].(map[string]any)
	roles := map[string]string{}
	if values, ok := stats["role_of_track"].(map[string]any); ok {
		for id, v := range values {
			if role, ok := v.(string); ok {
				roles[id] = role
			}
		}
	}
	if players, ok := stats["player_stats"].(map[string]any); ok {
		for id := range players {
			if roles[id] == "" {
				roles[id] = "player"
			}
		}
	}
	events, _ := result["events"].([]any)
	return result, roles, events, nil
}

func canonicalTrack(id string, tracks map[string]TrackCorrection) (string, error) {
	seen := map[string]bool{}
	for {
		if seen[id] {
			return "", fmt.Errorf("track merges contain a cycle")
		}
		seen[id] = true
		parent := tracks[id].CanonicalID
		if parent == "" || parent == id {
			return id, nil
		}
		id = parent
	}
}

func validateTrackingReview(doc *TrackingReviewDocument, roles map[string]string, eventCount int, roster map[uint]models.Player, home, away uint) error {
	if doc.Tracks == nil {
		doc.Tracks = map[string]TrackCorrection{}
	}
	if doc.Events == nil {
		doc.Events = map[string]EventCorrection{}
	}
	if doc.TeamClubs == nil {
		doc.TeamClubs = map[string]uint{}
	}
	lineup := map[uint]bool{}
	for _, id := range doc.Lineup {
		if _, ok := roster[id]; !ok {
			return fmt.Errorf("lineup player %d does not belong to either match club", id)
		}
		if lineup[id] {
			return fmt.Errorf("duplicate lineup player %d", id)
		}
		lineup[id] = true
	}
	for team, club := range doc.TeamClubs {
		if (team != "0" && team != "1") || (club != home && club != away) {
			return fmt.Errorf("invalid team-to-club assignment")
		}
	}
	if a, ok := doc.TeamClubs["0"]; ok && a == doc.TeamClubs["1"] {
		return fmt.Errorf("the two detected teams must map to different clubs")
	}
	assigned := map[string]uint{}
	for id, correction := range doc.Tracks {
		if _, ok := roles[id]; !ok {
			return fmt.Errorf("unknown track %s", id)
		}
		root, err := canonicalTrack(id, doc.Tracks)
		if err != nil {
			return err
		}
		if _, ok := roles[root]; !ok {
			return fmt.Errorf("unknown merge target %s", root)
		}
		role := correction.Role
		if role == "" {
			role = roles[id]
		}
		if role != "player" && role != "goalkeeper" && role != "referee" {
			return fmt.Errorf("invalid role for track %s", id)
		}
		rootRole := doc.Tracks[root].Role
		if rootRole == "" {
			rootRole = roles[root]
		}
		if role != rootRole {
			return fmt.Errorf("tracks %s and %s have different roles; correct their roles before merging", id, root)
		}
		if correction.PlayerID != nil {
			player, ok := roster[*correction.PlayerID]
			if !ok {
				return fmt.Errorf("assigned player is outside the match roster")
			}
			_ = player
			if !lineup[*correction.PlayerID] {
				return fmt.Errorf("add the assigned player to the match squad first")
			}
			if role == "referee" {
				return fmt.Errorf("referees cannot be assigned to a player")
			}
			if other, ok := assigned[root]; ok && other != *correction.PlayerID {
				return fmt.Errorf("merged tracks have conflicting player assignments")
			}
			assigned[root] = *correction.PlayerID
		}
		correction.CanonicalID = root
		doc.Tracks[id] = correction
	}
	for index, correction := range doc.Events {
		i, err := strconv.Atoi(index)
		if err != nil || i < 0 || i >= eventCount {
			return fmt.Errorf("unknown event %s", index)
		}
		if correction.Status != "confirmed" && correction.Status != "rejected" {
			return fmt.Errorf("event status must be confirmed or rejected")
		}
		if correction.Type != nil && (strings.TrimSpace(*correction.Type) == "" || len(*correction.Type) > 50) {
			return fmt.Errorf("invalid corrected event type")
		}
		if correction.Time != nil && (!finiteSeconds(*correction.Time) || *correction.Time < 0) {
			return fmt.Errorf("invalid corrected event timestamp")
		}
	}
	sort.Slice(doc.ExcludedSegments, func(i, j int) bool { return doc.ExcludedSegments[i].Start < doc.ExcludedSegments[j].Start })
	for i, s := range doc.ExcludedSegments {
		if !finiteSeconds(s.Start) || !finiteSeconds(s.End) || s.Start < 0 || s.End <= s.Start {
			return fmt.Errorf("invalid excluded segment")
		}
		if s.Reason != "closeup" && s.Reason != "replay" && s.Reason != "camera_cut" && s.Reason != "other" {
			return fmt.Errorf("invalid exclusion reason")
		}
		if i > 0 && s.Start < doc.ExcludedSegments[i-1].End {
			return fmt.Errorf("excluded segments must not overlap")
		}
	}
	return nil
}
func finiteSeconds(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func loadReviewJob(c *fiber.Ctx) (*models.VideoAnalysisJob, error) {
	matchID, err := parseMatchIDParam(c)
	if err != nil {
		return nil, fiber.NewError(400, err.Error())
	}
	var job models.VideoAnalysisJob
	err = database.DB.Where("match_id = ? AND job_id = ?", matchID, c.Params("job_id")).First(&job).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fiber.NewError(404, "Analysis job not found")
	}
	if err != nil {
		return nil, fiber.NewError(500, "Could not load analysis job")
	}
	return &job, nil
}
func reviewRoster(matchID uint) (models.Match, []models.Player, error) {
	var match models.Match
	if err := database.DB.Preload("Formations.Lineup").First(&match, matchID).Error; err != nil {
		return match, nil, err
	}
	var players []models.Player
	err := database.DB.Preload("Club").Where("club_id IN ?", []uint{match.HomeClubID, match.AwayClubID}).Order("club_id, jersey_number, name").Find(&players).Error
	return match, players, err
}

func GetVideoAnalysisReview(c *fiber.Ctx) error {
	job, err := loadReviewJob(c)
	if err != nil {
		return err
	}
	result, roles, events, err := trackingSource(job.ResultJSON)
	if err != nil {
		return fiber.NewError(409, err.Error())
	}
	match, players, err := reviewRoster(job.MatchID)
	if err != nil {
		return fiber.NewError(500, "Could not load match roster")
	}
	doc := emptyTrackingReview()
	var saved models.VideoAnalysisReview
	err = database.DB.Where("job_id = ?", job.JobID).First(&saved).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fiber.NewError(500, "Could not load identity review")
	}
	if saved.Version > 0 {
		if err := json.Unmarshal([]byte(saved.Document), &doc); err != nil {
			return fiber.NewError(500, "Invalid saved review")
		}
	} else {
		selected := map[uint]bool{}
		available := map[uint]bool{}
		for _, player := range players {
			available[player.ID] = true
		}
		for _, formation := range match.Formations {
			for _, p := range formation.Lineup {
				if available[p.PlayerID] && !selected[p.PlayerID] {
					doc.Lineup = append(doc.Lineup, p.PlayerID)
					selected[p.PlayerID] = true
				}
			}
		}
	}
	roster := []fiber.Map{}
	for _, p := range players {
		roster = append(roster, fiber.Map{"id": p.ID, "name": p.Name, "club_id": p.ClubID, "club_name": p.Club.Name, "jersey_number": p.JerseyNumber, "position": p.Position})
	}
	return c.JSON(fiber.Map{"version": saved.Version, "document": doc, "updated_at": saved.UpdatedAt, "roster": roster, "roles": roles, "events": events, "stats": result["stats"], "tracking_trace_available": result["tracking_trace"] != nil && result["tracking_trace"] != "", "clubs": fiber.Map{"home": match.HomeClubID, "away": match.AwayClubID}})
}

func SaveVideoAnalysisReview(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return fiber.ErrUnauthorized
	}
	job, err := loadReviewJob(c)
	if err != nil {
		return err
	}
	_, roles, events, err := trackingSource(job.ResultJSON)
	if err != nil {
		return fiber.NewError(409, err.Error())
	}
	if len(c.Body()) > 1024*1024 {
		return fiber.NewError(413, "Review is too large")
	}
	var request struct {
		Version  uint                   `json:"version"`
		Document TrackingReviewDocument `json:"document"`
	}
	if err := c.BodyParser(&request); err != nil {
		return fiber.NewError(400, "Invalid review")
	}
	match, players, err := reviewRoster(job.MatchID)
	if err != nil {
		return fiber.NewError(500, "Could not load match roster")
	}
	roster := map[uint]models.Player{}
	for _, p := range players {
		roster[p.ID] = p
	}
	if err := validateTrackingReview(&request.Document, roles, len(events), roster, match.HomeClubID, match.AwayClubID); err != nil {
		return fiber.NewError(400, err.Error())
	}
	raw, _ := json.Marshal(request.Document)
	var saved models.VideoAnalysisReview
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		// Lock the parent job even on the first save, when no review row exists.
		var locked models.VideoAnalysisJob
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("job_id = ? AND match_id = ?", job.JobID, job.MatchID).First(&locked).Error; err != nil {
			return err
		}
		err := tx.Where("job_id = ?", job.JobID).First(&saved).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if saved.Version != request.Version {
			return fiber.NewError(409, "This review changed in another session. Reload before saving.")
		}
		saved = models.VideoAnalysisReview{JobID: job.JobID, MatchID: job.MatchID, Version: saved.Version + 1, Document: string(raw), UpdatedBy: user.ID, UpdatedAt: time.Now().UTC()}
		if err := tx.Save(&saved).Error; err != nil {
			return err
		}
		return tx.Create(&models.VideoAnalysisReviewRevision{JobID: saved.JobID, Version: saved.Version, Document: saved.Document, UpdatedBy: user.ID, CreatedAt: saved.UpdatedAt}).Error
	})
	if err != nil {
		var conflict *fiber.Error
		if errors.As(err, &conflict) {
			return conflict
		}
		return fiber.NewError(500, "Could not save identity review")
	}
	return c.JSON(fiber.Map{"version": saved.Version, "document": request.Document, "updated_at": saved.UpdatedAt})
}

// Export only explicitly reviewed examples. Unreviewed AI guesses never become
// training labels; exclusions suppress replays and close-ups from positive data.
func ExportVideoAnalysisReview(c *fiber.Ctx) error {
	job, err := loadReviewJob(c)
	if err != nil {
		return err
	}
	result, _, events, err := trackingSource(job.ResultJSON)
	if err != nil {
		return fiber.NewError(409, err.Error())
	}
	var saved models.VideoAnalysisReview
	if err := database.DB.Where("job_id = ?", job.JobID).First(&saved).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fiber.NewError(409, "Save a review before exporting")
		}
		return fiber.NewError(500, "Could not load review")
	}
	doc := emptyTrackingReview()
	if err := json.Unmarshal([]byte(saved.Document), &doc); err != nil {
		return fiber.NewError(500, "Invalid saved review")
	}
	examples := []fiber.Map{}
	for i, raw := range events {
		correction, ok := doc.Events[strconv.Itoa(i)]
		if !ok {
			continue
		}
		event, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		timestamp, _ := event["t"].(float64)
		if correction.Time != nil {
			timestamp = *correction.Time
		}
		excluded := false
		for _, span := range doc.ExcludedSegments {
			if timestamp >= span.Start && timestamp < span.End {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}
		typ, _ := event["type"].(string)
		if correction.Type != nil {
			typ = strings.TrimSpace(*correction.Type)
		}
		label := 0
		if correction.Status == "confirmed" {
			label = 1
		}
		example := fiber.Map{"candidate_index": i, "t": timestamp, "type": typ, "label": label}
		for _, key := range []string{"from_tid", "to_tid"} {
			if id, ok := event[key].(float64); ok {
				canonical, _ := canonicalTrack(strconv.Itoa(int(id)), doc.Tracks)
				example[key] = canonical
				for source, identity := range doc.Tracks {
					root, _ := canonicalTrack(source, doc.Tracks)
					if root == canonical && identity.PlayerID != nil {
						example[strings.TrimSuffix(key, "_tid")+"_player_id"] = *identity.PlayerID
						break
					}
				}
			}
		}
		examples = append(examples, example)
	}
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=analysis-%s-reviewed.json", job.JobID))
	_, players, rosterErr := reviewRoster(job.MatchID)
	if rosterErr != nil {
		return fiber.NewError(500, "Could not export match roster")
	}
	roster := []fiber.Map{}
	for _, p := range players {
		roster = append(roster, fiber.Map{"id": p.ID, "name": p.Name, "club_id": p.ClubID, "jersey_number": p.JerseyNumber})
	}
	stats, _ := result["stats"].(map[string]any)
	source, _ := stats["tracking_source"].(map[string]any)
	return c.JSON(fiber.Map{"schema_version": 1, "match_id": job.MatchID, "video_id": job.VideoID, "job_id": job.JobID, "review_version": saved.Version, "reviewed_by": saved.UpdatedBy, "reviewed_at": saved.UpdatedAt, "lineup": doc.Lineup, "roster": roster, "source_video_sha256": source["sha256"], "track_identities": doc.Tracks, "team_clubs": doc.TeamClubs, "excluded_segments": doc.ExcludedSegments, "examples": examples})
}
