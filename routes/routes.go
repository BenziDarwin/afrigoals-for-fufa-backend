// routes/routes.go
package routes

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cache"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/logger"

	"afrigoals.com/middleware"
	"afrigoals.com/services"
)

func SetupRoutes(app *fiber.App) {
	// ========================================
	// GLOBAL MIDDLEWARES
	// ========================================
	app.Use(logger.New())

	// ✅ IMPORTANT: Do NOT compress upload endpoints (chunked PUT/POST)
	app.Use(compress.New(compress.Config{
		Next: func(c *fiber.Ctx) bool {
			p := c.Path()
			// Skip compression for chunked upload endpoints
			if strings.Contains(p, "/videos/upload/") {
				return true
			}
			return false
		},
	}))

	app.Use(middleware.CORS())

	// API v1 Group
	api := app.Group("/api/v1")

	// ========================================
	// PUBLIC ROUTES (No Authentication)
	// ========================================

	// Authentication Routes
	auth := api.Group("/auth")
	{
		auth.Post("/register", services.RegisterUser)
		auth.Post("/login", services.LoginUser)
		auth.Post("/reset-password", services.ResetPasswordRequest)
		auth.Get("/verify/:token", services.VerifyEmail)
	}

	// ========================================
	// PROTECTED ROUTES (Authentication Required)
	// ========================================

	// ✅ IMPORTANT: Never apply cache middleware to the whole protected API group.
	// It can break PUT/POST uploads (especially chunked binary).
	// Instead, apply cache only to safe GET endpoints/groups.

	// Apply authentication middleware (protected routes)
	api.Use(middleware.AuthMiddleware())

	// Optional: Cache middleware instance for safe GET endpoints only
	getCache := cache.New(cache.Config{
		Expiration:   30 * time.Second,
		CacheControl: true,
		Next: func(c *fiber.Ctx) bool {
			// Only cache GET requests
			return c.Method() != fiber.MethodGet
		},
	})

	// Auth Routes (Authenticated Users)
	authProtected := api.Group("/auth")
	{
		authProtected.Get("/me", services.GetCurrentUser)
		authProtected.Post("/logout", services.LogoutUser)
		authProtected.Post("/refresh", services.RefreshToken)
		authProtected.Post("/change-password", services.ChangePassword)
		authProtected.Put("/profile", services.UpdateProfile)
		authProtected.Get("/validate", services.ValidateToken)
	}

	// ========================================
	// USER ROUTES
	// ========================================
	users := api.Group("/users")
	{
		users.Get("/:id", services.GetUserByID)
		users.Get("/uuid/:uuid", services.GetUserByUUID)

		moderatorUsers := users.Group("")
		moderatorUsers.Use(middleware.LeagueAdminOrAbove())
		{
			// ✅ safe GET caching
			moderatorUsers.Get("", getCache, services.ListUsers)
			moderatorUsers.Get("/role/:role", getCache, services.GetUsersByRole)
			moderatorUsers.Get("/league/:league_id", getCache, services.GetUsersByLeague)
			moderatorUsers.Get("/club/:club_id", getCache, services.GetUsersByClub)
		}

		adminUsers := users.Group("")
		adminUsers.Use(middleware.AdminOnly())
		{
			adminUsers.Post("", services.CreateUser)
			adminUsers.Put("/:id", services.UpdateUser)
			adminUsers.Delete("/:id", services.DeleteUser)
		}
	}

	// ========================================
	// LEAGUE ROUTES
	// ========================================
	leagues := api.Group("/leagues")
	{
		leagues.Get("", getCache, services.ListLeagues)
		leagues.Get("/:id", getCache, services.GetLeagueByID)
		leagues.Get("/uuid/:uuid", getCache, services.GetLeagueByUUID)
		leagues.Get("/country/:country", getCache, services.GetLeaguesByCountry)
		leagues.Get("/:league_id/clubs", getCache, services.GetLeagueClubs)
		leagues.Get("/:league_id/users", getCache, services.GetLeagueUsers)
		leagues.Get("/:league_id/statistics", getCache, services.GetLeagueStatistics)

		adminLeagues := leagues.Group("")
		adminLeagues.Use(middleware.LeagueAdminOrAbove())
		{
			adminLeagues.Post("", services.CreateLeague)
			adminLeagues.Put("/:id", services.UpdateLeague)
			adminLeagues.Delete("/:id", services.DeleteLeague)
			adminLeagues.Post("/add-club", services.AddClubToLeague)
			adminLeagues.Post("/remove-club", services.RemoveClubFromLeague)
		}
	}

	// ========================================
	// CLUB ROUTES
	// ========================================
	clubs := api.Group("/clubs")
	{
		clubs.Get("", getCache, services.ListClubs)
		clubs.Get("/:id", getCache, services.GetClubByID)
		clubs.Get("/uuid/:uuid", getCache, services.GetClubByUUID)
		clubs.Get("/league/:league_id", getCache, services.GetClubsByLeague)
		clubs.Get("/:club_id/players", getCache, services.GetClubPlayers)
		clubs.Get("/:club_id/leagues", getCache, services.GetClubLeagues)
		clubs.Get("/:club_id/statistics", getCache, services.GetClubStatistics)

		clubManagerRoutes := clubs.Group("")
		clubManagerRoutes.Use(middleware.ClubManagerOrAbove())
		{
			clubManagerRoutes.Post("/create-player", services.CreateAndAssignPlayerToClub)
		}

		moderatorClubs := clubs.Group("")
		moderatorClubs.Use(middleware.LeagueAdminOrAbove())
		{
			moderatorClubs.Post("", services.CreateClub)
			moderatorClubs.Put("/:id", services.UpdateClub)
			moderatorClubs.Delete("/:id", services.DeleteClub)
			moderatorClubs.Post("/assign-player", services.AssignPlayerToClub)
			moderatorClubs.Delete("/remove-player/:id", services.RemovePlayerFromClub)
		}
	}

	// ========================================
	// PLAYER ROUTES
	// ========================================
	players := api.Group("/players")
	{
		players.Get("", getCache, services.ListPlayers)
		players.Get("/:id", getCache, services.GetPlayerByID)
		players.Get("/uuid/:uuid", getCache, services.GetPlayerByUUID)
		players.Get("/club/:club_id", getCache, services.GetPlayersByClub)
		players.Get("/position/:position", getCache, services.GetPlayersByPosition)
		players.Get("/nationality/:nationality", getCache, services.GetPlayersByNationality)
		players.Get("/free", getCache, services.GetFreePlayers)
		players.Get("/:player_id/statistics", getCache, services.GetPlayerStatistics)

		// Club managers create players through /clubs/create-player, so they
		// must be able to delete them too. DeletePlayer itself verifies the
		// player belongs to a club this user actually manages.
		//
		// The middleware is attached to the route rather than to a group:
		// moderatorPlayers below calls Use() on the same /players prefix, and
		// that applies to everything registered after it, so a grouped version
		// of this route would still be caught by LeagueAdminOrAbove.
		players.Delete("/:id", middleware.ClubManagerOrAbove(), services.DeletePlayer)

		moderatorPlayers := players.Group("")
		moderatorPlayers.Use(middleware.LeagueAdminOrAbove())
		{
			moderatorPlayers.Post("", services.CreatePlayer)
			moderatorPlayers.Put("/:id", services.UpdatePlayer)
			moderatorPlayers.Post("/transfer", services.TransferPlayer)
		}
	}

	// ========================================
	// MATCH ROUTES
	// ========================================
	matches := api.Group("/matches")
	{
		matches.Get("", getCache, services.ListMatches)
		matches.Get("/:id", getCache, services.GetMatchByID)
		matches.Get("/uuid/:uuid", getCache, services.GetMatchByUUID)
		matches.Get("/club/:club_id", getCache, services.GetMatchesByClub)
		matches.Get("/upcoming", getCache, services.GetUpcomingMatches)
		matches.Get("/:match_id/summary", getCache, services.GetMatchSummary)
		matches.Get("/:match_id/events", getCache, services.GetMatchEvents)
		matches.Get("/:match_id/available-analysts", getCache, services.GetAvailableAnalysts)

		moderatorMatches := matches.Group("")
		moderatorMatches.Use(middleware.LeagueAdminOrAbove())
		{
			moderatorMatches.Post("", services.CreateMatch)
			moderatorMatches.Put("/:id", services.UpdateMatch)
			moderatorMatches.Delete("/:id", services.DeleteMatch)
		}
	}

	// ========================================
	// FORMATION ROUTES
	// ========================================
	formations := api.Group("/formations")
	{
		formations.Get("", getCache, services.ListFormations)
		formations.Get("/:id", getCache, services.GetFormationByID)
		formations.Get("/uuid/:uuid", getCache, services.GetFormationByUUID)
		formations.Get("/match/:match_id", getCache, services.GetFormationsByMatch)
		formations.Get("/club/:club_id", getCache, services.GetFormationsByClub)
		formations.Get("/match/:match_id/club/:club_id", getCache, services.GetFormationByMatchAndClub)
	}

	formationsWrite := api.Group("/formations")
	formationsWrite.Use(middleware.ClubManagerOrAbove())
	{
		formationsWrite.Post("", services.CreateFormation)
		formationsWrite.Put("/:id", services.UpdateFormation)
	}

	formationsDelete := api.Group("/formations")
	formationsDelete.Use(middleware.LeagueAdminOrAbove())
	{
		formationsDelete.Delete("/:id", services.DeleteFormation)
	}

	// ========================================
	// LINEUP PLAYER ROUTES
	// ========================================
	lineupPlayers := api.Group("/lineup-players")
	{
		lineupPlayers.Get("/:id", getCache, services.GetLineupPlayer)
	}

	lineupPlayersWrite := api.Group("/lineup-players")
	lineupPlayersWrite.Use(middleware.ClubManagerOrAbove())
	{
		lineupPlayersWrite.Put("/:id", services.UpdateLineupPlayer)
		lineupPlayersWrite.Delete("/:id", services.DeleteLineupPlayer)
	}

	// ========================================
	// MATCH EVENTS ROUTES
	// ========================================
	events := api.Group("/events")
	{
		events.Get("/match/:match_id", getCache, services.GetMatchEvents)

		moderatorEvents := events.Group("")
		moderatorEvents.Use(middleware.LeagueAdminOrAbove())
		{
			moderatorEvents.Post("", services.CreateMatchEvent)
			moderatorEvents.Put("/:id", services.UpdateMatchEvent)
			moderatorEvents.Delete("/:id", services.DeleteMatchEvent)
		}
	}

	// ========================================
	// ANALYST ROUTES
	// ========================================
	analysts := api.Group("/analysts")
	{
		analysts.Get("", getCache, services.GetAllAnalysts)
		analysts.Get("/league/:league_id", getCache, services.GetLeagueAnalysts)
		analysts.Get("/:analyst_id/matches", getCache, services.GetAnalystMatches)
	}

	// ========================================
	// MATCH ANALYST ASSIGNMENT ROUTES
	// ========================================
	matchAnalysts := api.Group("/matches/:match_id/analysts")
	{
		matchAnalysts.Get("", getCache, services.GetMatchAnalysts)

		moderatorAnalysts := matchAnalysts.Group("")
		moderatorAnalysts.Use(middleware.LeagueAdminOrAbove())
		{
			moderatorAnalysts.Post("", services.AssignAnalystToMatch)
			moderatorAnalysts.Delete("", services.RemoveAnalystFromMatch)
		}
	}

	// ========================================
	// VIDEO ROUTES
	// ========================================
	videos := api.Group("/videos")
	{
		videos.Get("", getCache, services.ListVideos)
		videos.Get("/:id", getCache, services.GetVideoByID)

		moderatorVideos := videos.Group("")
		moderatorVideos.Use(middleware.LeagueAdminOrAbove())
		{
			moderatorVideos.Post("", services.CreateVideo)
			moderatorVideos.Put("/:id", services.UpdateVideo)
			moderatorVideos.Delete("/:id", services.DeleteVideo)
		}
	}

	// ========================================
	// CLIP ROUTES
	// ========================================
	clips := api.Group("/clips")
	{
		clips.Get("", getCache, services.ListClips)
		clips.Get("/:id", getCache, services.GetClipByID)

		writerClips := clips.Group("")
		writerClips.Use(middleware.LeagueAdminOrAbove())
		{
			writerClips.Post("", services.CreateClip)
			writerClips.Put("/:id", services.UpdateClip)
			writerClips.Delete("/:id", services.DeleteClip)
		}
	}

	// ========================================
	// ANALYST MATCH ROUTES (Video Analysis Workspace)
	// ========================================
	analystMatches := api.Group("/analyst-matches/:match_id")
	{
		analystMatches.Get("/videos", getCache, services.ListMatchVideos)
		analystMatches.Get("/clips", getCache, services.ListMatchClips)
		analystMatches.Get("/players", getCache, services.GetMatchRosterPlayers)
		analystMatches.Get("/analysis-events", getCache, services.ListAnalysisEvents)
		analystMatches.Get("/analysis-events/:id", getCache, services.GetAnalysisEventByID)
		analystMatches.Get("/player-reports", getCache, services.ListPlayerAnalysisReports)
		analystMatches.Get("/player-reports/:report_id/pdf", services.DownloadPlayerAnalysisReportPDF)
		analystMatches.Get("/team-performance/pdf", services.DownloadTeamPerformanceReportPDF)

		analystMatchesWrite := analystMatches.Group("")
		analystMatchesWrite.Use(middleware.DataAnalystOrAbove())
		{
			// Existing chunked upload endpoints
			analystMatchesWrite.Put("/score", services.UpdateAnalystMatchScore)
			analystMatchesWrite.Post("/videos/upload", services.UploadMatchVideo)
			analystMatchesWrite.Post("/videos/upload/init", services.InitMatchVideoUpload)
			analystMatchesWrite.Put("/videos/upload/:upload_id/chunk", services.UploadMatchVideoChunk)
			analystMatchesWrite.Post("/videos/upload/:upload_id/complete", services.CompleteMatchVideoUpload)
			analystMatchesWrite.Get("/videos/upload/:upload_id/progress", services.GetUploadProgress)
			analystMatchesWrite.Delete("/videos/upload/:upload_id/cancel", services.CancelMatchVideoUpload)

			// NEW: upload to model server + create stored job row in DB
			analystMatchesWrite.Post("/videos/upload-to-models", services.UploadMatchVideoToModelsAndStoreJob)

			// Stored job registry (DB)
			analystMatchesWrite.Post("/video-jobs", services.RegisterStoredMatchVideoJob)
			analystMatchesWrite.Get("/video-jobs", services.ListStoredMatchVideoJobs)
			analystMatchesWrite.Get("/video-jobs/:job_id", services.GetStoredVideoJob)
			analystMatchesWrite.Post("/video-jobs/:job_id/refresh", services.RefreshVideoJobStatus)

			// NEW: fetch result from model server, optionally store in DB + link video
			analystMatchesWrite.Get("/video-jobs/:job_id/result", services.FetchAndStoreVideoJobResult)

			// Video delete
			analystMatchesWrite.Delete("/videos/:video_id", services.DeleteMatchVideo)

			// Clip operations
			analystMatchesWrite.Post("/clips/cut-by-window", services.CutClipByWindow)
			analystMatchesWrite.Delete("/clips/:clip_id", services.DeleteMatchClip)

			// Analysis events
			analystMatchesWrite.Post("/analysis-events", services.CreateAnalysisEvent)
			analystMatchesWrite.Put("/analysis-events/:id", services.UpdateAnalysisEvent)
			analystMatchesWrite.Delete("/analysis-events/:id", services.DeleteAnalysisEvent)

			// Player reports
			analystMatchesWrite.Post("/player-reports", services.SavePlayerAnalysisReport)
			analystMatchesWrite.Delete("/player-reports/:report_id", services.DeletePlayerAnalysisReport)

			// Stats
			analystMatchesWrite.Post("/analysis-events/:event_id/stats", services.AttachStatsToAnalysisEvent)
			analystMatchesWrite.Get("/analysis-events/:event_id/stats", services.GetAnalysisEventStats)
			analystMatchesWrite.Put("/analysis-events/:event_id/stats/:stats_id", services.UpdateAnalysisEventStats)
			analystMatchesWrite.Delete("/analysis-events/:event_id/stats/:stats_id", services.DeleteAnalysisEventStats)

			analystMatchesWrite.Post("/stats/batch-attach", services.BatchAttachStatsFromJobResult)
		}
	}

	// ========================================
	// HEALTH CHECK & API INFO
	// ========================================
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"message": "Afrigoals API is running",
			"version": "1.0.0",
		})
	})

	api.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "Welcome to Afrigoals API",
			"version": "1.0.0",
			"endpoints": fiber.Map{
				"auth":       "/api/v1/auth",
				"users":      "/api/v1/users",
				"leagues":    "/api/v1/leagues",
				"clubs":      "/api/v1/clubs",
				"players":    "/api/v1/players",
				"matches":    "/api/v1/matches",
				"formations": "/api/v1/formations",
				"events":     "/api/v1/events",
				"analysts":   "/api/v1/analysts",
			},
		})
	})

	// ========================================
	// 404 HANDLER
	// ========================================
	app.Use(func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error":   "Route not found",
			"path":    c.Path(),
			"method":  c.Method(),
			"message": "The requested resource does not exist",
		})
	})
}
