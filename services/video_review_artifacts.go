package services

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"afrigoals.com/database"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
)

// This proxy retains match authorization and never accepts a caller-supplied upstream URL.
func ProxyVideoReviewArtifact(c *fiber.Ctx) error {
	job, err := loadReviewJob(c)
	if err != nil {
		return err
	}
	artifactURL, err := modelArtifactURL("tracking_trace", job.JobID)
	if err != nil {
		return fiber.NewError(503, err.Error())
	}
	base := strings.Split(artifactURL, "/download/")[0]
	suffix := "/jersey"
	if c.Params("track_id") != "" {
		suffix = "/tracks/" + url.PathEscape(c.Params("track_id")) + "/thumbnail"
	}
	if strings.HasSuffix(c.Path(), "/jersey/crop") {
		suffix += "/crop"
	}
	body := c.Body()
	if strings.HasSuffix(c.Path(), "/reviewed-stats") {
		suffix = "/reviewed-stats"
		var saved models.VideoAnalysisReview
		if database.DB.Where("job_id = ?", job.JobID).First(&saved).Error != nil {
			return fiber.NewError(409, "Save an identity review first")
		}
		var doc any
		if json.Unmarshal([]byte(saved.Document), &doc) != nil {
			return fiber.NewError(500, "Invalid saved review")
		}
		var result struct {
			Stats struct {
				Source struct {
					SHA string `json:"sha256"`
				} `json:"tracking_source"`
			} `json:"stats"`
		}
		if json.Unmarshal([]byte(job.ResultJSON), &result) != nil {
			return fiber.NewError(409, "Missing analysis source")
		}
		body, _ = json.Marshal(fiber.Map{"version": saved.Version, "document": doc, "source_video_sha256": result.Stats.Source.SHA})
	}
	query := url.Values{}
	if suffix == "/jersey/crop" {
		query.Set("crop", c.Query("crop"))
	}
	if c.Params("track_id") != "" {
		query.Set("at", c.Query("at", "first"))
	}
	req, err := http.NewRequestWithContext(c.UserContext(), c.Method(), base+suffix+"?"+query.Encode(), bytes.NewReader(body))
	if err != nil {
		return fiber.NewError(500, "Invalid model URL")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		return fiber.NewError(502, "Model service unavailable")
	}
	defer resp.Body.Close()
	content, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return fiber.NewError(502, "Cannot read model response")
	}
	c.Set("Content-Type", resp.Header.Get("Content-Type"))
	c.Set("Cache-Control", "private, no-store")
	return c.Status(resp.StatusCode).Send(content)
}
