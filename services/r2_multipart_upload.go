package services

import (
	"context"
	"fmt"
	"math"
	"path"
	"sort"
	"strconv"
	"strings"

	"afrigoals.com/database"
	"afrigoals.com/middleware"
	"afrigoals.com/models"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

const (
	multipartThresholdBytes = int64(500 * 1024 * 1024)
	multipartChunkSizeBytes = int64(50 * 1024 * 1024)
	minMultipartChunkBytes  = int64(5 * 1024 * 1024)
	maxMultipartParts       = 10000
)

type multipartUploadPart struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag,omitempty"`
	URL        string `json:"url,omitempty"`
}

func InitiateR2MultipartVideoUpload(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var req struct {
		MatchID     uint   `json:"match_id"`
		FileName    string `json:"file_name"`
		FileSize    int64  `json:"file_size"`
		ContentType string `json:"content_type"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	req.FileName = strings.TrimSpace(req.FileName)
	if req.MatchID == 0 || req.FileName == "" || req.FileSize <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "match_id, file_name, and file_size are required"})
	}
	if req.FileSize < multipartThresholdBytes {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Use direct upload for files under 500MB"})
	}

	cfg, err := loadR2Config()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error":   "Cloudflare R2 is not configured",
			"details": err.Error(),
		})
	}

	ctx := context.Background()
	client, err := newR2S3Client(ctx, cfg)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to create R2 client",
			"details": err.Error(),
		})
	}

	storageKey := buildMultipartMatchVideoObjectKey(req.MatchID, req.FileName)
	contentType := strings.TrimSpace(req.ContentType)
	if contentType == "" {
		contentType = "video/mp4"
	}

	createRes, err := client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(cfg.Bucket),
		Key:         aws.String(storageKey),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   "Failed to initiate R2 multipart upload",
			"details": err.Error(),
		})
	}

	totalParts := int(math.Ceil(float64(req.FileSize) / float64(multipartChunkSizeBytes)))
	if totalParts > maxMultipartParts {
		_, _ = client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(cfg.Bucket),
			Key:      aws.String(storageKey),
			UploadId: createRes.UploadId,
		})
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "File requires too many multipart chunks"})
	}

	upload := models.MatchVideoUpload{
		MatchID:       req.MatchID,
		UserID:        user.ID,
		FileName:      req.FileName,
		StorageKey:    storageKey,
		UploadID:      aws.ToString(createRes.UploadId),
		FileSize:      req.FileSize,
		ChunkSize:     multipartChunkSizeBytes,
		TotalParts:    totalParts,
		UploadedParts: datatypes.JSONMap{},
		Status:        models.MatchVideoUploadStatusInitiated,
	}
	if err := database.DB.Create(&upload).Error; err != nil {
		_, _ = client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(cfg.Bucket),
			Key:      aws.String(storageKey),
			UploadId: createRes.UploadId,
		})
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to create multipart upload session",
			"details": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"id":          upload.ID,
			"upload_id":   upload.UploadID,
			"key":         upload.StorageKey,
			"chunk_size":  upload.ChunkSize,
			"total_parts": upload.TotalParts,
			"status":      upload.Status,
		},
	})
}

func SignR2MultipartUploadParts(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var req struct {
		UploadID string `json:"upload_id"`
		Key      string `json:"key"`
		Parts    []int  `json:"parts"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	upload, err := findMultipartUpload(req.UploadID, req.Key, user.ID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}
	if upload.Status == models.MatchVideoUploadStatusCompleted || upload.Status == models.MatchVideoUploadStatusCancelled {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Upload session is not active"})
	}
	if len(req.Parts) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "parts are required"})
	}

	cfg, err := loadR2Config()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Cloudflare R2 is not configured", "details": err.Error()})
	}
	ctx := context.Background()
	client, err := newR2S3Client(ctx, cfg)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create R2 client", "details": err.Error()})
	}
	presigner := s3.NewPresignClient(client)

	signedParts := make([]multipartUploadPart, 0, len(req.Parts))
	for _, partNumber := range uniqueSortedParts(req.Parts) {
		if partNumber < 1 || partNumber > upload.TotalParts {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": fmt.Sprintf("Invalid part number %d", partNumber)})
		}
		presigned, err := presigner.PresignUploadPart(ctx, &s3.UploadPartInput{
			Bucket:     aws.String(cfg.Bucket),
			Key:        aws.String(upload.StorageKey),
			UploadId:   aws.String(upload.UploadID),
			PartNumber: aws.Int32(int32(partNumber)),
		}, s3.WithPresignExpires(r2PresignTTL))
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   "Failed to sign upload part",
				"details": err.Error(),
			})
		}
		signedParts = append(signedParts, multipartUploadPart{
			PartNumber: partNumber,
			URL:        presigned.URL,
		})
	}

	if upload.Status == models.MatchVideoUploadStatusInitiated {
		_ = database.DB.Model(&upload).Update("status", models.MatchVideoUploadStatusUploading).Error
	}

	return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"parts": signedParts}})
}

func CompleteR2MultipartVideoUpload(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var req struct {
		UploadID         string                `json:"upload_id"`
		Key              string                `json:"key"`
		Parts            []multipartUploadPart `json:"parts"`
		OriginalFilename string                `json:"original_filename"`
		DurationSec      *int                  `json:"duration_sec"`
		CreateLMV        bool                  `json:"create_lmv"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	upload, err := findMultipartUpload(req.UploadID, req.Key, user.ID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}
	if upload.Status == models.MatchVideoUploadStatusCompleted {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Upload session already completed"})
	}
	if len(req.Parts) != upload.TotalParts {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "All uploaded part ETags are required"})
	}

	completedParts := make([]types.CompletedPart, 0, len(req.Parts))
	uploadedParts := datatypes.JSONMap{}
	for _, part := range req.Parts {
		part.ETag = strings.TrimSpace(part.ETag)
		if part.PartNumber < 1 || part.PartNumber > upload.TotalParts || part.ETag == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid completed part list"})
		}
		completedParts = append(completedParts, types.CompletedPart{
			ETag:       aws.String(part.ETag),
			PartNumber: aws.Int32(int32(part.PartNumber)),
		})
		uploadedParts[strconv.Itoa(part.PartNumber)] = part.ETag
	}
	sort.Slice(completedParts, func(i, j int) bool {
		return aws.ToInt32(completedParts[i].PartNumber) < aws.ToInt32(completedParts[j].PartNumber)
	})

	cfg, err := loadR2Config()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Cloudflare R2 is not configured", "details": err.Error()})
	}
	ctx := context.Background()
	client, err := newR2S3Client(ctx, cfg)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create R2 client", "details": err.Error()})
	}

	if _, err := client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(cfg.Bucket),
		Key:      aws.String(upload.StorageKey),
		UploadId: aws.String(upload.UploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completedParts,
		},
	}); err != nil {
		_ = database.DB.Model(&upload).Update("status", models.MatchVideoUploadStatusFailed).Error
		return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
			"error":   "Failed to complete R2 multipart upload",
			"details": err.Error(),
		})
	}

	originalFilename := strings.TrimSpace(req.OriginalFilename)
	if originalFilename == "" {
		originalFilename = upload.FileName
	}
	videoURL := publicR2ObjectURL(cfg, upload.StorageKey)
	var lmvWarning string
	if req.CreateLMV {
		_, lmvURL, err := createLMVFromR2Object(ctx, cfg, upload.StorageKey)
		if err != nil {
			lmvWarning = err.Error()
		} else {
			videoURL = lmvURL
		}
	}

	video := models.Video{
		MatchID:          upload.MatchID,
		Title:            originalFilename,
		Provider:         models.VideoProviderUpload,
		URL:              videoURL,
		OriginalFilename: &originalFilename,
		VideoURL:         &videoURL,
		DurationSec:      req.DurationSec,
		CreatedBy:        user.ID,
	}

	tx := database.DB.Begin()
	if err := tx.Create(&video).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create video record", "details": err.Error()})
	}

	matchVideo := models.MatchVideo{
		MatchID:          upload.MatchID,
		OriginalFilename: originalFilename,
		VideoURL:         videoURL,
		DurationSec:      req.DurationSec,
		UploadedBy:       &user.ID,
	}
	if err := tx.Create(&matchVideo).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create match video record", "details": err.Error()})
	}

	if err := tx.Model(&upload).Updates(map[string]any{
		"uploaded_parts": uploadedParts,
		"status":         models.MatchVideoUploadStatusCompleted,
	}).Error; err != nil {
		tx.Rollback()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update upload session", "details": err.Error()})
	}
	if err := tx.Commit().Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save completed upload", "details": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"video":       video,
			"match_video": matchVideo,
			"upload":      upload,
			"lmv_warning": lmvWarning,
		},
	})
}

func CancelR2MultipartVideoUpload(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var req struct {
		UploadID string `json:"upload_id"`
		Key      string `json:"key"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	upload, err := findMultipartUpload(req.UploadID, req.Key, user.ID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}

	cfg, err := loadR2Config()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "Cloudflare R2 is not configured", "details": err.Error()})
	}
	ctx := context.Background()
	client, err := newR2S3Client(ctx, cfg)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create R2 client", "details": err.Error()})
	}

	if upload.Status != models.MatchVideoUploadStatusCompleted && upload.Status != models.MatchVideoUploadStatusCancelled {
		if _, err := client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
			Bucket:   aws.String(cfg.Bucket),
			Key:      aws.String(upload.StorageKey),
			UploadId: aws.String(upload.UploadID),
		}); err != nil {
			return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{"error": "Failed to abort R2 multipart upload", "details": err.Error()})
		}
	}

	if err := database.DB.Model(&upload).Update("status", models.MatchVideoUploadStatusCancelled).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to cancel upload session", "details": err.Error()})
	}

	return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"status": models.MatchVideoUploadStatusCancelled}})
}

func newR2S3Client(ctx context.Context, cfg r2Config) (*s3.Client, error) {
	awsCfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion("auto"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, err
	}

	return s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true
	}), nil
}

func findMultipartUpload(uploadID, key string, userID uint) (models.MatchVideoUpload, error) {
	var upload models.MatchVideoUpload
	uploadID = strings.TrimSpace(uploadID)
	key = strings.Trim(strings.TrimSpace(key), "/")
	if uploadID == "" || key == "" {
		return upload, fmt.Errorf("upload_id and key are required")
	}

	err := database.DB.
		Where("upload_id = ? AND storage_key = ? AND user_id = ?", uploadID, key, userID).
		First(&upload).Error
	if err != nil {
		return upload, fmt.Errorf("upload session not found")
	}
	return upload, nil
}

func buildMultipartMatchVideoObjectKey(matchID uint, filename string) string {
	ext := strings.ToLower(path.Ext(filename))
	if ext == "" {
		ext = ".mp4"
	}
	stem := strings.TrimSuffix(path.Base(filename), path.Ext(filename))
	return fmt.Sprintf(
		"matches/%d/%s-%s%s",
		matchID,
		uuid.New().String(),
		sanitizeObjectName(stem),
		ext,
	)
}

func uniqueSortedParts(parts []int) []int {
	seen := map[int]bool{}
	next := make([]int, 0, len(parts))
	for _, part := range parts {
		if seen[part] {
			continue
		}
		seen[part] = true
		next = append(next, part)
	}
	sort.Ints(next)
	return next
}
