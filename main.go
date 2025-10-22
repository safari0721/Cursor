package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// AuthService represents the external authentication microservice
type AuthService struct {
	BaseURL string
}

// User represents user data
type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email,omitempty"`
}

// AuthResponse represents response from auth service
type AuthResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Token   string `json:"token,omitempty"`
}

// PasswordChangeRequest represents password change request
type PasswordChangeRequest struct {
	Username    string `json:"username"`
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

var authService *AuthService

func main() {
	// Initialize auth service with environment variable or default
	authServiceURL := os.Getenv("AUTH_SERVICE_URL")
	if authServiceURL == "" {
		authServiceURL = "http://localhost:8081" // Default auth service URL
	}
	
	authService = &AuthService{BaseURL: authServiceURL}

	// Setup Gin router
	r := gin.Default()
	
	// Load HTML templates
	r.LoadHTMLGlob("templates/*")
	
	// Serve static files
	r.Static("/static", "./static")
	
	// Routes
	r.GET("/", showLoginPage)
	r.GET("/login", showLoginPage)
	r.POST("/login", handleLogin)
	r.GET("/signup", showSignupPage)
	r.POST("/signup", handleSignup)
	r.GET("/change-password", showChangePasswordPage)
	r.POST("/change-password", handleChangePassword)
	r.GET("/dashboard", showDashboard)
	r.POST("/logout", handleLogout)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting auth frontend service on port %s", port)
	log.Printf("Auth service URL: %s", authServiceURL)
	r.Run(":" + port)
}

func showLoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login.html", gin.H{
		"Title": "Login",
	})
}

func showSignupPage(c *gin.Context) {
	c.HTML(http.StatusOK, "signup.html", gin.H{
		"Title": "Sign Up",
	})
}

func showChangePasswordPage(c *gin.Context) {
	c.HTML(http.StatusOK, "change-password.html", gin.H{
		"Title": "Change Password",
	})
}

func showDashboard(c *gin.Context) {
	// Check if user is authenticated (in a real app, verify JWT token)
	token, err := c.Cookie("auth_token")
	if err != nil || token == "" {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	
	c.HTML(http.StatusOK, "dashboard.html", gin.H{
		"Title": "Dashboard",
	})
}

func handleLogin(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	
	if username == "" || password == "" {
		c.HTML(http.StatusBadRequest, "login.html", gin.H{
			"Title": "Login",
			"Error": "Username and password are required",
		})
		return
	}
	
	user := User{
		Username: username,
		Password: password,
	}
	
	response, err := callAuthService("/login", user)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "login.html", gin.H{
			"Title": "Login",
			"Error": "Failed to connect to authentication service",
		})
		return
	}
	
	if response.Success {
		// Set auth token as cookie
		c.SetCookie("auth_token", response.Token, 3600, "/", "", false, true)
		c.Redirect(http.StatusFound, "/dashboard")
	} else {
		c.HTML(http.StatusUnauthorized, "login.html", gin.H{
			"Title": "Login",
			"Error": response.Message,
		})
	}
}

func handleSignup(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")
	email := c.PostForm("email")
	confirmPassword := c.PostForm("confirm_password")
	
	if username == "" || password == "" || email == "" {
		c.HTML(http.StatusBadRequest, "signup.html", gin.H{
			"Title": "Sign Up",
			"Error": "All fields are required",
		})
		return
	}
	
	if password != confirmPassword {
		c.HTML(http.StatusBadRequest, "signup.html", gin.H{
			"Title": "Sign Up",
			"Error": "Passwords do not match",
		})
		return
	}
	
	user := User{
		Username: username,
		Password: password,
		Email:    email,
	}
	
	response, err := callAuthService("/signup", user)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "signup.html", gin.H{
			"Title": "Sign Up",
			"Error": "Failed to connect to authentication service",
		})
		return
	}
	
	if response.Success {
		c.HTML(http.StatusOK, "signup.html", gin.H{
			"Title": "Sign Up",
			"Success": "Account created successfully! You can now login.",
		})
	} else {
		c.HTML(http.StatusBadRequest, "signup.html", gin.H{
			"Title": "Sign Up",
			"Error": response.Message,
		})
	}
}

func handleChangePassword(c *gin.Context) {
	username := c.PostForm("username")
	oldPassword := c.PostForm("old_password")
	newPassword := c.PostForm("new_password")
	confirmPassword := c.PostForm("confirm_password")
	
	if username == "" || oldPassword == "" || newPassword == "" {
		c.HTML(http.StatusBadRequest, "change-password.html", gin.H{
			"Title": "Change Password",
			"Error": "All fields are required",
		})
		return
	}
	
	if newPassword != confirmPassword {
		c.HTML(http.StatusBadRequest, "change-password.html", gin.H{
			"Title": "Change Password",
			"Error": "New passwords do not match",
		})
		return
	}
	
	request := PasswordChangeRequest{
		Username:    username,
		OldPassword: oldPassword,
		NewPassword: newPassword,
	}
	
	response, err := callAuthService("/change-password", request)
	if err != nil {
		c.HTML(http.StatusInternalServerError, "change-password.html", gin.H{
			"Title": "Change Password",
			"Error": "Failed to connect to authentication service",
		})
		return
	}
	
	if response.Success {
		c.HTML(http.StatusOK, "change-password.html", gin.H{
			"Title": "Change Password",
			"Success": "Password changed successfully!",
		})
	} else {
		c.HTML(http.StatusBadRequest, "change-password.html", gin.H{
			"Title": "Change Password",
			"Error": response.Message,
		})
	}
}

func handleLogout(c *gin.Context) {
	c.SetCookie("auth_token", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/login")
}

func callAuthService(endpoint string, data interface{}) (*AuthResponse, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	
	resp, err := http.Post(authService.BaseURL+endpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	var authResponse AuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResponse); err != nil {
		return nil, err
	}
	
	return &authResponse, nil
}