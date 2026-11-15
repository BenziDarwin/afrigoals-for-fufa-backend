package services

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"path"
	"strconv"
	"strings"
	"time"

	"afrigoals.com/database"
	"afrigoals.com/models"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxPlayerHighlightSize = 500 * 1024 * 1024

func validHighlightUpload(contentType string, size int64) bool {
	return size > 0 && size <= maxPlayerHighlightSize && (contentType == "video/mp4" || contentType == "video/webm" || contentType == "video/quicktime")
}

func InitPlayerHighlightUpload(c *fiber.Ctx) error {
	player, user, err := portfolioPlayer(c, true)
	if err != nil {
		return err
	}
	var req highlightRequest
	if c.BodyParser(&req) != nil {
		return fiber.NewError(400, "Invalid highlight upload")
	}
	if !validHighlightUpload(req.ContentType, req.SizeBytes) {
		return fiber.NewError(400, "Upload an MP4, WebM or MOV video up to 500 MB")
	}
	if strings.TrimSpace(req.Filename) == "" {
		return fiber.NewError(400, "Choose a video file")
	}
	cfg, err := loadR2Config()
	if err != nil {
		return fiber.NewError(503, "Video storage is unavailable. You can add a video link")
	}
	var highlight models.PlayerHighlight
	var uploadURL string
	var expires time.Time
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockPortfolioPlayer(tx, &player, user); err != nil {
			return err
		}
		if req.UploadID != nil && *req.UploadID > 0 {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND player_id=? AND kind='upload' AND status='pending'", *req.UploadID, player.ID).First(&highlight).Error; err != nil {
				return fiber.NewError(409, "This upload cannot be resumed. Select the file again")
			}
			if highlight.SizeBytes != req.SizeBytes || highlight.ContentType != req.ContentType {
				return fiber.NewError(400, "Select the same video file when retrying an upload")
			}
		} else {
			extension := map[string]string{"video/mp4": ".mp4", "video/webm": ".webm", "video/quicktime": ".mov"}[req.ContentType]
			key := path.Join("players", strconv.FormatUint(uint64(player.ID), 10), "highlights", uuid.NewString()+extension)
			highlight = models.PlayerHighlight{PlayerID: player.ID, Kind: "upload", Status: "pending", ObjectKey: key, VideoURL: publicR2ObjectURL(cfg, key), ContentType: req.ContentType, SizeBytes: req.SizeBytes, CreatedBy: user.ID}
		}
		if err := applyHighlightDetails(tx, player.ID, req, &highlight); err != nil {
			return err
		}
		highlight.UpdatedBy = user.ID
		uploadURL, expires, err = presignR2PutObject(cfg, highlight.ObjectKey, r2PresignTTL)
		if err != nil {
			return fiber.NewError(503, "Could not prepare the video upload")
		}
		return tx.Omit("Club").Save(&highlight).Error
	})
	if err != nil {
		return err
	}

	return c.Status(201).JSON(fiber.Map{"id": highlight.ID, "upload_url": uploadURL, "expires_at": expires.Format(time.RFC3339), "content_type": req.ContentType})
}

func verifyPlayerHighlightObject(ctx context.Context, cfg r2Config, highlight models.PlayerHighlight) error {
	client, err := newR2S3Client(ctx, cfg)
	if err != nil {
		return err
	}
	head, err := client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(cfg.Bucket), Key: aws.String(highlight.ObjectKey)})
	if err != nil {
		var missing *types.NotFound
		var missingKey *types.NoSuchKey
		if errors.As(err, &missing) || errors.As(err, &missingKey) {
			return fiber.NewError(409, "The video upload is not available yet. Finish uploading and retry")
		}
		return fiber.NewError(503, "Video storage could not verify the upload. Retry shortly")
	}
	contentType, _, err := mime.ParseMediaType(aws.ToString(head.ContentType))
	if err != nil || contentType != highlight.ContentType || aws.ToInt64(head.ContentLength) != highlight.SizeBytes {
		return fiber.NewError(400, "The uploaded video's type or size does not match the selected file")
	}
	return nil
}

func CompletePlayerHighlightUpload(c *fiber.Ctx) error {
	player, user, err := portfolioPlayer(c, true)
	if err != nil {
		return err
	}
	id, err := strconv.ParseUint(c.Params("highlight_id"), 10, 32)
	if err != nil || id == 0 {
		return fiber.NewError(400, "Invalid highlight ID")
	}
	var highlight models.PlayerHighlight
	if err := database.DB.Where("id=? AND player_id=? AND kind='upload'", id, player.ID).First(&highlight).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fiber.ErrNotFound
		}
		return err
	}
	if highlight.Status == "ready" {
		return c.JSON(fiber.Map{"success": true, "id": highlight.ID})
	}
	cfg, err := loadR2Config()
	if err != nil {
		return fiber.NewError(503, "Video storage is unavailable")
	}
	ctx, cancel := context.WithTimeout(c.UserContext(), 20*time.Second)
	defer cancel()
	if err := verifyPlayerHighlightObject(ctx, cfg, highlight); err != nil {
		return err
	}
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		var current models.Player
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, player.ID).Error; err != nil {
			return err
		}
		if !canManagePlayerPortfolio(user, &current) {
			return fiber.ErrForbidden
		}
		result := tx.Model(&models.PlayerHighlight{}).Where("id=? AND player_id=?", highlight.ID, player.ID).Updates(map[string]interface{}{"status": "ready", "updated_by": user.ID})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fiber.ErrNotFound
		}
		return nil
	})
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"success": true, "id": highlight.ID, "video_url": highlight.VideoURL, "message": fmt.Sprintf("Highlight %d uploaded", highlight.ID)})
}
