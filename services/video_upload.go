// services/video_upload.go
package services

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"afrigoals.com/database"
	"afrigoals.com/middleware"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

/*
===========================================================
EXISTING SMALL-UPLOAD ENDPOINT (kept as-is)
===========================================================
*/

func UploadMatchVideo(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	matchID := c.Params("match_id")
	matchIDUint := mustParseUint(matchID)

	// ✅ prevent match_id=0 issues
	if matchIDUint == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid match_id",
		})
	}

	file, err := c.FormFile("video")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Video file is required",
		})
	}

	// Create uploads directory if it doesn't exist
	uploadDir := "./uploads/videos"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create upload directory",
		})
	}

	// Generate unique filenames
	timestamp := time.Now().Unix()
	ext := filepath.Ext(file.Filename)
	if strings.TrimSpace(ext) == "" {
		ext = ".mp4"
	}

	// ✅ Save original first (kept briefly for transcoding)
	originalFilenameOnDisk := fmt.Sprintf("match_%s_%d_original%s", matchID, timestamp, ext)
	originalSavePath := filepath.Join(uploadDir, originalFilenameOnDisk)

	// ✅ Final transcoded file name (browser-safe MP4)
	finalFilename := fmt.Sprintf("match_%s_%d.mp4", matchID, timestamp)
	finalSavePath := filepath.Join(uploadDir, finalFilename)

	// Streaming save original upload
	src, err := file.Open()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to open video file",
		})
	}
	defer src.Close()

	dst, err := os.Create(originalSavePath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create destination file",
		})
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		_ = os.Remove(originalSavePath)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save video file",
		})
	}

	// ✅ IMPORTANT: Transcode to H.264/AAC + faststart for browser playback/seek
	if err := ffmpegTranscodeToH264Faststart(originalSavePath, finalSavePath); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "ffmpeg transcode failed",
			"details": err.Error(),
		})
	}

	// Optional: delete original to save space
	_ = os.Remove(originalSavePath)

	// Create relative URL for serving (served by /uploads static)
	videoURL := "/uploads/videos/" + finalFilename
	originalFilename := file.Filename

	// Create video record (KEEPING your existing models.Video)
	video := models.Video{
		MatchID:          matchIDUint,
		Title:            originalFilename,
		Provider:         models.VideoProviderUpload,
		URL:              videoURL,
		OriginalFilename: &originalFilename,
		VideoURL:         &videoURL,
		CreatedBy:        user.ID,
	}

	if err := database.DB.Create(&video).Error; err != nil {
		_ = os.Remove(finalSavePath)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create video record",
		})
	}

	// ALSO create MatchVideo record (best-effort; does not fail request)
	matchVideo := models.MatchVideo{
		MatchID:          matchIDUint,
		OriginalFilename: originalFilename,
		VideoURL:         videoURL,
		UploadedBy:       &user.ID,
	}
	_ = database.DB.Create(&matchVideo).Error

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"video":       video,
			"match_video": matchVideo,
		},
	})
}

/*
===========================================================
NEW: LARGE-UPLOAD (CHUNKED) ENDPOINTS
===========================================================

Routes you should add (under analystMatchesWrite):
  POST   /videos/upload/init                    -> InitMatchVideoUpload
  PUT    /videos/upload/:upload_id/chunk?index=  -> UploadMatchVideoChunk
  GET    /videos/upload/:upload_id/progress      -> GetUploadProgress        (MISSING BEFORE)
  POST   /videos/upload/:upload_id/complete      -> CompleteMatchVideoUpload
  DELETE /videos/upload/:upload_id/cancel        -> CancelMatchVideoUpload   (MISSING BEFORE)

Optional server maintenance:
  Call CleanupStaleUploads() on startup or via cron.

Notes:
  - Chunk size must be <= your Fiber BodyLimit.
  - Chunk endpoint uses raw bytes (application/octet-stream).
*/

const (
	DefaultChunkSizeBytes = int64(50 * 1024 * 1024)  // 50MB
	MaxChunkSizeBytes     = int64(200 * 1024 * 1024) // 200MB safety cap
)

func InitMatchVideoUpload(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	matchIDUint := mustParseUint(c.Params("match_id"))
	if matchIDUint == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid match_id"})
	}

	var req struct {
		Filename  string `json:"filename"`
		TotalSize int64  `json:"total_size"`
		ChunkSize int64  `json:"chunk_size"` // optional
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	req.Filename = strings.TrimSpace(req.Filename)
	if req.Filename == "" || req.TotalSize <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "filename and total_size are required"})
	}

	chunkSize := req.ChunkSize
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSizeBytes
	}
	if chunkSize > MaxChunkSizeBytes {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("chunk_size too large (max %d bytes)", MaxChunkSizeBytes),
		})
	}

	totalChunks := int((req.TotalSize + chunkSize - 1) / chunkSize)
	if totalChunks <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid chunk calculation"})
	}

	sess := models.UploadSession{
		MatchID:     matchIDUint,
		CreatedBy:   user.ID,
		Filename:    req.Filename,
		TotalSize:   req.TotalSize,
		ChunkSize:   chunkSize,
		TotalChunks: totalChunks,
		Status:      models.UploadStatusUploading,
	}

	if err := database.DB.Create(&sess).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create upload session"})
	}

	// create temp dir: ./uploads/videos/tmp/<upload_id>/
	dir := uploadTempDir(sess.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		_ = database.DB.Delete(&sess).Error
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create temp dir"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"upload_id":    sess.ID,
			"match_id":     sess.MatchID,
			"chunk_size":   sess.ChunkSize,
			"total_chunks": sess.TotalChunks,
		},
	})
}

// Upload chunk as raw body
// PUT /videos/upload/:upload_id/chunk?index=N
// Headers (optional): X-Chunk-SHA256: <hex sha256 of body>
func UploadMatchVideoChunk(c *fiber.Ctx) error {
	_, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	uploadID := mustParseUint(c.Params("upload_id"))
	if uploadID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid upload_id"})
	}

	idx, err := strconv.Atoi(c.Query("index"))
	if err != nil || idx < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid chunk index"})
	}

	var sess models.UploadSession
	if err := database.DB.First(&sess, uploadID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Upload session not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	if sess.Status != models.UploadStatusUploading {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Upload session not in uploading state"})
	}
	if idx >= sess.TotalChunks {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Chunk index out of range"})
	}

	body := c.Body()
	if len(body) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Empty chunk body"})
	}
	if int64(len(body)) > sess.ChunkSize {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Chunk exceeds chunk_size"})
	}

	// optional checksum
	expected := strings.TrimSpace(c.Get("X-Chunk-SHA256"))
	if expected != "" {
		sum := sha256.Sum256(body)
		got := hex.EncodeToString(sum[:])
		if !strings.EqualFold(got, expected) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Chunk checksum mismatch"})
		}
	}

	dir := uploadTempDir(sess.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to create temp dir",
			"details": err.Error(),
			"dir":     dir,
		})
	}

	chunkPath := filepath.Join(dir, chunkFilename(idx))

	if err := os.WriteFile(chunkPath, body, 0644); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to write chunk",
			"details": err.Error(),
			"path":    chunkPath,
			"len":     len(body),
		})
	}

	_ = database.DB.Model(&sess).Update("updated_at", time.Now()).Error

	return c.JSON(fiber.Map{"success": true})
}

// ✅ ADDED (missing API #1): Get upload progress
// GET /videos/upload/:upload_id/progress
// ✅ Get upload progress
// GET /videos/upload/:upload_id/progress
func GetUploadProgress(c *fiber.Ctx) error {
	_, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	uploadID := mustParseUint(c.Params("upload_id"))
	if uploadID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid upload_id"})
	}

	var sess models.UploadSession
	if err := database.DB.First(&sess, uploadID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Upload session not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	// scan chunk dir
	dir := uploadTempDir(sess.ID)
	uploadedChunks := make([]int, 0, sess.TotalChunks)

	for i := 0; i < sess.TotalChunks; i++ {
		chunkPath := filepath.Join(dir, chunkFilename(i))
		if _, err := os.Stat(chunkPath); err == nil {
			uploadedChunks = append(uploadedChunks, i)
		}
	}

	progress := 0.0
	if sess.TotalChunks > 0 {
		progress = float64(len(uploadedChunks)) / float64(sess.TotalChunks) * 100.0
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"upload_id":       sess.ID,
			"status":          sess.Status,
			"total_chunks":    sess.TotalChunks,
			"uploaded_chunks": uploadedChunks,
			"uploaded_count":  len(uploadedChunks),
			"progress":        progress,
			"error_msg":       sess.ErrorMsg,
		},
	})
}

func CompleteMatchVideoUpload(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	uploadID := mustParseUint(c.Params("upload_id"))
	if uploadID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid upload_id"})
	}

	var sess models.UploadSession
	if err := database.DB.First(&sess, uploadID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Upload session not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	if sess.Status != models.UploadStatusUploading {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Upload session not in uploading state"})
	}

	_ = database.DB.Model(&sess).Update("status", models.UploadStatusAssembling).Error

	dir := uploadTempDir(sess.ID)

	// verify chunks exist
	for i := 0; i < sess.TotalChunks; i++ {
		p := filepath.Join(dir, chunkFilename(i))
		if _, err := os.Stat(p); err != nil {
			msg := fmt.Sprintf("missing chunk %d", i)
			_ = database.DB.Model(&sess).Updates(map[string]any{
				"status":    models.UploadStatusFailed,
				"error_msg": msg,
			}).Error
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": msg})
		}
	}

	uploadDir := "./uploads/videos"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		_ = database.DB.Model(&sess).Updates(map[string]any{
			"status":    models.UploadStatusFailed,
			"error_msg": "failed to create upload dir",
		}).Error
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create upload directory"})
	}

	timestamp := time.Now().Unix()
	finalFilename := fmt.Sprintf("match_%d_%d.mp4", sess.MatchID, timestamp)
	finalSavePath := filepath.Join(uploadDir, finalFilename)

	out, err := os.Create(finalSavePath)
	if err != nil {
		_ = database.DB.Model(&sess).Updates(map[string]any{
			"status":    models.UploadStatusFailed,
			"error_msg": "failed to create final file",
		}).Error
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create destination file"})
	}
	defer out.Close()

	var written int64
	for i := 0; i < sess.TotalChunks; i++ {
		p := filepath.Join(dir, chunkFilename(i))
		in, err := os.Open(p)
		if err != nil {
			_ = os.Remove(finalSavePath)
			_ = database.DB.Model(&sess).Updates(map[string]any{
				"status":    models.UploadStatusFailed,
				"error_msg": fmt.Sprintf("failed to open chunk %d", i),
			}).Error
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to open chunk"})
		}

		n, err := io.Copy(out, in)
		in.Close()
		if err != nil {
			_ = os.Remove(finalSavePath)
			_ = database.DB.Model(&sess).Updates(map[string]any{
				"status":    models.UploadStatusFailed,
				"error_msg": fmt.Sprintf("failed to assemble chunk %d", i),
			}).Error
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to assemble file"})
		}
		written += n
	}

	// optional size check (non-fatal)
	if sess.TotalSize > 0 && written != sess.TotalSize {
		msg := fmt.Sprintf("size mismatch: expected %d, got %d", sess.TotalSize, written)
		_ = database.DB.Model(&sess).Update("error_msg", msg).Error
	}

	// cleanup chunks
	_ = os.RemoveAll(dir)

	// Create relative URL for serving
	videoURL := "/uploads/videos/" + finalFilename
	originalFilename := sess.Filename

	video := models.Video{
		MatchID:          sess.MatchID,
		Title:            originalFilename,
		Provider:         models.VideoProviderUpload,
		URL:              videoURL,
		OriginalFilename: &originalFilename,
		VideoURL:         &videoURL,
		CreatedBy:        user.ID,
	}

	if err := database.DB.Create(&video).Error; err != nil {
		_ = os.Remove(finalSavePath)
		_ = database.DB.Model(&sess).Updates(map[string]any{
			"status":    models.UploadStatusFailed,
			"error_msg": "failed to create video record",
		}).Error
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create video record"})
	}

	matchVideo := models.MatchVideo{
		MatchID:          sess.MatchID,
		OriginalFilename: originalFilename,
		VideoURL:         videoURL,
		UploadedBy:       &user.ID,
	}
	_ = database.DB.Create(&matchVideo).Error

	finalPath := finalSavePath
	_ = database.DB.Model(&sess).Updates(map[string]any{
		"status":     models.UploadStatusComplete,
		"final_path": &finalPath,
	}).Error

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"video":       video,
			"match_video": matchVideo,
		},
	})
}

// ✅ ADDED (missing API #2): Cancel upload + cleanup temp chunks
// DELETE /videos/upload/:upload_id/cancel
func CancelMatchVideoUpload(c *fiber.Ctx) error {
	_, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	uploadID := mustParseUint(c.Params("upload_id"))
	if uploadID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid upload_id"})
	}

	var sess models.UploadSession
	if err := database.DB.First(&sess, uploadID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Upload session not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	// delete temp dir (best-effort)
	_ = os.RemoveAll(uploadTempDir(sess.ID))

	// mark cancelled/failed (depending on what statuses you have)
	// If you don't have UploadStatusCancelled, keep it "failed".
	newStatus := models.UploadStatusFailed
	if sess.Status == models.UploadStatusComplete {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Upload already completed"})
	}

	_ = database.DB.Model(&sess).Updates(map[string]any{
		"status":    newStatus,
		"error_msg": "cancelled by user",
	}).Error

	return c.JSON(fiber.Map{"success": true, "message": "Upload cancelled"})
}

/*
===========================================================
EXISTING DELETE + CLIP SERVICES (kept as-is)
===========================================================
*/

func DeleteMatchVideo(c *fiber.Ctx) error {
	matchID := c.Params("match_id")
	videoID := c.Params("video_id")

	// Load video first so we can delete physical file too
	var video models.Video
	if err := database.DB.Where("match_id = ? AND id = ?", matchID, videoID).First(&video).Error; err == nil {
		// Try delete physical file if it is local (/uploads/...)
		var url string
		if video.VideoURL != nil && *video.VideoURL != "" {
			url = *video.VideoURL
		} else if video.URL != "" {
			url = video.URL
		}

		if url != "" {
			if localPath, ok := uploadsURLToLocalPath(url); ok {
				_ = os.Remove(localPath)
			}
		}
	}

	// KEEPING your delete logic
	result := database.DB.Where("match_id = ? AND id = ?", matchID, videoID).Delete(&models.Video{})
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete video",
		})
	}

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Video not found",
		})
	}

	// Also delete from MatchVideo table (best-effort)
	_ = database.DB.Where("match_id = ? AND id = ?", matchID, videoID).Delete(&models.MatchVideo{}).Error

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Video deleted",
	})
}

func DeleteMatchClip(c *fiber.Ctx) error {
	matchID := c.Params("match_id")
	clipID := c.Params("clip_id")

	// ✅ ADDED: load clip so we can delete physical file too
	var clip models.Clip
	if err := database.DB.Where("match_id = ? AND id = ?", matchID, clipID).First(&clip).Error; err == nil {
		if clip.ClipURL != nil && *clip.ClipURL != "" {
			if localPath, ok := uploadsURLToLocalPath(*clip.ClipURL); ok {
				_ = os.Remove(localPath)
			}
		}
	}

	result := database.DB.Where("match_id = ? AND id = ?", matchID, clipID).Delete(&models.Clip{})
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete clip",
		})
	}

	if result.RowsAffected == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Clip not found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Clip deleted",
	})
}

func CutClipByWindow(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	matchID := c.Params("match_id")
	matchIDUint := mustParseUint(matchID)

	// prevent match_id=0
	if matchIDUint == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid match_id",
		})
	}

	var req struct {
		EventID     uint `json:"event_id"`
		PreSeconds  int  `json:"pre_seconds"`
		PostSeconds int  `json:"post_seconds"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// ✅ ADDED: basic validation
	if req.EventID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid event_id",
		})
	}
	if req.PreSeconds < 0 {
		req.PreSeconds = 0
	}
	if req.PostSeconds < 0 {
		req.PostSeconds = 0
	}

	// Get the event
	var event models.AnalysisEvent
	if err := database.DB.
		Where("match_id = ? AND id = ?", matchIDUint, req.EventID).
		First(&event).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Event not found",
		})
	}

	// Calculate clip window
	startSec := int(event.TimestampSeconds) - req.PreSeconds
	if startSec < 0 {
		startSec = 0
	}
	endSec := int(event.TimestampSeconds) + req.PostSeconds
	durationSec := endSec - startSec
	if durationSec <= 0 {
		durationSec = 1
	}

	// Generate clip title
	clipTitle := fmt.Sprintf("%s", event.Type)
	if event.PlayerName != nil && *event.PlayerName != "Team Event" {
		clipTitle = fmt.Sprintf("%s - %s", event.Type, *event.PlayerName)
	}

	// Create clip (DB first so we can name the file using clip.ID)
	clip := models.Clip{
		MatchID:   matchIDUint,
		VideoID:   event.VideoID,
		EventID:   &req.EventID,
		Title:     clipTitle,
		StartSec:  startSec,
		EndSec:    &endSec,
		CreatedBy: user.ID,
	}

	if err := database.DB.Create(&clip).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create clip",
		})
	}

	// ✅ ADDED: ensure clips directory exists
	clipsDir := "./uploads/clips"
	if err := os.MkdirAll(clipsDir, 0755); err != nil {
		_ = database.DB.Delete(&clip).Error
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create clips directory",
		})
	}

	// ✅ ADDED: resolve the input video path from the video record referenced by event.VideoID
	var v models.Video
	if err := database.DB.Where("match_id = ? AND id = ?", matchIDUint, event.VideoID).First(&v).Error; err != nil {
		_ = database.DB.Delete(&clip).Error
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Video not found for this event",
		})
	}

	var videoURL string
	if v.VideoURL != nil && *v.VideoURL != "" {
		videoURL = *v.VideoURL
	} else if strings.TrimSpace(v.URL) != "" {
		videoURL = v.URL
	} else {
		_ = database.DB.Delete(&clip).Error
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Video URL missing for this event",
		})
	}

	inputPath, isLocal, ok := videoURLToClipInput(videoURL)
	if !ok {
		_ = database.DB.Delete(&clip).Error
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Video URL is not usable for clip cutting",
		})
	}

	if isLocal {
		if _, err := os.Stat(inputPath); err != nil {
			_ = database.DB.Delete(&clip).Error
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error":   "Video file not found on disk",
				"details": inputPath,
			})
		}
	}

	if !isLocal && strings.TrimSpace(inputPath) == "" {
		_ = database.DB.Delete(&clip).Error
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Video URL missing for clip cutting"})
	}

	// ✅ ADDED: run ffmpeg to actually generate the clip file (browser-safe mp4 + faststart)
	outFile := fmt.Sprintf("clip_%d.mp4", clip.ID)
	outPath := filepath.Join(clipsDir, outFile)

	if err := ffmpegCutClipToH264Faststart(inputPath, outPath, startSec, durationSec); err != nil {
		_ = os.Remove(outPath)
		_ = database.DB.Delete(&clip).Error
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "ffmpeg clip cut failed",
			"details": err.Error(),
		})
	}

	// ✅ Now generate a REAL clip URL that exists in /uploads/clips/
	clipURL := "/uploads/clips/" + outFile
	clip.ClipURL = &clipURL

	// Update the clip with URL
	database.DB.Save(&clip)

	// Also update the event with clip URL
	event.ClipURL = &clipURL
	database.DB.Save(&event)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"clip": clip,
		},
	})
}

/*
===========================================================
HELPERS (existing + chunked helpers)
===========================================================
*/

func mustParseUint(s string) uint {
	v, _ := strconv.ParseUint(s, 10, 32)
	return uint(v)
}

func uploadsURLToLocalPath(url string) (string, bool) {
	u := strings.TrimSpace(url)
	if !strings.HasPrefix(u, "/uploads/") {
		return "", false
	}
	rel := strings.TrimPrefix(u, "/")
	return filepath.Clean("./" + filepath.FromSlash(rel)), true
}

func videoURLToClipInput(videoURL string) (string, bool, bool) {
	if localPath, ok := uploadsURLToLocalPath(videoURL); ok {
		return localPath, true, true
	}

	u := strings.TrimSpace(videoURL)
	if strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://") {
		return u, false, true
	}

	return "", false, false
}

func uploadTempDir(uploadID uint) string {
	return filepath.Clean(filepath.Join("./uploads/videos/tmp", fmt.Sprintf("%d", uploadID)))
}

func chunkFilename(idx int) string {
	return fmt.Sprintf("chunk_%06d.part", idx)
}

/*
===========================================================
FFMPEG HELPERS (kept as-is)
===========================================================
*/

func ffmpegTranscodeToH264Faststart(inputPath, outputPath string) error {
	_ = os.MkdirAll(filepath.Dir(outputPath), 0755)

	cmd := exec.Command(
		"ffmpeg",
		"-y",
		"-i", inputPath,
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "23",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+faststart",
		outputPath,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func ffmpegCutClipToH264Faststart(inputPath, outputPath string, startSec int, durationSec int) error {
	_ = os.MkdirAll(filepath.Dir(outputPath), 0755)

	if startSec < 0 {
		startSec = 0
	}
	if durationSec <= 0 {
		durationSec = 1
	}

	cmd := exec.Command(
		"ffmpeg",
		"-y",
		"-ss", strconv.Itoa(startSec),
		"-i", inputPath,
		"-t", strconv.Itoa(durationSec),
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "23",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+faststart",
		outputPath,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

var _ = bytes.NewBuffer
