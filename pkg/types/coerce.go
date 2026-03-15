package types

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ============================================================================
// Flexible type coercion: any → pgtype
// ============================================================================
//
// Use these when the input type is unknown at compile time:
//   - JSON unmarshalled data (map[string]any)
//   - CSV/Excel import (everything is string)
//   - Form values
//   - Dynamic query builders
//
// For typed code (service layer, known structs), prefer the strict Set* functions
// in pgtype.go — they catch type errors at compile time.
//
// Naming: Coerce* (not Set*) to make intent clear — "I'm coercing an unknown
// type, I accept the runtime cost."
// ============================================================================

// CoerceText converts any value to pgtype.Text.
//
//	string      → Text{String: s, Valid: s != ""}
//	[]byte      → Text{String: string(b), Valid: true}
//	*string     → Text{String: *s, Valid: true} (nil → invalid)
//	fmt.Stringer → Text{String: s.String(), Valid: true}
//	other       → Text{String: fmt.Sprint(v), Valid: true}
//	nil         → Text{Valid: false}
func CoerceText(v any) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	switch s := v.(type) {
	case string:
		return pgtype.Text{String: s, Valid: s != ""}
	case *string:
		if s == nil {
			return pgtype.Text{}
		}
		return pgtype.Text{String: *s, Valid: true}
	case []byte:
		return pgtype.Text{String: string(s), Valid: len(s) > 0}
	case fmt.Stringer:
		str := s.String()
		return pgtype.Text{String: str, Valid: str != ""}
	default:
		str := fmt.Sprint(v)
		return pgtype.Text{String: str, Valid: str != ""}
	}
}

// CoerceInt4 converts any numeric/string value to pgtype.Int4.
//
//	int, int8, int16, int32, int64  → Int4{Int32: int32(n), Valid: true}
//	float32, float64                → Int4{Int32: int32(f), Valid: true} (truncates)
//	string                          → parsed via strconv.Atoi
//	*int32                          → Int4{Int32: *p, Valid: true} (nil → invalid)
//	nil                             → Int4{Valid: false}
func CoerceInt4(v any) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{}
	}
	switch n := v.(type) {
	case int:
		return pgtype.Int4{Int32: int32(n), Valid: true}
	case int8:
		return pgtype.Int4{Int32: int32(n), Valid: true}
	case int16:
		return pgtype.Int4{Int32: int32(n), Valid: true}
	case int32:
		return pgtype.Int4{Int32: n, Valid: true}
	case int64:
		return pgtype.Int4{Int32: int32(n), Valid: true}
	case *int32:
		if n == nil {
			return pgtype.Int4{}
		}
		return pgtype.Int4{Int32: *n, Valid: true}
	case float32:
		return pgtype.Int4{Int32: int32(n), Valid: true}
	case float64:
		return pgtype.Int4{Int32: int32(n), Valid: true}
	case string:
		i, err := strconv.Atoi(n)
		if err != nil {
			return pgtype.Int4{}
		}
		return pgtype.Int4{Int32: int32(i), Valid: true}
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return pgtype.Int4{}
		}
		return pgtype.Int4{Int32: int32(i), Valid: true}
	default:
		return pgtype.Int4{}
	}
}

// CoerceInt8 converts any numeric/string value to pgtype.Int8.
func CoerceInt8(v any) pgtype.Int8 {
	if v == nil {
		return pgtype.Int8{}
	}
	switch n := v.(type) {
	case int:
		return pgtype.Int8{Int64: int64(n), Valid: true}
	case int8:
		return pgtype.Int8{Int64: int64(n), Valid: true}
	case int16:
		return pgtype.Int8{Int64: int64(n), Valid: true}
	case int32:
		return pgtype.Int8{Int64: int64(n), Valid: true}
	case int64:
		return pgtype.Int8{Int64: n, Valid: true}
	case *int64:
		if n == nil {
			return pgtype.Int8{}
		}
		return pgtype.Int8{Int64: *n, Valid: true}
	case float32:
		return pgtype.Int8{Int64: int64(n), Valid: true}
	case float64:
		return pgtype.Int8{Int64: int64(n), Valid: true}
	case string:
		i, err := strconv.ParseInt(n, 10, 64)
		if err != nil {
			return pgtype.Int8{}
		}
		return pgtype.Int8{Int64: i, Valid: true}
	case json.Number:
		i, err := n.Int64()
		if err != nil {
			return pgtype.Int8{}
		}
		return pgtype.Int8{Int64: i, Valid: true}
	default:
		return pgtype.Int8{}
	}
}

// CoerceFloat8 converts any numeric/string value to pgtype.Float8.
func CoerceFloat8(v any) pgtype.Float8 {
	if v == nil {
		return pgtype.Float8{}
	}
	switch f := v.(type) {
	case float64:
		return pgtype.Float8{Float64: f, Valid: true}
	case float32:
		return pgtype.Float8{Float64: float64(f), Valid: true}
	case *float64:
		if f == nil {
			return pgtype.Float8{}
		}
		return pgtype.Float8{Float64: *f, Valid: true}
	case int:
		return pgtype.Float8{Float64: float64(f), Valid: true}
	case int32:
		return pgtype.Float8{Float64: float64(f), Valid: true}
	case int64:
		return pgtype.Float8{Float64: float64(f), Valid: true}
	case string:
		n, err := strconv.ParseFloat(f, 64)
		if err != nil {
			return pgtype.Float8{}
		}
		return pgtype.Float8{Float64: n, Valid: true}
	case json.Number:
		n, err := f.Float64()
		if err != nil {
			return pgtype.Float8{}
		}
		return pgtype.Float8{Float64: n, Valid: true}
	case pgtype.Numeric:
		fv, err := f.Float64Value()
		if err != nil {
			return pgtype.Float8{}
		}
		return fv
	default:
		return pgtype.Float8{}
	}
}

// CoerceBool converts any value to pgtype.Bool.
//
//	bool          → Bool{Bool: b, Valid: true}
//	*bool         → Bool{Bool: *b, Valid: true} (nil → invalid)
//	int/float     → Bool{Bool: n != 0, Valid: true}
//	string        → "true"/"t"/"1" = true, "false"/"f"/"0" = false
//	nil           → Bool{Valid: false}
func CoerceBool(v any) pgtype.Bool {
	if v == nil {
		return pgtype.Bool{}
	}
	switch b := v.(type) {
	case bool:
		return pgtype.Bool{Bool: b, Valid: true}
	case *bool:
		if b == nil {
			return pgtype.Bool{}
		}
		return pgtype.Bool{Bool: *b, Valid: true}
	case int:
		return pgtype.Bool{Bool: b != 0, Valid: true}
	case int32:
		return pgtype.Bool{Bool: b != 0, Valid: true}
	case int64:
		return pgtype.Bool{Bool: b != 0, Valid: true}
	case float64:
		return pgtype.Bool{Bool: b != 0, Valid: true}
	case string:
		lower := strings.ToLower(strings.TrimSpace(b))
		switch lower {
		case "true", "t", "1", "yes":
			return pgtype.Bool{Bool: true, Valid: true}
		case "false", "f", "0", "no":
			return pgtype.Bool{Bool: false, Valid: true}
		default:
			return pgtype.Bool{}
		}
	default:
		return pgtype.Bool{}
	}
}

// CoerceDate converts any date-like value to pgtype.Date.
//
//	time.Time     → Date{Time: t, Valid: !t.IsZero()}
//	*time.Time    → Date{Time: *t, Valid: true} (nil → invalid)
//	string        → parsed with multiple formats (2006-01-02, RFC3339, etc.)
//	nil           → Date{Valid: false}
func CoerceDate(v any) pgtype.Date {
	if v == nil {
		return pgtype.Date{}
	}
	switch d := v.(type) {
	case time.Time:
		if d.IsZero() {
			return pgtype.Date{}
		}
		return pgtype.Date{Time: d, Valid: true}
	case *time.Time:
		if d == nil || d.IsZero() {
			return pgtype.Date{}
		}
		return pgtype.Date{Time: *d, Valid: true}
	case string:
		if d == "" || d == "null" {
			return pgtype.Date{}
		}
		t, err := ParseDate(d)
		if err != nil {
			return pgtype.Date{}
		}
		return pgtype.Date{Time: t, Valid: true}
	case *string:
		if d == nil || *d == "" {
			return pgtype.Date{}
		}
		t, err := ParseDate(*d)
		if err != nil {
			return pgtype.Date{}
		}
		return pgtype.Date{Time: t, Valid: true}
	default:
		return pgtype.Date{}
	}
}

// CoerceTimestamp converts any time-like value to pgtype.Timestamptz.
//
//	time.Time     → Timestamptz{Time: t, Valid: !t.IsZero()}
//	*time.Time    → Timestamptz{Time: *t, Valid: true} (nil → invalid)
//	string        → parsed with multiple formats
//	nil           → Timestamptz{Valid: false}
func CoerceTimestamp(v any) pgtype.Timestamptz {
	if v == nil {
		return pgtype.Timestamptz{}
	}
	switch t := v.(type) {
	case time.Time:
		if t.IsZero() {
			return pgtype.Timestamptz{}
		}
		return pgtype.Timestamptz{Time: t, Valid: true}
	case *time.Time:
		if t == nil || t.IsZero() {
			return pgtype.Timestamptz{}
		}
		return pgtype.Timestamptz{Time: *t, Valid: true}
	case string:
		if t == "" || t == "null" {
			return pgtype.Timestamptz{}
		}
		parsed, err := ParseDate(t)
		if err != nil {
			return pgtype.Timestamptz{}
		}
		return pgtype.Timestamptz{Time: parsed, Valid: true}
	default:
		return pgtype.Timestamptz{}
	}
}

// CoerceUUID converts any UUID-like value to pgtype.UUID.
//
//	uuid.UUID     → UUID{Bytes: u, Valid: true}
//	*uuid.UUID    → UUID{Bytes: *u, Valid: true} (nil → invalid)
//	string        → parsed via uuid.Parse
//	[16]byte      → UUID{Bytes: b, Valid: true}
//	nil           → UUID{Valid: false}
func CoerceUUID(v any) pgtype.UUID {
	if v == nil {
		return pgtype.UUID{}
	}
	switch u := v.(type) {
	case uuid.UUID:
		return pgtype.UUID{Bytes: u, Valid: true}
	case *uuid.UUID:
		if u == nil {
			return pgtype.UUID{}
		}
		return pgtype.UUID{Bytes: *u, Valid: true}
	case string:
		if u == "" {
			return pgtype.UUID{}
		}
		parsed, err := uuid.Parse(u)
		if err != nil {
			return pgtype.UUID{}
		}
		return pgtype.UUID{Bytes: parsed, Valid: true}
	case [16]byte:
		return pgtype.UUID{Bytes: u, Valid: true}
	default:
		return pgtype.UUID{}
	}
}
