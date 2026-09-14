package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"afrigoals.com/database"
	"afrigoals.com/middleware"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
)

const r2PresignTTL = 2 * time.Hour

type r2Config struct {
	Endpoint        string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	PublicBaseURL   string
}

func InitR2MatchVideoUpload(c *fiber.Ctx) error {
	_, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	matchIDUint := mustParseUint(c.Params("match_id"))
	if matchIDUint == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid match_id"})
	}

	cfg, err := loadR2Config()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error":   "Cloudflare R2 is not configured",
			"details": err.Error(),
		})
	}

	var req struct {
		Filename    string `json:"filename"`
		ContentType string `json:"content_type"`
		Size        int64  `json:"size"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	req.Filename = strings.TrimSpace(req.Filename)
	if req.Filename == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "filename is required"})
	}

	objectKey := buildMatchVideoObjectKey(matchIDUint, req.Filename)
	uploadURL, expiresAt, err := presignR2PutObject(cfg, objectKey, r2PresignTTL)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to create R2 upload URL",
			"details": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"method":      fiber.MethodPut,
			"upload_url":  uploadURL,
			"object_key":  objectKey,
			"public_url":  publicR2ObjectURL(cfg, objectKey),
			"expires_at":  expiresAt.Format(time.RFC3339),
			"headers":     fiber.Map{},
			"max_size":    req.Size,
			"contentType": req.ContentType,
		},
	})
}

func CompleteR2MatchVideoUpload(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	matchIDUint := mustParseUint(c.Params("match_id"))
	if matchIDUint == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid match_id"})
	}

	cfg, err := loadR2Config()
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error":   "Cloudflare R2 is not configured",
			"details": err.Error(),
		})
	}

	var req struct {
		ObjectKey        string `json:"object_key"`
		OriginalFilename string `json:"original_filename"`
		DurationSec      *int   `json:"duration_sec"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	req.ObjectKey = strings.Trim(strings.TrimSpace(req.ObjectKey), "/")
	if req.ObjectKey == "" || !strings.HasPrefix(req.ObjectKey, fmt.Sprintf("matches/%d/", matchIDUint)) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid object_key"})
	}
	if strings.TrimSpace(req.OriginalFilename) == "" {
		req.OriginalFilename = path.Base(req.ObjectKey)
	}

	videoURL := publicR2ObjectURL(cfg, req.ObjectKey)
	originalFilename := req.OriginalFilename

	video := models.Video{
		MatchID:          matchIDUint,
		Title:            originalFilename,
		Provider:         models.VideoProviderUpload,
		URL:              videoURL,
		OriginalFilename: &originalFilename,
		VideoURL:         &videoURL,
		DurationSec:      req.DurationSec,
		CreatedBy:        user.ID,
	}

	if err := database.DB.Create(&video).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to create video record",
			"details": err.Error(),
		})
	}

	matchVideo := models.MatchVideo{
		MatchID:          matchIDUint,
		OriginalFilename: originalFilename,
		VideoURL:         videoURL,
		DurationSec:      req.DurationSec,
		UploadedBy:       &user.ID,
	}
	if err := database.DB.Create(&matchVideo).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error":   "Failed to create match video record",
			"details": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"video":       video,
			"match_video": matchVideo,
		},
	})
}

func loadR2Config() (r2Config, error) {
	cfg := r2Config{
		Endpoint:        strings.TrimSpace(os.Getenv("R2_ENDPOINT")),
		Bucket:          strings.TrimSpace(os.Getenv("R2_BUCKET")),
		AccessKeyID:     strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID")),
		SecretAccessKey: strings.TrimSpace(os.Getenv("R2_SECRET_ACCESS_KEY")),
		PublicBaseURL:   strings.TrimSpace(os.Getenv("R2_PUBLIC_BASE_URL")),
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = strings.TrimSpace(os.Getenv("CLOUDFLARE_R2_ENDPOINT"))
	}
	if cfg.Bucket == "" {
		cfg.Bucket = strings.TrimSpace(os.Getenv("CLOUDFLARE_R2_BUCKET"))
	}
	if cfg.AccessKeyID == "" {
		cfg.AccessKeyID = strings.TrimSpace(os.Getenv("CLOUDFLARE_R2_ACCESS_KEY_ID"))
	}
	if cfg.SecretAccessKey == "" {
		cfg.SecretAccessKey = strings.TrimSpace(os.Getenv("CLOUDFLARE_R2_SECRET_ACCESS_KEY"))
	}
	if cfg.PublicBaseURL == "" {
		cfg.PublicBaseURL = strings.TrimSpace(os.Getenv("CLOUDFLARE_R2_PUBLIC_BASE_URL"))
	}

	if cfg.Endpoint == "" {
		accountID := strings.TrimSpace(os.Getenv("R2_ACCOUNT_ID"))
		if accountID == "" {
			accountID = strings.TrimSpace(os.Getenv("CLOUDFLARE_ACCOUNT_ID"))
		}
		if accountID != "" {
			cfg.Endpoint = fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
		}
	}

	if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" || cfg.PublicBaseURL == "" {
		return cfg, fmt.Errorf("set R2_ENDPOINT, R2_BUCKET, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY, and R2_PUBLIC_BASE_URL")
	}

	cfg.Endpoint = strings.TrimRight(cfg.Endpoint, "/")
	cfg.PublicBaseURL = strings.TrimRight(cfg.PublicBaseURL, "/")
	if strings.HasSuffix(cfg.Endpoint, "/"+cfg.Bucket) {
		cfg.Endpoint = strings.TrimSuffix(cfg.Endpoint, "/"+cfg.Bucket)
	}
	return cfg, nil
}

func buildMatchVideoObjectKey(matchID uint, filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		ext = ".mp4"
	}
	stem := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	return fmt.Sprintf(
		"matches/%d/%d-%s%s",
		matchID,
		time.Now().UnixNano(),
		sanitizeObjectName(stem),
		ext,
	)
}

func sanitizeObjectName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "match-video"
	}
	if len(value) > 80 {
		return value[:80]
	}
	return value
}

func publicR2ObjectURL(cfg r2Config, objectKey string) string {
	return cfg.PublicBaseURL + "/" + strings.TrimLeft(objectKey, "/")
}

func presignR2PutObject(cfg r2Config, objectKey string, ttl time.Duration) (string, time.Time, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(ttl)
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	expires := strconv.Itoa(int(ttl.Seconds()))
	credentialScope := dateStamp + "/auto/s3/aws4_request"

	baseURL, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return "", time.Time{}, err
	}
	baseURL.Path = path.Join("/", cfg.Bucket, objectKey)
	baseURL.RawQuery = ""

	query := baseURL.Query()
	query.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	query.Set("X-Amz-Credential", cfg.AccessKeyID+"/"+credentialScope)
	query.Set("X-Amz-Date", amzDate)
	query.Set("X-Amz-Expires", expires)
	query.Set("X-Amz-SignedHeaders", "host")
	baseURL.RawQuery = query.Encode()

	canonicalRequest := strings.Join([]string{
		"PUT",
		baseURL.EscapedPath(),
		baseURL.RawQuery,
		"host:" + baseURL.Host + "\n",
		"host",
		"UNSIGNED-PAYLOAD",
	}, "\n")

	hash := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		hex.EncodeToString(hash[:]),
	}, "\n")

	signingKey := awsV4SigningKey(cfg.SecretAccessKey, dateStamp, "auto", "s3")
	signature := hmacSHA256Hex(signingKey, stringToSign)

	query.Set("X-Amz-Signature", signature)
	baseURL.RawQuery = query.Encode()

	return baseURL.String(), expiresAt, nil
}

func awsV4SigningKey(secret, date, region, service string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), date)
	kRegion := hmacSHA256(kDate, region)
	kService := hmacSHA256(kRegion, service)
	return hmacSHA256(kService, "aws4_request")
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

func hmacSHA256Hex(key []byte, data string) string {
	return hex.EncodeToString(hmacSHA256(key, data))
}
