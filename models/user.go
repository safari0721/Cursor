package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// JSONB represents a PostgreSQL JSONB field
type JSONB map[string]interface{}

// Value implements the driver.Valuer interface
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, j)
}

// User represents the main user entity
type User struct {
	ID                   uuid.UUID  `json:"id" db:"id"`
	Username             string     `json:"username" db:"username"`
	Email                string     `json:"email" db:"email"`
	PasswordHash         string     `json:"-" db:"password_hash"`
	CreatedAt            time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at" db:"updated_at"`
	LastLogin            *time.Time `json:"last_login" db:"last_login"`
	IsActive             bool       `json:"is_active" db:"is_active"`
	EmailVerified        bool       `json:"email_verified" db:"email_verified"`
	FailedLoginAttempts  int        `json:"failed_login_attempts" db:"failed_login_attempts"`
	LockedUntil          *time.Time `json:"locked_until" db:"locked_until"`
}

// UserProfile represents extended user profile information
type UserProfile struct {
	ID                uuid.UUID  `json:"id" db:"id"`
	UserID            uuid.UUID  `json:"user_id" db:"user_id"`
	FirstName         *string    `json:"first_name" db:"first_name"`
	LastName          *string    `json:"last_name" db:"last_name"`
	Phone             *string    `json:"phone" db:"phone"`
	DateOfBirth       *time.Time `json:"date_of_birth" db:"date_of_birth"`
	Gender            *string    `json:"gender" db:"gender"`
	Address           *string    `json:"address" db:"address"`
	City              *string    `json:"city" db:"city"`
	State             *string    `json:"state" db:"state"`
	Country           *string    `json:"country" db:"country"`
	PostalCode        *string    `json:"postal_code" db:"postal_code"`
	ProfilePictureURL *string    `json:"profile_picture_url" db:"profile_picture_url"`
	Bio               *string    `json:"bio" db:"bio"`
	Website           *string    `json:"website" db:"website"`
	Occupation        *string    `json:"occupation" db:"occupation"`
	Company           *string    `json:"company" db:"company"`
	Department        *string    `json:"department" db:"department"`
	JobTitle          *string    `json:"job_title" db:"job_title"`
	LinkedinURL       *string    `json:"linkedin_url" db:"linkedin_url"`
	TwitterURL        *string    `json:"twitter_url" db:"twitter_url"`
	GithubURL         *string    `json:"github_url" db:"github_url"`
	Preferences       JSONB      `json:"preferences" db:"preferences"`
	Settings          JSONB      `json:"settings" db:"settings"`
	CreatedAt         time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at" db:"updated_at"`
}

// UserWithProfile combines user and profile information
type UserWithProfile struct {
	User    User        `json:"user"`
	Profile UserProfile `json:"profile"`
}

// CreateUserRequest represents the request to create a new user
type CreateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

// LoginRequest represents the login request
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// ChangePasswordRequest represents the password change request
type ChangePasswordRequest struct {
	Username    string `json:"username" binding:"required"`
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// UpdateProfileRequest represents the profile update request
type UpdateProfileRequest struct {
	FirstName         *string           `json:"first_name"`
	LastName          *string           `json:"last_name"`
	Phone             *string           `json:"phone"`
	DateOfBirth       *string           `json:"date_of_birth"` // Format: YYYY-MM-DD
	Gender            *string           `json:"gender"`
	Address           *string           `json:"address"`
	City              *string           `json:"city"`
	State             *string           `json:"state"`
	Country           *string           `json:"country"`
	PostalCode        *string           `json:"postal_code"`
	ProfilePictureURL *string           `json:"profile_picture_url"`
	Bio               *string           `json:"bio"`
	Website           *string           `json:"website"`
	Occupation        *string           `json:"occupation"`
	Company           *string           `json:"company"`
	Department        *string           `json:"department"`
	JobTitle          *string           `json:"job_title"`
	LinkedinURL       *string           `json:"linkedin_url"`
	TwitterURL        *string           `json:"twitter_url"`
	GithubURL         *string           `json:"github_url"`
	Preferences       map[string]interface{} `json:"preferences"`
	Settings          map[string]interface{} `json:"settings"`
}

// AuthResponse represents authentication response
type AuthResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Token   string `json:"token,omitempty"`
	User    *User  `json:"user,omitempty"`
}

// ProfileResponse represents profile response
type ProfileResponse struct {
	Success bool             `json:"success"`
	Message string           `json:"message"`
	Data    *UserWithProfile `json:"data,omitempty"`
}

// PaginationRequest represents pagination parameters
type PaginationRequest struct {
	Page     int `form:"page,default=1" binding:"min=1"`
	PageSize int `form:"page_size,default=20" binding:"min=1,max=100"`
}

// PaginatedResponse represents a paginated response
type PaginatedResponse struct {
	Data       interface{} `json:"data"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	Total      int64       `json:"total"`
	TotalPages int         `json:"total_pages"`
}

// IsAccountLocked checks if the user account is currently locked
func (u *User) IsAccountLocked() bool {
	if u.LockedUntil == nil {
		return false
	}
	return time.Now().Before(*u.LockedUntil)
}

// ShouldLockAccount determines if account should be locked based on failed attempts
func (u *User) ShouldLockAccount() bool {
	return u.FailedLoginAttempts >= 5 // Lock after 5 failed attempts
}

// GetLockDuration returns the duration for which account should be locked
func (u *User) GetLockDuration() time.Duration {
	switch {
	case u.FailedLoginAttempts >= 10:
		return 24 * time.Hour // 24 hours for 10+ attempts
	case u.FailedLoginAttempts >= 5:
		return 30 * time.Minute // 30 minutes for 5+ attempts
	default:
		return 0
	}
}