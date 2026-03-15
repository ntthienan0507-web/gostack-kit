package types

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// ============================================================================
// Query Filter Parsing
// ============================================================================
//
// Client sends filters as a single base64-encoded JSON string in query param:
//
//   GET /api/v1/orders?filters=eyJzdGF0dXMiOiJwZW5kaW5nIiwidXNlcl9pZCI6IjEyMyJ9
//
// This avoids:
//   - Long query strings with many &key=value params
//   - Complex nested filter syntax (?status=active&role=admin&date_gte=2024-01-01)
//   - URL encoding issues with special characters
//   - Query param naming conflicts
//
// Backend decodes:
//   base64 → JSON → typed struct
//
//   type OrderFilters struct {
//       Status string    `json:"status"`
//       UserID string    `json:"user_id"`
//       DateGTE *string  `json:"date_gte"`
//   }
//   filters, err := types.ParseFilters[OrderFilters](ctx.Query("filters"))
// ============================================================================

// ParseFilters decodes a base64-encoded JSON filter string into a typed struct.
// Supports both standard base64 and URL-safe base64 (with or without padding).
// Returns zero value if input is empty.
//
// Example:
//
//	// Client sends: ?filters=eyJzdGF0dXMiOiJhY3RpdmUiLCJyb2xlIjoiYWRtaW4ifQ==
//	type UserFilters struct {
//	    Status string `json:"status"`
//	    Role   string `json:"role"`
//	}
//	filters, err := types.ParseFilters[UserFilters](ctx.Query("filters"))
//	// filters = UserFilters{Status: "active", Role: "admin"}
func ParseFilters[T any](encoded string) (T, error) {
	var result T

	if encoded == "" {
		return result, nil
	}

	// Trim quotes (frontend sometimes sends JSON.stringify(base64string))
	encoded = strings.Trim(encoded, "\"")

	// Decode base64 (try standard first, then URL-safe)
	decoded, err := decodeBase64(encoded)
	if err != nil {
		return result, fmt.Errorf("decode filters: %w", err)
	}

	// Unmarshal JSON into typed struct
	if err := json.Unmarshal(decoded, &result); err != nil {
		return result, fmt.Errorf("parse filters JSON: %w", err)
	}

	return result, nil
}

// MustParseFilters is like ParseFilters but returns zero value on error.
// Use in controllers where you want to treat bad filters as "no filters".
func MustParseFilters[T any](encoded string) T {
	result, _ := ParseFilters[T](encoded)
	return result
}

// EncodeFilters encodes a filter struct to base64 JSON string.
// Useful for building URLs in tests or internal service calls.
//
//	encoded := types.EncodeFilters(UserFilters{Status: "active", Role: "admin"})
//	// "eyJzdGF0dXMiOiJhY3RpdmUiLCJyb2xlIjoiYWRtaW4ifQ=="
func EncodeFilters(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(data)
}

// decodeBase64 tries standard encoding first, falls back to URL-safe, then raw (no padding).
func decodeBase64(s string) ([]byte, error) {
	// Standard base64
	if decoded, err := base64.StdEncoding.DecodeString(s); err == nil {
		return decoded, nil
	}

	// URL-safe base64 (with padding)
	if decoded, err := base64.URLEncoding.DecodeString(s); err == nil {
		return decoded, nil
	}

	// Raw URL-safe base64 (no padding — common from browsers)
	if decoded, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return decoded, nil
	}

	// Raw standard (no padding)
	if decoded, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return decoded, nil
	}

	return nil, fmt.Errorf("invalid base64 encoding (tried standard, URL-safe, and raw variants)")
}

// ============================================================================
// Struct to Map Conversion
// ============================================================================

// StructToMap converts a struct to map[string]any using JSON tags as keys.
// Optionally skip specified fields. Useful for building dynamic WHERE clauses
// or PATCH-style partial updates.
//
// Example:
//
//	type UpdateParams struct {
//	    Name   string `json:"name"`
//	    Email  string `json:"email"`
//	    Secret string `json:"secret"`
//	}
//	m := types.StructToMap(params, "secret")
//	// {"name": "John", "email": "john@example.com"} — secret excluded
func StructToMap(obj any, skip ...string) map[string]any {
	skipSet := make(map[string]bool, len(skip))
	for _, s := range skip {
		skipSet[s] = true
	}

	result := make(map[string]any)

	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return result
		}
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return result
	}

	typ := val.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}

		// Use json tag as key, fallback to field name
		key := field.Tag.Get("json")
		if key == "" || key == "-" {
			key = field.Name
		}
		// Handle json:"name,omitempty" — take only the name part
		if idx := strings.Index(key, ","); idx != -1 {
			key = key[:idx]
		}

		if skipSet[key] {
			continue
		}

		result[key] = val.Field(i).Interface()
	}

	return result
}

// NonZeroFields returns a map of only the non-zero-value fields from a struct.
// Useful for building PATCH updates — only include fields the client actually sent.
//
// Example:
//
//	type UpdateReq struct {
//	    Name  *string `json:"name"`   // nil = not sent
//	    Email *string `json:"email"`  // nil = not sent
//	    Role  *string `json:"role"`   // set = sent
//	}
//	// Client sends: {"role": "admin"}
//	m := types.NonZeroFields(req)
//	// {"role": "admin"} — name and email excluded because nil
func NonZeroFields(obj any) map[string]any {
	result := make(map[string]any)

	val := reflect.ValueOf(obj)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return result
		}
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return result
	}

	typ := val.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		if !field.IsExported() {
			continue
		}

		// Skip zero values (nil pointers, empty strings, 0, false)
		if fieldVal.IsZero() {
			continue
		}

		// Dereference pointer for the value
		actualVal := fieldVal.Interface()
		if fieldVal.Kind() == reflect.Ptr && !fieldVal.IsNil() {
			actualVal = fieldVal.Elem().Interface()
		}

		key := field.Tag.Get("json")
		if key == "" || key == "-" {
			key = field.Name
		}
		if idx := strings.Index(key, ","); idx != -1 {
			key = key[:idx]
		}

		result[key] = actualVal
	}

	return result
}
