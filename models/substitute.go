// models/substitute.go
package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Substitute struct {
	gorm.Model
	UUID         string `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	FormationID  uint   `json:"formation_id"`
	PlayerID     uint   `json:"player_id"`
	Name         string `json:"name"`
	JerseyNumber int    `json:"jersey_number"`
	Position     string `json:"position"`
}

func (s *Substitute) BeforeCreate(tx *gorm.DB) (err error) {
	s.UUID = uuid.New().String()
	return
}