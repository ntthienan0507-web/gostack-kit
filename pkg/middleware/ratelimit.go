package middleware

import (
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"

	"github.com/ntthienan0507-web/gostack-kit/pkg/apperror"
)

type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimit returns a middleware that enforces per-IP rate limiting using
// a token bucket algorithm. rps sets the sustained requests per second,
// and burst sets the maximum burst size.
func RateLimit(rps int, burst int) gin.HandlerFunc {
	var mu sync.Mutex
	limiters := make(map[string]*ipLimiter)

	// Background cleanup goroutine — removes entries not seen for 10 minutes.
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			mu.Lock()
			for ip, l := range limiters {
				if time.Since(l.lastSeen) > 10*time.Minute {
					delete(limiters, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(ctx *gin.Context) {
		ip := ctx.ClientIP()

		mu.Lock()
		entry, exists := limiters[ip]
		if !exists {
			entry = &ipLimiter{
				limiter:  rate.NewLimiter(rate.Limit(rps), burst),
				lastSeen: time.Now(),
			}
			limiters[ip] = entry
		}
		entry.lastSeen = time.Now()
		mu.Unlock()

		// Set rate limit headers.
		reservation := entry.limiter.Reserve()
		if !reservation.OK() {
			// This should not happen with valid parameters.
			apperror.Abort(ctx, apperror.ErrRateLimited)
			return
		}

		delay := reservation.Delay()
		if delay > 0 {
			// Token not immediately available — cancel reservation and reject.
			reservation.Cancel()
			retryAfter := delay.Seconds()
			ctx.Header("X-RateLimit-Limit", strconv.Itoa(rps))
			ctx.Header("X-RateLimit-Remaining", "0")
			ctx.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(delay).Unix(), 10))
			ctx.Header("Retry-After", fmt.Sprintf("%.0f", math.Ceil(retryAfter)))
			apperror.Abort(ctx, apperror.ErrRateLimited)
			return
		}

		// Token granted — compute remaining tokens for headers.
		tokens := int(entry.limiter.Tokens())
		if tokens < 0 {
			tokens = 0
		}

		ctx.Header("X-RateLimit-Limit", strconv.Itoa(rps))
		ctx.Header("X-RateLimit-Remaining", strconv.Itoa(tokens))
		ctx.Header("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(time.Second).Unix(), 10))
		ctx.Next()
	}
}
