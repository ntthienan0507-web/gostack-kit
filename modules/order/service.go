package order

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/ntthienan0507-web/go-api-template/pkg/async"
	"github.com/ntthienan0507-web/go-api-template/pkg/cache"
	"github.com/ntthienan0507-web/go-api-template/pkg/database"
)

// cacheTTL is the time-to-live for cached order responses.
// 5 minutes balances freshness against database load.
const cacheTTL = 5 * time.Minute

// cacheKey returns the Redis key for an order by ID.
// Format: "order:{uuid}" — namespaced to avoid collisions with other modules.
func cacheKey(id uuid.UUID) string {
	return fmt.Sprintf("order:%s", id.String())
}

// Service holds business logic for the order module.
// Demonstrates how to combine the template's infrastructure packages:
//   - Transaction: atomic writes across multiple tables
//   - Outbox:      reliable event publishing (no dual-write problem)
//   - Cache:       cache-aside pattern for reads
//   - WorkerPool:  fire-and-forget async tasks
type Service struct {
	repo    Repository
	db      *gorm.DB          // for managing transactions
	cache   *cache.Client     // Redis cache for read-through caching
	workers *async.WorkerPool // background task execution
	logger  *zap.Logger
}

// NewService creates an order service with all infrastructure dependencies injected.
func NewService(repo Repository, db *gorm.DB, cache *cache.Client, workers *async.WorkerPool, logger *zap.Logger) *Service {
	return &Service{
		repo:    repo,
		db:      db,
		cache:   cache,
		workers: workers,
		logger:  logger,
	}
}

// List returns a paginated list of orders.
func (s *Service) List(ctx context.Context, params ListParams) (*ListResult, error) {
	limit := int32(params.PageSize)
	offset := int32((params.Page - 1) * params.PageSize)

	orders, err := s.repo.List(ctx, params, limit, offset)
	if err != nil {
		s.logger.Error("list orders failed", zap.Error(err))
		return nil, err
	}

	count, err := s.repo.Count(ctx, params)
	if err != nil {
		s.logger.Error("count orders failed", zap.Error(err))
		return nil, err
	}

	items := make([]OrderResponse, len(orders))
	for i, o := range orders {
		items[i] = ToResponse(o)
	}

	return &ListResult{Items: items, Total: count}, nil
}

// GetByID returns a single order by ID, using the cache-aside pattern.
//
// Flow:
//  1. Check Redis cache — fast path, no DB hit.
//  2. Cache miss → query the database.
//  3. Store the result in cache with TTL for subsequent reads.
//  4. Return the response.
//
// Why cache-aside: the caller controls cache population, so stale data is bounded
// by TTL and explicit invalidation on writes.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*OrderResponse, error) {
	// Step 1: Try Redis cache first. On hit, skip the database entirely.
	if s.cache != nil {
		cached, err := cache.GetJSON[OrderResponse](ctx, s.cache, cacheKey(id))
		if err == nil {
			s.logger.Debug("cache hit", zap.String("order_id", id.String()))
			return &cached, nil
		}
		if !errors.Is(err, cache.ErrCacheMiss) {
			// Log but don't fail — cache is an optimization, not a hard dependency.
			s.logger.Warn("cache get failed, falling back to DB", zap.Error(err))
		}
	}

	// Step 2: Cache miss → query the database.
	o, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	resp := ToResponse(o)

	// Step 3: Populate cache asynchronously-safe (best effort).
	// Even if this fails, the next request will just hit the DB again.
	if s.cache != nil {
		if err := s.cache.SetJSON(ctx, cacheKey(id), resp, cacheTTL); err != nil {
			s.logger.Warn("cache set failed", zap.Error(err))
		}
	}

	return &resp, nil
}

// Create creates a new order with all items in a single transaction.
//
// This method demonstrates three patterns working together:
//
//  1. TRANSACTION: Order + OrderItems + Outbox entry are written atomically.
//     If any step fails, the entire operation rolls back — no partial state.
//
//  2. OUTBOX PATTERN: The event is written to the outbox table inside the SAME
//     transaction as the business data. A separate relay process picks up
//     unpublished outbox entries and publishes them to Kafka.
//     This avoids the "dual-write" problem where the DB commit succeeds
//     but the Kafka publish fails (or vice versa).
//
//  3. WORKER POOL: After the transaction commits, we fire-and-forget a
//     notification task via the worker pool. This is non-critical work
//     (e.g., sending an email) that should not block the HTTP response.
func (s *Service) Create(ctx context.Context, req CreateOrderRequest) (*OrderResponse, error) {
	// Step 1: Build the domain model and calculate total price.
	orderID := uuid.New()
	var totalPrice float64

	items := make([]OrderItem, len(req.Items))
	for i, item := range req.Items {
		items[i] = OrderItem{
			ID:        uuid.New(),
			OrderID:   orderID,
			ProductID: item.ProductID,
			Name:      item.Name,
			Quantity:  item.Quantity,
			UnitPrice: item.UnitPrice,
		}
		totalPrice += item.UnitPrice * float64(item.Quantity)
	}

	newOrder := &Order{
		ID:         orderID,
		UserID:     req.UserID,
		Status:     StatusPending,
		TotalPrice: totalPrice,
		Currency:   req.Currency,
		Note:       req.Note,
		Items:      items,
	}

	// Step 2: Execute everything in a single database transaction.
	// database.WithTransactionResult wraps Begin/Commit/Rollback and handles panics.
	created, err := database.WithTransactionResult[*Order](ctx, s.db, s.logger, func(tx *gorm.DB) (*Order, error) {
		// 2a: Insert the order and its items (repo uses the tx, not the default db).
		result, err := s.repo.Create(ctx, tx, newOrder)
		if err != nil {
			return nil, fmt.Errorf("create order: %w", err)
		}

		// TODO: add outbox event for order.created (broker.WriteOutbox)

		return result, nil
	})
	if err != nil {
		s.logger.Error("create order transaction failed", zap.Error(err))
		return nil, err
	}

	// Step 3: Invalidate cache (if any stale list/entry exists).
	// Done AFTER the transaction commits so we don't invalidate on rollback.
	if s.cache != nil {
		if err := s.cache.Delete(ctx, cacheKey(created.ID)); err != nil {
			s.logger.Warn("cache invalidation failed", zap.Error(err))
		}
	}

	// Step 4: Fire-and-forget notification via the worker pool.
	// Copy values BEFORE submitting — never capture *gin.Context in a closure.
	// The worker receives a detached background context, safe to use after the
	// HTTP handler returns.
	oid := created.ID
	uid := created.UserID
	s.workers.Submit(func(ctx context.Context) error {
		s.logger.Info("sending order confirmation notification",
			zap.String("order_id", oid.String()),
			zap.String("user_id", uid.String()),
		)
		// In a real app: call notification service, send email, push notification, etc.
		return nil
	})

	// Step 5: Return the response.
	resp := ToResponse(created)
	return &resp, nil
}

// Cancel cancels a pending or confirmed order.
//
// Demonstrates transaction + outbox for a state-change operation:
//  1. Validate the current status (only pending/confirmed can be cancelled).
//  2. Transaction: update status + write cancellation event to outbox.
//  3. Invalidate cache so subsequent reads see the updated status.
func (s *Service) Cancel(ctx context.Context, id uuid.UUID) error {
	// Step 1: Fetch current order to validate cancellability.
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if existing.Status != StatusPending && existing.Status != StatusConfirmed {
		return ErrNotCancellable
	}

	// Step 2: Transaction — update status + write outbox event atomically.
	err = database.WithTransaction(ctx, s.db, s.logger, func(tx *gorm.DB) error {
		if err := s.repo.UpdateStatus(ctx, tx, id, StatusCancelled, 0); err != nil {
			return fmt.Errorf("update status: %w", err)
		}

		// TODO: add outbox event for order.cancelled

		return nil
	})
	if err != nil {
		s.logger.Error("cancel order transaction failed", zap.Error(err))
		return err
	}

	// Step 3: Invalidate cache so the next read sees the updated status.
	if s.cache != nil {
		if err := s.cache.Delete(ctx, cacheKey(id)); err != nil {
			s.logger.Warn("cache invalidation failed", zap.Error(err))
		}
	}

	return nil
}
