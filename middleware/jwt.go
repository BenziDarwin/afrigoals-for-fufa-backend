package middleware

import (
	"crypto/rand"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"afrigoals.com/database"
	"afrigoals.com/models"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

// minJWTSecretLength is the shortest secret accepted for HS256. A key shorter
// than the 256-bit digest weakens the signature.
const minJWTSecretLength = 32

var (
	jwtSecretOnce sync.Once
	jwtSecret     string
)

// mustJWTSecret returns the configured signing key, terminating the process if
// it is absent or too short. The environment is read once and cached.
func mustJWTSecret() string {
	jwtSecretOnce.Do(func() {
		secret := strings.TrimSpace(os.Getenv("JWT_SECRET_KEY"))
		if secret == "" {
			log.Fatal("JWT_SECRET_KEY is required")
		}
		if len(secret) < minJWTSecretLength {
			log.Fatalf("JWT_SECRET_KEY must be at least %d characters", minJWTSecretLength)
		}
		jwtSecret = secret
	})
	return jwtSecret
}

// MustLoadAuthConfig validates authentication configuration at startup.
//
// Call this from main once the environment is loaded. DefaultAuthConfig runs on
// every request, so an invalid secret has to fail the boot rather than take
// down a server that is already serving traffic.
func MustLoadAuthConfig() {
	mustJWTSecret()
}

// AuthConfig holds configuration for authentication middleware
type AuthConfig struct {
	JWTSecret       string
	CookieName      string
	SessionDuration time.Duration
}

// DefaultAuthConfig returns default configuration
func DefaultAuthConfig() AuthConfig {
	return AuthConfig{
		JWTSecret:       mustJWTSecret(),
		CookieName:      "session_token",
		SessionDuration: 7 * 24 * time.Hour, // 7 days
	}
}

// UserClaims represents JWT claims structure
type UserClaims struct {
	UserID   uint            `json:"user_id"`
	UserUUID string          `json:"user_uuid"`
	Email    string          `json:"email"`
	Role     models.UserRole `json:"role"`
	ClubID   *uint           `json:"club_id,omitempty"`
	LeagueID uint            `json:"league_id"`
	jwt.RegisteredClaims
}

// AuthMiddleware validates JWT tokens for API requests
func AuthMiddleware(config ...AuthConfig) fiber.Handler {
	cfg := DefaultAuthConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	return func(c *fiber.Ctx) error {
		var user models.User
		var authenticated bool

		// Try Bearer token from Authorization header first
		authHeader := c.Get("Authorization")
		if token, ok := strings.CutPrefix(authHeader, "Bearer "); ok && token != "" {
			claims, err := validateToken(token, cfg.JWTSecret)
			if err != nil {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "Invalid or expired token",
				})
			}

			// Fetch user from database
			if err := database.DB.Preload("League").Preload("Club").
				Where("uuid = ?", claims.UserUUID).
				First(&user).Error; err != nil {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "User not found",
				})
			}

			authenticated = true
		}

		// If no Bearer token, try cookie
		if !authenticated {
			token := c.Cookies(cfg.CookieName)
			if token == "" {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "Authentication required",
				})
			}

			claims, err := validateToken(token, cfg.JWTSecret)
			if err != nil {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "Invalid or expired token",
				})
			}

			// Fetch user from database
			if err := database.DB.Preload("League").Preload("Club").
				Where("uuid = ?", claims.UserUUID).
				First(&user).Error; err != nil {
				return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
					"error": "User not found",
				})
			}

			authenticated = true
		}

		if !authenticated {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Authentication required",
			})
		}

		// Save user info in context
		c.Locals("user", user)
		c.Locals("authenticated", true)
		c.Locals("userRole", user.Role)
		c.Locals("userEmail", user.Email)
		c.Locals("userUUID", user.UUID.String())

		return c.Next()
	}
}

// GetUserFromContext retrieves user from Fiber context
func GetUserFromContext(c *fiber.Ctx) (*models.User, error) {
	user, ok := c.Locals("user").(models.User)
	if !ok {
		return nil, fiber.ErrUnauthorized
	}
	return &user, nil
}

// GetUserFromToken retrieves user from JWT token in cookie or bearer header
func GetUserFromToken(c *fiber.Ctx) (*models.User, error) {
	cfg := DefaultAuthConfig()

	// Try Bearer token first
	authHeader := c.Get("Authorization")
	var tokenString string

	if token, ok := strings.CutPrefix(authHeader, "Bearer "); ok && token != "" {
		tokenString = token
	} else {
		tokenString = c.Cookies(cfg.CookieName)
	}

	if tokenString == "" {
		return nil, fiber.ErrUnauthorized
	}

	claims, err := validateToken(tokenString, cfg.JWTSecret)
	if err != nil {
		return nil, fiber.ErrUnauthorized
	}

	var user models.User
	if err := database.DB.Preload("League").Preload("Club").
		Where("uuid = ?", claims.UserUUID).
		First(&user).Error; err != nil {
		return nil, fiber.ErrUnauthorized
	}

	return &user, nil
}

// GetUserClaims extracts user claims from JWT token
func GetUserClaims(c *fiber.Ctx) (*UserClaims, error) {
	cfg := DefaultAuthConfig()

	authHeader := c.Get("Authorization")
	var tokenString string

	if token, ok := strings.CutPrefix(authHeader, "Bearer "); ok && token != "" {
		tokenString = token
	} else {
		tokenString = c.Cookies(cfg.CookieName)
	}

	if tokenString == "" {
		return nil, fiber.ErrUnauthorized
	}

	claims, err := validateToken(tokenString, cfg.JWTSecret)
	if err != nil {
		return nil, fiber.ErrUnauthorized
	}

	return claims, nil
}

// CreateSession creates a new session and returns JWT token
func CreateSession(c *fiber.Ctx, user *models.User) (string, error) {
	cfg := DefaultAuthConfig()

	var leagueID uint
	if user.LeagueID != nil {
		leagueID = *user.LeagueID
	}

	claims := UserClaims{
		UserID:   user.ID,
		UserUUID: user.UUID.String(),
		Email:    user.Email,
		Role:     user.Role,
		ClubID:   user.ClubID,
		LeagueID: leagueID, // 0 allowed for afrigoals_admin
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(cfg.SessionDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		return "", err
	}

	// Session token cookie
	c.Cookie(&fiber.Cookie{
		Name:     cfg.CookieName,
		Value:    tokenString,
		Path:     "/",
		MaxAge:   int(cfg.SessionDuration.Seconds()),
		Secure:   false, // set true with HTTPS in production
		HTTPOnly: true,
		SameSite: "Lax",
	})

	// Role cookie (HTTPOnly so frontend JS can't read; Next middleware can)
	c.Cookie(&fiber.Cookie{
		Name:     "role",
		Value:    string(user.Role),
		Path:     "/",
		MaxAge:   int(cfg.SessionDuration.Seconds()),
		Secure:   false, // set true with HTTPS in production
		HTTPOnly: true,
		SameSite: "Lax",
	})

	return tokenString, nil
}

// ClearSession removes session cookies (session_token + role)
func ClearSession(c *fiber.Ctx) {
	cfg := DefaultAuthConfig()

	c.Cookie(&fiber.Cookie{
		Name:     cfg.CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Secure:   false,
		HTTPOnly: true,
		SameSite: "Lax",
	})

	// ✅ ALSO clear role cookie
	c.Cookie(&fiber.Cookie{
		Name:     "role",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Secure:   false,
		HTTPOnly: true,
		SameSite: "Lax",
	})
}

// validateToken validates JWT token and returns claims
func validateToken(tokenString, secret string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*UserClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrSignatureInvalid
}

// RoleMiddleware checks if the user has one of the required roles
func RoleMiddleware(allowedRoles ...models.UserRole) fiber.Handler {
	return func(c *fiber.Ctx) error {
		user, ok := c.Locals("user").(models.User)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "User not authenticated",
			})
		}

		for _, role := range allowedRoles {
			if user.Role == role {
				return c.Next()
			}
		}

		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Insufficient permissions",
		})
	}
}

// AdminOnly middleware - shorthand for AfrigoalsAdmin role
func AdminOnly() fiber.Handler {
	return RoleMiddleware(models.AfrigoalsAdmin)
}

// LeagueAdminOrAbove middleware - allows league admins and system admins
func LeagueAdminOrAbove() fiber.Handler {
	return RoleMiddleware(models.AfrigoalsAdmin, models.LeagueAdmin)
}

// ClubManagerOrAbove middleware - allows club managers, league admins, and system admins
func ClubManagerOrAbove() fiber.Handler {
	return RoleMiddleware(models.AfrigoalsAdmin, models.LeagueAdmin, models.ClubManager)
}

// ✅ NEW: DataAnalystOnly middleware
func DataAnalystOnly() fiber.Handler {
	return RoleMiddleware(models.DataAnalyst)
}

// DataAnalystOrAbove allows access to data analysts, league admins, and system admins
func DataAnalystOrAbove() fiber.Handler {
	return func(c *fiber.Ctx) error {
		user, err := GetUserFromContext(c)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Unauthorized",
			})
		}

		if user.Role != models.DataAnalyst &&
			user.Role != models.LeagueAdmin &&
			user.Role != models.AfrigoalsAdmin {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Access denied. Data Analyst role or above required.",
			})
		}

		return c.Next()
	}
}

// SecurityHeaders adds security headers
func SecurityHeaders() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Content-Security-Policy", "default-src 'self'")
		return c.Next()
	}
}

// RequestLogger logs incoming requests
func RequestLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()

		_ = time.Since(start)
		_ = c.Response().StatusCode()

		return err
	}
}

// GenerateSecureToken generates a cryptographically secure random token
func GenerateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	const hexChars = "0123456789abcdef"
	result := make([]byte, length*2)
	for i, b := range bytes {
		result[i*2] = hexChars[b>>4]
		result[i*2+1] = hexChars[b&0x0f]
	}
	return string(result), nil
}
