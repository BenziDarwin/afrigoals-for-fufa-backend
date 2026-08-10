// models/video_analysis_job.go
package models

import "time"

type VideoAnalysisJobStatus string

const (
	JobQueued     VideoAnalysisJobStatus = "queued"
	JobProcessing VideoAnalysisJobStatus = "processing"
	JobCompleted  VideoAnalysisJobStatus = "completed"
	JobFailed     VideoAnalysisJobStatus = "failed"
)

// models/video_analysis_job.go
type VideoAnalysisJob struct {
    ID                uint                   `gorm:"primaryKey" json:"id"`
    MatchID           uint                   `gorm:"index;not null" json:"match_id"`
    RequestedByUserID *uint                    `json:"requested_by_user_id" gorm:"column:requested_by_user_id"`

    JobID   string                `gorm:"uniqueIndex;size:128;not null" json:"job_id"`
    Status  VideoAnalysisJobStatus `gorm:"index;size:32;not null;default:'queued'" json:"status"`

    OriginalFilename string   `gorm:"size:255" json:"original_filename,omitempty"`
    ModelMessage     string   `gorm:"type:text" json:"model_message,omitempty"` // ✅ rename json tag
    Stage            string   `gorm:"size:128" json:"stage,omitempty"`
    Progress         *float64 `json:"progress,omitempty"`

    ResultJSON         string `gorm:"type:text" json:"result_json,omitempty"`
    AnnotatedVideoPath string `gorm:"type:text" json:"annotated_video,omitempty"` // ✅ tag matches TS
    SiglipHTMLPath     string `gorm:"type:text" json:"siglip_html,omitempty"`      // ✅ tag matches TS

    VideoID    *uint     `gorm:"index" json:"video_id,omitempty"`
    CreatedAt  time.Time `json:"created_at"`
    UpdatedAt  time.Time `json:"updated_at"`
}
