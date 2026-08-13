package middleware

import (
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// CORS middleware for API routes with security hardening
func CORS() fiber.Handler {
	// Read allowed origins from environment or config
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS") // e.g., "https://example.com,https://admin.example.com"

	return func(c *fiber.Ctx) error {
		origin := c.Get("Origin")
		if origin == "" {
			return c.Next()
		}

		// Default: block origin
		allowed := false
		for _, o := range strings.Split(allowedOrigins, ",") {
			if strings.TrimSpace(o) == origin {
				allowed = true
				break
			}
		}

		if allowed {
			c.Set("Access-Control-Allow-Origin", origin)
			c.Set("Vary", "Origin")
		}

		c.Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Requested-With")
		c.Set("Access-Control-Expose-Headers", "Content-Disposition, Content-Length, Content-Type")
		c.Set("Access-Control-Allow-Credentials", "true")
		c.Set("Access-Control-Max-Age", "3600") // cache preflight 1h

		// Security Headers (optional but recommended)
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Cross-Origin-Resource-Policy", "same-origin")
		c.Set("Cross-Origin-Opener-Policy", "same-origin")
		c.Set("Cross-Origin-Embedder-Policy", "require-corp")
		c.Set("Permissions-Policy", "geolocation=(), camera=(), microphone=()")

		if c.Method() == fiber.MethodOptions {
			return c.SendStatus(fiber.StatusNoContent)
		}

		return c.Next()
	}
}
