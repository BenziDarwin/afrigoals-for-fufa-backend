package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LineupPlayer struct {
	gorm.Model
	UUID         string  `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	FormationID  uint    `json:"formation_id"`
	PlayerID     uint    `json:"player_id"`
	Name         string  `json:"name"`
	JerseyNumber int     `json:"jersey_number"`
	Position     string  `json:"position"`
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
}

func (lp *LineupPlayer) BeforeCreate(tx *gorm.DB) (err error) {
	lp.UUID = uuid.New().String()
	return
}
