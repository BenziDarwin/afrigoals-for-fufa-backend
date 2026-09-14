// services/video_jobs.go
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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

type VideoMaskRegion struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
	Label  string  `json:"label,omitempty"`
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

func mustMarshalJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return b
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

func setResultString(result any, key string, value string) {
	if value == "" {
		return
	}

	m, ok := result.(map[string]any)
	if !ok {
		return
	}

	m[key] = value
	if vv, ok := m["video"].(map[string]any); ok {
		vv[key] = value
	}
}

func modelArtifactURL(artifactType string, jobID string) (string, error) {
	artifactTpl := strings.TrimSpace(os.Getenv("MODEL_SERVER_ARTIFACT_URL"))
	if artifactTpl == "" {
		uploadURL := strings.TrimSpace(os.Getenv("MODEL_SERVER_UPLOAD_URL"))
		if uploadURL != "" {
			artifactTpl = strings.TrimSuffix(uploadURL, "/upload") + "/job/:job_id/download/:artifact_type"
		}
	}
	if artifactTpl == "" {
		return "", fmt.Errorf("MODEL_SERVER_ARTIFACT_URL not set")
	}

	artifactURL := strings.ReplaceAll(artifactTpl, ":job_id", jobID)
	artifactURL = strings.ReplaceAll(artifactURL, ":artifact_type", artifactType)
	return artifactURL, nil
}

func uploadModelArtifactToR2(ctx context.Context, matchID uint, jobID string, artifactType string) (string, string, error) {
	cfg, err := loadR2Config()
	if err != nil {
		return "", "", err
	}

	client, err := newR2S3Client(ctx, cfg)
	if err != nil {
		return "", "", err
	}

	artifactURL, err := modelArtifactURL(artifactType, jobID)
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", artifactURL, nil)
	if err != nil {
		return "", "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("model artifact returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		if artifactType == "annotated_video" {
			contentType = "video/mp4"
		} else {
			contentType = "application/octet-stream"
		}
	}

	tmp, err := os.CreateTemp("", "afrigoals-r2-artifact-*.mp4")
	if err != nil {
		return "", "", err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		return "", "", err
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return "", "", err
	}

	key := fmt.Sprintf("matches/%d/ai-jobs/%s/%s.mp4", matchID, sanitizeObjectName(jobID), sanitizeObjectName(artifactType))
	if _, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(cfg.Bucket),
		Key:         aws.String(key),
		Body:        tmp,
		ContentType: aws.String(contentType),
	}); err != nil {
		return "", "", err
	}

	return key, publicR2ObjectURL(cfg, key), nil
}

func parseMatchIDParam(c *fiber.Ctx) (uint, error) {
	matchIDStr := strings.TrimSpace(c.Params("match_id"))
	matchID64, err := strconv.ParseUint(matchIDStr, 10, 32)
	if err != nil || matchID64 == 0 {
		return 0, fmt.Errorf("invalid match_id")
	}
	return uint(matchID64), nil
}

func parseVideoMaskRegions(raw string) []VideoMaskRegion {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var regions []VideoMaskRegion
	if err := json.Unmarshal([]byte(raw), &regions); err != nil {
		return nil
	}

	valid := make([]VideoMaskRegion, 0, len(regions))
	for _, region := range regions {
		if region.Width <= 0 || region.Height <= 0 {
			continue
		}
		if region.X < 0 {
			region.X = 0
		}
		if region.Y < 0 {
			region.Y = 0
		}
		if region.X > 1 {
			region.X = 1
		}
		if region.Y > 1 {
			region.Y = 1
		}
		if region.Width > 1-region.X {
			region.Width = 1 - region.X
		}
		if region.Height > 1-region.Y {
			region.Height = 1 - region.Y
		}
		if region.Width > 0 && region.Height > 0 {
			valid = append(valid, region)
		}
	}
	return valid
}

func copyStoredVideoToTemp(videoURL string, outputPath string) error {
	videoURL = strings.TrimSpace(videoURL)
	if videoURL == "" {
		return fmt.Errorf("selected video has no URL")
	}

	if strings.HasPrefix(videoURL, "/uploads/") {
		localPath := "." + filepath.FromSlash(videoURL)
		src, err := os.Open(localPath)
		if err != nil {
			return err
		}
		defer src.Close()

		dst, err := os.Create(outputPath)
		if err != nil {
			return err
		}
		defer dst.Close()

		_, err = io.Copy(dst, src)
		return err
	}

	if strings.HasPrefix(videoURL, "http://") || strings.HasPrefix(videoURL, "https://") {
		resp, err := http.Get(videoURL)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("video download returned HTTP %d", resp.StatusCode)
		}

		dst, err := os.Create(outputPath)
		if err != nil {
			return err
		}
		defer dst.Close()

		_, err = io.Copy(dst, resp.Body)
		return err
	}

	return fmt.Errorf("unsupported video URL: %s", videoURL)
}

func burnVideoMaskRegions(inputPath string, outputPath string, regions []VideoMaskRegion) error {
	filters := make([]string, 0, len(regions))
	for _, region := range regions {
		filters = append(filters, fmt.Sprintf(
			"drawbox=x=iw*%.6f:y=ih*%.6f:w=iw*%.6f:h=ih*%.6f:color=black@1:t=fill",
			region.X,
			region.Y,
			region.Width,
			region.Height,
		))
	}
	if len(filters) == 0 {
		return nil
	}

	cmd := exec.Command(
		"ffmpeg",
		"-y",
		"-i", inputPath,
		"-vf", strings.Join(filters, ","),
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "23",
		"-pix_fmt", "yuv420p",
		"-c:a", "copy",
		"-movflags", "+faststart",
		outputPath,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func createVideoFormFile(writer *multipart.Writer, fieldName string, filename string) (io.Writer, error) {
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(filename)))
	if contentType == "" || !strings.HasPrefix(contentType, "video/") {
		contentType = "video/mp4"
	}

	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, filename))
	header.Set("Content-Type", contentType)

	return writer.CreatePart(header)
}

func uploadLocalVideoToModel(matchID uint, userID uint, filename string, videoPath string) (ModelUploadResponse, int, []byte, error) {
	modelURL := strings.TrimSpace(os.Getenv("MODEL_SERVER_UPLOAD_URL"))
	if modelURL == "" {
		return ModelUploadResponse{}, 500, nil, fmt.Errorf("MODEL_SERVER_UPLOAD_URL not set")
	}

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	req, err := http.NewRequest("POST", modelURL, pr)
	if err != nil {
		_ = pr.Close()
		_ = pw.Close()
		return ModelUploadResponse{}, 500, nil, fmt.Errorf("failed to create request")
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	go func() {
		defer pw.Close()

		part, err := createVideoFormFile(writer, "file", filename)
		if err != nil {
			_ = pw.CloseWithError(fmt.Errorf("failed to create multipart: %w", err))
			return
		}

		videoFile, err := os.Open(videoPath)
		if err != nil {
			_ = pw.CloseWithError(fmt.Errorf("failed to open video for model forwarding: %w", err))
			return
		}
		defer videoFile.Close()

		if _, err := io.Copy(part, videoFile); err != nil {
			_ = pw.CloseWithError(fmt.Errorf("failed to stream video: %w", err))
			return
		}

		_ = writer.WriteField("match_id", fmt.Sprintf("%d", matchID))
		_ = writer.WriteField("requested_by_user_id", fmt.Sprintf("%d", userID))
		_ = writer.WriteField("use_commentary", "true")

		if err := writer.Close(); err != nil {
			_ = pw.CloseWithError(fmt.Errorf("failed to finalize multipart: %w", err))
			return
		}
	}()

	client := &http.Client{Timeout: 20 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return ModelUploadResponse{}, 502, nil, fmt.Errorf("model server upload failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ModelUploadResponse{}, resp.StatusCode, body, fmt.Errorf("model server returned non-2xx")
	}

	var up ModelUploadResponse
	if err := json.Unmarshal(body, &up); err != nil {
		return ModelUploadResponse{}, 502, body, fmt.Errorf("invalid model server response")
	}
	if strings.TrimSpace(up.JobID) == "" {
		return ModelUploadResponse{}, 502, body, fmt.Errorf("model server did not return job_id")
	}

	return up, resp.StatusCode, body, nil
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

	videoTmp, err := os.CreateTemp("", "afrigoals-model-video-*"+filepath.Ext(fileHeader.Filename))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to create temp video file"})
	}
	videoTmpPath := videoTmp.Name()
	defer os.Remove(videoTmpPath)

	if _, err := io.Copy(videoTmp, f); err != nil {
		_ = videoTmp.Close()
		return c.Status(500).JSON(fiber.Map{"error": "failed to save upload for model processing"})
	}
	if err := videoTmp.Close(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to finalize temp video file"})
	}

	modelVideoPath := videoTmpPath
	if regions := parseVideoMaskRegions(c.FormValue("mask_regions")); len(regions) > 0 {
		maskedTmp, err := os.CreateTemp("", "afrigoals-model-video-masked-*"+filepath.Ext(fileHeader.Filename))
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to create masked temp video file"})
		}
		maskedTmpPath := maskedTmp.Name()
		_ = maskedTmp.Close()
		defer os.Remove(maskedTmpPath)

		if err := burnVideoMaskRegions(videoTmpPath, maskedTmpPath, regions); err != nil {
			return c.Status(500).JSON(fiber.Map{
				"error":  "failed to apply video mask regions",
				"detail": err.Error(),
			})
		}
		modelVideoPath = maskedTmpPath
	}

	// forward to model server
	modelURL := strings.TrimSpace(os.Getenv("MODEL_SERVER_UPLOAD_URL"))
	if modelURL == "" {
		return c.Status(500).JSON(fiber.Map{"error": "MODEL_SERVER_UPLOAD_URL not set"})
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := createVideoFormFile(writer, "file", fileHeader.Filename)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to create multipart"})
	}
	videoFile, err := os.Open(modelVideoPath)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to reopen upload for model forwarding"})
	}
	defer videoFile.Close()
	if _, err := io.Copy(part, videoFile); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to stream file"})
	}

	_ = writer.WriteField("match_id", fmt.Sprintf("%d", matchID))
	_ = writer.WriteField("requested_by_user_id", fmt.Sprintf("%d", userID))
	_ = writer.WriteField("use_commentary", "true")

	audioTmp, err := os.CreateTemp("", "afrigoals-commentary-*.mp3")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to create temp commentary file"})
	}
	audioTmpPath := audioTmp.Name()
	_ = audioTmp.Close()
	defer os.Remove(audioTmpPath)

	if hasCommentary, err := extractCommentaryAudioIfPresent(videoTmpPath, audioTmpPath); err != nil {
		_ = writer.WriteField("commentary_error", err.Error())
	} else if hasCommentary {
		audioPart, err := writer.CreateFormFile("commentary_audio", "commentary.mp3")
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to create commentary multipart"})
		}
		audioFile, err := os.Open(audioTmpPath)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to open commentary audio"})
		}
		defer audioFile.Close()
		if _, err := io.Copy(audioPart, audioFile); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to stream commentary audio"})
		}
		_ = writer.WriteField("commentary_audio_format", "mp3")
	}

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

// POST /api/v1/analyst-matches/:match_id/videos/:video_id/analyze-with-models
// Sends the already-selected/stored match video to the model server and stores
// the resulting job against this match.
func AnalyzeStoredMatchVideoWithModels(c *fiber.Ctx) error {
	matchID, err := parseMatchIDParam(c)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	videoID64, err := strconv.ParseUint(strings.TrimSpace(c.Params("video_id")), 10, 32)
	if err != nil || videoID64 == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "invalid video_id"})
	}

	userID := extractUserID(c.Locals("user"))
	if userID == 0 {
		return c.Status(401).JSON(fiber.Map{"error": "unauthorized (user id missing)"})
	}

	var video models.Video
	if err := database.DB.Where("match_id = ? AND id = ?", matchID, uint(videoID64)).First(&video).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(404).JSON(fiber.Map{"error": "video not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch video", "detail": err.Error()})
	}

	videoURL := ""
	if video.VideoURL != nil && strings.TrimSpace(*video.VideoURL) != "" {
		videoURL = *video.VideoURL
	} else {
		videoURL = video.URL
	}

	filename := video.Title
	if video.OriginalFilename != nil && strings.TrimSpace(*video.OriginalFilename) != "" {
		filename = *video.OriginalFilename
	}
	if strings.TrimSpace(filename) == "" {
		filename = fmt.Sprintf("match-%d-video-%d.mp4", matchID, video.ID)
	}

	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".mp4"
	}
	videoTmp, err := os.CreateTemp("", "afrigoals-stored-model-video-*"+ext)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to create temp video file"})
	}
	videoTmpPath := videoTmp.Name()
	_ = videoTmp.Close()
	defer os.Remove(videoTmpPath)

	if err := copyStoredVideoToTemp(videoURL, videoTmpPath); err != nil {
		log.Printf("analyze-with-models: failed to load selected video match_id=%d video_id=%d url=%q err=%v", matchID, video.ID, videoURL, err)
		return c.Status(502).JSON(fiber.Map{"error": "failed to load selected video", "detail": err.Error()})
	}

	modelVideoPath := videoTmpPath
	if regions := parseVideoMaskRegions(c.FormValue("mask_regions")); len(regions) > 0 {
		maskedTmp, err := os.CreateTemp("", "afrigoals-stored-model-video-masked-*"+ext)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to create masked temp video file"})
		}
		maskedTmpPath := maskedTmp.Name()
		_ = maskedTmp.Close()
		defer os.Remove(maskedTmpPath)

		if err := burnVideoMaskRegions(videoTmpPath, maskedTmpPath, regions); err != nil {
			log.Printf("analyze-with-models: failed to apply mask regions match_id=%d video_id=%d err=%v", matchID, video.ID, err)
			return c.Status(500).JSON(fiber.Map{
				"error":  "failed to apply video mask regions",
				"detail": err.Error(),
			})
		}
		modelVideoPath = maskedTmpPath
	}

	up, statusCode, body, err := uploadLocalVideoToModel(matchID, userID, filename, modelVideoPath)
	if err != nil {
		log.Printf(
			"analyze-with-models: model upload failed match_id=%d video_id=%d model_url=%q status=%d err=%v body=%s",
			matchID,
			video.ID,
			strings.TrimSpace(os.Getenv("MODEL_SERVER_UPLOAD_URL")),
			statusCode,
			err,
			strings.TrimSpace(string(body)),
		)
		if len(body) > 0 {
			return c.Status(502).JSON(fiber.Map{"error": err.Error(), "status": statusCode, "body": string(body)})
		}
		return c.Status(statusCode).JSON(fiber.Map{"error": err.Error()})
	}

	job := models.VideoAnalysisJob{
		MatchID:           matchID,
		RequestedByUserID: &userID,
		JobID:             up.JobID,
		Status:            normalizeJobStatus(up.Status),
		OriginalFilename:  filename,
		ModelMessage:      up.Message,
		VideoID:           &video.ID,
	}

	if err := database.DB.Create(&job).Error; err != nil {
		log.Printf("analyze-with-models: job created but DB persist failed match_id=%d video_id=%d job_id=%s err=%v", matchID, video.ID, up.JobID, err)
		return c.Status(500).JSON(fiber.Map{
			"error":  "job created but failed to persist",
			"job_id": up.JobID,
			"detail": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"job_id":   job.JobID,
		"status":   job.Status,
		"message":  job.ModelMessage,
		"video_id": video.ID,
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

// POST /api/v1/analyst-matches/:match_id/video-jobs/:job_id/cancel
// Marks the local stored job as cancelled/failed so the studio stops resuming it.
func CancelStoredVideoJob(c *fiber.Ctx) error {
	matchID, err := parseMatchIDParam(c)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	jobID := strings.TrimSpace(c.Params("job_id"))
	if jobID == "" {
		return c.Status(400).JSON(fiber.Map{"error": "missing job_id"})
	}

	var job models.VideoAnalysisJob
	if err := database.DB.Where("match_id = ? AND job_id = ?", matchID, jobID).First(&job).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(404).JSON(fiber.Map{"error": "job not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch job", "detail": err.Error()})
	}

	job.Status = models.JobFailed
	job.ModelMessage = "Cancelled by user. Stored data was preserved."
	if zero := 0.0; job.Progress == nil {
		job.Progress = &zero
	}

	if err := database.DB.Save(&job).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to cancel job", "detail": err.Error()})
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
	annotated, siglip := extractArtifactsFromResult(result)
	if annotated != "" {
		job.AnnotatedVideoPath = annotated
	}
	if siglip != "" {
		job.SiglipHTMLPath = siglip
	}

	if annotated != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		if _, r2URL, err := uploadModelArtifactToR2(ctx, matchID, jobID, "annotated_video"); err != nil {
			return c.Status(502).JSON(fiber.Map{
				"error":  "failed to upload annotated video to Cloudflare R2",
				"detail": err.Error(),
			})
		} else {
			job.AnnotatedVideoPath = r2URL
			setResultString(result, "annotated_video", r2URL)
		}
	}
	job.ResultJSON = compactJSON(mustMarshalJSON(result))

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

	enriched, err := json.Marshal(result)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to encode enriched result", "detail": err.Error()})
	}

	c.Set("Content-Type", "application/json")
	return c.Send(enriched)
}

// GET /api/v1/analyst-matches/:match_id/video-jobs/:job_id/artifacts/:artifact_type
// Streams an artifact from the model server through the authenticated Go API.
func DownloadVideoJobArtifact(c *fiber.Ctx) error {
	matchID, err := parseMatchIDParam(c)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	jobID := strings.TrimSpace(c.Params("job_id"))
	artifactType := strings.TrimSpace(c.Params("artifact_type"))
	if jobID == "" || artifactType == "" {
		return c.Status(400).JSON(fiber.Map{"error": "missing job_id or artifact_type"})
	}

	var job models.VideoAnalysisJob
	if err := database.DB.Where("match_id = ? AND job_id = ?", matchID, jobID).First(&job).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(404).JSON(fiber.Map{"error": "job not found"})
		}
		return c.Status(500).JSON(fiber.Map{"error": "failed to fetch job", "detail": err.Error()})
	}

	if artifactType == "annotated_video" && strings.TrimSpace(job.AnnotatedVideoPath) != "" {
		return c.Redirect(job.AnnotatedVideoPath, fiber.StatusTemporaryRedirect)
	}

	artifactURL, err := modelArtifactURL(artifactType, jobID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	req, _ := http.NewRequest("GET", artifactURL, nil)
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return c.Status(502).JSON(fiber.Map{"error": "model server artifact fetch failed", "detail": err.Error()})
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return c.Status(502).JSON(fiber.Map{
			"error":  "model server returned non-2xx",
			"status": resp.StatusCode,
			"body":   string(body),
		})
	}

	if contentType := resp.Header.Get("Content-Type"); contentType != "" {
		c.Set("Content-Type", contentType)
	}
	if disposition := resp.Header.Get("Content-Disposition"); disposition != "" {
		c.Set("Content-Disposition", disposition)
	}
	return c.SendStream(resp.Body)
}
