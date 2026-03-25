package models

import "time"

type MatchAnalyst struct {
	MatchID    uint      `gorm:"primaryKey" json:"match_id"`
	UserID     uint      `gorm:"primaryKey" json:"user_id"`
	AssignedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"assigned_at"`
	AssignedBy uint      `json:"assigned_by"`
	Notes      string    `json:"notes"`
}
