package models

import (
	"time"

	"github.com/google/uuid"
)

// MatchAIInsight is one AI-answered question about a match, saved so it
// persists as part of the match's performance report and is visible to
// anyone who later opens that report - not just the browser session that
// asked it. Every row in this table is an AI-generated answer; there is no
// analyst/coach-authored equivalent stored here, so the attribution is
// implicit in which table a Q&A pair lives in.
type MatchAIInsight struct {
	ID       uint      `gorm:"primaryKey" json:"id"`
	UUID     uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();uniqueIndex" json:"uuid"`
	MatchID  uint      `gorm:"index;not null" json:"match_id"`
	Question string    `gorm:"type:text;not null" json:"question"`
	Answer   string    `gorm:"type:text;not null" json:"answer"`

	AskedBy *uint `gorm:"index" json:"asked_by,omitempty"`
	Asker   *User `gorm:"foreignKey:AskedBy" json:"asker,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

func (MatchAIInsight) TableName() string { return "match_ai_insights" }
