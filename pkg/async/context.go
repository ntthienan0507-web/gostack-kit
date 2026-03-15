package async

import "context"

type detachedKey string

const (
	keyRequestID  detachedKey = "request_id"
	keyUserID     detachedKey = "user_id"
	keyTraceID    detachedKey = "trace_id"
)

// DetachedValues holds values extracted from a request context
// for safe use in background goroutines.
//
// Use this to copy values from gin.Context BEFORE passing to WorkerPool:
//
//	vals := async.ExtractValues(ctx)
//	app.Workers.Submit(func(bgCtx context.Context) error {
//	    bgCtx = vals.Inject(bgCtx)
//	    return doWork(bgCtx, vals.UserID)
//	})
type DetachedValues struct {
	RequestID string
	UserID    string
	TraceID   string
	Extra     map[string]string // additional key-value pairs
}

// ExtractFromGin extracts commonly needed values from a gin context.
// Call this in the HTTP handler, BEFORE submitting to WorkerPool.
func ExtractFromGin(getUserID func() string, getRequestID func() string) DetachedValues {
	return DetachedValues{
		UserID:    getUserID(),
		RequestID: getRequestID(),
	}
}

// Extract creates DetachedValues with explicit values.
// Use when you already have the values outside of gin context.
func Extract(userID, requestID string) DetachedValues {
	return DetachedValues{
		UserID:    userID,
		RequestID: requestID,
	}
}

// Inject stores the detached values into a background context.
// Use inside the Task function to make values available to downstream code.
func (d DetachedValues) Inject(ctx context.Context) context.Context {
	if d.RequestID != "" {
		ctx = context.WithValue(ctx, keyRequestID, d.RequestID)
	}
	if d.UserID != "" {
		ctx = context.WithValue(ctx, keyUserID, d.UserID)
	}
	if d.TraceID != "" {
		ctx = context.WithValue(ctx, keyTraceID, d.TraceID)
	}
	return ctx
}

// RequestIDFromContext retrieves the request ID from a background context.
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(keyRequestID).(string)
	return v
}

// UserIDFromContext retrieves the user ID from a background context.
func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(keyUserID).(string)
	return v
}
