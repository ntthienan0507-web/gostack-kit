package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDate(t *testing.T) {
	t.Run("ISO format", func(t *testing.T) {
		result, err := ParseDate("2024-06-15")
		require.NoError(t, err)
		assert.Equal(t, 2024, result.Year())
		assert.Equal(t, time.June, result.Month())
		assert.Equal(t, 15, result.Day())
	})

	t.Run("RFC3339 format", func(t *testing.T) {
		result, err := ParseDate("2024-06-15T10:30:00Z")
		require.NoError(t, err)
		assert.Equal(t, 2024, result.Year())
	})

	t.Run("dd/mm/yyyy format", func(t *testing.T) {
		result, err := ParseDate("15/06/2024")
		require.NoError(t, err)
		assert.Equal(t, 2024, result.Year())
		assert.Equal(t, time.June, result.Month())
		assert.Equal(t, 15, result.Day())
	})

	t.Run("invalid format returns error", func(t *testing.T) {
		_, err := ParseDate("not-a-date")
		assert.Error(t, err)
	})

	t.Run("trims whitespace", func(t *testing.T) {
		result, err := ParseDate("  2024-06-15  ")
		require.NoError(t, err)
		assert.Equal(t, 15, result.Day())
	})
}

func TestParseDateOrNil(t *testing.T) {
	t.Run("valid date", func(t *testing.T) {
		result := ParseDateOrNil("2024-06-15")
		require.NotNil(t, result)
		assert.Equal(t, 15, result.Day())
	})

	t.Run("empty string returns nil", func(t *testing.T) {
		assert.Nil(t, ParseDateOrNil(""))
	})

	t.Run("whitespace only returns nil", func(t *testing.T) {
		assert.Nil(t, ParseDateOrNil("   "))
	})

	t.Run("invalid returns nil", func(t *testing.T) {
		assert.Nil(t, ParseDateOrNil("garbage"))
	})
}

func TestFormatDate(t *testing.T) {
	d := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	assert.Equal(t, "2024-06-15", FormatDate(d))
}

func TestFormatDateTime(t *testing.T) {
	d := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	assert.Equal(t, "2024-06-15T10:30:00Z", FormatDateTime(d))
}

func TestFormatDatePtr(t *testing.T) {
	t.Run("non-nil", func(t *testing.T) {
		d := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
		assert.Equal(t, "2024-06-15", FormatDatePtr(&d))
	})

	t.Run("nil returns empty", func(t *testing.T) {
		assert.Equal(t, "", FormatDatePtr(nil))
	})
}

func TestStringToInt32(t *testing.T) {
	assert.Equal(t, int32(42), StringToInt32("42"))
	assert.Equal(t, int32(-1), StringToInt32("-1"))
	assert.Equal(t, int32(0), StringToInt32(""))
	assert.Equal(t, int32(0), StringToInt32("abc"))
	assert.Equal(t, int32(10), StringToInt32("  10  "))
}

func TestStringToInt64(t *testing.T) {
	assert.Equal(t, int64(1000000), StringToInt64("1000000"))
	assert.Equal(t, int64(0), StringToInt64("nope"))
}

func TestStringToFloat64(t *testing.T) {
	assert.InDelta(t, 3.14, StringToFloat64("3.14"), 0.001)
	assert.Equal(t, float64(0), StringToFloat64(""))
	assert.Equal(t, float64(0), StringToFloat64("xyz"))
}

func TestRoundFloat64(t *testing.T) {
	assert.InDelta(t, 3.14, RoundFloat64(3.14159, 2), 0.001)
	assert.InDelta(t, 3.1, RoundFloat64(3.14159, 1), 0.01)
	assert.InDelta(t, 3.0, RoundFloat64(3.14159, 0), 0.1)
	assert.InDelta(t, 100.0, RoundFloat64(99.999, 0), 0.1)
}

func TestPtr(t *testing.T) {
	p := Ptr(42)
	require.NotNil(t, p)
	assert.Equal(t, 42, *p)

	s := Ptr("hello")
	require.NotNil(t, s)
	assert.Equal(t, "hello", *s)
}

func TestDeref(t *testing.T) {
	t.Run("non-nil", func(t *testing.T) {
		v := 42
		assert.Equal(t, 42, Deref(&v))
	})

	t.Run("nil returns zero value", func(t *testing.T) {
		var p *int
		assert.Equal(t, 0, Deref(p))
	})

	t.Run("nil string pointer returns empty", func(t *testing.T) {
		var p *string
		assert.Equal(t, "", Deref(p))
	})
}
