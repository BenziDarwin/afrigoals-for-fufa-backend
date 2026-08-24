package services

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func createLMVFromR2Object(ctx context.Context, cfg r2Config, objectKey string) (string, string, error) {
	lmvKey := lmvObjectKey(objectKey)

	client, err := newR2S3Client(ctx, cfg)
	if err != nil {
		return "", "", err
	}

	inputFile, err := os.CreateTemp("", "afrigoals-lmv-source-*.mp4")
	if err != nil {
		return "", "", err
	}
	inputPath := inputFile.Name()
	defer os.Remove(inputPath)

	object, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(cfg.Bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		_ = inputFile.Close()
		return "", "", err
	}
	defer object.Body.Close()

	if _, err := io.Copy(inputFile, object.Body); err != nil {
		_ = inputFile.Close()
		return "", "", err
	}
	if err := inputFile.Close(); err != nil {
		return "", "", err
	}

	outputFile, err := os.CreateTemp("", "afrigoals-lmv-output-*.mp4")
	if err != nil {
		return "", "", err
	}
	outputPath := outputFile.Name()
	_ = outputFile.Close()
	defer os.Remove(outputPath)

	if err := ffmpegTranscodeToLMV(inputPath, outputPath); err != nil {
		return "", "", err
	}

	file, err := os.Open(outputPath)
	if err != nil {
		return "", "", err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return "", "", err
	}

	if _, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(cfg.Bucket),
		Key:           aws.String(lmvKey),
		Body:          file,
		ContentLength: aws.Int64(info.Size()),
		ContentType:   aws.String("video/mp4"),
	}); err != nil {
		return "", "", err
	}

	return lmvKey, publicR2ObjectURL(cfg, lmvKey), nil
}

func lmvObjectKey(objectKey string) string {
	dir := path.Dir(objectKey)
	ext := path.Ext(objectKey)
	name := strings.TrimSuffix(path.Base(objectKey), ext)
	if ext == "" {
		ext = ".mp4"
	}
	return path.Join(dir, "lmv", name+"-lmv"+ext)
}

func ffmpegTranscodeToLMV(inputPath, outputPath string) error {
	args := []string{
		"-y",
		"-i", inputPath,
		"-vf", "scale='min(1280,iw)':-2",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "30",
		"-c:a", "aac",
		"-b:a", "96k",
		"-movflags", "+faststart",
		outputPath,
	}

	cmd := exec.Command("ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg lmv transcode failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
