package models

import "time"

type NationalCallup struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	TeamID       uint       `json:"team_id"`
	PlayerID     uint       `json:"player_id"`
	CalledBy     uint       `json:"called_by"`
	CalledAt     time.Time  `json:"called_at"`
	JerseyNumber *int       `json:"jersey_number"`
	WithdrawnBy  *uint      `json:"withdrawn_by"`
	WithdrawnAt  *time.Time `json:"withdrawn_at"`
}
