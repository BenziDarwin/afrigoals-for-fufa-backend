package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Player struct {
	gorm.Model
	UUID         string      `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	ClubID       uint        // FK to Club ID
	Club         Club        `gorm:"foreignKey:ClubID"`
	Name         string      `json:"name"`
	JerseyNumber int         `json:"jersey_number"`
	Position     string      `json:"position"`
	PhotoURL     *string     `json:"photo_url"`
	DateOfBirth  time.Time   `json:"date_of_birth"`
	Nationality  string      `json:"nationality"`
	Stats        PlayerStats `gorm:"foreignKey:PlayerID"`
}

func (p *Player) BeforeCreate(tx *gorm.DB) (err error) {
	p.UUID = uuid.New().String()
	return
}
