package types

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// SetText converts a string to pgtype.Text. An empty string produces an invalid (null) value.
func SetText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

// SetNullableText converts a *string to pgtype.Text. A nil pointer produces an invalid (null) value.
func SetNullableText(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

// SetInt4 converts an int32 to pgtype.Int4.
func SetInt4(i int32) pgtype.Int4 {
	return pgtype.Int4{Int32: i, Valid: true}
}

// SetInt8 converts an int64 to pgtype.Int8.
func SetInt8(i int64) pgtype.Int8 {
	return pgtype.Int8{Int64: i, Valid: true}
}

// SetFloat8 converts a float64 to pgtype.Float8.
func SetFloat8(f float64) pgtype.Float8 {
	return pgtype.Float8{Float64: f, Valid: true}
}

// SetBool converts a bool to pgtype.Bool.
func SetBool(b bool) pgtype.Bool {
	return pgtype.Bool{Bool: b, Valid: true}
}

// SetNullableBool converts a *bool to pgtype.Bool. A nil pointer produces an invalid (null) value.
func SetNullableBool(b *bool) pgtype.Bool {
	if b == nil {
		return pgtype.Bool{}
	}
	return pgtype.Bool{Bool: *b, Valid: true}
}

// SetDate converts a time.Time to pgtype.Date. A zero time produces an invalid (null) value.
func SetDate(t time.Time) pgtype.Date {
	if t.IsZero() {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: t, Valid: true}
}

// SetNullableDate converts a *time.Time to pgtype.Date. A nil pointer produces an invalid (null) value.
func SetNullableDate(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *t, Valid: true}
}

// SetTimestamp converts a time.Time to pgtype.Timestamptz. A zero time produces an invalid (null) value.
func SetTimestamp(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// SetNullableTimestamp converts a *time.Time to pgtype.Timestamptz. A nil pointer produces an invalid (null) value.
func SetNullableTimestamp(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// SetUUID converts a uuid.UUID to pgtype.UUID.
func SetUUID(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: true}
}

// SetNullableUUID converts a *uuid.UUID to pgtype.UUID. A nil pointer produces an invalid (null) value.
func SetNullableUUID(u *uuid.UUID) pgtype.UUID {
	if u == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *u, Valid: true}
}

// SetJSON marshals any value to json.RawMessage. Returns "{}" if the value is nil or marshalling fails.
func SetJSON(v any) json.RawMessage {
	if v == nil {
		return json.RawMessage("{}")
	}
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("{}")
	}
	return data
}
