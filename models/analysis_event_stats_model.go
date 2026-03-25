package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// JSONB type for PostgreSQL JSONB columns
type JSONB map[string]interface{}

// Value implements driver.Valuer interface
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements sql.Scanner interface
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONB)
		return nil
	}
	
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("failed to scan JSONB value")
	}
	
	result := make(JSONB)
	err := json.Unmarshal(bytes, &result)
	*j = result
	return err
}

// AnalysisEventStats stores statistics attached to an analysis event
type AnalysisEventStats struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	UUID             uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();uniqueIndex" json:"uuid"`
	AnalysisEventID  uint      `gorm:"not null;index" json:"analysis_event_id"`
	
	// Player info (should match parent event)
	PlayerID         *uint   `json:"player_id,omitempty"`
	PlayerName       *string `gorm:"size:255" json:"player_name,omitempty"`
	
	// Statistics storage
	Stats            JSONB   `gorm:"type:jsonb;not null;default:'{}'" json:"stats"`
	
	// Denormalized common stats for quick access
	DistanceCoveredM *float64 `gorm:"type:decimal(10,2)" json:"distance_covered_m,omitempty"`
	AverageSpeedKmh  *float64 `gorm:"type:decimal(10,2)" json:"average_speed_kmh,omitempty"`
	MaxSpeedKmh      *float64 `gorm:"type:decimal(10,2)" json:"max_speed_kmh,omitempty"`
	SprintsCount     *int     `json:"sprints_count,omitempty"`
	TouchesCount     *int     `json:"touches_count,omitempty"`
	
	// Metadata
	Source           string  `gorm:"size:100;default:'ai_model'" json:"source"`
	ModelVersion     *string `gorm:"size:50" json:"model_version,omitempty"`
	JobID            *string `gorm:"size:255;index" json:"job_id,omitempty"`
	
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	
	// Relationships
	AnalysisEvent    *AnalysisEvent `gorm:"foreignKey:AnalysisEventID;constraint:OnDelete:CASCADE" json:"analysis_event,omitempty"`
}

// TableName specifies the table name for GORM
func (AnalysisEventStats) TableName() string {
	return "analysis_event_stats"
}

// Update AnalysisEvent model to include stats relationship
// Add this to your existing AnalysisEvent model:
/*
type AnalysisEvent struct {
	// ... existing fields ...
	
	// Add these fields:
	StatsID          *uint                `json:"stats_id,omitempty"`
	Stats            *AnalysisEventStats  `gorm:"foreignKey:StatsID" json:"stats,omitempty"`
}
*/