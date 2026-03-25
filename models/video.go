// models/video.go
package models

import "time"

type VideoProvider string

const (
	VideoProviderYouTube VideoProvider = "youtube"
	VideoProviderVimeo   VideoProvider = "vimeo"
	VideoProviderCustom  VideoProvider = "custom"
	VideoProviderUpload  VideoProvider = "upload"
)

type Video struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	MatchID  uint  `gorm:"index;not null" json:"match_id"`
	LeagueID *uint `gorm:"index" json:"league_id,omitempty"`
	ClubID   *uint `gorm:"index" json:"club_id,omitempty"`

	Title        string        `gorm:"size:200;not null" json:"title"`
	Provider     VideoProvider `gorm:"size:50;not null;default:'upload'" json:"provider"`
	URL          string        `gorm:"size:500;not null" json:"url"` // Can be external URL or local path
	ThumbnailURL string        `gorm:"size:500" json:"thumbnail_url"`
	DurationSec  *int          `json:"duration_sec,omitempty"`

	// For uploaded videos
	OriginalFilename *string `gorm:"size:255" json:"original_filename,omitempty"`
	VideoURL         *string `gorm:"size:500" json:"video_url,omitempty"` // Full path for serving

	CreatedBy uint `gorm:"index;not null" json:"created_by"`
}
