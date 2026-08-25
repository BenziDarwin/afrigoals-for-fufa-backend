package models

import "time"

type Clip struct {

	ID uint `gorm:"primaryKey" json:"id"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`


	// Match relationship
	MatchID uint `gorm:"index;not null" json:"match_id"`


	// Original uploaded match video
	VideoID *uint `gorm:"index" json:"video_id,omitempty"`


	// Event this clip represents
	EventID *uint `gorm:"index" json:"event_id,omitempty"`


	Title string `gorm:"size:200;not null" json:"title"`


	// Clip timing
	StartSec int `gorm:"not null" json:"start_sec"`

	EndSec *int `json:"end_sec,omitempty"`



	// Cloudflare R2 URL
	ClipURL *string `gorm:"size:500" json:"clip_url,omitempty"`

	// ObjectKey is the canonical storage path for R2-backed clips. ClipURL is
	// derived from it for browser playback.
	ObjectKey *string `gorm:"size:500" json:"object_key,omitempty"`

	// pending -> processing -> completed -> failed
	Status string `gorm:"size:20;default:'pending'" json:"status"`



	ErrorMessage *string `gorm:"size:500" json:"error_message,omitempty"`



	Tags *string `gorm:"size:500" json:"tags,omitempty"`



	CreatedBy uint `gorm:"index;not null" json:"created_by"`
}
