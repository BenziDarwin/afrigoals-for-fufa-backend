package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type MatchReportReviewStatus string

const (
	// ReportReviewDraft is the initial state: the analyst is still tagging
	// the match. Draft reports are invisible to the league manager's queue.
	ReportReviewDraft            MatchReportReviewStatus = "draft"
	ReportReviewSubmitted        MatchReportReviewStatus = "submitted"
	ReportReviewApproved         MatchReportReviewStatus = "approved"
	ReportReviewChangesRequested MatchReportReviewStatus = "changes_requested"
	ReportReviewDistributed      MatchReportReviewStatus = "distributed"
)

// MatchReportReview tracks a match's report through the analyst -> league
// manager workflow: draft -> submitted -> approved -> distributed, with an
// optional submitted -> changes_requested -> submitted loop. Exactly one row
// per match, created lazily on first read rather than at match-creation time.
type MatchReportReview struct {
	ID      uint      `gorm:"primaryKey" json:"id"`
	UUID    uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();uniqueIndex" json:"uuid"`
	MatchID uint      `gorm:"uniqueIndex;not null" json:"match_id"`

	Status MatchReportReviewStatus `gorm:"size:32;not null;default:'draft';index" json:"status"`

	SubmittedBy *uint      `gorm:"index" json:"submitted_by,omitempty"`
	Submitter   *User      `gorm:"foreignKey:SubmittedBy" json:"submitter,omitempty"`
	SubmittedAt *time.Time `json:"submitted_at,omitempty"`

	ReviewedBy  *uint      `gorm:"index" json:"reviewed_by,omitempty"`
	Reviewer    *User      `gorm:"foreignKey:ReviewedBy" json:"reviewer,omitempty"`
	ReviewedAt  *time.Time `json:"reviewed_at,omitempty"`
	ReviewNotes *string    `gorm:"type:text" json:"review_notes,omitempty"`

	DistributedBy *uint      `gorm:"index" json:"distributed_by,omitempty"`
	Distributor   *User      `gorm:"foreignKey:DistributedBy" json:"distributor,omitempty"`
	DistributedAt *time.Time `json:"distributed_at,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (MatchReportReview) TableName() string { return "match_report_reviews" }

func (r *MatchReportReview) BeforeCreate(tx *gorm.DB) error {
	if r.UUID == uuid.Nil {
		r.UUID = uuid.New()
	}
	return nil
}
