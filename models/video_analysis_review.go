package models

import "time"

// Reviews are separate from raw inference results so a result refresh cannot
// overwrite analyst corrections. Each save also creates an immutable revision.
type VideoAnalysisReview struct {
	JobID     string    `gorm:"primaryKey;size:128" json:"job_id"`
	MatchID   uint      `json:"match_id"`
	Version   uint      `json:"version"`
	Document  string    `gorm:"type:jsonb" json:"-"`
	UpdatedBy uint      `json:"updated_by"`
	UpdatedAt time.Time `json:"updated_at"`
}

type VideoAnalysisReviewRevision struct {
	JobID     string `gorm:"primaryKey;size:128"`
	Version   uint   `gorm:"primaryKey"`
	Document  string `gorm:"type:jsonb"`
	UpdatedBy uint
	CreatedAt time.Time
}
