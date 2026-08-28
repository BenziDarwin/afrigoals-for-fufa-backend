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

	// Match and video references. VideoID is optional: an analyst may tag
	// events before a video has been attached to the match.
	//
	// VideoID holds a videos.id, which is what ListMatchVideos returns and what
	// CutClipByWindow resolves against. It is deliberately not declared as an
	// association: the older model pointed it at MatchVideo, and since videos
	// and match_videos are separate tables with separate id sequences, a foreign
	// key in either direction would reject ids the API itself hands out.
	MatchID uint  `gorm:"not null;index" json:"match_id"`
	VideoID *uint `gorm:"index" json:"video_id,omitempty"`

	// Event details
	Type             string  `gorm:"size:50;not null;index" json:"type"` // e.g., "goal", "shot", "cross", "foul"
	TimestampSeconds float64 `gorm:"not null" json:"timestamp_seconds"`  // When event occurred in video
	EventTypeID      *uint   `gorm:"index" json:"event_type_id,omitempty"`
	TeamID           *uint   `gorm:"index" json:"team_id,omitempty"`

	// Player association (optional - some events are team-level)
	PlayerID          *uint   `gorm:"index" json:"player_id,omitempty"`
	SecondaryPlayerID *uint   `gorm:"index" json:"secondary_player_id,omitempty"`
	PlayerName        *string `gorm:"size:255" json:"player_name,omitempty"`

	// Additional context
	PitchZone       *string  `gorm:"size:80" json:"pitch_zone,omitempty"`
	Outcome         *string  `gorm:"size:80" json:"outcome,omitempty"`
	Notes           *string  `gorm:"type:text" json:"notes,omitempty"`
	ConfidenceScore *float64 `json:"confidence_score,omitempty"`

	// Associated clip (if generated)
	ClipURL *string `gorm:"size:500" json:"clip_url,omitempty"`

	// Most recently attached statistics row. This is a plain pointer column,
	// not an association: AnalysisEventStats.AnalysisEventID is the owning side
	// of the relationship, and declaring both directions as foreign keys makes
	// the two tables circularly dependent and impossible to migrate.
	StatsID *uint `json:"stats_id,omitempty"`

	// Analyst who tagged the event
	CreatedBy uint `gorm:"index;not null" json:"created_by"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relationships
	//
	// Stats is declared as a has-one against AnalysisEventStats.AnalysisEventID
	// rather than as a belongs-to against StatsID. Both directions would make
	// the two tables reference each other and AutoMigrate cannot order a cycle,
	// so this keeps Preload("Stats") working while leaving the only foreign key
	// on the child, where it already exists.
	Match     *Match              `gorm:"foreignKey:MatchID;constraint:OnDelete:CASCADE" json:"match,omitempty"`
	Player    *Player             `gorm:"foreignKey:PlayerID" json:"player,omitempty"`
	EventType *EventType          `gorm:"foreignKey:EventTypeID" json:"event_type,omitempty"`
	Stats     *AnalysisEventStats `gorm:"foreignKey:AnalysisEventID;references:ID;constraint:OnDelete:CASCADE" json:"stats,omitempty"`
}

// TableName specifies the table name for GORM
func (AnalysisEvent) TableName() string {
	return "analysis_events"
}

// Common event types as constants
const (
	EventTypeGoal           = "goal"
	EventTypeGoalDisallowed = "goal_disallowed"
	EventTypeShot           = "shot"
	EventTypeCross          = "cross"
	EventTypeFoul           = "foul"
	EventTypeTackle         = "tackle"
	EventTypeCorner         = "corner"
	EventTypeFreeKick       = "freekick"
	EventTypeOffside        = "offside"
	EventTypeSave           = "save"
	EventTypeYellowCard     = "yellow_card"
	EventTypeRedCard        = "red_card"
	EventTypeSubstitution   = "substitution"
	EventTypePenalty        = "penalty"
	EventTypePass           = "pass"
	EventTypeDribble        = "dribble"
)

// ValidEventTypes returns a list of valid event types
func ValidEventTypes() []string {
	return []string{
		EventTypeGoal,
		EventTypeGoalDisallowed,
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
