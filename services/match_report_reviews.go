package services

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"afrigoals.com/database"
	"afrigoals.com/middleware"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// getOrCreateMatchReportReview returns the review row for a match, creating a
// default "draft" one on first view - draft reports are invisible to the
// league manager's queue until the analyst explicitly submits.
// idx_match_report_reviews_match_id guarantees at most one row per match, so
// a unique-violation race between two concurrent first-views is resolved by
// simply re-reading the row the other request just created.
func getOrCreateMatchReportReview(matchID uint) (*models.MatchReportReview, error) {
	var review models.MatchReportReview
	err := database.DB.
		Preload("Submitter").
		Preload("Reviewer").
		Preload("Distributor").
		Where(models.MatchReportReview{MatchID: matchID}).
		Attrs(models.MatchReportReview{Status: models.ReportReviewDraft}).
		FirstOrCreate(&review).Error
	if err == nil {
		return &review, nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
		if retryErr := database.DB.
			Preload("Submitter").
			Preload("Reviewer").
			Preload("Distributor").
			Where("match_id = ?", matchID).
			First(&review).Error; retryErr == nil {
			return &review, nil
		}
	}
	return nil, err
}

// GetMatchReportReview returns the league manager review status for a match
// plus the full team-performance breakdown (the same aggregation the
// team-performance PDF uses) for rendering the review UI's KPIs, heatmap,
// and charts.
func GetMatchReportReview(c *fiber.Ctx) error {
	matchID, err := strconv.ParseUint(c.Params("match_id"), 10, 32)
	if err != nil || matchID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid match ID"})
	}

	payload, err := computeTeamPerformanceSummaries(uint(matchID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Match not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	review, err := getOrCreateMatchReportReview(uint(matchID))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to load report review"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"review":           review,
			"team_performance": payload,
		},
	})
}

// SubmitMatchReport moves a match's report from draft (or changes_requested,
// after the analyst has addressed feedback) to submitted, making it visible
// in the league manager's review queue. Gated DataAnalystOrAbove() at the
// route level, same tier as the rest of the analyst write endpoints.
func SubmitMatchReport(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	matchID, err := strconv.ParseUint(c.Params("match_id"), 10, 32)
	if err != nil || matchID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid match ID"})
	}

	payload, err := computeTeamPerformanceSummaries(uint(matchID))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Match not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}
	if payload.TotalReports == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Save at least one player report before submitting to the league manager.",
		})
	}

	review, err := getOrCreateMatchReportReview(uint(matchID))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to load report review"})
	}
	if review.Status != models.ReportReviewDraft && review.Status != models.ReportReviewChangesRequested {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error":  "report has already been submitted",
			"status": review.Status,
		})
	}

	now := time.Now()
	review.Status = models.ReportReviewSubmitted
	review.SubmittedBy = &user.ID
	review.SubmittedAt = &now

	if err := database.DB.Save(review).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save report review"})
	}

	return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"review": review}})
}

// canReviewMatch reports whether user (a league manager or admin) may
// review/distribute the report for match. Scoped by whichever of the match's
// two clubs the caller has authority over.
func canReviewMatch(user *models.User, match *models.Match) bool {
	return middleware.CanManageClub(user, match.HomeClubID) || middleware.CanManageClub(user, match.AwayClubID)
}

// ReviewMatchReport approves or requests changes on a match's report.
// Gated LeagueAdminOrAbove() at the route level; this handler additionally
// scopes the caller to matches in their own league.
func ReviewMatchReport(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	matchID, err := strconv.ParseUint(c.Params("match_id"), 10, 32)
	if err != nil || matchID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid match ID"})
	}

	var match models.Match
	if err := database.DB.First(&match, uint(matchID)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Match not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}
	if !canReviewMatch(user, &match) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You do not manage either club in this match"})
	}

	var req struct {
		Action string `json:"action"`
		Notes  string `json:"notes"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	req.Notes = strings.TrimSpace(req.Notes)

	var nextStatus models.MatchReportReviewStatus
	switch req.Action {
	case "approve":
		nextStatus = models.ReportReviewApproved
	case "request_changes":
		if req.Notes == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "notes are required when requesting changes"})
		}
		nextStatus = models.ReportReviewChangesRequested
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "action must be \"approve\" or \"request_changes\""})
	}

	review, err := getOrCreateMatchReportReview(uint(matchID))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to load report review"})
	}
	if review.Status != models.ReportReviewSubmitted {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error":  "report has not been submitted for review yet",
			"status": review.Status,
		})
	}

	now := time.Now()
	review.Status = nextStatus
	review.ReviewedBy = &user.ID
	review.ReviewedAt = &now
	if req.Notes != "" {
		review.ReviewNotes = &req.Notes
	} else {
		review.ReviewNotes = nil
	}

	if err := database.DB.Save(review).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save report review"})
	}

	return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"review": review}})
}

// DistributeMatchReport marks an approved match report as distributed.
// Gated LeagueAdminOrAbove() at the route level, scoped the same way as
// ReviewMatchReport.
func DistributeMatchReport(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	matchID, err := strconv.ParseUint(c.Params("match_id"), 10, 32)
	if err != nil || matchID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid match ID"})
	}

	var match models.Match
	if err := database.DB.First(&match, uint(matchID)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Match not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}
	if !canReviewMatch(user, &match) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You do not manage either club in this match"})
	}

	var review models.MatchReportReview
	if err := database.DB.Where("match_id = ?", uint(matchID)).First(&review).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "No review found for this match"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}
	if review.Status != models.ReportReviewApproved {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"error":  "report must be approved before it can be distributed",
			"status": review.Status,
		})
	}

	now := time.Now()
	review.Status = models.ReportReviewDistributed
	review.DistributedBy = &user.ID
	review.DistributedAt = &now

	if err := database.DB.Save(&review).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save report review"})
	}

	return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"review": review}})
}

type reportQueueMatch struct {
	MatchID  uint        `json:"match_id"`
	UUID     string      `json:"uuid"`
	HomeClub models.Club `json:"home_club"`
	AwayClub models.Club `json:"away_club"`
	Date     time.Time   `json:"date"`
	Status   string      `json:"status"`
}

// ListLeagueReportQueue returns the matches in a league that need review, and
// the ones already approved but not yet distributed. Gated
// LeagueAdminOrAbove() at the route level plus CanManageLeague scoping.
func ListLeagueReportQueue(c *fiber.Ctx) error {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}

	leagueID, err := strconv.ParseUint(c.Params("league_id"), 10, 32)
	if err != nil || leagueID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid league ID"})
	}
	if !middleware.CanManageLeague(user, uint(leagueID)) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "You do not manage this league"})
	}

	var clubIDs []uint
	if err := database.DB.
		Table("club_leagues").
		Where("league_id = ?", uint(leagueID)).
		Pluck("club_id", &clubIDs).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to resolve league clubs"})
	}

	type row struct {
		models.Match
		ReviewStatus string
	}
	var rows []row
	if len(clubIDs) > 0 {
		if err := database.DB.
			Table("matches").
			Select("matches.*, COALESCE(match_report_reviews.status, 'draft') AS review_status").
			Joins("LEFT JOIN match_report_reviews ON match_report_reviews.match_id = matches.id").
			Where("matches.home_club_id IN ? OR matches.away_club_id IN ?", clubIDs, clubIDs).
			Preload("HomeClub").
			Preload("AwayClub").
			Order("matches.date DESC").
			Find(&rows).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to list league matches"})
		}
	}

	needsReview := []reportQueueMatch{}
	approvedPendingDistribution := []reportQueueMatch{}
	for _, r := range rows {
		item := reportQueueMatch{
			MatchID:  r.Match.ID,
			UUID:     r.Match.UUID,
			HomeClub: r.Match.HomeClub,
			AwayClub: r.Match.AwayClub,
			Date:     r.Match.Date,
			Status:   r.ReviewStatus,
		}
		switch r.ReviewStatus {
		case string(models.ReportReviewApproved):
			approvedPendingDistribution = append(approvedPendingDistribution, item)
		case string(models.ReportReviewSubmitted), string(models.ReportReviewChangesRequested):
			needsReview = append(needsReview, item)
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"league_id":                     leagueID,
			"needs_review":                  needsReview,
			"approved_pending_distribution": approvedPendingDistribution,
		},
	})
}
