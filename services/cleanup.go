// services/cleanup.go
package services

import (
	"log"
	"os"
	"time"

	"afrigoals.com/database"
	"afrigoals.com/models"
)

// StartCleanupScheduler runs cleanup on startup, then periodically.
func StartCleanupScheduler() {
	go func() {
		log.Println("Running initial cleanup of stale uploads...")
		CleanupStaleUploads()

		ticker := time.NewTicker(6 * time.Hour) // adjust as needed
		defer ticker.Stop()

		for range ticker.C {
			log.Println("Running scheduled cleanup of stale uploads...")
			CleanupStaleUploads()
		}
	}()
}

// CleanupStaleUploads marks old sessions failed and deletes temp folders.
func CleanupStaleUploads() {
	cutoff := time.Now().Add(-24 * time.Hour)

	var stale []models.UploadSession
	err := database.DB.
		Where("status IN (?, ?) AND updated_at < ?",
			models.UploadStatusUploading,
			models.UploadStatusAssembling,
			cutoff).
		Find(&stale).Error
	if err != nil {
		log.Printf("CleanupStaleUploads: query error: %v", err)
		return
	}

	cleaned := 0
	for _, sess := range stale {
		// delete temp dir (best-effort)
		_ = os.RemoveAll(uploadTempDir(sess.ID))

		// mark failed
		_ = database.DB.Model(&sess).Updates(map[string]any{
			"status":    models.UploadStatusFailed,
			"error_msg": "stale upload cleanup",
		}).Error

		cleaned++
	}

	if cleaned > 0 {
		log.Printf("CleanupStaleUploads: cleaned %d sessions", cleaned)
	}
}
