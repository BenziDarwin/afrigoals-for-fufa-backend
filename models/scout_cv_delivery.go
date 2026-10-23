package models

import "time"

type ScoutCVDelivery struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ScoutID   uint      `json:"-"`
	PlayerID  uint      `json:"player_id"`
	SenderID  uint      `json:"-"`
	CreatedAt time.Time `json:"created_at"`
}
