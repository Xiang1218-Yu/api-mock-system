// Package middleware groups the cross-cutting HTTP middlewares. Each middleware
// is a plain gin.HandlerFunc factory, self-contained, and depends only on its
// explicit inputs (no globals) so handlers compose predictably.
package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"api-mock-system/internal/auth"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// CtxKey is the type used to stash values under gin.Context keys.
type CtxKey string

const (
	// UserIDKey holds the authenticated user's id.
	UserIDKey CtxKey = "userID"
	// UserEmailKey holds the authenticated user's email.
	UserEmailKey CtxKey = "userEmail"
)

// CORS adds permissive cross-origin headers for local development. In production
// this should be tightened to known origins; kept permissive here to match the
// dev-first posture of the rest of the codebase.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,PATCH,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization")
		c.Header("Access-Control-Max-Age", "86400")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// Recover traps panics in downstream handlers, logs them, and returns a 500.
func Recover(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("panic recovered",
					zap.Any("recover", r),
					zap.String("path", c.Request.URL.Path),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": "internal server error",
				})
			}
		}()
		c.Next()
	}
}

// Logger records one line per request with method, path, status, and duration.
func Logger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info("http",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Int("size", c.Writer.Size()),
			zap.Duration("duration", time.Since(start)),
		)
	}
}

// RequireAuth validates a Bearer JWT and stashes the user id/email on context.
// Missing or invalid tokens yield 401.
func RequireAuth(a *auth.Auth) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			abortAuth(c, "missing authorization header")
			return
		}
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok {
			abortAuth(c, "authorization header must be 'Bearer <token>'")
			return
		}
		claims, err := a.Parse(strings.TrimSpace(token))
		if err != nil {
			abortAuth(c, "invalid or expired token")
			return
		}
		// The user id is the stable subject. Prefer the uid claim (set on
		// issue) and fall back to the registered subject for tokens minted
		// before uid was populated. Email is stashed only for logging.
		uid := claims.UserID
		if uid == "" {
			uid = claims.Subject
		}
		if uid == "" {
			abortAuth(c, "token has no subject")
			return
		}
		c.Set(string(UserIDKey), uid)
		c.Set(string(UserEmailKey), claims.Email)
		c.Next()
	}
}

func abortAuth(c *gin.Context, msg string) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "message": msg})
}

// current user lookup for member checks is done in services, not here.

// rateBucket is a per-user token-bucket entry. Kept simple: one refill per
// request, capped at the burst size.
type rateBucket struct {
	tokens float64
	last   time.Time
}

// RateLimit is a simple in-memory per-key token bucket. Key is extracted by the
// caller-supplied function (user id, ip, etc.). This replaces a Redis-backed
// limiter for local development.
func RateLimit(rps float64, burst int, keyFn func(*gin.Context) string) gin.HandlerFunc {
	var mu sync.Mutex
	buckets := make(map[string]*rateBucket)
	return func(c *gin.Context) {
		key := keyFn(c)
		if key == "" {
			c.Next()
			return
		}
		mu.Lock()
		now := time.Now()
		b, ok := buckets[key]
		if !ok {
			b = &rateBucket{tokens: float64(burst), last: now}
			buckets[key] = b
		}
		elapsed := now.Sub(b.last).Seconds()
		b.tokens += rps * elapsed
		if b.tokens > float64(burst) {
			b.tokens = float64(burst)
		}
		b.last = now
		if b.tokens < 1 {
			mu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"message": "rate limit exceeded",
			})
			return
		}
		b.tokens--
		mu.Unlock()
		c.Next()
	}
}
