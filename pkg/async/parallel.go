package async

import (
	"context"
	"sync"
)

// Parallel runs multiple functions concurrently and collects all errors.
// Returns nil if all succeed. Respects context cancellation.
//
// Usage:
//
//	err := async.Parallel(ctx,
//	    func(ctx context.Context) error { return svcA.DoSomething(ctx) },
//	    func(ctx context.Context) error { return svcB.DoSomething(ctx) },
//	    func(ctx context.Context) error { return svcC.DoSomething(ctx) },
//	)
func Parallel(ctx context.Context, fns ...func(ctx context.Context) error) error {
	errs := make([]error, len(fns))
	var wg sync.WaitGroup

	wg.Add(len(fns))
	for i, fn := range fns {
		go func() {
			defer wg.Done()
			errs[i] = fn(ctx)
		}()
	}
	wg.Wait()

	// Return first non-nil error
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// ParallelCollect runs multiple functions concurrently and returns all results.
// Each function returns a value and an error.
// Returns on first error (other goroutines are cancelled via context).
//
// Usage:
//
//	results, err := async.ParallelCollect(ctx,
//	    func(ctx context.Context) (User, error) { return userSvc.Get(ctx, id) },
//	    func(ctx context.Context) (User, error) { return cacheSvc.Get(ctx, id) },
//	)
func ParallelCollect[T any](ctx context.Context, fns ...func(ctx context.Context) (T, error)) ([]T, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type result struct {
		index int
		value T
		err   error
	}

	ch := make(chan result, len(fns))

	for i, fn := range fns {
		go func() {
			v, err := fn(ctx)
			ch <- result{index: i, value: v, err: err}
		}()
	}

	results := make([]T, len(fns))
	for range fns {
		r := <-ch
		if r.err != nil {
			cancel() // signal other goroutines to stop
			return nil, r.err
		}
		results[r.index] = r.value
	}

	return results, nil
}
