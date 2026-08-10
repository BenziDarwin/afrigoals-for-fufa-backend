package database

import (
	"fmt"
	"log"
	"os"

	"afrigoals.com/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func ConnectDB() *gorm.DB {
	// Create variables for default values
	defaultHost := "localhost"
	defaultUser := "postgres"
	defaultPassword := "yourpassword"
	defaultDBName := "registry_db"
	defaultSSLMode := "disable"
	port := getEnv("DB_PORT", nil)

	var dsn string
	if port != "" {
		dsn = fmt.Sprintf(
			"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
			getEnv("DB_HOST", &defaultHost),
			getEnv("DB_USER", &defaultUser),
			getEnv("DB_PASSWORD", &defaultPassword),
			getEnv("DB_NAME", &defaultDBName),
			port,
			getEnv("DB_SSLMODE", &defaultSSLMode),
		)
	} else {
		dsn = fmt.Sprintf(
			"host=%s user=%s password=%s dbname=%s sslmode=%s",
			getEnv("DB_HOST", &defaultHost),
			getEnv("DB_USER", &defaultUser),
			getEnv("DB_PASSWORD", &defaultPassword),
			getEnv("DB_NAME", &defaultDBName),
			getEnv("DB_SSLMODE", &defaultSSLMode),
		)
	}

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to PostgreSQL:", err)
	}

	// Ordered so that a referenced table is always created before the table
	// that references it.
	err = DB.AutoMigrate(
		// Base tables
		&models.League{}, // no FK
		&models.User{},   // FK to League or Club

		// Football domain
		&models.Club{},              // FK to League
		&models.Player{},            // FK to Club
		&models.PlayerStats{},       // FK to Player
		&models.Match{},             // FK to League (optional) or Clubs
		&models.Formation{},         // FK to Match & Club
		&models.LineupPlayer{},      // FK to Formation & Player
		&models.Substitute{},        // FK to Formation & Player
		&models.UnavailablePlayer{}, // FK to Formation & Player
		&models.MatchEvent{},        // FK to Match (and optionally Player & Club)

		// Content
		&models.Article{},    // FK to User (author)
		&models.ArticleTag{}, // FK to Article (optional)

		// Video & analysis
		&models.UploadSession{},
		&models.MatchVideo{},         // FK to Match
		&models.Video{},              // FK to Match
		&models.Clip{},               // FK to Match & Video
		&models.AnalysisEvent{},      // FK to Match & MatchVideo
		&models.AnalysisEventStats{}, // FK to AnalysisEvent
		&models.VideoAnalysisJob{},   // FK to Match & User
	)
	if err != nil {
		log.Fatal("AutoMigrate failed:", err)
	}
	return DB
}

func getEnv(key string, fallback *string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	if fallback != nil {
		return *fallback
	}
	return ""
}
