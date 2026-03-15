package types

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestSetText(t *testing.T) {
	t.Run("non-empty string", func(t *testing.T) {
		result := SetText("hello")
		assert.True(t, result.Valid)
		assert.Equal(t, "hello", result.String)
	})

	t.Run("empty string produces null", func(t *testing.T) {
		result := SetText("")
		assert.False(t, result.Valid)
		assert.Equal(t, "", result.String)
	})
}

func TestSetNullableText(t *testing.T) {
	t.Run("non-nil pointer", func(t *testing.T) {
		s := "world"
		result := SetNullableText(&s)
		assert.True(t, result.Valid)
		assert.Equal(t, "world", result.String)
	})

	t.Run("nil pointer produces null", func(t *testing.T) {
		result := SetNullableText(nil)
		assert.False(t, result.Valid)
	})

	t.Run("pointer to empty string is valid", func(t *testing.T) {
		s := ""
		result := SetNullableText(&s)
		assert.True(t, result.Valid)
		assert.Equal(t, "", result.String)
	})
}

func TestSetInt4(t *testing.T) {
	result := SetInt4(42)
	assert.True(t, result.Valid)
	assert.Equal(t, int32(42), result.Int32)

	result = SetInt4(0)
	assert.True(t, result.Valid)
	assert.Equal(t, int32(0), result.Int32)

	result = SetInt4(-1)
	assert.True(t, result.Valid)
	assert.Equal(t, int32(-1), result.Int32)
}

func TestSetInt8(t *testing.T) {
	result := SetInt8(1_000_000)
	assert.True(t, result.Valid)
	assert.Equal(t, int64(1_000_000), result.Int64)
}

func TestSetFloat8(t *testing.T) {
	result := SetFloat8(3.14)
	assert.True(t, result.Valid)
	assert.InDelta(t, 3.14, result.Float64, 0.001)

	result = SetFloat8(0)
	assert.True(t, result.Valid)
	assert.Equal(t, float64(0), result.Float64)
}

func TestSetBool(t *testing.T) {
	result := SetBool(true)
	assert.True(t, result.Valid)
	assert.True(t, result.Bool)

	result = SetBool(false)
	assert.True(t, result.Valid)
	assert.False(t, result.Bool)
}

func TestSetNullableBool(t *testing.T) {
	t.Run("true pointer", func(t *testing.T) {
		b := true
		result := SetNullableBool(&b)
		assert.True(t, result.Valid)
		assert.True(t, result.Bool)
	})

	t.Run("false pointer", func(t *testing.T) {
		b := false
		result := SetNullableBool(&b)
		assert.True(t, result.Valid)
		assert.False(t, result.Bool)
	})

	t.Run("nil produces null", func(t *testing.T) {
		result := SetNullableBool(nil)
		assert.False(t, result.Valid)
	})
}

func TestSetDate(t *testing.T) {
	t.Run("valid date", func(t *testing.T) {
		d := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
		result := SetDate(d)
		assert.True(t, result.Valid)
		assert.Equal(t, d, result.Time)
	})

	t.Run("zero time produces null", func(t *testing.T) {
		result := SetDate(time.Time{})
		assert.False(t, result.Valid)
	})
}

func TestSetNullableDate(t *testing.T) {
	t.Run("non-nil pointer", func(t *testing.T) {
		d := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		result := SetNullableDate(&d)
		assert.True(t, result.Valid)
		assert.Equal(t, d, result.Time)
	})

	t.Run("nil pointer produces null", func(t *testing.T) {
		result := SetNullableDate(nil)
		assert.False(t, result.Valid)
	})
}

func TestSetTimestamp(t *testing.T) {
	t.Run("valid timestamp", func(t *testing.T) {
		ts := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
		result := SetTimestamp(ts)
		assert.True(t, result.Valid)
		assert.Equal(t, ts, result.Time)
	})

	t.Run("zero time produces null", func(t *testing.T) {
		result := SetTimestamp(time.Time{})
		assert.False(t, result.Valid)
	})
}

func TestSetNullableTimestamp(t *testing.T) {
	t.Run("non-nil pointer", func(t *testing.T) {
		ts := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
		result := SetNullableTimestamp(&ts)
		assert.True(t, result.Valid)
		assert.Equal(t, ts, result.Time)
	})

	t.Run("nil pointer produces null", func(t *testing.T) {
		result := SetNullableTimestamp(nil)
		assert.False(t, result.Valid)
	})
}

func TestSetUUID(t *testing.T) {
	id := uuid.New()
	result := SetUUID(id)
	assert.True(t, result.Valid)
	assert.Equal(t, [16]byte(id), result.Bytes)
}

func TestSetNullableUUID(t *testing.T) {
	t.Run("non-nil", func(t *testing.T) {
		id := uuid.New()
		result := SetNullableUUID(&id)
		assert.True(t, result.Valid)
		assert.Equal(t, [16]byte(id), result.Bytes)
	})

	t.Run("nil produces null", func(t *testing.T) {
		result := SetNullableUUID(nil)
		assert.False(t, result.Valid)
	})
}

func TestSetJSON(t *testing.T) {
	t.Run("struct value", func(t *testing.T) {
		v := map[string]string{"key": "value"}
		result := SetJSON(v)
		assert.JSONEq(t, `{"key":"value"}`, string(result))
	})

	t.Run("nil value returns empty object", func(t *testing.T) {
		result := SetJSON(nil)
		assert.Equal(t, json.RawMessage("{}"), result)
	})

	t.Run("unmarshalable value returns empty object", func(t *testing.T) {
		result := SetJSON(make(chan int))
		assert.Equal(t, json.RawMessage("{}"), result)
	})
}
