package models

import (
	"time"

	"github.com/google/uuid"
)

type MatchEvent struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UUID      uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();uniqueIndex" json:"uuid"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	MatchID uint `gorm:"index;not null" json:"match_id"`

	// Your service uses Team as *string in request; keep it nullable.
	// Typical values: "home", "away" or club name; your choice.
	Team *string `gorm:"size:50;index" json:"team,omitempty"`

	// Your service uses Player as *string in request; keep it nullable.
	Player *string `gorm:"size:120" json:"player,omitempty"`

	JerseyNumber *int `json:"jersey_number,omitempty"`

	// Your service uses Type as string in request.
	// Examples: "goal", "yellow_card", "red_card", "substitution"
	Type string `gorm:"size:50;index;not null" json:"type"`

	Minute int `gorm:"not null" json:"minute"`

	// In your current request it's string, not pointer; keep it required or allow empty.
	Description string `gorm:"size:500" json:"description"`
}
