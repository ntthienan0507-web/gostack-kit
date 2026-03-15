package broker

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// Batcher accumulates messages and flushes them to a BatchHandler either
// when the batch reaches the configured size or the flush interval elapses,
// whichever comes first.
type Batcher struct {
	handler  BatchHandler
	opts     BatchOpts
	logger   *zap.Logger

	mu       sync.Mutex
	buffer   []*Message
	timer    *time.Timer
	done     chan struct{}
	stopped  bool
}

// NewBatcher creates a Batcher that flushes to handler according to opts.
func NewBatcher(handler BatchHandler, opts BatchOpts, logger *zap.Logger) *Batcher {
	b := &Batcher{
		handler: handler,
		opts:    opts,
		logger:  logger,
		buffer:  make([]*Message, 0, opts.Size),
		done:    make(chan struct{}),
	}

	b.timer = time.AfterFunc(opts.Interval, b.timerFlush)

	return b
}

// Add appends a message to the batch. Flushes immediately if the batch is full.
func (b *Batcher) Add(msg *Message) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.stopped {
		return
	}

	b.buffer = append(b.buffer, msg)

	if len(b.buffer) >= b.opts.Size {
		b.flushLocked()
	}
}

// AsHandler returns a Handler that adds each message to the batcher.
// This allows a Batcher to be used where a single-message Handler is expected.
func (b *Batcher) AsHandler() Handler {
	return func(msg *Message) error {
		b.Add(msg)
		return nil
	}
}

// Shutdown stops the timer and flushes any remaining messages.
func (b *Batcher) Shutdown() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.stopped {
		return
	}

	b.stopped = true
	b.timer.Stop()

	if len(b.buffer) > 0 {
		b.flushLocked()
	}

	close(b.done)
}

func (b *Batcher) timerFlush() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.stopped {
		return
	}

	if len(b.buffer) > 0 {
		b.flushLocked()
	}

	b.timer.Reset(b.opts.Interval)
}

func (b *Batcher) flushLocked() {
	batch := b.buffer
	b.buffer = make([]*Message, 0, b.opts.Size)

	// Run flush in a goroutine-safe, panic-safe manner.
	go b.safeFlush(batch)
}

func (b *Batcher) safeFlush(batch []*Message) {
	defer func() {
		if r := recover(); r != nil {
			b.logger.Error("batch handler panic recovered",
				zap.Int("batch_size", len(batch)),
				zap.Any("panic", r),
			)
		}
	}()

	if err := b.handler(batch); err != nil {
		b.logger.Error("batch handler error",
			zap.Int("batch_size", len(batch)),
			zap.Error(err),
		)
	}
}
