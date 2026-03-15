package types

import (
	"reflect"
	"sync"
)

// ============================================================================
// Struct Field Copying (reflection-based)
// ============================================================================
//
// Use case: SQLC generates separate structs for query vs count params.
// They share the same fields EXCEPT Limit/Offset.
//
//   db.ListUsersParams       → { Search, Role, Status, Limit, Offset }
//   db.CountUsersParams      → { Search, Role, Status }
//
// Instead of copying field-by-field every time:
//
//   countParams := types.CopyFields[db.CountUsersParams](queryParams)
//
// HOW: Reflects over source fields, copies matching fields (same name + type)
// to target. Skips Limit, Offset, and unexported fields automatically.
//
// PERFORMANCE: Field mapping is cached per type pair — reflect runs once per
// unique (Source, Target) combination, then it's a direct field copy.
//
// WHEN TO USE:
//   - SQLC query/count param conversion (the main use case)
//   - Copying between similar structs from different packages
//
// WHEN NOT TO USE:
//   - Copying 2-3 fields (just write it explicitly — clearer)
//   - Performance-critical hot paths (use explicit assignment)
//   - Structs with different field semantics but same names
// ============================================================================

// skipFields are fields that should never be copied (pagination-specific).
var skipFields = map[string]bool{
	"Limit":  true,
	"Offset": true,
}

// CopyFields copies matching fields from source to a new instance of T.
// Fields are matched by name AND type — mismatched types are silently skipped.
// Limit and Offset fields are always skipped.
//
// Example:
//
//	// SQLC generated:
//	// type ListUsersParams struct { Search pgtype.Text; Role pgtype.Text; Limit int32; Offset int32 }
//	// type CountUsersParams struct { Search pgtype.Text; Role pgtype.Text }
//
//	queryParams := db.ListUsersParams{Search: "john", Role: "admin", Limit: 20, Offset: 0}
//	countParams := types.CopyFields[db.CountUsersParams](queryParams)
//	// countParams = {Search: "john", Role: "admin"} — Limit/Offset stripped
func CopyFields[T any](source any) T {
	var target T

	sourceVal := reflect.ValueOf(source)
	targetVal := reflect.ValueOf(&target).Elem()

	// Dereference pointer if needed
	if sourceVal.Kind() == reflect.Ptr {
		if sourceVal.IsNil() {
			return target
		}
		sourceVal = sourceVal.Elem()
	}

	if sourceVal.Kind() != reflect.Struct || targetVal.Kind() != reflect.Struct {
		return target
	}

	mapping := getFieldMapping(sourceVal.Type(), targetVal.Type())

	for _, m := range mapping {
		targetVal.Field(m.targetIdx).Set(sourceVal.Field(m.sourceIdx))
	}

	return target
}

// --- Field mapping cache ---

type fieldMap struct {
	sourceIdx int
	targetIdx int
}

var (
	mappingCache sync.Map // key: "source.PkgPath.Name→target.PkgPath.Name"
)

func getFieldMapping(sourceType, targetType reflect.Type) []fieldMap {
	key := sourceType.PkgPath() + "." + sourceType.Name() + "→" + targetType.PkgPath() + "." + targetType.Name()

	if cached, ok := mappingCache.Load(key); ok {
		return cached.([]fieldMap)
	}

	// Build mapping: for each target field, find matching source field
	var mapping []fieldMap

	for i := 0; i < targetType.NumField(); i++ {
		tf := targetType.Field(i)

		// Skip unexported fields
		if !tf.IsExported() {
			continue
		}

		// Skip pagination fields
		if skipFields[tf.Name] {
			continue
		}

		// Find matching source field (same name + same type)
		for j := 0; j < sourceType.NumField(); j++ {
			sf := sourceType.Field(j)
			if sf.Name == tf.Name && sf.Type == tf.Type {
				mapping = append(mapping, fieldMap{sourceIdx: j, targetIdx: i})
				break
			}
		}
	}

	mappingCache.Store(key, mapping)
	return mapping
}
