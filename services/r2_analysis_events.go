package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"afrigoals.com/models"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func saveAnalysisEventManifestToR2(ctx context.Context, event models.AnalysisEvent) (string, error) {
	cfg, err := loadR2Config()
	if err != nil {
		return "", err
	}
	client, err := newR2S3Client(ctx, cfg)
	if err != nil {
		return "", err
	}

	key := analysisEventManifestKey(event.MatchID, event.ID)
	body, err := json.MarshalIndent(map[string]any{
		"kind":  "analysis_event",
		"event": event,
	}, "", "  ")
	if err != nil {
		return "", err
	}

	if _, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(cfg.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String("application/json"),
	}); err != nil {
		return "", err
	}

	return key, nil
}

func deleteAnalysisEventManifestFromR2(ctx context.Context, matchID uint, eventID uint) error {
	cfg, err := loadR2Config()
	if err != nil {
		return err
	}
	client, err := newR2S3Client(ctx, cfg)
	if err != nil {
		return err
	}

	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(cfg.Bucket),
		Key:    aws.String(analysisEventManifestKey(matchID, eventID)),
	})
	return err
}

func analysisEventManifestKey(matchID uint, eventID uint) string {
	return fmt.Sprintf("matches/%d/analysis-events/%d.json", matchID, eventID)
}
