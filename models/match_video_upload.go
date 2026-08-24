package models

import (
	"time"

	"gorm.io/datatypes"
)

type MatchVideoUploadStatus string

const (
	MatchVideoUploadStatusInitiated MatchVideoUploadStatus = "initiated"
	MatchVideoUploadStatusUploading MatchVideoUploadStatus = "uploading"
	MatchVideoUploadStatusCompleted MatchVideoUploadStatus = "completed"
	MatchVideoUploadStatusFailed    MatchVideoUploadStatus = "failed"
	MatchVideoUploadStatusCancelled MatchVideoUploadStatus = "cancelled"
)

type MatchVideoUpload struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	MatchID uint `gorm:"index;not null" json:"match_id"`
	UserID  uint `gorm:"index;not null" json:"user_id"`

	FileName      string                 `gorm:"size:255;not null" json:"file_name"`
	StorageKey    string                 `gorm:"size:500;not null;index" json:"storage_key"`
	UploadID      string                 `gorm:"size:500;not null;index" json:"upload_id"`
	FileSize      int64                  `gorm:"not null" json:"file_size"`
	ChunkSize     int64                  `gorm:"not null" json:"chunk_size"`
	TotalParts    int                    `gorm:"not null" json:"total_parts"`
	UploadedParts datatypes.JSONMap      `gorm:"type:jsonb;not null;default:'{}'" json:"uploaded_parts"`
	Status        MatchVideoUploadStatus `gorm:"size:32;not null;default:'initiated';index" json:"status"`
}
