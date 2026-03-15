package types

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// Supported date formats for ParseDate.
var dateFormats = []string{
	"2006-01-02",
	time.RFC3339,
	"2006-01-02T15:04:05Z",
	"02/01/2006",
}

// ParseDate parses a date string using multiple supported formats:
// "2006-01-02", RFC3339, "2006-01-02T15:04:05Z", "02/01/2006".
func ParseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, format := range dateFormats {
		t, err := time.Parse(format, s)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, &time.ParseError{
		Layout:     dateFormats[0],
		Value:      s,
		LayoutElem: "",
		ValueElem:  "",
		Message:    ": unable to parse with any supported format",
	}
}

// ParseDateOrNil is like ParseDate but returns nil on empty or invalid input.
func ParseDateOrNil(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	t, err := ParseDate(s)
	if err != nil {
		return nil
	}
	return &t
}

// FormatDate formats a time.Time to "2006-01-02".
func FormatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

// FormatDateTime formats a time.Time to RFC3339.
func FormatDateTime(t time.Time) string {
	return t.Format(time.RFC3339)
}

// FormatDatePtr formats a *time.Time to "2006-01-02", or returns "" if nil.
func FormatDatePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

// StringToInt32 converts a string to int32, returning 0 on error.
func StringToInt32(s string) int32 {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 32)
	if err != nil {
		return 0
	}
	return int32(n)
}

// StringToInt64 converts a string to int64, returning 0 on error.
func StringToInt64(s string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// StringToFloat64 converts a string to float64, returning 0 on error.
func StringToFloat64(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return f
}

// RoundFloat64 rounds a float64 to the given number of decimal places.
func RoundFloat64(value float64, precision int) float64 {
	p := math.Pow10(precision)
	return math.Round(value*p) / p
}

// Ptr returns a pointer to the given value. Useful for optional fields.
func Ptr[T any](v T) *T {
	return &v
}

// Deref dereferences a pointer, returning the zero value of T if the pointer is nil.
func Deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}
