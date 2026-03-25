package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"afrigoals.com/database"
	"afrigoals.com/routes"
	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
)

const MaxBody2GBMinus1 = 2147483647

func main() {
	envFile := flag.String("env", ".env", "env file to load")
	flag.Parse()

	_ = godotenv.Load(*envFile)

	app := fiber.New(fiber.Config{
		ReadBufferSize: 16384,
		BodyLimit:      MaxBody2GBMinus1,
		ReadTimeout:    30 * time.Minute,
		WriteTimeout:   30 * time.Minute,
		IdleTimeout:    30 * time.Minute,

		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			fmt.Printf("Error: %v, Code: %d, Path: %s\n", err, code, c.Path())
			return c.Status(code).JSON(fiber.Map{"error": err.Error()})
		},
	})

	// ✅ IMPORTANT: compute ABSOLUTE path for ./uploads based on current working directory
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("failed to get cwd: %v", err)
	}
	uploadsAbs := filepath.Clean(filepath.Join(cwd, "uploads"))

	// Optional: log it so you see exactly where Fiber is serving from
	log.Printf("CWD: %s", cwd)
	log.Printf("Serving /uploads from: %s", uploadsAbs)

	// Ensure folder exists (won’t hurt if already exists)
	_ = os.MkdirAll(filepath.Join(uploadsAbs, "videos"), 0755)
	_ = os.MkdirAll(filepath.Join(uploadsAbs, "clips"), 0755)

	// ✅ MUST be BEFORE routes.SetupRoutes(app), because your SetupRoutes adds a catch-all 404
	app.Static("/uploads", uploadsAbs)

	database.ConnectDB()
	database.InitDB()

	routes.SetupRoutes(app)

	log.Println("Server starting on port 6767...")
	log.Fatal(app.Listen(":6767"))
}
