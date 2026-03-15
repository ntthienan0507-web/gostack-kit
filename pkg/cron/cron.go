package cron

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Job is a function executed by the scheduler.
type Job func(ctx context.Context) error

// Schedule determines whether a job should run at a given time.
type Schedule interface {
	Matches(t time.Time) bool
}

// entry pairs a schedule with a job and a name.
type entry struct {
	name     string
	schedule Schedule
	job      Job
}

// Scheduler manages and runs cron jobs.
type Scheduler struct {
	entries []entry
	logger  *zap.Logger
	mu      sync.Mutex
}

// NewScheduler creates a new Scheduler.
func NewScheduler(logger *zap.Logger) *Scheduler {
	return &Scheduler{
		logger: logger.Named("cron"),
	}
}

// Register adds a named job with the given cron expression.
func (s *Scheduler) Register(name, expr string, job Job) error {
	sched, err := Parse(expr)
	if err != nil {
		return fmt.Errorf("cron: register %q: %w", name, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, entry{
		name:     name,
		schedule: sched,
		job:      job,
	})
	s.logger.Info("registered job", zap.String("name", name), zap.String("expr", expr))
	return nil
}

// Start begins the scheduler loop. It blocks until the context is cancelled.
// The scheduler ticks every minute, aligned to the clock.
func (s *Scheduler) Start(ctx context.Context) {
	s.logger.Info("scheduler started", zap.Int("jobs", len(s.entries)))

	// Align to the next minute boundary.
	now := time.Now()
	next := now.Truncate(time.Minute).Add(time.Minute)
	alignTimer := time.NewTimer(time.Until(next))

	select {
	case <-ctx.Done():
		alignTimer.Stop()
		s.logger.Info("scheduler stopped")
		return
	case <-alignTimer.C:
	}

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	s.tick(ctx, time.Now())

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("scheduler stopped")
			return
		case t := <-ticker.C:
			s.tick(ctx, t)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context, t time.Time) {
	s.mu.Lock()
	entries := make([]entry, len(s.entries))
	copy(entries, s.entries)
	s.mu.Unlock()

	for _, e := range entries {
		if e.schedule.Matches(t) {
			go s.runJob(ctx, e)
		}
	}
}

func (s *Scheduler) runJob(ctx context.Context, e entry) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("panic in cron job",
				zap.String("name", e.name),
				zap.Any("recover", r),
			)
		}
	}()

	s.logger.Debug("running job", zap.String("name", e.name))
	if err := e.job(ctx); err != nil {
		s.logger.Error("job failed", zap.String("name", e.name), zap.Error(err))
	}
}

// cronSchedule represents a standard 5-field cron schedule.
type cronSchedule struct {
	minute map[int]bool
	hour   map[int]bool
	dom    map[int]bool // day of month
	month  map[int]bool
	dow    map[int]bool // day of week (0=Sunday)
}

func (cs *cronSchedule) Matches(t time.Time) bool {
	return cs.minute[t.Minute()] &&
		cs.hour[t.Hour()] &&
		cs.dom[t.Day()] &&
		cs.month[int(t.Month())] &&
		cs.dow[int(t.Weekday())]
}

// intervalSchedule runs at a fixed interval.
type intervalSchedule struct {
	interval time.Duration
	start    time.Time
}

func (is *intervalSchedule) Matches(t time.Time) bool {
	if is.start.IsZero() {
		is.start = t
		return true
	}
	elapsed := t.Sub(is.start)
	// Check if current minute falls on an interval boundary.
	intervalMinutes := int(is.interval.Minutes())
	if intervalMinutes <= 0 {
		return false
	}
	elapsedMinutes := int(elapsed.Minutes())
	return elapsedMinutes%intervalMinutes == 0
}

// Parse parses a cron expression string and returns a Schedule.
// Supported formats:
//   - Standard 5-field: "minute hour dom month dow"
//   - @hourly: equivalent to "0 * * * *"
//   - @daily:  equivalent to "0 0 * * *"
//   - @weekly: equivalent to "0 0 * * 0"
//   - @every <duration>: e.g., "@every 5m", "@every 1h30m"
func Parse(expr string) (Schedule, error) {
	expr = strings.TrimSpace(expr)

	if strings.HasPrefix(expr, "@") {
		return parseShortcut(expr)
	}

	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("cron: expected 5 fields, got %d in %q", len(fields), expr)
	}

	minute, err := parseField(fields[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("cron: minute field: %w", err)
	}
	hour, err := parseField(fields[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("cron: hour field: %w", err)
	}
	dom, err := parseField(fields[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("cron: day-of-month field: %w", err)
	}
	month, err := parseField(fields[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("cron: month field: %w", err)
	}
	dow, err := parseField(fields[4], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("cron: day-of-week field: %w", err)
	}

	return &cronSchedule{
		minute: minute,
		hour:   hour,
		dom:    dom,
		month:  month,
		dow:    dow,
	}, nil
}

func parseShortcut(expr string) (Schedule, error) {
	switch expr {
	case "@hourly":
		return Parse("0 * * * *")
	case "@daily":
		return Parse("0 0 * * *")
	case "@weekly":
		return Parse("0 0 * * 0")
	}

	if strings.HasPrefix(expr, "@every ") {
		durStr := strings.TrimPrefix(expr, "@every ")
		dur, err := time.ParseDuration(durStr)
		if err != nil {
			return nil, fmt.Errorf("cron: invalid @every duration %q: %w", durStr, err)
		}
		if dur < time.Minute {
			return nil, fmt.Errorf("cron: @every duration must be at least 1 minute, got %v", dur)
		}
		return &intervalSchedule{interval: dur}, nil
	}

	return nil, fmt.Errorf("cron: unknown shortcut %q", expr)
}

// parseField parses a single cron field and returns a set of valid values.
// Supports: *, */N, N, N-M, and comma-separated combinations.
func parseField(field string, min, max int) (map[int]bool, error) {
	values := make(map[int]bool)

	parts := strings.Split(field, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)

		if part == "*" {
			for i := min; i <= max; i++ {
				values[i] = true
			}
			continue
		}

		if strings.HasPrefix(part, "*/") {
			stepStr := strings.TrimPrefix(part, "*/")
			step, err := strconv.Atoi(stepStr)
			if err != nil || step <= 0 {
				return nil, fmt.Errorf("invalid step %q", part)
			}
			for i := min; i <= max; i += step {
				values[i] = true
			}
			continue
		}

		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			lo, err := strconv.Atoi(bounds[0])
			if err != nil {
				return nil, fmt.Errorf("invalid range start %q", bounds[0])
			}
			hi, err := strconv.Atoi(bounds[1])
			if err != nil {
				return nil, fmt.Errorf("invalid range end %q", bounds[1])
			}
			if lo < min || hi > max || lo > hi {
				return nil, fmt.Errorf("range %d-%d out of bounds [%d, %d]", lo, hi, min, max)
			}
			for i := lo; i <= hi; i++ {
				values[i] = true
			}
			continue
		}

		val, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid value %q", part)
		}
		if val < min || val > max {
			return nil, fmt.Errorf("value %d out of bounds [%d, %d]", val, min, max)
		}
		values[val] = true
	}

	if len(values) == 0 {
		return nil, fmt.Errorf("empty field %q", field)
	}

	return values, nil
}
