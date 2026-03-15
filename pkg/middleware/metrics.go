package middleware

import (
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	httpRequestsTotal    *prometheus.CounterVec
	httpRequestDuration  *prometheus.HistogramVec
	httpRequestsInFlight prometheus.Gauge

	metricsOnce sync.Once

	// uuidPattern matches UUIDs and numeric IDs in URL paths.
	uuidPattern    = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	numericPattern = regexp.MustCompile(`/\d+`)
)

func initMetrics() {
	metricsOnce.Do(func() {
		httpRequestsTotal = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests.",
			},
			[]string{"method", "path", "status_code"},
		)

		httpRequestDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "Duration of HTTP requests in seconds.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		)

		httpRequestsInFlight = prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "http_requests_in_flight",
				Help: "Number of HTTP requests currently being processed.",
			},
		)

		prometheus.MustRegister(httpRequestsTotal, httpRequestDuration, httpRequestsInFlight)
	})
}

// NormalizePath replaces UUIDs and numeric path segments with :id
// to prevent high-cardinality label values in Prometheus metrics.
func NormalizePath(path string) string {
	path = uuidPattern.ReplaceAllString(path, ":id")
	path = numericPattern.ReplaceAllString(path, "/:id")
	return path
}

// Metrics returns a Gin middleware that records per-request Prometheus metrics.
func Metrics() gin.HandlerFunc {
	initMetrics()

	return func(ctx *gin.Context) {
		start := time.Now()
		path := NormalizePath(ctx.FullPath())
		if path == "" {
			path = NormalizePath(ctx.Request.URL.Path)
		}

		httpRequestsInFlight.Inc()
		ctx.Next()
		httpRequestsInFlight.Dec()

		status := strconv.Itoa(ctx.Writer.Status())
		duration := time.Since(start).Seconds()

		httpRequestsTotal.WithLabelValues(ctx.Request.Method, path, status).Inc()
		httpRequestDuration.WithLabelValues(ctx.Request.Method, path).Observe(duration)
	}
}
