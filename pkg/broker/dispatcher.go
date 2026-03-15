package broker

import (
	"hash/fnv"
	"sync"

	"go.uber.org/zap"
)

// dispatcher distributes messages to a pool of workers, sharded by message key
// using FNV-32a hashing. Messages with the same key are always routed to the
// same worker, preserving per-key ordering.
type dispatcher struct {
	workers []chan *Message
	wg      sync.WaitGroup
	logger  *zap.Logger
}

// newDispatcher creates a dispatcher with the given number of workers and
// per-worker buffer size. Each worker processes messages sequentially using
// the provided handler.
func newDispatcher(numWorkers, bufferSize int, handler Handler, logger *zap.Logger) *dispatcher {
	d := &dispatcher{
		workers: make([]chan *Message, numWorkers),
		logger:  logger,
	}

	for i := 0; i < numWorkers; i++ {
		d.workers[i] = make(chan *Message, bufferSize)
		d.wg.Add(1)
		go d.runWorker(i, d.workers[i], handler)
	}

	return d
}

// Dispatch routes a message to the appropriate worker based on its key.
// Messages with empty keys are routed to worker 0.
func (d *dispatcher) Dispatch(msg *Message) {
	idx := d.shardIndex(msg.Key)
	d.workers[idx] <- msg
}

// Shutdown closes all worker channels and waits for them to finish processing.
func (d *dispatcher) Shutdown() {
	for _, ch := range d.workers {
		close(ch)
	}
	d.wg.Wait()
}

func (d *dispatcher) shardIndex(key string) int {
	if key == "" || len(d.workers) == 1 {
		return 0
	}

	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return int(h.Sum32()) % len(d.workers)
}

func (d *dispatcher) runWorker(id int, ch <-chan *Message, handler Handler) {
	defer d.wg.Done()

	for msg := range ch {
		d.safeHandle(id, msg, handler)
	}
}

func (d *dispatcher) safeHandle(workerID int, msg *Message, handler Handler) {
	defer func() {
		if r := recover(); r != nil {
			d.logger.Error("worker panic recovered",
				zap.Int("worker_id", workerID),
				zap.String("topic", string(msg.Topic)),
				zap.String("key", msg.Key),
				zap.Int64("offset", msg.Offset),
				zap.Any("panic", r),
			)
		}
	}()

	if err := handler(msg); err != nil {
		d.logger.Error("handler error",
			zap.Int("worker_id", workerID),
			zap.String("topic", string(msg.Topic)),
			zap.String("key", msg.Key),
			zap.Int64("offset", msg.Offset),
			zap.Error(err),
		)
	}
}
