// services/video_jobs.go
package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"afrigoals.com/database"
	"afrigoals.com/models"
)

// =======================
// Types (model server)
// =======================

// If your model server returns {"job_id":"...","status":"queued","message":"..."}
type ModelUploadResponse struct {
	JobID   string `json:"job_id"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type ModelJobStatusResponse struct {
	JobID    string   `json:"job_id"`
	Status   string   `json:"status"`
	Message  string   `json:"message,omitempty"`
	Stage    string   `json:"stage,omitempty"`
	Progress *float64 `json:"progress,omitempty"`
	Error    *string  `json:"error,omitempty"`
}

// =======================
// Types (API requests)
// =======================

type StoredVideoJobCreateRequest struct {
	JobID            string `json:"job_id"`
	Status           string `json:"status,omitempty"`
	OriginalFilename string `json:"original_filename,omitempty"`
	ModelMessage     string `json:"model_message,omitempty"`
}

// =======================
// Helpers
// =======================

// extractUserID attempts to read user id from Fiber Locals("user") in multiple shapes.
// IMPORTANT: We enforce auth (401) when we cannot extract a valid user id.
// This prevents FK violations on video_analysis_jobs.requested_by_user_id.
func extractUserID(u any) uint {
	if u == nil {
		return 0
	}

	// Common: *models.User
	if uu, ok := u.(*models.User); ok && uu != nil {
		if uu.ID > 0 {
			return uu.ID
		}
	}

	// Common: models.User
	if uu, ok := u.(models.User); ok {
		if uu.ID > 0 {
			return uu.ID
		}
	}

	// Common: map claims
	if m, ok := u.(map[string]any); ok {
		// id
		if v, ok := m["id"]; ok {
			switch t := v.(type) {
			case float64:
				if t > 0 {
					return uint(t)
				}
			case int:
				if t > 0 {
					return uint(t)
				}
			case int64:
				if t > 0 {
					return uint(t)
				}
			case string:
				n, _ := strconv.ParseUint(t, 10, 32)
				if n > 0 {
					return uint(n)
				}
			}
		}

		// user_id
		if v, ok := m["user_id"]; ok {
			switch t := v.(type) {
			case float64:
				if t > 0 {
					return uint(t)
				}
			case int:
				if t > 0 {
					return uint(t)
				}
			case int64:
				if t > 0 {
					return uint(t)
				}
			case string:
				n, _ := strconv.ParseUint(t, 10, 32)
				if n > 0 {
					return uint(n)
				}
			}
		}

		// sub (sometimes string user id)
		if v, ok := m["sub"]; ok {
			switch t := v.(type) {
			case string:
				n, _ := strconv.ParseUint(t, 10, 32)
				if n > 0 {
					return uint(n)
				}
			}
		}

		// uuid -> lookup
		if v, ok := m["uuid"]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				var user models.User
				if err := database.DB.Where("uuid = ?", strings.TrimSpace(s)).First(&user).Error; err == nil {
					if user.ID > 0 {
						return user.ID
					}
				}
			}
		}
	}

	// Sometimes locals contains UUID directly
	if s, ok := u.(string); ok && strings.TrimSpace(s) != "" {
		var user models.User
		if err := database.DB.Where("uuid = ?", strings.TrimSpace(s)).First(&user).Error; err == nil {
			if user.ID > 0 {
				return user.ID
			}
		}
	}

	return 0
}

func normalizeJobStatus(s string) models.VideoAnalysisJobStatus {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "queued":
		return models.JobQueued
	case "processing":
		return models.JobProcessing
	case "completed":
		return models.JobCompleted
	case "failed":
		return models.JobFailed
	default:
		return models.JobQueued
	}
}

// Compact JSON helper (optional)
func compactJSON(raw []byte) string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return string(raw)
	}
	return string(b)
}

// Extract annotated_video/siglip_html from result JSON (supports direct or nested "video")
func extractArtifactsFromResult(result any) (annotated string, siglip string) {
	m, ok := result.(map[string]any)
	if !ok {
		return "", ""
	}

	// direct
	if v, ok := m["annotated_video"].(string); ok {
		annotated = v
	}
	if v, ok := m["siglip_html"].(string); ok {
		siglip = v
	}

	// nested video
	if vv, ok := m["video"].(map[string]any); ok {
		if annotated == "" {
			if v, ok := vv["annotated_video"].(string); ok {
				annotated = v
			}
		}
		if siglip == "" {
			if v, ok := vv["siglip_html"].(string); ok {
				siglip = v
			}
		}
	}

	return annotated, siglip
}

func parseMatchIDParam(c *fiber.Ctx) (uint, error) {
	matchIDStr := strings.TrimSpace(c.Params("match_id"))
	matchID64, err := strconv.ParseUint(matchIDStr, 10, 32)
	if err != nil || matchID64 == 0 {
		return 0, fmt.Errorf("invalid match_id")
	}
	return uint(matchID64), nil
}

// =======================
// Handlers
// =======================

// POST /api/v1/analyst-matches/:match_id/videos/upload-to-models
// multipart/form-data: file=<video>
func UploadMatchVideoToModelsAndStoreJob(c *fiber.Ctx) error {
	matchID, err := parseMatchIDParam(c)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	// user id (required)
	userID := extractUserID(c.Locals("user"))
	if userID == 0 {
		return c.Status(401).JSON(fiber.Map{"error": "unauthorized (user id missing)"})
	}

	// file
	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "missing video file (form field 'file')"})
	}
	f, err := fileHeader.Open()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to open upload"})
	}
	defer f.Close()

	// forward to model server
	modelURL := strings.TrimSpace(os.Getenv("MODEL_SERVER_UPLOAD_URL"))
	if modelURL == "" {
		return c.Status(500).JSON(fiber.Map{"error": "MODEL_SERVER_UPLOAD_URL not set"})
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", fileHeader.Filename)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to create multipart"})
	}
	if _, err := io.Copy(part, f); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to stream file"})
	}

	_ = writer.WriteField("match_id", fmt.Sprintf("%d", matchID))
	_ = writer.WriteField("requested_by_user_id", fmt.Sprintf("%d", userID))

	if err := writer.Close(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to finalize multipart"})
	}

	req, err := http.NewRequest("POST", modelURL, &buf)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to create request"})
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 20 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "model server upload failed", "detail": err.Error()})
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.Status(502).JSON(fiber.Map{
			"error":  "model server returned non-2xx",
			"status": resp.StatusCode,
			"body":   string(body),
		})
	}

	var up ModelUploadResponse
	if err := json.Unmarshal(body, &up); err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "invalid model server response", "body": string(body)})
	}
	if strings.TrimSpace(up.JobID) == "" {
		return c.Status(502).JSON(fiber.Map{"error": "model server did not return job_id", "body": string(body)})
	}

	// persist job immediately (match-scoped)
	db := database.DB

	job := models.VideoAnalysisJob{
		MatchID:           matchID,
		RequestedByUserID: &userID,
		JobID:             up.JobID,
		Status:            normalizeJobStatus(up.Status),
		OriginalFilename:  fileHeader.Filename,
		ModelMessage:      up.Message,
	}

	if err := db.Create(&job).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":  "job created but failed to persist",
			"job_id": up.JobID,
			"detail": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"job_id":  job.JobID,
		"status":  job.Status,
		"message": job.ModelMessage,
	})
}

// POST /api/v1/analyst-matches/:match_id/video-jobs
// Register a job id to DB (useful when user pastes a job_id or you want to upsert)
//
// IMPORTANT: This endpoint MUST be authenticated because requested_by_user_id
// has an FK to users. If user id cannot be extracted => 401.
func RegisterStoredMatchVideoJob(c *fiber.Ctx) error {
	matchID, err := parseMatchIDParam(c)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	// user id (required)
	userID := extractUserID(c.Locals("user"))
	if userID == 0 {
		return c.Status(401).JSON(fiber.Map{
			"error":  "unauthorized",
			"detail": "user id missing from auth context; check middleware locals(user)",
		})
	}

	var req StoredVideoJobCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid JSON body"})
	}

	jobID := strings.TrimSpace(req.JobID)
	if jobID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "job_id is required"})
	}

	db := database.DB

	// Upsert by (match_id, job_id)
	var job models.VideoAnalysisJob
	findErr := db.Where("match_id = ? AND job_id = ?", matchID, jobID).First(&job).Error
	if findErr == nil {
		// Update if supplied
		if strings.TrimSpace(req.Status) != "" {
			job.Status = normalizeJobStatus(req.Status)
		}
		if strings.TrimSpace(req.OriginalFilename) != "" {
			job.OriginalFilename = req.OriginalFilename
		}
		if strings.TrimSpace(req.ModelMessage) != "" {
			job.ModelMessage = req.ModelMessage
		}

		// Keep ownership consistent; if it was 0 for any reason, set it.
		if job.RequestedByUserID == nil || *job.RequestedByUserID == 0 {
			job.RequestedByUserID = &userID
		}

		if err := db.Save(&job).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to update job", "detail": err.Error()})
		}
		return c.JSON(job)
	}

	if findErr != nil && findErr != gorm.ErrRecordNotFound {
		return c.Status(500).JSON(fiber.Map{"error": "failed to query existing job", "detail": findErr.Error()})
	}

	// Create new
	job = models.VideoAnalysisJob{
		MatchID:           matchID,
		RequestedByUserID: &userID, // REQUIRED (FK)
		JobID:             jobID,
		Status:            normalizeJobStatus(req.Status),
		OriginalFilename:  req.OriginalFilename,
		ModelMessage:      req.ModelMessage,
	}

	if err := db.Create(&job).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to create job", "detail": err.Error()})
	}

	return c.JSON(job)
}

// GET /api/v1/analyst-matches/:match_id/video-jobs
// List stored jobs for a match
func ListStoredMatchVideoJobs(c *fiber.Ctx) error {
	matchID, err := parseMatchIDParam(c)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	db := database.DB
	var jobs []models.VideoAnalysisJob
	if err := db.Where("match_id = ?", matchID).Order("created_at DESC").Find(&jobs).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to list jobs", "detail": err.Error()})
	}

	return c.JSON(fiber.Map{"jobs": jobs})
}

// GET /api/v1/analyst-matches/:match_id/video-jobs/:job_id
// Returns the stored job record from DB (so you never lose it).
func GetStoredVideoJob(c *fiber.Ctx) error {
	matchID, err := parseMatchIDParam(c)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	jobID := strings.TrimSpace(c.Params("job_id"))
	if jobID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "missing job_id"})
	}

	db := database.DB

	var job models.VideoAnalysisJob
	if err := db.Where("match_id = ? AND job_id = ?", matchID, jobID).First(&job).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(404).JSON(fiber.Map{"error": "job not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch job", "detail": err.Error()})
	}

	return c.JSON(job)
}

// POST /api/v1/analyst-matches/:match_id/video-jobs/:job_id/refresh
// Calls model server status endpoint, updates DB, returns updated record.
func RefreshVideoJobStatus(c *fiber.Ctx) error {
	matchID, err := parseMatchIDParam(c)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	jobID := strings.TrimSpace(c.Params("job_id"))
	if jobID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "missing job_id"})
	}

	db := database.DB

	var job models.VideoAnalysisJob
	if err := db.Where("match_id = ? AND job_id = ?", matchID, jobID).First(&job).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(404).JSON(fiber.Map{"error": "job not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch job", "detail": err.Error()})
	}

	modelStatusTpl := strings.TrimSpace(os.Getenv("MODEL_SERVER_STATUS_URL")) // e.g. http://ml:8000/api/jobs/:job_id
	if modelStatusTpl == "" {
		return c.Status(500).JSON(fiber.Map{"error": "MODEL_SERVER_STATUS_URL not set"})
	}
	statusURL := strings.ReplaceAll(modelStatusTpl, ":job_id", jobID)

	req, _ := http.NewRequest("GET", statusURL, nil)
	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "model server poll failed", "detail": err.Error()})
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.Status(502).JSON(fiber.Map{
			"error":  "model server returned non-2xx",
			"status": resp.StatusCode,
			"body":   string(body),
		})
	}

	var st ModelJobStatusResponse
	if err := json.Unmarshal(body, &st); err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "invalid model status response", "body": string(body)})
	}

	// Update DB fields
	job.Status = normalizeJobStatus(st.Status)
	job.ModelMessage = st.Message
	job.Stage = st.Stage
	job.Progress = st.Progress
	if st.Error != nil {
		if strings.TrimSpace(job.ModelMessage) == "" {
			job.ModelMessage = *st.Error
		} else {
			job.ModelMessage = fmt.Sprintf("%s | error: %s", job.ModelMessage, *st.Error)
		}
	}

	if err := db.Save(&job).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to update job in DB", "detail": err.Error()})
	}

	return c.JSON(job)
}

// GET /api/v1/analyst-matches/:match_id/video-jobs/:job_id/result
// Fetch result from model server, store in DB (ResultJSON + artifacts).
// Returns raw model result JSON to match frontend expectations.
func FetchAndStoreVideoJobResult(c *fiber.Ctx) error {
	matchID, err := parseMatchIDParam(c)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	jobID := strings.TrimSpace(c.Params("job_id"))
	if jobID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "missing job_id"})
	}

	db := database.DB

	var job models.VideoAnalysisJob
	if err := db.Where("match_id = ? AND job_id = ?", matchID, jobID).First(&job).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(404).JSON(fiber.Map{"error": "job not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch job", "detail": err.Error()})
	}

	resultTpl := strings.TrimSpace(os.Getenv("MODEL_SERVER_RESULT_URL")) // e.g. http://ml:8000/api/jobs/:job_id/result
	if resultTpl == "" {
		return c.Status(500).JSON(fiber.Map{"error": "MODEL_SERVER_RESULT_URL not set"})
	}
	resultURL := strings.ReplaceAll(resultTpl, ":job_id", jobID)

	req, _ := http.NewRequest("GET", resultURL, nil)
	client := &http.Client{Timeout: 60 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "model server result fetch failed", "detail": err.Error()})
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.Status(502).JSON(fiber.Map{
			"error":  "model server returned non-2xx",
			"status": resp.StatusCode,
			"body":   string(raw),
		})
	}

	// Parse result as generic JSON
	var result any
	if err := json.Unmarshal(raw, &result); err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "invalid model result JSON", "body": string(raw)})
	}

	// Persist compact JSON + artifact pointers
	job.ResultJSON = compactJSON(raw)
	annotated, siglip := extractArtifactsFromResult(result)
	if annotated != "" {
		job.AnnotatedVideoPath = annotated
	}
	if siglip != "" {
		job.SiglipHTMLPath = siglip
	}

	// Optional: if result payload contains status/message, update job
	if m, ok := result.(map[string]any); ok {
		if s, ok := m["status"].(string); ok && strings.TrimSpace(s) != "" {
			job.Status = normalizeJobStatus(s)
		}
		if msg, ok := m["message"].(string); ok && strings.TrimSpace(msg) != "" {
			job.ModelMessage = msg
		}
		// nested "video"
		if vid, ok := m["video"].(map[string]any); ok {
			if s, ok := vid["status"].(string); ok && strings.TrimSpace(s) != "" {
				job.Status = normalizeJobStatus(s)
			}
			if msg, ok := vid["message"].(string); ok && strings.TrimSpace(msg) != "" {
				job.ModelMessage = msg
			}
		}
	}

	if err := db.Save(&job).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to persist result", "detail": err.Error()})
	}

	c.Set("Content-Type", "application/json")
	return c.Send(raw)
}
