package middleware

import (
	"bytes"
	"encoding/json"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// auditWriter wraps gin.ResponseWriter to capture the response body.
type auditWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *auditWriter) Write(data []byte) (int, error) {
	w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

// ResponseAudit checks that all JSON responses follow the standard format.
// Only active when mode is "debug" — zero overhead in production.
//
// Valid success format:  { "status": "success", "data": ... }
// Valid error format:    { "error_code": N, "error_message": "...", "error_detail": "..." }
//
// Logs a WARNING with path + offending body when a response doesn't match.
// This catches hardcoded ctx.JSON() calls that bypass response/apperror helpers.
func ResponseAudit(logger *zap.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		aw := &auditWriter{
			ResponseWriter: ctx.Writer,
			body:           &bytes.Buffer{},
		}
		ctx.Writer = aw

		ctx.Next()

		// Skip non-JSON, 204, redirects, swagger
		ct := aw.Header().Get("Content-Type")
		status := aw.Status()
		if status == 204 || status == 304 || ct == "" {
			return
		}

		body := aw.body.Bytes()
		if len(body) == 0 {
			return
		}

		var raw map[string]json.RawMessage
		if err := json.Unmarshal(body, &raw); err != nil {
			return // not JSON, skip
		}

		if isValidSuccess(raw, status) || isValidError(raw, status) {
			return
		}

		logger.Warn("non-standard response format detected",
			zap.String("method", ctx.Request.Method),
			zap.String("path", ctx.Request.URL.Path),
			zap.Int("status", status),
			zap.String("body", truncate(string(body), 300)),
			zap.String("hint", "use response.OK/Created/OKList or apperror.Respond/HandleError"),
		)
	}
}

// isValidSuccess checks for { "status": "success", "data": ... }
func isValidSuccess(raw map[string]json.RawMessage, status int) bool {
	if status >= 400 {
		return false
	}
	s, hasStatus := raw["status"]
	_, hasData := raw["data"]
	if !hasStatus || !hasData {
		return false
	}
	var v string
	if err := json.Unmarshal(s, &v); err != nil {
		return false
	}
	return v == "success"
}

// isValidError checks for { "error_code": N, "error_message": "...", "error_detail": "..." }
func isValidError(raw map[string]json.RawMessage, status int) bool {
	if status < 400 {
		return false
	}
	_, hasCode := raw["error_code"]
	_, hasMsg := raw["error_message"]
	_, hasDetail := raw["error_detail"]
	return hasCode && hasMsg && hasDetail
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
