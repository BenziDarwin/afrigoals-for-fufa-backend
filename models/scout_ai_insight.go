package models

import "time"

// A pending reservation counts toward the trial until completed, failed or expired.
type ScoutAIInsight struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `gorm:"not null;uniqueIndex:idx_scout_request" json:"-"`
	RequestKey   string    `gorm:"size:64;not null;uniqueIndex:idx_scout_request" json:"request_id"`
	CriteriaHash string    `gorm:"size:64;not null" json:"-"`
	Question     string    `gorm:"type:text;not null" json:"question"`
	Answer       string    `gorm:"type:text;not null;default:''" json:"answer"`
	PlayerIDs    UintSlice `gorm:"type:jsonb;not null;default:'[]'" json:"player_ids"`
	Status       string    `gorm:"size:20;not null" json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
