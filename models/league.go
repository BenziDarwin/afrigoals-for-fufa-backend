package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type League struct {
	gorm.Model
	// gen_random_uuid() is built into PostgreSQL 13+ and is what every other
	// model here uses. uuid_generate_v4() comes from the uuid-ossp extension,
	// which is not enabled by default, so a default of that form makes every
	// league insert fail with "function uuid_generate_v4() does not exist".
	UUID        uuid.UUID `gorm:"type:uuid;not null;default:gen_random_uuid();uniqueIndex" json:"uuid"`
	Name        string    `gorm:"not null" json:"name"`
	Country     string    `json:"country"`
	Description string    `json:"description"`

	// Many-to-many relationship with Club
	Clubs []Club `gorm:"many2many:club_leagues;" json:"clubs"`

	// One-to-many relationship with User
	Users []User `gorm:"foreignKey:LeagueID" json:"users"`
}

// BeforeCreate fills the UUID in Go so that league creation does not depend on
// a database default being present.
func (l *League) BeforeCreate(tx *gorm.DB) error {
	if l.UUID == uuid.Nil {
		l.UUID = uuid.New()
	}
	return nil
}
