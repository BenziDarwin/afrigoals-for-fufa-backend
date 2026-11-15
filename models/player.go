package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Player struct {
	gorm.Model
	UUID                 string `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	ClubID               uint   `gorm:"index:idx_players_club_jersey,unique,where:deleted_at IS NULL AND club_id > 0 AND jersey_number > 0"` // FK to Club ID
	Club                 Club   `gorm:"foreignKey:ClubID"`
	Name                 string `json:"name"`
	NationalTeamID       *uint  `gorm:"index:idx_players_national_jersey,unique,where:deleted_at IS NULL AND national_jersey_number > 0" json:"national_team_id"`
	NationalTeam         *Club  `gorm:"foreignKey:NationalTeamID" json:"national_team,omitempty"`
	NationalJerseyNumber *int   `gorm:"index:idx_players_national_jersey,unique,where:deleted_at IS NULL AND national_jersey_number > 0" json:"national_jersey_number"`
	ClubJerseyNumber     int    `gorm:"-" json:"club_jersey_number"`
	ClubTeam             *Club  `gorm:"-" json:"club_team,omitempty"`

	// One shirt number per club. The index is partial because Player is soft
	// deleted: without the WHERE clause a deleted player would reserve their
	// number forever.
	JerseyNumber int         `gorm:"index:idx_players_club_jersey,unique,where:deleted_at IS NULL AND club_id > 0 AND jersey_number > 0" json:"jersey_number"`
	Position     string      `json:"position"`
	PhotoURL     *string     `json:"photo_url"`
	DateOfBirth  time.Time   `json:"date_of_birth"`
	Nationality  string      `json:"nationality"`
	CountryCode  *string     `json:"country_code"`
	Stats        PlayerStats `gorm:"foreignKey:PlayerID"`
}

func (p *Player) BeforeCreate(tx *gorm.DB) (err error) {
	p.UUID = uuid.New().String()
	return
}
