package services

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"afrigoals.com/database"
	"afrigoals.com/models"
)



// ProcessClip generates one clip
func ProcessClip(clipID uint) {


	var clip models.Clip


	// Get clip record
	if err := database.DB.First(&clip, clipID).Error; err != nil {
		return
	}



	// Mark processing

	clip.Status = "processing"

	database.DB.Save(&clip)



	// Ensure output directory exists

	outputDir := "/tmp/clips"

	os.MkdirAll(outputDir, 0755)



	outputFile := filepath.Join(
		outputDir,
		fmt.Sprintf(
			"clip_%d.mp4",
			clip.ID,
		),
	)



	// Get original match video

	videoPath, err := getVideoPath(
		clip.MatchID,
	)


	if err != nil {


		msg := err.Error()


		clip.Status = "failed"

		clip.ErrorMessage = &msg


		database.DB.Save(&clip)


		return
	}



	// Default clip length

	duration := 20



	// FFmpeg command

	cmd := exec.Command(

		"ffmpeg",

		"-y",

		"-ss",

		strconv.Itoa(
			clip.StartSec,
		),


		"-i",

		videoPath,


		"-t",

		strconv.Itoa(duration),


		"-c:v",

		"libx264",


		"-c:a",

		"aac",


		outputFile,
	)



	err = cmd.Run()



	if err != nil {


		msg := err.Error()


		clip.Status = "failed"

		clip.ErrorMessage = &msg


		database.DB.Save(&clip)


		return

	}





	// Upload to Cloudflare R2

	objectKey, url, err := uploadClipFileToR2(

		context.Background(),

		clip.MatchID,

		clip.ID,

		outputFile,

	)



	if err != nil {


		msg := err.Error()


		clip.Status = "failed"

		clip.ErrorMessage = &msg


		database.DB.Save(&clip)


		return

	}




	// Save final URL

	clip.ClipURL = &url
	clip.ObjectKey = &objectKey

	clip.Status = "completed"



	database.DB.Save(&clip)



	// optional cleanup

	os.Remove(outputFile)

}






// Generate video path from MatchVideo database
func getVideoPath(matchID uint) (string,error) {


	/*
	
	IMPORTANT:
	This is temporary.

	Next we replace this with:

	Match
	  |
	MatchVideo
	  |
	R2 URL/path


	*/


	path := fmt.Sprintf(
		"/uploads/matches/%d/video.mp4",
		matchID,
	)



	if _,err:=os.Stat(path);err!=nil{


		return "",
		fmt.Errorf(
			"match video not found: %s",
			path,
		)

	}



	return path,nil

}
