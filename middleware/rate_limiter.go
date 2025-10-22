package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"golang.org/x/time/rate"
)

// RateLimiter represents a rate limiter with Redis backend
type RateLimiter struct {
	redis  *redis.Client
	limit  rate.Limit
	burst  int
	window time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(redisClient *redis.Client, requestsPerSecond float64, burst int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		redis:  redisClient,
		limit:  rate.Limit(requestsPerSecond),
		burst:  burst,
		window: window,
	}
}

// RateLimitMiddleware creates a rate limiting middleware
func (rl *RateLimiter) RateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get client identifier (IP address)
		clientIP := c.ClientIP()
		
		// If Redis is not available, use in-memory rate limiter
		if rl.redis == nil {
			rl.inMemoryRateLimit(c, clientIP)
			return
		}
		
		// Use Redis-based rate limiter for distributed systems
		rl.redisRateLimit(c, clientIP)
	}
}

// inMemoryRateLimit implements in-memory rate limiting
func (rl *RateLimiter) inMemoryRateLimit(c *gin.Context, clientIP string) {
	limiter := rate.NewLimiter(rl.limit, rl.burst)
	
	if !limiter.Allow() {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":   "Rate limit exceeded",
			"message": "Too many requests. Please try again later.",
		})
		c.Abort()
		return
	}
	
	c.Next()
}

// redisRateLimit implements Redis-based distributed rate limiting
func (rl *RateLimiter) redisRateLimit(c *gin.Context, clientIP string) {
	ctx := context.Background()
	key := fmt.Sprintf("rate_limit:%s", clientIP)
	
	// Get current count
	current, err := rl.redis.Get(ctx, key).Int()
	if err != nil && err != redis.Nil {
		// If Redis fails, allow the request
		c.Next()
		return
	}
	
	// Check if limit exceeded
	if current >= rl.burst {
		// Get TTL to provide retry-after header
		ttl, _ := rl.redis.TTL(ctx, key).Result()
		c.Header("Retry-After", strconv.Itoa(int(ttl.Seconds())))
		
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error":      "Rate limit exceeded",
			"message":    "Too many requests. Please try again later.",
			"retry_after": int(ttl.Seconds()),
		})
		c.Abort()
		return
	}
	
	// Increment counter
	pipe := rl.redis.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, rl.window)
	_, err = pipe.Exec(ctx)
	
	if err != nil {
		// If Redis fails, allow the request
		c.Next()
		return
	}
	
	// Add rate limit headers
	c.Header("X-RateLimit-Limit", strconv.Itoa(rl.burst))
	c.Header("X-RateLimit-Remaining", strconv.Itoa(rl.burst-current-1))
	c.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(rl.window).Unix(), 10))
	
	c.Next()
}

// LoginRateLimitMiddleware creates a stricter rate limiter for login attempts
func (rl *RateLimiter) LoginRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		
		if rl.redis == nil {
			// In-memory rate limiting for login (stricter)
			limiter := rate.NewLimiter(rate.Every(time.Minute), 5) // 5 attempts per minute
			if !limiter.Allow() {
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error":   "Login rate limit exceeded",
					"message": "Too many login attempts. Please try again in a minute.",
				})
				c.Abort()
				return
			}
			c.Next()
			return
		}
		
		// Redis-based login rate limiting
		ctx := context.Background()
		key := fmt.Sprintf("login_rate_limit:%s", clientIP)
		
		current, err := rl.redis.Get(ctx, key).Int()
		if err != nil && err != redis.Nil {
			c.Next()
			return
		}
		
		if current >= 5 { // 5 login attempts per minute
			ttl, _ := rl.redis.TTL(ctx, key).Result()
			c.Header("Retry-After", strconv.Itoa(int(ttl.Seconds())))
			
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":      "Login rate limit exceeded",
				"message":    "Too many login attempts. Please try again later.",
				"retry_after": int(ttl.Seconds()),
			})
			c.Abort()
			return
		}
		
		// Increment login attempt counter
		pipe := rl.redis.Pipeline()
		pipe.Incr(ctx, key)
		pipe.Expire(ctx, key, time.Minute)
		pipe.Exec(ctx)
		
		c.Next()
	}
}

// UserSpecificRateLimitMiddleware creates rate limiting per user
func (rl *RateLimiter) UserSpecificRateLimitMiddleware(userID string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rl.redis == nil {
			c.Next()
			return
		}
		
		ctx := context.Background()
		key := fmt.Sprintf("user_rate_limit:%s", userID)
		
		current, err := rl.redis.Get(ctx, key).Int()
		if err != nil && err != redis.Nil {
			c.Next()
			return
		}
		
		// Higher limit for authenticated users
		userLimit := rl.burst * 2
		if current >= userLimit {
			ttl, _ := rl.redis.TTL(ctx, key).Result()
			c.Header("Retry-After", strconv.Itoa(int(ttl.Seconds())))
			
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":      "User rate limit exceeded",
				"message":    "Too many requests. Please try again later.",
				"retry_after": int(ttl.Seconds()),
			})
			c.Abort()
			return
		}
		
		// Increment user-specific counter
		pipe := rl.redis.Pipeline()
		pipe.Incr(ctx, key)
		pipe.Expire(ctx, key, rl.window)
		pipe.Exec(ctx)
		
		c.Header("X-RateLimit-Limit", strconv.Itoa(userLimit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(userLimit-current-1))
		
		c.Next()
	}
}