package models

import "time"

type Clip struct {
    ID        uint      `gorm:"primaryKey" json:"id"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`

    MatchID uint `gorm:"index;not null" json:"match_id"`
    VideoID *uint `gorm:"index" json:"video_id,omitempty"` // clip can reference a parent video

    // Optional: link to match event
    EventID *uint `gorm:"index" json:"event_id,omitempty"`

    Title string `gorm:"size:200;not null" json:"title"`

    // Time range inside the video (seconds)
    StartSec int  `gorm:"not null" json:"start_sec"`
    EndSec   *int `json:"end_sec,omitempty"`

    // Or direct URL to a clipped asset (if you store pre-cut clips)
    ClipURL *string `gorm:"size:500" json:"clip_url,omitempty"`

    Tags *string `gorm:"size:500" json:"tags,omitempty"` // comma-separated or JSON later

    CreatedBy uint `gorm:"index;not null" json:"created_by"`
}
