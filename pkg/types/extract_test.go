package types

import (
	"math/big"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

func TestExtractText(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		assert.Equal(t, "hello", ExtractText(pgtype.Text{String: "hello", Valid: true}))
	})

	t.Run("null returns empty", func(t *testing.T) {
		assert.Equal(t, "", ExtractText(pgtype.Text{}))
	})
}

func TestExtractInt4(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		assert.Equal(t, int32(42), ExtractInt4(pgtype.Int4{Int32: 42, Valid: true}))
	})

	t.Run("null returns zero", func(t *testing.T) {
		assert.Equal(t, int32(0), ExtractInt4(pgtype.Int4{}))
	})
}

func TestExtractInt8(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		assert.Equal(t, int64(999), ExtractInt8(pgtype.Int8{Int64: 999, Valid: true}))
	})

	t.Run("null returns zero", func(t *testing.T) {
		assert.Equal(t, int64(0), ExtractInt8(pgtype.Int8{}))
	})
}

func TestExtractFloat8(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		assert.InDelta(t, 3.14, ExtractFloat8(pgtype.Float8{Float64: 3.14, Valid: true}), 0.001)
	})

	t.Run("null returns zero", func(t *testing.T) {
		assert.Equal(t, float64(0), ExtractFloat8(pgtype.Float8{}))
	})
}

func TestExtractBool(t *testing.T) {
	t.Run("valid true", func(t *testing.T) {
		assert.True(t, ExtractBool(pgtype.Bool{Bool: true, Valid: true}))
	})

	t.Run("valid false", func(t *testing.T) {
		assert.False(t, ExtractBool(pgtype.Bool{Bool: false, Valid: true}))
	})

	t.Run("null returns false", func(t *testing.T) {
		assert.False(t, ExtractBool(pgtype.Bool{}))
	})
}

func TestExtractDate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		d := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
		result := ExtractDate(pgtype.Date{Time: d, Valid: true})
		assert.NotNil(t, result)
		assert.Equal(t, d, *result)
	})

	t.Run("null returns nil", func(t *testing.T) {
		assert.Nil(t, ExtractDate(pgtype.Date{}))
	})
}

func TestExtractTimestamp(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		ts := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
		result := ExtractTimestamp(pgtype.Timestamptz{Time: ts, Valid: true})
		assert.NotNil(t, result)
		assert.Equal(t, ts, *result)
	})

	t.Run("null returns nil", func(t *testing.T) {
		assert.Nil(t, ExtractTimestamp(pgtype.Timestamptz{}))
	})
}

func TestExtractUUID(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		id := uuid.New()
		result := ExtractUUID(pgtype.UUID{Bytes: id, Valid: true})
		assert.Equal(t, id, result)
	})

	t.Run("null returns uuid.Nil", func(t *testing.T) {
		assert.Equal(t, uuid.Nil, ExtractUUID(pgtype.UUID{}))
	})
}

func TestExtractNumericFloat64(t *testing.T) {
	t.Run("valid integer", func(t *testing.T) {
		n := pgtype.Numeric{Int: big.NewInt(12345), Exp: 0, Valid: true}
		assert.InDelta(t, 12345.0, ExtractNumericFloat64(n), 0.001)
	})

	t.Run("valid decimal", func(t *testing.T) {
		// 12345 * 10^-2 = 123.45
		n := pgtype.Numeric{Int: big.NewInt(12345), Exp: -2, Valid: true}
		assert.InDelta(t, 123.45, ExtractNumericFloat64(n), 0.001)
	})

	t.Run("null returns zero", func(t *testing.T) {
		assert.Equal(t, float64(0), ExtractNumericFloat64(pgtype.Numeric{}))
	})

	t.Run("nil Int returns zero", func(t *testing.T) {
		assert.Equal(t, float64(0), ExtractNumericFloat64(pgtype.Numeric{Valid: true}))
	})
}
