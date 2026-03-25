package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PlayerStats struct {
	gorm.Model
	UUID        string `gorm:"type:uuid;uniqueIndex" json:"uuid"`
	PlayerID    uint   `json:"player_id"`
	Goals       int    `json:"goals"`
	Assists     int    `json:"assists"`
	Matches     int    `json:"matches"`
	Minutes     int    `json:"minutes"`
	YellowCards int    `json:"yellow_cards"`
	RedCards    int    `json:"red_cards"`
	Shots       int    `json:"shots"`
	OnTarget    int    `json:"on_target"`
	Passes      int    `json:"passes"`
	Tackles     int    `json:"tackles"`
	Saves       *int   `json:"saves,omitempty"`
	CleanSheets *int   `json:"clean_sheets,omitempty"`
}

func (ps *PlayerStats) BeforeCreate(tx *gorm.DB) (err error) {
	ps.UUID = uuid.New().String()
	return
}
