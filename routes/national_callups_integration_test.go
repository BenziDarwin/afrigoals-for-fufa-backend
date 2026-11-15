package routes

import (
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"afrigoals.com/database"
	"afrigoals.com/middleware"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Exercise the complete route stack, including JWT and role middleware. All
// changes are rolled back, including nested transactions in the handler.
func TestNationalCallupRouteAuthorization(t *testing.T) {
	dsn := os.Getenv("AFRIGOALS_COUNTRY_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("AFRIGOALS_COUNTRY_TEST_DATABASE_URL is required")
	}
	t.Setenv("JWT_SECRET_KEY", "callup-route-integration-test-secret-32-characters")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	original := database.DB
	database.DB = tx
	defer func() { database.DB = original }()
	var manager models.User
	if err := tx.Joins("JOIN clubs ON clubs.id=users.club_id AND clubs.team_type='national' AND clubs.deleted_at IS NULL").Where("users.role=?", models.ClubManager).First(&manager).Error; err != nil {
		t.Fatal(err)
	}
	var player models.Player
	if err := tx.Where("national_team_id=?", *manager.ClubID).First(&player).Error; err != nil {
		t.Fatal(err)
	}
	claims := middleware.UserClaims{UserUUID: manager.UUID.String(), RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(middleware.DefaultAuthConfig().JWTSecret))
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	SetupRoutes(app)
	path := "/api/v1/clubs/" + fmt.Sprint(*manager.ClubID) + "/call-ups/" + fmt.Sprint(player.ID)
	request := httptest.NewRequest("PUT", path, strings.NewReader(`{"called_up":true}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := app.Test(request, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != 200 {
		t.Fatalf("manager call-up returned %d: %s", response.StatusCode, body)
	}
	var count int64
	if err := tx.Model(&models.NationalCallup{}).Where("team_id=? AND player_id=? AND withdrawn_at IS NULL", *manager.ClubID, player.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one active call-up, got %d", count)
	}
	for _, test := range []struct {
		name       string
		clubID     uint
		authorized bool
		status     int
	}{
		{"another team", player.ClubID, true, fiber.StatusForbidden},
		{"signed out", *manager.ClubID, false, fiber.StatusUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest("PUT", fmt.Sprintf("/api/v1/clubs/%d/call-ups/%d", test.clubID, player.ID), strings.NewReader(`{"called_up":true}`))
			request.Header.Set("Content-Type", "application/json")
			if test.authorized {
				request.Header.Set("Authorization", "Bearer "+token)
			}
			response, err := app.Test(request, -1)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != test.status {
				t.Fatalf("expected %d, got %d", test.status, response.StatusCode)
			}
		})
	}
}
