package metrics

import (
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// DBQueryDuration tracks database query durations by operation.
	DBQueryDuration *prometheus.HistogramVec

	// CacheMisses counts cache miss events.
	CacheMisses prometheus.Counter

	// ExternalRequestDuration tracks external HTTP call durations by service.
	ExternalRequestDuration *prometheus.HistogramVec

	once sync.Once
)

func init() {
	initMetrics()
}

func initMetrics() {
	once.Do(func() {
		DBQueryDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "db_query_duration_seconds",
				Help:    "Duration of database queries in seconds.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation"},
		)

		CacheMisses = prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "cache_misses_total",
				Help: "Total number of cache misses.",
			},
		)

		ExternalRequestDuration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "external_request_duration_seconds",
				Help:    "Duration of external HTTP requests in seconds.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"service"},
		)

		prometheus.MustRegister(DBQueryDuration, CacheMisses, ExternalRequestDuration)
	})
}

// Handler returns an http.Handler that serves Prometheus metrics.
func Handler() http.Handler {
	return promhttp.Handler()
}

// RecordDBQuery records the duration of a database query for the given operation.
func RecordDBQuery(duration time.Duration, operation string) {
	DBQueryDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// RecordCacheMiss increments the cache miss counter.
func RecordCacheMiss() {
	CacheMisses.Inc()
}

// RecordExternalRequest records the duration of an external HTTP call to the named service.
func RecordExternalRequest(service string, duration time.Duration) {
	ExternalRequestDuration.WithLabelValues(service).Observe(duration.Seconds())
}
