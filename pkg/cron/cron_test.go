package cron

import (
	"testing"
	"time"
)

func TestParseStandardExpressions(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		wantErr bool
	}{
		{"every minute", "* * * * *", false},
		{"specific minute", "5 * * * *", false},
		{"specific time", "30 14 * * *", false},
		{"with ranges", "0 9-17 * * 1-5", false},
		{"with steps", "*/15 * * * *", false},
		{"with commas", "0,30 * * * *", false},
		{"complex", "0,15,30,45 9-17 * 1-6 1-5", false},
		{"too few fields", "* * *", true},
		{"too many fields", "* * * * * *", true},
		{"invalid value", "60 * * * *", true},
		{"invalid range", "5-3 * * * *", true},
		{"invalid step", "*/0 * * * *", true},
		{"non-numeric", "abc * * * *", true},
		{"empty string", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse(%q) error = %v, wantErr %v", tt.expr, err, tt.wantErr)
			}
		})
	}
}

func TestParseShortcuts(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		wantErr bool
	}{
		{"hourly", "@hourly", false},
		{"daily", "@daily", false},
		{"weekly", "@weekly", false},
		{"unknown shortcut", "@monthly", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse(%q) error = %v, wantErr %v", tt.expr, err, tt.wantErr)
			}
		})
	}
}

func TestParseEveryInterval(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		wantErr bool
	}{
		{"every 5 minutes", "@every 5m", false},
		{"every 1 hour", "@every 1h", false},
		{"every 1h30m", "@every 1h30m", false},
		{"every 30 seconds (too short)", "@every 30s", true},
		{"invalid duration", "@every invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse(%q) error = %v, wantErr %v", tt.expr, err, tt.wantErr)
			}
		})
	}
}

func TestCronScheduleMatches(t *testing.T) {
	// 2024-01-15 is a Monday (weekday=1)
	base := time.Date(2024, time.January, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		expr  string
		time  time.Time
		match bool
	}{
		{
			name:  "every minute matches",
			expr:  "* * * * *",
			time:  base,
			match: true,
		},
		{
			name:  "specific minute matches",
			expr:  "30 * * * *",
			time:  base.Add(30 * time.Minute),
			match: true,
		},
		{
			name:  "specific minute does not match",
			expr:  "30 * * * *",
			time:  base.Add(15 * time.Minute),
			match: false,
		},
		{
			name:  "specific hour matches",
			expr:  "0 14 * * *",
			time:  time.Date(2024, time.January, 15, 14, 0, 0, 0, time.UTC),
			match: true,
		},
		{
			name:  "specific hour does not match",
			expr:  "0 14 * * *",
			time:  time.Date(2024, time.January, 15, 10, 0, 0, 0, time.UTC),
			match: false,
		},
		{
			name:  "day range matches weekday",
			expr:  "0 0 * * 1-5",
			time:  base, // Monday
			match: true,
		},
		{
			name:  "day range does not match weekend",
			expr:  "0 0 * * 1-5",
			time:  time.Date(2024, time.January, 14, 0, 0, 0, 0, time.UTC), // Sunday
			match: false,
		},
		{
			name:  "step values every 15 minutes",
			expr:  "*/15 * * * *",
			time:  base, // minute 0
			match: true,
		},
		{
			name:  "step values every 15 minutes at 15",
			expr:  "*/15 * * * *",
			time:  base.Add(15 * time.Minute),
			match: true,
		},
		{
			name:  "step values every 15 minutes at 10 does not match",
			expr:  "*/15 * * * *",
			time:  base.Add(10 * time.Minute),
			match: false,
		},
		{
			name:  "hourly shortcut at minute 0",
			expr:  "@hourly",
			time:  base,
			match: true,
		},
		{
			name:  "hourly shortcut at minute 5 does not match",
			expr:  "@hourly",
			time:  base.Add(5 * time.Minute),
			match: false,
		},
		{
			name:  "daily at midnight",
			expr:  "@daily",
			time:  base,
			match: true,
		},
		{
			name:  "daily at noon does not match",
			expr:  "@daily",
			time:  time.Date(2024, time.January, 15, 12, 0, 0, 0, time.UTC),
			match: false,
		},
		{
			name:  "weekly on Sunday at midnight",
			expr:  "@weekly",
			time:  time.Date(2024, time.January, 14, 0, 0, 0, 0, time.UTC), // Sunday
			match: true,
		},
		{
			name:  "weekly on Monday does not match",
			expr:  "@weekly",
			time:  base, // Monday
			match: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sched, err := Parse(tt.expr)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.expr, err)
			}
			got := sched.Matches(tt.time)
			if got != tt.match {
				t.Errorf("Matches(%v) = %v, want %v", tt.time, got, tt.match)
			}
		})
	}
}

func TestIntervalScheduleMatches(t *testing.T) {
	sched, err := Parse("@every 5m")
	if err != nil {
		t.Fatalf("Parse(@every 5m) unexpected error: %v", err)
	}

	start := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	// First call always matches (sets start).
	if !sched.Matches(start) {
		t.Error("expected first call to match")
	}

	// At 5 minutes should match.
	if !sched.Matches(start.Add(5 * time.Minute)) {
		t.Error("expected match at 5m")
	}

	// At 3 minutes should not match.
	if sched.Matches(start.Add(3 * time.Minute)) {
		t.Error("expected no match at 3m")
	}

	// At 10 minutes should match.
	if !sched.Matches(start.Add(10 * time.Minute)) {
		t.Error("expected match at 10m")
	}
}
