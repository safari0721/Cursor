package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

type Database struct {
	DB    *sql.DB
	Redis *redis.Client
}

type Config struct {
	PostgresHost     string
	PostgresPort     string
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string
	PostgresSSLMode  string
	RedisHost        string
	RedisPort        string
	RedisPassword    string
	RedisDB          int
	MaxOpenConns     int
	MaxIdleConns     int
	ConnMaxLifetime  time.Duration
}

func LoadConfig() *Config {
	config := &Config{
		PostgresHost:     getEnv("POSTGRES_HOST", "localhost"),
		PostgresPort:     getEnv("POSTGRES_PORT", "5432"),
		PostgresUser:     getEnv("POSTGRES_USER", "auth_user"),
		PostgresPassword: getEnv("POSTGRES_PASSWORD", "auth_password"),
		PostgresDB:       getEnv("POSTGRES_DB", "auth_db"),
		PostgresSSLMode:  getEnv("POSTGRES_SSLMODE", "disable"),
		RedisHost:        getEnv("REDIS_HOST", "localhost"),
		RedisPort:        getEnv("REDIS_PORT", "6379"),
		RedisPassword:    getEnv("REDIS_PASSWORD", ""),
		RedisDB:          getEnvAsInt("REDIS_DB", 0),
		MaxOpenConns:     getEnvAsInt("DB_MAX_OPEN_CONNS", 100),
		MaxIdleConns:     getEnvAsInt("DB_MAX_IDLE_CONNS", 10),
		ConnMaxLifetime:  time.Duration(getEnvAsInt("DB_CONN_MAX_LIFETIME", 3600)) * time.Second,
	}
	return config
}

func NewDatabase(config *Config) (*Database, error) {
	// PostgreSQL connection
	psqlInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		config.PostgresHost, config.PostgresPort, config.PostgresUser,
		config.PostgresPassword, config.PostgresDB, config.PostgresSSLMode)

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool for scalability
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Redis connection for caching and session management
	redisClient := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%s", config.RedisHost, config.RedisPort),
		Password:     config.RedisPassword,
		DB:           config.RedisDB,
		PoolSize:     50,  // Connection pool size for high traffic
		MinIdleConns: 10,  // Minimum idle connections
		MaxRetries:   3,
		RetryDelay:   time.Second,
	})

	// Test Redis connection
	_, err = redisClient.Ping(ctx).Result()
	if err != nil {
		log.Printf("Warning: Redis connection failed: %v", err)
		// Don't fail if Redis is not available, just log warning
	}

	log.Println("Database connections established successfully")
	return &Database{DB: db, Redis: redisClient}, nil
}

func (d *Database) RunMigrations() error {
	driver, err := postgres.WithInstance(d.DB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations",
		"postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("Database migrations completed successfully")
	return nil
}

func (d *Database) Close() error {
	if d.Redis != nil {
		if err := d.Redis.Close(); err != nil {
			log.Printf("Error closing Redis connection: %v", err)
		}
	}
	
	if d.DB != nil {
		if err := d.DB.Close(); err != nil {
			log.Printf("Error closing database connection: %v", err)
			return err
		}
	}
	
	log.Println("Database connections closed")
	return nil
}

func (d *Database) HealthCheck(ctx context.Context) error {
	// Check PostgreSQL
	if err := d.DB.PingContext(ctx); err != nil {
		return fmt.Errorf("postgres health check failed: %w", err)
	}

	// Check Redis
	if d.Redis != nil {
		if _, err := d.Redis.Ping(ctx).Result(); err != nil {
			return fmt.Errorf("redis health check failed: %w", err)
		}
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}