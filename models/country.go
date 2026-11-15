package models

import "time"

type Country struct {
	Code    string   `gorm:"primaryKey;size:2" json:"code"`
	Name    string   `json:"name"`
	Aliases []string `gorm:"serializer:json;type:jsonb" json:"aliases"`
}

type PlayerClubHistory struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	PlayerID       uint      `json:"player_id"`
	FromClubID     *uint     `json:"from_club_id"`
	ToClubID       *uint     `json:"to_club_id"`
	FromClub       *Club     `gorm:"foreignKey:FromClubID" json:"from_club"`
	ToClub         *Club     `gorm:"foreignKey:ToClubID" json:"to_club"`
	PreviousJersey int       `json:"previous_jersey"`
	NewJersey      int       `json:"new_jersey"`
	TransferredAt  time.Time `json:"transferred_at"`
}
