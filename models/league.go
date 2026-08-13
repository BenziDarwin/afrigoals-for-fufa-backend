package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type League struct {
	gorm.Model
	UUID        uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();uniqueIndex" json:"uuid"`
	Name        string    `gorm:"not null" json:"name"`
	Country     string    `json:"country"`
	Description string    `json:"description"`

	// Many-to-many relationship with Club
	Clubs []Club `gorm:"many2many:club_leagues;" json:"clubs"`

	// One-to-many relationship with User
	Users []User `gorm:"foreignKey:LeagueID" json:"users"`
}
