package middleware

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWTClaims represents the JWT claims
type JWTClaims struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
	jwt.RegisteredClaims
}

// AuthMiddleware represents the authentication middleware
type AuthMiddleware struct {
	jwtSecret []byte
	redis     *redis.Client
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(redisClient *redis.Client) *AuthMiddleware {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-super-secret-jwt-key-change-in-production"
	}
	
	return &AuthMiddleware{
		jwtSecret: []byte(jwtSecret),
		redis:     redisClient,
	}
}

// GenerateToken generates a new JWT token for a user
func (am *AuthMiddleware) GenerateToken(userID uuid.UUID, username, email string) (string, error) {
	claims := JWTClaims{
		UserID:   userID,
		Username: username,
		Email:    email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "auth-service",
			Subject:   userID.String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(am.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	// Store token in Redis for session management (optional)
	if am.redis != nil {
		ctx := context.Background()
		key := fmt.Sprintf("session:%s", userID.String())
		am.redis.Set(ctx, key, tokenString, 24*time.Hour)
	}

	return tokenString, nil
}

// ValidateToken validates a JWT token and returns the claims
func (am *AuthMiddleware) ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return am.jwtSecret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		// Check if token is blacklisted (if using Redis)
		if am.redis != nil {
			ctx := context.Background()
			blacklistKey := fmt.Sprintf("blacklist:%s", tokenString)
			exists, _ := am.redis.Exists(ctx, blacklistKey).Result()
			if exists > 0 {
				return nil, fmt.Errorf("token is blacklisted")
			}
		}
		
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

// RequireAuth middleware that requires authentication
func (am *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Authorization header required",
				"message": "Please provide a valid authentication token",
			})
			c.Abort()
			return
		}

		// Check if header starts with "Bearer "
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Invalid authorization header format",
				"message": "Authorization header must be in format: Bearer <token>",
			})
			c.Abort()
			return
		}

		tokenString := tokenParts[1]

		// Validate token
		claims, err := am.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Invalid token",
				"message": err.Error(),
			})
			c.Abort()
			return
		}

		// Set user information in context
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("email", claims.Email)
		c.Set("token", tokenString)

		c.Next()
	}
}

// OptionalAuth middleware that optionally checks for authentication
func (am *AuthMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.Next()
			return
		}

		tokenString := tokenParts[1]
		claims, err := am.ValidateToken(tokenString)
		if err != nil {
			c.Next()
			return
		}

		// Set user information in context if token is valid
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("email", claims.Email)
		c.Set("token", tokenString)

		c.Next()
	}
}

// RefreshToken generates a new token for an existing valid token
func (am *AuthMiddleware) RefreshToken(oldToken string) (string, error) {
	claims, err := am.ValidateToken(oldToken)
	if err != nil {
		return "", err
	}

	// Generate new token
	newToken, err := am.GenerateToken(claims.UserID, claims.Username, claims.Email)
	if err != nil {
		return "", err
	}

	// Blacklist old token (if using Redis)
	if am.redis != nil {
		ctx := context.Background()
		blacklistKey := fmt.Sprintf("blacklist:%s", oldToken)
		// Set expiration to remaining time of old token
		remainingTime := time.Until(claims.ExpiresAt.Time)
		if remainingTime > 0 {
			am.redis.Set(ctx, blacklistKey, "1", remainingTime)
		}
	}

	return newToken, nil
}

// BlacklistToken adds a token to the blacklist
func (am *AuthMiddleware) BlacklistToken(tokenString string) error {
	if am.redis == nil {
		return nil // Can't blacklist without Redis
	}

	claims, err := am.ValidateToken(tokenString)
	if err != nil {
		return err
	}

	ctx := context.Background()
	blacklistKey := fmt.Sprintf("blacklist:%s", tokenString)
	
	// Set expiration to remaining time of token
	remainingTime := time.Until(claims.ExpiresAt.Time)
	if remainingTime > 0 {
		return am.redis.Set(ctx, blacklistKey, "1", remainingTime).Err()
	}

	return nil
}

// GetUserIDFromContext extracts user ID from gin context
func GetUserIDFromContext(c *gin.Context) (uuid.UUID, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return uuid.Nil, fmt.Errorf("user not authenticated")
	}

	id, ok := userID.(uuid.UUID)
	if !ok {
		return uuid.Nil, fmt.Errorf("invalid user ID format")
	}

	return id, nil
}

// GetUsernameFromContext extracts username from gin context
func GetUsernameFromContext(c *gin.Context) (string, error) {
	username, exists := c.Get("username")
	if !exists {
		return "", fmt.Errorf("user not authenticated")
	}

	name, ok := username.(string)
	if !ok {
		return "", fmt.Errorf("invalid username format")
	}

	return name, nil
}