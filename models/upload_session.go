package models

import "time"

type UploadStatus string

const (
	UploadStatusUploading  UploadStatus = "uploading"
	UploadStatusAssembling UploadStatus = "assembling"
	UploadStatusComplete   UploadStatus = "complete"
	UploadStatusFailed     UploadStatus = "failed"
)

type UploadSession struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	MatchID   uint `gorm:"index;not null" json:"match_id"`
	CreatedBy uint `gorm:"index;not null" json:"created_by"`

	Filename    string `gorm:"size:255;not null" json:"filename"`
	TotalSize   int64  `gorm:"not null" json:"total_size"`
	ChunkSize   int64  `gorm:"not null" json:"chunk_size"`
	TotalChunks int    `gorm:"not null" json:"total_chunks"`

	Status    UploadStatus `gorm:"size:30;not null;default:'uploading';index" json:"status"`
	FinalPath *string      `gorm:"size:500" json:"final_path,omitempty"`
	ErrorMsg  *string      `gorm:"size:500" json:"error_msg,omitempty"`
}