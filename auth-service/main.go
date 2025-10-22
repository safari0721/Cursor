package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"auth-service/database"
	"auth-service/middleware"
	"auth-service/models"
	"auth-service/repositories"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Server struct {
	db         *database.Database
	userRepo   *repositories.UserRepository
	authMW     *middleware.AuthMiddleware
	rateLimiter *middleware.RateLimiter
}

func main() {
	// Load database configuration
	config := database.LoadConfig()
	
	// Initialize database connections
	db, err := database.NewDatabase(config)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Run migrations
	if err := db.RunMigrations(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Initialize repositories
	userRepo := repositories.NewUserRepository(db.DB)

	// Initialize middleware
	authMW := middleware.NewAuthMiddleware(db.Redis)
	rateLimiter := middleware.NewRateLimiter(db.Redis, 10, 100, time.Minute) // 100 requests per minute

	// Initialize server
	server := &Server{
		db:          db,
		userRepo:    userRepo,
		authMW:      authMW,
		rateLimiter: rateLimiter,
	}

	// Setup Gin router
	r := gin.Default()

	// Global middleware
	r.Use(gin.Recovery())
	r.Use(server.corsMiddleware())
	r.Use(rateLimiter.RateLimitMiddleware())

	// Health check endpoint
	r.GET("/health", server.handleHealth)

	// Public endpoints
	public := r.Group("/api/v1")
	{
		// Authentication endpoints with stricter rate limiting
		auth := public.Group("/auth")
		auth.Use(rateLimiter.LoginRateLimitMiddleware())
		{
			auth.POST("/signup", server.handleSignup)
			auth.POST("/login", server.handleLogin)
			auth.POST("/refresh", server.handleRefreshToken)
		}

		// Password reset (public but rate limited)
		public.POST("/change-password", rateLimiter.LoginRateLimitMiddleware(), server.handleChangePassword)
	}

	// Protected endpoints
	protected := r.Group("/api/v1")
	protected.Use(authMW.RequireAuth())
	{
		// User profile endpoints
		profile := protected.Group("/profile")
		{
			profile.GET("", server.handleGetProfile)
			profile.PUT("", server.handleUpdateProfile)
		}

		// User management
		user := protected.Group("/user")
		{
			user.GET("/me", server.handleGetCurrentUser)
			user.POST("/logout", server.handleLogout)
		}

		// Admin endpoints (you can add role-based access later)
		admin := protected.Group("/admin")
		{
			admin.GET("/users", server.handleListUsers)
		}
	}

	port := os.Getenv("AUTH_PORT")
	if port == "" {
		port = "8081"
	}

	log.Printf("Starting auth service on port %s", port)
	log.Printf("Database: PostgreSQL")
	log.Printf("Cache: Redis")
	log.Printf("Rate limiting: Enabled")

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func (s *Server) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Expose-Headers", "X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func (s *Server) handleHealth(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Check database health
	if err := s.db.HealthCheck(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "unhealthy",
			"error":   err.Error(),
			"time":    time.Now(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"time":   time.Now(),
		"services": gin.H{
			"database": "connected",
			"redis":    "connected",
		},
	})
}

func (s *Server) handleSignup(c *gin.Context) {
	var req models.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.AuthResponse{
			Success: false,
			Message: "Invalid request format: " + err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create user
	user, err := s.userRepo.CreateUser(ctx, req)
	if err != nil {
		// Check for duplicate username/email
		if err.Error() == "pq: duplicate key value violates unique constraint \"users_username_key\"" {
			c.JSON(http.StatusConflict, models.AuthResponse{
				Success: false,
				Message: "Username already exists",
			})
			return
		}
		if err.Error() == "pq: duplicate key value violates unique constraint \"users_email_key\"" {
			c.JSON(http.StatusConflict, models.AuthResponse{
				Success: false,
				Message: "Email already exists",
			})
			return
		}

		log.Printf("Failed to create user: %v", err)
		c.JSON(http.StatusInternalServerError, models.AuthResponse{
			Success: false,
			Message: "Failed to create user",
		})
		return
	}

	c.JSON(http.StatusCreated, models.AuthResponse{
		Success: true,
		Message: "Account created successfully",
		User:    user,
	})
}

func (s *Server) handleLogin(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.AuthResponse{
			Success: false,
			Message: "Invalid request format: " + err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get user by username
	user, err := s.userRepo.GetUserByUsername(ctx, req.Username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.AuthResponse{
			Success: false,
			Message: "Invalid username or password",
		})
		return
	}

	// Check if account is locked
	if user.IsAccountLocked() {
		c.JSON(http.StatusLocked, models.AuthResponse{
			Success: false,
			Message: "Account is temporarily locked due to multiple failed login attempts",
		})
		return
	}

	// Verify password
	if !s.userRepo.VerifyPassword(user.PasswordHash, req.Password) {
		// Increment failed login attempts
		s.userRepo.IncrementFailedLoginAttempts(ctx, user.ID)
		
		c.JSON(http.StatusUnauthorized, models.AuthResponse{
			Success: false,
			Message: "Invalid username or password",
		})
		return
	}

	// Generate JWT token
	token, err := s.authMW.GenerateToken(user.ID, user.Username, user.Email)
	if err != nil {
		log.Printf("Failed to generate token: %v", err)
		c.JSON(http.StatusInternalServerError, models.AuthResponse{
			Success: false,
			Message: "Failed to generate authentication token",
		})
		return
	}

	// Update last login
	s.userRepo.UpdateLastLogin(ctx, user.ID)

	// Remove password hash from response
	user.PasswordHash = ""

	c.JSON(http.StatusOK, models.AuthResponse{
		Success: true,
		Message: "Login successful",
		Token:   token,
		User:    user,
	})
}

func (s *Server) handleChangePassword(c *gin.Context) {
	var req models.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.AuthResponse{
			Success: false,
			Message: "Invalid request format: " + err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get user by username
	user, err := s.userRepo.GetUserByUsername(ctx, req.Username)
	if err != nil {
		c.JSON(http.StatusNotFound, models.AuthResponse{
			Success: false,
			Message: "User not found",
		})
		return
	}

	// Verify old password
	if !s.userRepo.VerifyPassword(user.PasswordHash, req.OldPassword) {
		c.JSON(http.StatusUnauthorized, models.AuthResponse{
			Success: false,
			Message: "Current password is incorrect",
		})
		return
	}

	// Update password
	if err := s.userRepo.UpdatePassword(ctx, user.ID, req.NewPassword); err != nil {
		log.Printf("Failed to update password: %v", err)
		c.JSON(http.StatusInternalServerError, models.AuthResponse{
			Success: false,
			Message: "Failed to update password",
		})
		return
	}

	c.JSON(http.StatusOK, models.AuthResponse{
		Success: true,
		Message: "Password changed successfully",
	})
}

func (s *Server) handleGetProfile(c *gin.Context) {
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	userWithProfile, err := s.userRepo.GetUserWithProfile(ctx, userID)
	if err != nil {
		log.Printf("Failed to get user profile: %v", err)
		c.JSON(http.StatusInternalServerError, models.ProfileResponse{
			Success: false,
			Message: "Failed to get user profile",
		})
		return
	}

	// Remove password hash
	userWithProfile.User.PasswordHash = ""

	c.JSON(http.StatusOK, models.ProfileResponse{
		Success: true,
		Message: "Profile retrieved successfully",
		Data:    userWithProfile,
	})
}

func (s *Server) handleUpdateProfile(c *gin.Context) {
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	var req models.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ProfileResponse{
			Success: false,
			Message: "Invalid request format: " + err.Error(),
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	profile, err := s.userRepo.UpdateUserProfile(ctx, userID, req)
	if err != nil {
		log.Printf("Failed to update profile: %v", err)
		c.JSON(http.StatusInternalServerError, models.ProfileResponse{
			Success: false,
			Message: "Failed to update profile",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Profile updated successfully",
		"data":    profile,
	})
}

func (s *Server) handleGetCurrentUser(c *gin.Context) {
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		log.Printf("Failed to get user: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to get user information",
		})
		return
	}

	// Remove password hash
	user.PasswordHash = ""

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    user,
	})
}

func (s *Server) handleLogout(c *gin.Context) {
	token, exists := c.Get("token")
	if exists {
		if tokenStr, ok := token.(string); ok {
			// Blacklist the token
			s.authMW.BlacklistToken(tokenStr)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Logged out successfully",
	})
}

func (s *Server) handleRefreshToken(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"message": err.Error(),
		})
		return
	}

	newToken, err := s.authMW.RefreshToken(req.Token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Invalid token",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"token":   newToken,
		"message": "Token refreshed successfully",
	})
}

func (s *Server) handleListUsers(c *gin.Context) {
	// Parse pagination parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := s.userRepo.ListUsers(ctx, page, pageSize)
	if err != nil {
		log.Printf("Failed to list users: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to list users",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}