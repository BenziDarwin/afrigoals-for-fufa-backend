package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type StringSlice []string

func (s StringSlice) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	return json.Marshal(s)
}

func (s *StringSlice) Scan(value interface{}) error {
	if value == nil {
		*s = StringSlice{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("failed to scan StringSlice")
	}
	return json.Unmarshal(bytes, s)
}

type UintSlice []uint

func (s UintSlice) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	return json.Marshal(s)
}

func (s *UintSlice) Scan(value interface{}) error {
	if value == nil {
		*s = UintSlice{}
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return errors.New("failed to scan UintSlice")
	}
	return json.Unmarshal(bytes, s)
}

type PlayerAnalysisReport struct {
	ID                   uint        `gorm:"primaryKey" json:"id"`
	UUID                 string      `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	MatchID              uint        `gorm:"index;not null" json:"match_id"`
	PlayerID             uint        `gorm:"index;not null" json:"player_id"`
	PlayerName           string      `gorm:"size:255;not null" json:"player_name"`
	EventCount           int         `gorm:"not null;default:0" json:"event_count"`
	EventTypes           StringSlice `gorm:"type:jsonb;not null;default:'[]'" json:"event_types"`
	Score                int         `gorm:"not null;default:0" json:"score"`
	LastEventTimeSeconds *float64    `json:"last_event_time_seconds,omitempty"`
	AnalystComment       *string     `gorm:"type:text" json:"analyst_comment,omitempty"`
	ReportText           string      `gorm:"type:text;not null" json:"report_text"`
	EventIDs             UintSlice   `gorm:"type:jsonb;not null;default:'[]'" json:"event_ids"`
	AIStatsKeys          StringSlice `gorm:"type:jsonb;not null;default:'[]'" json:"ai_stats_keys"`
	Metadata             JSONB       `gorm:"type:jsonb;not null;default:'{}'" json:"metadata"`
	CreatedBy            uint        `gorm:"index;not null" json:"created_by"`
	CreatedAt            time.Time   `json:"created_at"`
	UpdatedAt            time.Time   `json:"updated_at"`
}

func (PlayerAnalysisReport) TableName() string {
	return "player_analysis_reports"
}

func (r *PlayerAnalysisReport) BeforeCreate(tx *gorm.DB) error {
	r.UUID = uuid.New().String()
	return nil
}
