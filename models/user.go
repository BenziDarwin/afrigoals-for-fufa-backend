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

// SelfRegisterableRoles are the roles the public may choose at registration.
// AfrigoalsAdmin is deliberately excluded: platform administrators are seeded
// by InitDB or created by another administrator, never self-assigned.
func SelfRegisterableRoles() []UserRole {
	return []UserRole{LeagueAdmin, ClubManager, DataAnalyst}
}

// IsSelfRegisterable reports whether r may be chosen at registration.
func IsSelfRegisterable(r UserRole) bool {
	for _, allowed := range SelfRegisterableRoles() {
		if r == allowed {
			return true
		}
	}
	return false
}

// IsValidRole reports whether r is a known role, including AfrigoalsAdmin.
// Use this on administrator-only paths such as CreateUser.
func IsValidRole(r UserRole) bool {
	switch r {
	case AfrigoalsAdmin, LeagueAdmin, ClubManager, DataAnalyst:
		return true
	default:
		return false
	}
}

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
