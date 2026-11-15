package models

import (
	"gorm.io/gorm"
	"time"
)

type PlayerCareerEntry struct {
	gorm.Model
	PlayerID    uint       `json:"player_id"`
	ClubID      uint       `json:"club_id"`
	Club        Club       `gorm:"foreignKey:ClubID" json:"club"`
	Season      string     `json:"season"`
	Competition string     `json:"competition"`
	StartDate   *time.Time `json:"start_date"`
	EndDate     *time.Time `json:"end_date"`
	Appearances *int       `json:"appearances"`
	Goals       *int       `json:"goals"`
	Assists     *int       `json:"assists"`
	Minutes     *int       `json:"minutes"`
	SourceURL   string     `json:"source_url"`
	Notes       string     `json:"notes"`
	CreatedBy   uint       `json:"created_by"`
	UpdatedBy   uint       `json:"updated_by"`
}

type PlayerHighlight struct {
	gorm.Model
	PlayerID      uint   `json:"player_id"`
	CareerEntryID *uint  `json:"career_entry_id"`
	ClubID        *uint  `json:"club_id"`
	Club          *Club  `gorm:"foreignKey:ClubID" json:"club,omitempty"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	VideoURL      string `json:"video_url"`
	ObjectKey     string `json:"-"`
	ContentType   string `json:"content_type"`
	SizeBytes     int64  `json:"size_bytes"`
	Kind          string `json:"kind"`
	Status        string `json:"status"`
	CreatedBy     uint   `json:"created_by"`
	UpdatedBy     uint   `json:"updated_by"`
}
