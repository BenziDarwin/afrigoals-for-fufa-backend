// services/video_probe.go
package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// errInvalidVideo means ffprobe read the file and found no usable video stream.
var errInvalidVideo = errors.New("The uploaded file could not be decoded as a valid video.")

// errProbeUnavailable means ffprobe is not installed. Validation is skipped in
// that case so the existing flow keeps working.
var errProbeUnavailable = errors.New("ffprobe not available")

const videoProbeTimeout = 5 * time.Minute

// VideoProbeInfo is the subset of ffprobe output the pipeline uses.
type VideoProbeInfo struct {
	DurationSec float64 `json:"duration_sec"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	FrameRate   float64 `json:"frame_rate"`
	Codec       string  `json:"codec"`
	BitRate     int64   `json:"bit_rate,omitempty"`
	SizeBytes   int64   `json:"size_bytes"`
}

type ffprobeOutput struct {
	Streams []struct {
		CodecType    string `json:"codec_type"`
		CodecName    string `json:"codec_name"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		AvgFrameRate string `json:"avg_frame_rate"`
		RFrameRate   string `json:"r_frame_rate"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
		BitRate  string `json:"bit_rate"`
		Size     string `json:"size"`
	} `json:"format"`
}

// probeVideoFile runs ffprobe on a local file. It returns errInvalidVideo when the
// file has no video stream or no duration, and errProbeUnavailable when the
// ffprobe binary is missing.
func probeVideoFile(ctx context.Context, path string) (VideoProbeInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, videoProbeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx,
		"ffprobe",
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return VideoProbeInfo{}, errProbeUnavailable
		}
		return VideoProbeInfo{}, fmt.Errorf("%w: ffprobe: %s", errInvalidVideo, strings.TrimSpace(stderr.String()))
	}
	return parseFFprobeOutput(stdout.Bytes())
}

func parseFFprobeOutput(raw []byte) (VideoProbeInfo, error) {
	var out ffprobeOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		return VideoProbeInfo{}, fmt.Errorf("%w: unreadable ffprobe output", errInvalidVideo)
	}

	var info VideoProbeInfo
	found := false
	for _, s := range out.Streams {
		if s.CodecType != "video" {
			continue
		}
		found = true
		info.Width = s.Width
		info.Height = s.Height
		info.Codec = s.CodecName
		info.FrameRate = parseFrameRate(s.AvgFrameRate)
		if info.FrameRate == 0 {
			info.FrameRate = parseFrameRate(s.RFrameRate)
		}
		break
	}
	if !found {
		return VideoProbeInfo{}, fmt.Errorf("%w: no video stream", errInvalidVideo)
	}

	info.DurationSec, _ = strconv.ParseFloat(strings.TrimSpace(out.Format.Duration), 64)
	if info.DurationSec <= 0 {
		return VideoProbeInfo{}, fmt.Errorf("%w: no duration", errInvalidVideo)
	}
	info.BitRate, _ = strconv.ParseInt(strings.TrimSpace(out.Format.BitRate), 10, 64)
	info.SizeBytes, _ = strconv.ParseInt(strings.TrimSpace(out.Format.Size), 10, 64)
	return info, nil
}

// parseFrameRate turns "25/1" or "30000/1001" into frames per second.
// It returns 0 when the value is unknown.
func parseFrameRate(value string) float64 {
	num, den, ok := strings.Cut(strings.TrimSpace(value), "/")
	if !ok {
		return 0
	}
	n, errN := strconv.ParseFloat(num, 64)
	d, errD := strconv.ParseFloat(den, 64)
	if errN != nil || errD != nil || d == 0 {
		return 0
	}
	return n / d
}

// validateStoredVideo logs the probe result. It returns an error only for
// invalid files. A missing ffprobe is logged and allowed through.
func validateStoredVideo(ctx context.Context, path string, matchID uint, videoID uint) (VideoProbeInfo, error) {
	log.Printf("VIDEO_VALIDATION_STARTED match_id=%d video_id=%d", matchID, videoID)
	info, err := probeVideoFile(ctx, path)
	if errors.Is(err, errProbeUnavailable) {
		log.Printf("VIDEO_VALIDATION_SKIPPED match_id=%d video_id=%d reason=ffprobe_missing", matchID, videoID)
		return VideoProbeInfo{}, nil
	}
	if err != nil {
		log.Printf("VIDEO_VALIDATION_FAILED match_id=%d video_id=%d err=%v", matchID, videoID, err)
		return VideoProbeInfo{}, err
	}
	if fi, statErr := os.Stat(path); statErr == nil && info.SizeBytes == 0 {
		info.SizeBytes = fi.Size()
	}
	log.Printf("VIDEO_VALIDATION_COMPLETED match_id=%d video_id=%d duration_sec=%.1f width=%d height=%d fps=%.2f codec=%s bitrate=%d size_bytes=%d",
		matchID, videoID, info.DurationSec, info.Width, info.Height, info.FrameRate, info.Codec, info.BitRate, info.SizeBytes)
	return info, nil
}
