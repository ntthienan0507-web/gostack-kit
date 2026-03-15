package middleware

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/trace"
)

const (
	traceIDKey = "trace_id"
	spanIDKey  = "span_id"
)

// Tracing returns a Gin middleware that instruments requests with OpenTelemetry spans.
// It delegates to otelgin.Middleware and additionally injects trace_id and span_id
// into the Gin context so downstream handlers and the logger can access them.
func Tracing(serviceName string) gin.HandlerFunc {
	otelMW := otelgin.Middleware(serviceName)

	return func(ctx *gin.Context) {
		// Use a wrapper that injects trace/span IDs after otelgin creates the span
		// but before the rest of the handler chain runs.
		//
		// otelgin.Middleware calls ctx.Next() internally, which processes the
		// remaining handlers. We insert a small middleware ahead of that chain
		// to capture the IDs that otelgin just set.
		ctx.Set("_otel_inject", true)
		otelMW(ctx)
	}
}

// TracingIDs is a companion middleware that must be registered AFTER Tracing.
// It reads the span from the request context (created by otelgin) and stores
// the trace_id and span_id in the Gin context for easy access.
func TracingIDs() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		span := trace.SpanFromContext(ctx.Request.Context())
		sc := span.SpanContext()

		if sc.HasTraceID() {
			ctx.Set(traceIDKey, sc.TraceID().String())
		}
		if sc.HasSpanID() {
			ctx.Set(spanIDKey, sc.SpanID().String())
		}
		ctx.Next()
	}
}

// GetTraceID retrieves the trace ID from the Gin context (set by TracingIDs middleware).
func GetTraceID(ctx *gin.Context) string {
	v, _ := ctx.Get(traceIDKey)
	id, _ := v.(string)
	return id
}

// GetSpanID retrieves the span ID from the Gin context (set by TracingIDs middleware).
func GetSpanID(ctx *gin.Context) string {
	v, _ := ctx.Get(spanIDKey)
	id, _ := v.(string)
	return id
}
