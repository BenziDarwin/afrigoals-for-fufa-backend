package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"afrigoals.com/database"
	"afrigoals.com/models"
)

// ProcessClip generates one clip
func ProcessClip(clipID uint) {

	var clip models.Clip

	if err := database.DB.First(&clip, clipID).Error; err != nil {
		return
	}

	clip.Status = "processing"
	database.DB.Save(&clip)

	outputDir := "/tmp/clips"

	os.MkdirAll(outputDir, 0755)

	outputFile := filepath.Join(
		outputDir,
		fmt.Sprintf(
			"clip_%d.mp4",
			clip.ID,
		),
	)

	// Get original video source

	videoURL, err := getVideoPath(
		clip.MatchID,
	)

	if err != nil {

		failClip(
			&clip,
			err.Error(),
		)

		return
	}

	// Download R2 video locally before FFmpeg

	inputFile := videoURL

	removeInput := false

	if len(videoURL) > 4 &&
		(videoURL[:4] == "http" ||
			videoURL[:5] == "https") {

		inputFile = filepath.Join(
			outputDir,
			fmt.Sprintf(
				"source_%d.mp4",
				clip.ID,
			),
		)

		err := downloadVideo(
			videoURL,
			inputFile,
		)

		if err != nil {

			failClip(
				&clip,
				fmt.Sprintf(
					"download failed: %v",
					err,
				),
			)

			return
		}

		removeInput = true

	}

	defer func() {

		if removeInput {
			os.Remove(inputFile)
		}

	}()

	duration := 20

	cmd := exec.Command(

		"ffmpeg",

		"-y",

		"-ss",

		strconv.Itoa(
			clip.StartSec,
		),

		"-i",

		inputFile,

		"-t",

		strconv.Itoa(duration),

		"-c:v",

		"libx264",

		"-preset",

		"veryfast",

		"-c:a",

		"aac",

		"-movflags",

		"+faststart",

		outputFile,
	)

	output, err := cmd.CombinedOutput()

	if err != nil {

		failClip(
			&clip,
			fmt.Sprintf(
				"ffmpeg failed: %v %s",
				err,
				string(output),
			),
		)

		return
	}

	objectKey, url, err :=
		uploadClipFileToR2(

			context.Background(),

			clip.MatchID,

			clip.ID,

			outputFile,
		)

	if err != nil {

		failClip(
			&clip,
			err.Error(),
		)

		return
	}

	clip.ClipURL = &url

	clip.ObjectKey = &objectKey

	clip.Status = "completed"

	clip.ErrorMessage = nil

	database.DB.Save(&clip)

	os.Remove(outputFile)

}

func failClip(
	clip *models.Clip,
	message string,
) {

	clip.Status = "failed"

	clip.ErrorMessage = &message

	database.DB.Save(clip)

}

func downloadVideo(
	url string,
	output string,
) error {

	client := &http.Client{

		Timeout: 3 * time.Hour,
	}

	resp, err := client.Get(url)

	if err != nil {

		return err

	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {

		return fmt.Errorf(
			"download returned status %d",
			resp.StatusCode,
		)

	}

	file, err := os.Create(output)

	if err != nil {

		return err

	}

	defer file.Close()

	_, err = io.Copy(
		file,
		resp.Body,
	)

	return err
}

// Get original match video location
func getVideoPath(matchID uint) (string, error) {

	var video models.Video


	err := database.DB.
		Where("match_id = ?", matchID).
		First(&video).
		Error


	if err != nil {

		return "",
			fmt.Errorf(
				"match video not found: %v",
				err,
			)
	}



	// Prefer R2 URL

	if video.VideoURL != nil &&
		*video.VideoURL != "" {

		return *video.VideoURL, nil
	}



	if video.URL != "" {

		return video.URL, nil
	}



	return "",
		fmt.Errorf(
			"video URL is empty for match %d",
			matchID,
		)

}