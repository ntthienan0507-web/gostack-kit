package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/ntthienan0507-web/go-api-template/pkg/async"
)

// Action represents what happened.
type Action string

const (
	ActionCreate Action = "create"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
	ActionView   Action = "view"   // for sensitive data access logging
	ActionExport Action = "export"
	ActionLogin  Action = "login"
	ActionLogout Action = "logout"
)

// Entry is an audit log record.
type Entry struct {
	ID         uuid.UUID       `json:"id" gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID     string          `json:"user_id" gorm:"type:varchar(255);not null;index"`
	Action     Action          `json:"action" gorm:"type:varchar(50);not null;index"`
	Resource   string          `json:"resource" gorm:"type:varchar(100);not null;index"` // e.g. "user", "order"
	ResourceID string          `json:"resource_id" gorm:"type:varchar(255);not null"`
	Changes    json.RawMessage `json:"changes,omitempty" gorm:"type:jsonb"`  // diff: what changed
	Metadata   json.RawMessage `json:"metadata,omitempty" gorm:"type:jsonb"` // extra context
	IP         string          `json:"ip" gorm:"type:varchar(45)"`
	UserAgent  string          `json:"user_agent" gorm:"type:varchar(500)"`
	CreatedAt  time.Time       `json:"created_at" gorm:"autoCreateTime;index"`
}

// TableName returns the database table name for GORM.
func (Entry) TableName() string { return "audit_log" }

// Logger records audit events. Uses worker pool for async writes (non-blocking).
type Logger struct {
	db      *gorm.DB
	workers *async.WorkerPool
	logger  *zap.Logger
}

// New creates an audit Logger.
func New(db *gorm.DB, workers *async.WorkerPool, logger *zap.Logger) *Logger {
	return &Logger{
		db:      db,
		workers: workers,
		logger:  logger,
	}
}

// Log records an audit event asynchronously (fire-and-forget via worker pool).
// Never blocks the HTTP handler — audit failures are logged, not returned.
func (l *Logger) Log(ctx context.Context, entry Entry) {
	// Assign ID if not set.
	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}

	// Copy entry — do not capture caller's context in the closure.
	e := entry
	l.workers.Submit(func(bgCtx context.Context) error {
		if err := l.db.WithContext(bgCtx).Create(&e).Error; err != nil {
			l.logger.Error("failed to write audit log",
				zap.String("action", string(e.Action)),
				zap.String("resource", e.Resource),
				zap.String("resource_id", e.ResourceID),
				zap.Error(err),
			)
			return err
		}
		return nil
	})
}

// LogFromGin is a convenience that extracts IP/UserAgent from gin context.
// The changes parameter is marshalled to JSON; pass nil if there are no changes.
func (l *Logger) LogFromGin(c *gin.Context, action Action, resource, resourceID string, changes any) {
	// Copy values out of gin.Context BEFORE submitting to the worker pool.
	userID := c.GetString("user_id")
	ip := c.ClientIP()
	userAgent := c.Request.UserAgent()

	var changesJSON json.RawMessage
	if changes != nil {
		data, err := json.Marshal(changes)
		if err != nil {
			l.logger.Warn("failed to marshal audit changes", zap.Error(err))
		} else {
			changesJSON = data
		}
	}

	l.Log(c.Request.Context(), Entry{
		UserID:     userID,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Changes:    changesJSON,
		IP:         ip,
		UserAgent:  userAgent,
	})
}

// QueryParams holds filters for querying audit entries.
type QueryParams struct {
	UserID     string
	Action     Action
	Resource   string
	ResourceID string
	From       *time.Time
	To         *time.Time
	Limit      int
	Offset     int
}

// Query returns audit entries with filters (for admin API).
// Returns the matching entries, total count, and any error.
func (l *Logger) Query(ctx context.Context, params QueryParams) ([]Entry, int64, error) {
	query := l.db.WithContext(ctx).Model(&Entry{})

	if params.UserID != "" {
		query = query.Where("user_id = ?", params.UserID)
	}
	if params.Action != "" {
		query = query.Where("action = ?", params.Action)
	}
	if params.Resource != "" {
		query = query.Where("resource = ?", params.Resource)
	}
	if params.ResourceID != "" {
		query = query.Where("resource_id = ?", params.ResourceID)
	}
	if params.From != nil {
		query = query.Where("created_at >= ?", *params.From)
	}
	if params.To != nil {
		query = query.Where("created_at <= ?", *params.To)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 50
	}

	var entries []Entry
	err := query.
		Order("created_at DESC").
		Limit(limit).
		Offset(params.Offset).
		Find(&entries).Error
	if err != nil {
		return nil, 0, err
	}

	return entries, total, nil
}
