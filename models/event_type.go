package models

import "time"

type EventPriority string

const (
	EventPriorityCritical EventPriority = "critical"
	EventPriorityHigh     EventPriority = "high"
	EventPriorityMedium   EventPriority = "medium"
	EventPriorityLow      EventPriority = "low"
)

type EventType struct {
	ID                      uint          `gorm:"primaryKey" json:"id"`
	Name                    string        `gorm:"size:120;not null;uniqueIndex" json:"name"`
	Value                   string        `gorm:"size:80;not null;uniqueIndex" json:"value"`
	Category                string        `gorm:"size:80;not null;index" json:"category"`
	Shortcut                string        `gorm:"size:8" json:"shortcut"`
	Priority                EventPriority `gorm:"size:20;not null;default:'low';index" json:"priority"`
	RequiresPlayer          bool          `gorm:"not null;default:false" json:"requires_player"`
	RequiresSecondaryPlayer bool          `gorm:"not null;default:false" json:"requires_secondary_player"`
	CreatedAt               time.Time     `json:"created_at"`
	UpdatedAt               time.Time     `json:"updated_at"`
}

func (EventType) TableName() string {
	return "event_types"
}
