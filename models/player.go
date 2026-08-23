package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Player struct {
	gorm.Model
	UUID   string `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	ClubID uint   `gorm:"index:idx_players_club_jersey,unique,where:deleted_at IS NULL"` // FK to Club ID
	Club   Club   `gorm:"foreignKey:ClubID"`
	Name   string `json:"name"`

	// One shirt number per club. The index is partial because Player is soft
	// deleted: without the WHERE clause a deleted player would reserve their
	// number forever.
	JerseyNumber int         `gorm:"index:idx_players_club_jersey,unique,where:deleted_at IS NULL" json:"jersey_number"`
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
