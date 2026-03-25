package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserRole string

const (
	AfrigoalsAdmin UserRole = "afrigoals_admin"
	LeagueAdmin    UserRole = "league"
	ClubManager    UserRole = "club_manager"
	DataAnalyst    UserRole = "data_analyst"
)

type User struct {
	ID        uint           `gorm:"primarykey" json:"ID"`
	UUID      uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();unique;not null" json:"UUID"`
	Email     string         `gorm:"uniqueIndex;not null" json:"Email"`
	Password  string         `gorm:"not null" json:"-"`
	Role      UserRole       `gorm:"type:varchar(50);not null" json:"role"`
	ClubID    *uint          `gorm:"index" json:"ClubID,omitempty"`
	Club      *Club          `gorm:"foreignKey:ClubID" json:"Club,omitempty"`
	LeagueID  *uint          `gorm:"index" json:"LeagueID,omitempty"`
	League    *League        `gorm:"foreignKey:LeagueID" json:"League,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}
