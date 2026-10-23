package models

import "time"

type ScoutCVRequest struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ScoutID   uint      `json:"scout_id"`
	PlayerID  uint      `json:"player_id"`
	ClubID    uint      `json:"club_id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
