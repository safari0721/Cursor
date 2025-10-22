package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"auth-service/models"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// CreateUser creates a new user with hashed password
func (r *UserRepository) CreateUser(ctx context.Context, req models.CreateUserRequest) (*models.User, error) {
	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Start transaction
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert user
	user := &models.User{}
	query := `
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, username, email, created_at, updated_at, last_login, 
		          is_active, email_verified, failed_login_attempts, locked_until`

	err = tx.QueryRowContext(ctx, query, req.Username, req.Email, string(hashedPassword)).Scan(
		&user.ID, &user.Username, &user.Email, &user.CreatedAt, &user.UpdatedAt,
		&user.LastLogin, &user.IsActive, &user.EmailVerified,
		&user.FailedLoginAttempts, &user.LockedUntil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Create empty profile
	profileQuery := `
		INSERT INTO user_profiles (user_id, preferences, settings)
		VALUES ($1, '{}', '{}')`

	_, err = tx.ExecContext(ctx, profileQuery, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create user profile: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return user, nil
}

// GetUserByUsername retrieves user by username
func (r *UserRepository) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	user := &models.User{}
	query := `
		SELECT id, username, email, password_hash, created_at, updated_at, 
		       last_login, is_active, email_verified, failed_login_attempts, locked_until
		FROM users 
		WHERE username = $1 AND is_active = true`

	err := r.db.QueryRowContext(ctx, query, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash,
		&user.CreatedAt, &user.UpdatedAt, &user.LastLogin, &user.IsActive,
		&user.EmailVerified, &user.FailedLoginAttempts, &user.LockedUntil,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// GetUserByEmail retrieves user by email
func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	user := &models.User{}
	query := `
		SELECT id, username, email, password_hash, created_at, updated_at, 
		       last_login, is_active, email_verified, failed_login_attempts, locked_until
		FROM users 
		WHERE email = $1 AND is_active = true`

	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash,
		&user.CreatedAt, &user.UpdatedAt, &user.LastLogin, &user.IsActive,
		&user.EmailVerified, &user.FailedLoginAttempts, &user.LockedUntil,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// GetUserByID retrieves user by ID
func (r *UserRepository) GetUserByID(ctx context.Context, userID uuid.UUID) (*models.User, error) {
	user := &models.User{}
	query := `
		SELECT id, username, email, password_hash, created_at, updated_at, 
		       last_login, is_active, email_verified, failed_login_attempts, locked_until
		FROM users 
		WHERE id = $1 AND is_active = true`

	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash,
		&user.CreatedAt, &user.UpdatedAt, &user.LastLogin, &user.IsActive,
		&user.EmailVerified, &user.FailedLoginAttempts, &user.LockedUntil,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// UpdatePassword updates user password
func (r *UserRepository) UpdatePassword(ctx context.Context, userID uuid.UUID, newPassword string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	query := `
		UPDATE users 
		SET password_hash = $1, failed_login_attempts = 0, locked_until = NULL
		WHERE id = $2`

	result, err := r.db.ExecContext(ctx, query, string(hashedPassword), userID)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// UpdateLastLogin updates the last login timestamp
func (r *UserRepository) UpdateLastLogin(ctx context.Context, userID uuid.UUID) error {
	query := `
		UPDATE users 
		SET last_login = CURRENT_TIMESTAMP, failed_login_attempts = 0, locked_until = NULL
		WHERE id = $1`

	_, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}

	return nil
}

// IncrementFailedLoginAttempts increments failed login attempts and locks account if necessary
func (r *UserRepository) IncrementFailedLoginAttempts(ctx context.Context, userID uuid.UUID) error {
	// Get current user to check failed attempts
	user, err := r.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	newAttempts := user.FailedLoginAttempts + 1
	var lockUntil *time.Time

	// Check if account should be locked
	if newAttempts >= 5 {
		lockDuration := user.GetLockDuration()
		if lockDuration > 0 {
			lockTime := time.Now().Add(lockDuration)
			lockUntil = &lockTime
		}
	}

	query := `
		UPDATE users 
		SET failed_login_attempts = $1, locked_until = $2
		WHERE id = $3`

	_, err = r.db.ExecContext(ctx, query, newAttempts, lockUntil, userID)
	if err != nil {
		return fmt.Errorf("failed to increment failed login attempts: %w", err)
	}

	return nil
}

// GetUserProfile retrieves user profile by user ID
func (r *UserRepository) GetUserProfile(ctx context.Context, userID uuid.UUID) (*models.UserProfile, error) {
	profile := &models.UserProfile{}
	query := `
		SELECT id, user_id, first_name, last_name, phone, date_of_birth, gender,
		       address, city, state, country, postal_code, profile_picture_url,
		       bio, website, occupation, company, department, job_title,
		       linkedin_url, twitter_url, github_url, preferences, settings,
		       created_at, updated_at
		FROM user_profiles 
		WHERE user_id = $1`

	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&profile.ID, &profile.UserID, &profile.FirstName, &profile.LastName,
		&profile.Phone, &profile.DateOfBirth, &profile.Gender, &profile.Address,
		&profile.City, &profile.State, &profile.Country, &profile.PostalCode,
		&profile.ProfilePictureURL, &profile.Bio, &profile.Website,
		&profile.Occupation, &profile.Company, &profile.Department,
		&profile.JobTitle, &profile.LinkedinURL, &profile.TwitterURL,
		&profile.GithubURL, &profile.Preferences, &profile.Settings,
		&profile.CreatedAt, &profile.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("profile not found")
		}
		return nil, fmt.Errorf("failed to get user profile: %w", err)
	}

	return profile, nil
}

// UpdateUserProfile updates user profile information
func (r *UserRepository) UpdateUserProfile(ctx context.Context, userID uuid.UUID, req models.UpdateProfileRequest) (*models.UserProfile, error) {
	// Parse date of birth if provided
	var dateOfBirth *time.Time
	if req.DateOfBirth != nil && *req.DateOfBirth != "" {
		if dob, err := time.Parse("2006-01-02", *req.DateOfBirth); err == nil {
			dateOfBirth = &dob
		}
	}

	query := `
		UPDATE user_profiles 
		SET first_name = COALESCE($1, first_name),
		    last_name = COALESCE($2, last_name),
		    phone = COALESCE($3, phone),
		    date_of_birth = COALESCE($4, date_of_birth),
		    gender = COALESCE($5, gender),
		    address = COALESCE($6, address),
		    city = COALESCE($7, city),
		    state = COALESCE($8, state),
		    country = COALESCE($9, country),
		    postal_code = COALESCE($10, postal_code),
		    profile_picture_url = COALESCE($11, profile_picture_url),
		    bio = COALESCE($12, bio),
		    website = COALESCE($13, website),
		    occupation = COALESCE($14, occupation),
		    company = COALESCE($15, company),
		    department = COALESCE($16, department),
		    job_title = COALESCE($17, job_title),
		    linkedin_url = COALESCE($18, linkedin_url),
		    twitter_url = COALESCE($19, twitter_url),
		    github_url = COALESCE($20, github_url),
		    preferences = COALESCE($21, preferences),
		    settings = COALESCE($22, settings)
		WHERE user_id = $23
		RETURNING id, user_id, first_name, last_name, phone, date_of_birth, gender,
		          address, city, state, country, postal_code, profile_picture_url,
		          bio, website, occupation, company, department, job_title,
		          linkedin_url, twitter_url, github_url, preferences, settings,
		          created_at, updated_at`

	profile := &models.UserProfile{}
	err := r.db.QueryRowContext(ctx, query,
		req.FirstName, req.LastName, req.Phone, dateOfBirth, req.Gender,
		req.Address, req.City, req.State, req.Country, req.PostalCode,
		req.ProfilePictureURL, req.Bio, req.Website, req.Occupation,
		req.Company, req.Department, req.JobTitle, req.LinkedinURL,
		req.TwitterURL, req.GithubURL, req.Preferences, req.Settings,
		userID,
	).Scan(
		&profile.ID, &profile.UserID, &profile.FirstName, &profile.LastName,
		&profile.Phone, &profile.DateOfBirth, &profile.Gender, &profile.Address,
		&profile.City, &profile.State, &profile.Country, &profile.PostalCode,
		&profile.ProfilePictureURL, &profile.Bio, &profile.Website,
		&profile.Occupation, &profile.Company, &profile.Department,
		&profile.JobTitle, &profile.LinkedinURL, &profile.TwitterURL,
		&profile.GithubURL, &profile.Preferences, &profile.Settings,
		&profile.CreatedAt, &profile.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update user profile: %w", err)
	}

	return profile, nil
}

// GetUserWithProfile retrieves user along with profile information
func (r *UserRepository) GetUserWithProfile(ctx context.Context, userID uuid.UUID) (*models.UserWithProfile, error) {
	user, err := r.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	profile, err := r.GetUserProfile(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &models.UserWithProfile{
		User:    *user,
		Profile: *profile,
	}, nil
}

// ListUsers retrieves paginated list of users (for admin purposes)
func (r *UserRepository) ListUsers(ctx context.Context, page, pageSize int) (*models.PaginatedResponse, error) {
	offset := (page - 1) * pageSize

	// Get total count
	var total int64
	countQuery := `SELECT COUNT(*) FROM users WHERE is_active = true`
	err := r.db.QueryRowContext(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to get user count: %w", err)
	}

	// Get users
	query := `
		SELECT id, username, email, created_at, updated_at, last_login, 
		       is_active, email_verified, failed_login_attempts, locked_until
		FROM users 
		WHERE is_active = true
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := r.db.QueryContext(ctx, query, pageSize, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		err := rows.Scan(
			&user.ID, &user.Username, &user.Email, &user.CreatedAt,
			&user.UpdatedAt, &user.LastLogin, &user.IsActive,
			&user.EmailVerified, &user.FailedLoginAttempts, &user.LockedUntil,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))

	return &models.PaginatedResponse{
		Data:       users,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	}, nil
}

// VerifyPassword verifies if the provided password matches the stored hash
func (r *UserRepository) VerifyPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}