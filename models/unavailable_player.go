// models/unavailable_player.go
package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UnavailablePlayer struct {
	gorm.Model
	UUID         string `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	FormationID  uint   `json:"formation_id"`
	PlayerID     uint   `json:"player_id"`
	Name         string `json:"name"`
	JerseyNumber int    `json:"jersey_number"`
	Position     string `json:"position"`
	Reason       string `json:"reason"`  // "injured", "suspended", "personal", "other"
	Details      string `json:"details"` // Additional info
}

func (u *UnavailablePlayer) BeforeCreate(tx *gorm.DB) (err error) {
	u.UUID = uuid.New().String()
	return
}