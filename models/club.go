package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Club struct {
	gorm.Model          // Provides ID (uint), CreatedAt, UpdatedAt, DeletedAt
	UUID       string   `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	Name       string   `gorm:"not null" json:"name"`
	TeamType   string   `gorm:"not null;default:club" json:"team_type"`
	Country    string   `json:"country"`
	Players    []Player `gorm:"foreignKey:ClubID" json:"players"`

	// Many-to-many relationship with League
	Leagues []League `gorm:"many2many:club_leagues;" json:"leagues"`
}

func (c *Club) BeforeCreate(tx *gorm.DB) (err error) {
	c.UUID = uuid.New().String()
	return
}
