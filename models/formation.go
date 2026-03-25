// models/formation.go
package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Formation struct {
	gorm.Model
	UUID      string `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	MatchID   uint   `json:"match_id"`
	ClubID    uint   `json:"club_id"`
	Formation string `json:"formation"`

	Lineup       []LineupPlayer       `gorm:"foreignKey:FormationID;constraint:OnDelete:CASCADE" json:"lineup"`
	Substitutes  []Substitute         `gorm:"foreignKey:FormationID;constraint:OnDelete:CASCADE" json:"substitutes"`
	Unavailable  []UnavailablePlayer  `gorm:"foreignKey:FormationID;constraint:OnDelete:CASCADE" json:"unavailable"`
}

func (f *Formation) BeforeCreate(tx *gorm.DB) (err error) {
	f.UUID = uuid.New().String()
	return
}
