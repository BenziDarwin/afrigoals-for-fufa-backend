package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Match struct {
	gorm.Model
	UUID string `gorm:"type:uuid;uniqueIndex" json:"uuid"`

	HomeClubID uint `json:"home_club_id"`
	HomeClub   Club `gorm:"foreignKey:HomeClubID;references:ID" json:"home_club"`

	AwayClubID uint `json:"away_club_id"`
	AwayClub   Club `gorm:"foreignKey:AwayClubID;references:ID" json:"away_club"`

	LeagueID *uint   `gorm:"index" json:"league_id"`
	League   *League `gorm:"foreignKey:LeagueID;references:ID" json:"league,omitempty"`

	Date      time.Time `json:"date"`
	ScoreHome *int      `json:"score_home,omitempty"`
	ScoreAway *int      `json:"score_away,omitempty"`

	Formations []Formation `gorm:"foreignKey:MatchID" json:"formations"`
	Analysts   []User      `gorm:"many2many:match_analysts;" json:"analysts,omitempty"`

	Events []MatchEvent `gorm:"foreignKey:MatchID" json:"events"`
}

func (m *Match) BeforeCreate(tx *gorm.DB) (err error) {
	m.UUID = uuid.New().String()
	return
}
