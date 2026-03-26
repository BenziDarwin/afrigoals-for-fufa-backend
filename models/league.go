package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type League struct {
	gorm.Model
	UUID        uuid.UUID `gorm:"type:uuid;not null;uniqueIndex" json:"uuid"`
	Name        string    `gorm:"not null" json:"name"`
	Country     string    `json:"country"`
	Description string    `json:"description"`

	Clubs []Club `gorm:"many2many:club_leagues;" json:"clubs"`
	Users []User `gorm:"foreignKey:LeagueID" json:"users"`
}

// Hook to generate UUID before insert
func (l *League) BeforeCreate(tx *gorm.DB) error {
	if l.UUID == uuid.Nil {
		l.UUID = uuid.New()
	}
	return nil
}