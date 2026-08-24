package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MatchVideo struct {
	gorm.Model
	UUID string `gorm:"type:uuid;uniqueIndex" json:"uuid"`

	MatchID uint  `gorm:"index" json:"match_id"`
	Match   Match `gorm:"foreignKey:MatchID;references:ID" json:"-"`

	OriginalFilename string `json:"original_filename"`
	VideoURL         string `json:"video_url"`
	DurationSec      *int   `json:"duration_sec,omitempty"`
	UploadedBy       *uint  `gorm:"index" json:"uploaded_by,omitempty"`
}

func (v *MatchVideo) BeforeCreate(tx *gorm.DB) (err error) {
	v.UUID = uuid.New().String()
	return
}
