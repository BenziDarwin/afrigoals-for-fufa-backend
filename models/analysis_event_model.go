package models

import (
	"time"

	"github.com/google/uuid"
)

// AnalysisEvent represents a tagged event during match video analysis
// Used by data analysts to mark key moments (goals, shots, fouls, etc.)
type AnalysisEvent struct {
	ID   uint      `gorm:"primaryKey" json:"id"`
	UUID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();uniqueIndex" json:"uuid"`

	// Match and video references
	MatchID uint `gorm:"not null;index" json:"match_id"`
	VideoID uint `gorm:"not null;index" json:"video_id"`

	// Event details
	Type             string  `gorm:"size:50;not null;index" json:"type"` // e.g., "goal", "shot", "cross", "foul"
	TimestampSeconds float64 `gorm:"not null" json:"timestamp_seconds"`  // When event occurred in video

	// Player association (optional - some events are team-level)
	PlayerID   *uint   `gorm:"index" json:"player_id,omitempty"`
	PlayerName *string `gorm:"size:255" json:"player_name,omitempty"`
	CreatedBy  uint    `gorm:"index" json:"created_by"`

	// Additional context
	Notes *string `gorm:"type:text" json:"notes,omitempty"`

	// Associated clip (if generated)
	ClipURL *string `gorm:"size:500" json:"clip_url,omitempty"`

	// Statistics reference (optional)
	StatsID *uint `json:"stats_id,omitempty"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relationships
	Match  *Match              `gorm:"foreignKey:MatchID;constraint:OnDelete:CASCADE" json:"match,omitempty"`
	Video  *Video              `gorm:"foreignKey:VideoID;constraint:OnDelete:CASCADE" json:"video,omitempty"`
	Player *Player             `gorm:"foreignKey:PlayerID" json:"player,omitempty"`
	Stats  *AnalysisEventStats `gorm:"foreignKey:StatsID" json:"stats,omitempty"`
}

// TableName specifies the table name for GORM
func (AnalysisEvent) TableName() string {
	return "analysis_events"
}

// Common event types as constants
const (
	EventTypeGoal         = "goal"
	EventTypeShot         = "shot"
	EventTypeCross        = "cross"
	EventTypeFoul         = "foul"
	EventTypeTackle       = "tackle"
	EventTypeCorner       = "corner"
	EventTypeFreeKick     = "freekick"
	EventTypeOffside      = "offside"
	EventTypeSave         = "save"
	EventTypeYellowCard   = "yellow_card"
	EventTypeRedCard      = "red_card"
	EventTypeSubstitution = "substitution"
	EventTypePenalty      = "penalty"
	EventTypePass         = "pass"
	EventTypeDribble      = "dribble"
)

// ValidEventTypes returns a list of valid event types
func ValidEventTypes() []string {
	return []string{
		EventTypeGoal,
		EventTypeShot,
		EventTypeCross,
		EventTypeFoul,
		EventTypeTackle,
		EventTypeCorner,
		EventTypeFreeKick,
		EventTypeOffside,
		EventTypeSave,
		EventTypeYellowCard,
		EventTypeRedCard,
		EventTypeSubstitution,
		EventTypePenalty,
		EventTypePass,
		EventTypeDribble,
	}
}

// IsValidEventType checks if an event type is valid
func IsValidEventType(eventType string) bool {
	validTypes := ValidEventTypes()
	for _, t := range validTypes {
		if t == eventType {
			return true
		}
	}
	return false
}
