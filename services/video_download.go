package services

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

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
