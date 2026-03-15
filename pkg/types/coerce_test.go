package types

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

// --- CoerceText ---

func TestCoerceText(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  pgtype.Text
	}{
		{"string", "hello", pgtype.Text{String: "hello", Valid: true}},
		{"empty string", "", pgtype.Text{Valid: false}},
		{"*string", Ptr("world"), pgtype.Text{String: "world", Valid: true}},
		{"nil *string", (*string)(nil), pgtype.Text{}},
		{"[]byte", []byte("bytes"), pgtype.Text{String: "bytes", Valid: true}},
		{"int (fmt.Sprint)", 42, pgtype.Text{String: "42", Valid: true}},
		{"nil", nil, pgtype.Text{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CoerceText(tt.input))
		})
	}
}

// --- CoerceInt4 ---

func TestCoerceInt4(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  pgtype.Int4
	}{
		{"int", 42, pgtype.Int4{Int32: 42, Valid: true}},
		{"int32", int32(100), pgtype.Int4{Int32: 100, Valid: true}},
		{"int64", int64(200), pgtype.Int4{Int32: 200, Valid: true}},
		{"float64", float64(3.9), pgtype.Int4{Int32: 3, Valid: true}},
		{"string", "123", pgtype.Int4{Int32: 123, Valid: true}},
		{"invalid string", "abc", pgtype.Int4{}},
		{"json.Number", json.Number("456"), pgtype.Int4{Int32: 456, Valid: true}},
		{"*int32", Ptr(int32(77)), pgtype.Int4{Int32: 77, Valid: true}},
		{"nil *int32", (*int32)(nil), pgtype.Int4{}},
		{"nil", nil, pgtype.Int4{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CoerceInt4(tt.input))
		})
	}
}

// --- CoerceInt8 ---

func TestCoerceInt8(t *testing.T) {
	assert.Equal(t, pgtype.Int8{Int64: 42, Valid: true}, CoerceInt8(42))
	assert.Equal(t, pgtype.Int8{Int64: 100, Valid: true}, CoerceInt8("100"))
	assert.Equal(t, pgtype.Int8{Int64: 99, Valid: true}, CoerceInt8(int64(99)))
	assert.Equal(t, pgtype.Int8{}, CoerceInt8(nil))
	assert.Equal(t, pgtype.Int8{}, CoerceInt8("bad"))
}

// --- CoerceFloat8 ---

func TestCoerceFloat8(t *testing.T) {
	assert.Equal(t, pgtype.Float8{Float64: 3.14, Valid: true}, CoerceFloat8(3.14))
	assert.Equal(t, pgtype.Float8{Float64: 42, Valid: true}, CoerceFloat8(42))
	assert.Equal(t, pgtype.Float8{Float64: 1.5, Valid: true}, CoerceFloat8("1.5"))
	assert.Equal(t, pgtype.Float8{Float64: 9.9, Valid: true}, CoerceFloat8(json.Number("9.9")))
	assert.Equal(t, pgtype.Float8{}, CoerceFloat8(nil))
	assert.Equal(t, pgtype.Float8{}, CoerceFloat8("bad"))
}

// --- CoerceBool ---

func TestCoerceBool(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  pgtype.Bool
	}{
		{"true", true, pgtype.Bool{Bool: true, Valid: true}},
		{"false", false, pgtype.Bool{Bool: false, Valid: true}},
		{"int 1", 1, pgtype.Bool{Bool: true, Valid: true}},
		{"int 0", 0, pgtype.Bool{Bool: false, Valid: true}},
		{"string true", "true", pgtype.Bool{Bool: true, Valid: true}},
		{"string t", "t", pgtype.Bool{Bool: true, Valid: true}},
		{"string 1", "1", pgtype.Bool{Bool: true, Valid: true}},
		{"string false", "false", pgtype.Bool{Bool: false, Valid: true}},
		{"string f", "f", pgtype.Bool{Bool: false, Valid: true}},
		{"string yes", "yes", pgtype.Bool{Bool: true, Valid: true}},
		{"string no", "no", pgtype.Bool{Bool: false, Valid: true}},
		{"string garbage", "maybe", pgtype.Bool{}},
		{"*bool true", Ptr(true), pgtype.Bool{Bool: true, Valid: true}},
		{"nil *bool", (*bool)(nil), pgtype.Bool{}},
		{"nil", nil, pgtype.Bool{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CoerceBool(tt.input))
		})
	}
}

// --- CoerceDate ---

func TestCoerceDate(t *testing.T) {
	now := time.Now()
	d := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		input   any
		valid   bool
		checkFn func(pgtype.Date)
	}{
		{"time.Time", now, true, nil},
		{"zero time", time.Time{}, false, nil},
		{"*time.Time", &now, true, nil},
		{"nil *time.Time", (*time.Time)(nil), false, nil},
		{"string ISO", "2024-06-15", true, func(dt pgtype.Date) {
			assert.Equal(t, d, dt.Time)
		}},
		{"string RFC3339", "2024-06-15T10:30:00Z", true, nil},
		{"empty string", "", false, nil},
		{"null string", "null", false, nil},
		{"nil", nil, false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CoerceDate(tt.input)
			assert.Equal(t, tt.valid, result.Valid)
			if tt.checkFn != nil {
				tt.checkFn(result)
			}
		})
	}
}

// --- CoerceTimestamp ---

func TestCoerceTimestamp(t *testing.T) {
	now := time.Now()
	assert.True(t, CoerceTimestamp(now).Valid)
	assert.True(t, CoerceTimestamp(&now).Valid)
	assert.True(t, CoerceTimestamp("2024-06-15T10:30:00Z").Valid)
	assert.False(t, CoerceTimestamp(nil).Valid)
	assert.False(t, CoerceTimestamp("").Valid)
	assert.False(t, CoerceTimestamp(time.Time{}).Valid)
}

// --- CoerceUUID ---

func TestCoerceUUID(t *testing.T) {
	id := uuid.New()

	tests := []struct {
		name  string
		input any
		valid bool
	}{
		{"uuid.UUID", id, true},
		{"*uuid.UUID", &id, true},
		{"nil *uuid.UUID", (*uuid.UUID)(nil), false},
		{"string", id.String(), true},
		{"empty string", "", false},
		{"invalid string", "not-a-uuid", false},
		{"[16]byte", [16]byte(id), true},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CoerceUUID(tt.input)
			assert.Equal(t, tt.valid, result.Valid)
			if tt.valid {
				assert.Equal(t, id, uuid.UUID(result.Bytes))
			}
		})
	}
}
