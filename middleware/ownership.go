package middleware

import (
	"afrigoals.com/database"
	"afrigoals.com/models"
)

// Role middleware answers "what kind of user is this?". It does not answer
// "does this user own the thing they are acting on?", so a league admin can
// currently reach another league's data and a club manager another club's.
//
// The helpers below close that gap for the handlers that use them. They are
// deliberately conservative: anything they cannot positively confirm is denied.

// CanManageClub reports whether user may administer clubID.
//
//	afrigoals_admin  any club
//	league admin     clubs affiliated with their league
//	club manager     only their own club
func CanManageClub(user *models.User, clubID uint) bool {
	if user == nil || clubID == 0 {
		return false
	}

	switch user.Role {
	case models.AfrigoalsAdmin:
		return true

	case models.LeagueAdmin:
		if user.LeagueID == nil {
			return false
		}
		// club_leagues is the many2many join table behind Club.Leagues.
		var count int64
		if err := database.DB.
			Table("club_leagues").
			Where("club_id = ? AND league_id = ?", clubID, *user.LeagueID).
			Count(&count).Error; err != nil {
			return false
		}
		return count > 0

	case models.ClubManager:
		return user.ClubID != nil && *user.ClubID == clubID

	default:
		return false
	}
}

// CanManagePlayer reports whether user may administer a player. A player with
// no club can only be managed by a platform admin, because there is no
// club or league to scope ownership against.
func CanManagePlayer(user *models.User, player *models.Player) bool {
	if user == nil || player == nil {
		return false
	}
	if user.Role == models.AfrigoalsAdmin {
		return true
	}
	if player.ClubID == 0 {
		return false
	}
	return CanManageClub(user, player.ClubID)
}
