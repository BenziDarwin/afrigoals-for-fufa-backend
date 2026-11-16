package models

import "time"

// TeamMatchReport binds a reviewed report snapshot to one match participant.
type TeamMatchReport struct {
	ID              uint                    `gorm:"primaryKey" json:"id"`
	MatchID         uint                    `gorm:"uniqueIndex:idx_team_match_report;not null" json:"match_id"`
	ClubID          uint                    `gorm:"uniqueIndex:idx_team_match_report;not null" json:"club_id"`
	Status          MatchReportReviewStatus `gorm:"not null;default:draft" json:"status"`
	Summary         string                  `json:"summary"`
	Strengths       string                  `json:"strengths"`
	Weaknesses      string                  `json:"weaknesses"`
	Recommendations string                  `json:"recommendations"`
	Snapshot        JSONB                   `gorm:"type:jsonb;not null" json:"snapshot"`
	AuthoredBy      uint                    `json:"authored_by"`
	SubmittedAt     *time.Time              `json:"submitted_at,omitempty"`
	ReviewedBy      *uint                   `json:"reviewed_by,omitempty"`
	ReviewedAt      *time.Time              `json:"reviewed_at,omitempty"`
	ReviewNotes     string                  `json:"review_notes"`
	DistributedBy   *uint                   `json:"distributed_by,omitempty"`
	DistributedAt   *time.Time              `json:"distributed_at,omitempty"`
	CreatedAt       time.Time               `json:"created_at"`
	UpdatedAt       time.Time               `json:"updated_at"`
}
