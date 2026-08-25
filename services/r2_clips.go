package services

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func uploadClipFileToR2(ctx context.Context, matchID uint, clipID uint, filePath string) (string, string, error) {
	cfg, err := loadR2Config()
	if err != nil {
		return "", "", err
	}

	client, err := newR2S3Client(ctx, cfg)
	if err != nil {
		return "", "", err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", "", err
	}
	defer file.Close()

	key := matchClipObjectKey(matchID, clipID)
	if _, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(cfg.Bucket),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String("video/mp4"),
	}); err != nil {
		return "", "", err
	}

	return key, publicR2ObjectURL(cfg, key), nil
}

func deleteClipFileFromR2(ctx context.Context, matchID uint, clipID uint) error {
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
		Key:    aws.String(matchClipObjectKey(matchID, clipID)),
	})
	return err
}

func matchClipObjectKey(matchID uint, clipID uint) string {
	return fmt.Sprintf("matches/%d/clips/clip_%d.mp4", matchID, clipID)
}
