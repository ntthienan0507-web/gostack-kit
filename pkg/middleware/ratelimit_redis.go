package middleware

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/ntthienan0507-web/gostack-kit/pkg/apperror"
)

// slidingWindowScript is a Lua script implementing a sliding window counter.
// It uses a sorted set where scores are timestamps in milliseconds.
var slidingWindowScript = redis.NewScript(`
local key = KEYS[1]
local window = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

redis.call('ZREMRANGEBYSCORE', key, 0, now - window)

local count = redis.call('ZCARD', key)

if count < limit then
    redis.call('ZADD', key, now, now .. '-' .. math.random(1000000))
    redis.call('PEXPIRE', key, window)
    return {0, limit - count - 1}
else
    local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
    local reset = 0
    if #oldest >= 2 then
        reset = tonumber(oldest[2]) + window
    end
    return {1, 0, reset}
end
`)

// RedisRateLimit enforces distributed rate limiting via Redis sliding window.
// keyFunc extracts the rate limit key from the request (e.g., IP, user ID, endpoint).
// limit is the max requests allowed within the window duration.
func RedisRateLimit(rdb *redis.Client, limit int, window time.Duration, keyFunc func(*gin.Context) string, logger *zap.Logger) gin.HandlerFunc {
	windowMs := window.Milliseconds()

	return func(ctx *gin.Context) {
		key := "ratelimit:" + keyFunc(ctx)
		now := time.Now().UnixMilli()

		result, err := slidingWindowScript.Run(ctx.Request.Context(), rdb, []string{key}, windowMs, limit, now).Int64Slice()
		if err != nil {
			logger.Warn("redis rate limiter unavailable, allowing request", zap.Error(err))
			ctx.Next()
			return
		}

		denied := result[0] == 1
		remaining := result[1]

		ctx.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		ctx.Header("X-RateLimit-Remaining", strconv.FormatInt(remaining, 10))

		if denied {
			resetMs := int64(0)
			if len(result) >= 3 {
				resetMs = result[2]
			}
			resetTime := time.UnixMilli(resetMs)
			retryAfter := time.Until(resetTime).Seconds()
			if retryAfter < 1 {
				retryAfter = 1
			}
			ctx.Header("X-RateLimit-Reset", strconv.FormatInt(resetTime.Unix(), 10))
			ctx.Header("Retry-After", fmt.Sprintf("%.0f", retryAfter))
			apperror.Abort(ctx, apperror.ErrRateLimited)
			return
		}

		resetTime := time.Now().Add(window)
		ctx.Header("X-RateLimit-Reset", strconv.FormatInt(resetTime.Unix(), 10))
		ctx.Next()
	}
}

// KeyByIP returns client IP as rate limit key.
func KeyByIP() func(*gin.Context) string {
	return func(ctx *gin.Context) string {
		return "ip:" + ctx.ClientIP()
	}
}

// KeyByUserID returns authenticated user ID as rate limit key (falls back to IP).
func KeyByUserID() func(*gin.Context) string {
	return func(ctx *gin.Context) string {
		if claims, ok := GetClaims(ctx); ok && claims.UserID != "" {
			return "user:" + claims.UserID
		}
		return "ip:" + ctx.ClientIP()
	}
}

// KeyByEndpoint returns method+path as rate limit key (per-endpoint limiting).
func KeyByEndpoint() func(*gin.Context) string {
	return func(ctx *gin.Context) string {
		return "endpoint:" + ctx.Request.Method + ":" + ctx.FullPath()
	}
}

// KeyByUserAndEndpoint returns userID+method+path for per-user-per-endpoint limiting.
func KeyByUserAndEndpoint() func(*gin.Context) string {
	return func(ctx *gin.Context) string {
		identity := ctx.ClientIP()
		if claims, ok := GetClaims(ctx); ok && claims.UserID != "" {
			identity = claims.UserID
		}
		return "user_endpoint:" + identity + ":" + ctx.Request.Method + ":" + ctx.FullPath()
	}
}
