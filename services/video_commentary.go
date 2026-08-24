package services

import (
	"fmt"
	"os/exec"
	"strings"
)

func extractCommentaryAudioIfPresent(inputPath, outputPath string) (bool, error) {
	hasAudio, err := videoHasAudioTrack(inputPath)
	if err != nil {
		return false, err
	}
	if !hasAudio {
		return false, nil
	}

	cmd := exec.Command(
		"ffmpeg",
		"-y",
		"-i", inputPath,
		"-map", "0:a:0",
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-c:a", "libmp3lame",
		"-b:a", "64k",
		outputPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("ffmpeg commentary extraction failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return true, nil
}

func videoHasAudioTrack(inputPath string) (bool, error) {
	cmd := exec.Command(
		"ffprobe",
		"-v", "error",
		"-select_streams", "a:0",
		"-show_entries", "stream=index",
		"-of", "csv=p=0",
		inputPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("ffprobe audio detection failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)) != "", nil
}
