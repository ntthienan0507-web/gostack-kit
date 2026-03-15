package types

import (
	"math"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ExtractText returns the string from a pgtype.Text, or "" if null.
func ExtractText(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

// ExtractInt4 returns the int32 from a pgtype.Int4, or 0 if null.
func ExtractInt4(i pgtype.Int4) int32 {
	if !i.Valid {
		return 0
	}
	return i.Int32
}

// ExtractInt8 returns the int64 from a pgtype.Int8, or 0 if null.
func ExtractInt8(i pgtype.Int8) int64 {
	if !i.Valid {
		return 0
	}
	return i.Int64
}

// ExtractFloat8 returns the float64 from a pgtype.Float8, or 0 if null.
func ExtractFloat8(f pgtype.Float8) float64 {
	if !f.Valid {
		return 0
	}
	return f.Float64
}

// ExtractBool returns the bool from a pgtype.Bool, or false if null.
func ExtractBool(b pgtype.Bool) bool {
	if !b.Valid {
		return false
	}
	return b.Bool
}

// ExtractDate returns a *time.Time from a pgtype.Date, or nil if null.
func ExtractDate(d pgtype.Date) *time.Time {
	if !d.Valid {
		return nil
	}
	t := d.Time
	return &t
}

// ExtractTimestamp returns a *time.Time from a pgtype.Timestamptz, or nil if null.
func ExtractTimestamp(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	ts := t.Time
	return &ts
}

// ExtractUUID returns a uuid.UUID from a pgtype.UUID, or uuid.Nil if null.
func ExtractUUID(u pgtype.UUID) uuid.UUID {
	if !u.Valid {
		return uuid.Nil
	}
	return u.Bytes
}

// ExtractNumericFloat64 returns a float64 from a pgtype.Numeric, or 0 if null.
// Uses big.Float for precise conversion.
func ExtractNumericFloat64(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}

	// Convert Int * 10^Exp to float64.
	if n.Int == nil {
		return 0
	}

	f := new(big.Float).SetInt(n.Int)
	if n.Exp != 0 {
		exp := new(big.Float).SetFloat64(math.Pow10(int(n.Exp)))
		f.Mul(f, exp)
	}

	result, _ := f.Float64()
	return result
}
