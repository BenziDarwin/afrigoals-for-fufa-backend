package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func uploadClipFileToR2(ctx context.Context, matchID uint, eventID uint, filePath string) (string, string, error) {
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

	key := matchEventClipObjectKey(matchID, eventID)
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

func deleteClipFileFromR2(ctx context.Context, matchID uint, eventID uint) error {
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
		Key:    aws.String(matchEventClipObjectKey(matchID, eventID)),
	})
	return err
}

func matchEventClipObjectKey(matchID uint, eventID uint) string {
	return fmt.Sprintf("matches/%d/clips/event_%d.mp4", matchID, eventID)
}

func matchClipsPrefix(matchID uint) string {
	return fmt.Sprintf("matches/%d/clips/", matchID)
}

// headClipObject checks whether a clip has already been cut for this event,
// so CutClipByWindow can skip re-generating it.
func headClipObject(ctx context.Context, matchID uint, eventID uint) (bool, string, error) {
	cfg, err := loadR2Config()
	if err != nil {
		return false, "", err
	}

	client, err := newR2S3Client(ctx, cfg)
	if err != nil {
		return false, "", err
	}

	key := matchEventClipObjectKey(matchID, eventID)
	if _, err := client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(cfg.Bucket),
		Key:    aws.String(key),
	}); err != nil {
		var notFound *types.NotFound
		if errors.As(err, &notFound) {
			return false, "", nil
		}
		var noSuchKey *types.NoSuchKey
		if errors.As(err, &noSuchKey) {
			return false, "", nil
		}
		return false, "", err
	}

	return true, publicR2ObjectURL(cfg, key), nil
}

var clipObjectKeyPattern = regexp.MustCompile(`event_(\d+)\.mp4$`)

type clipObjectSummary struct {
	EventID      uint
	ObjectKey    string
	ClipURL      string
	LastModified time.Time
}

// listMatchClipObjects lists every clip object stored in R2 for a match by
// walking the bucket directly - there is no DB row to query, so this is the
// source of truth for "which clips exist" for the analyst clip-cutting
// workflow. Objects left over from the old clip-id-keyed naming scheme don't
// match clipObjectKeyPattern and are silently skipped.
func listMatchClipObjects(ctx context.Context, matchID uint) ([]clipObjectSummary, error) {
	cfg, err := loadR2Config()
	if err != nil {
		return nil, err
	}

	client, err := newR2S3Client(ctx, cfg)
	if err != nil {
		return nil, err
	}

	prefix := matchClipsPrefix(matchID)
	var summaries []clipObjectSummary
	var continuationToken *string

	for {
		out, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(cfg.Bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return nil, err
		}

		for _, obj := range out.Contents {
			if obj.Key == nil {
				continue
			}
			matches := clipObjectKeyPattern.FindStringSubmatch(*obj.Key)
			if len(matches) != 2 {
				continue
			}
			var eventID uint
			if _, err := fmt.Sscanf(matches[1], "%d", &eventID); err != nil {
				continue
			}
			lastModified := time.Time{}
			if obj.LastModified != nil {
				lastModified = *obj.LastModified
			}
			summaries = append(summaries, clipObjectSummary{
				EventID:      eventID,
				ObjectKey:    *obj.Key,
				ClipURL:      publicR2ObjectURL(cfg, *obj.Key),
				LastModified: lastModified,
			})
		}

		if out.IsTruncated == nil || !*out.IsTruncated {
			break
		}
		continuationToken = out.NextContinuationToken
	}

	return summaries, nil
}
