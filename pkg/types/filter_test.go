package types

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- ParseFilters ---

type testFilters struct {
	Status string  `json:"status"`
	Role   string  `json:"role"`
	Page   int     `json:"page,omitempty"`
	Search *string `json:"search,omitempty"`
}

func TestParseFilters_Standard(t *testing.T) {
	// {"status":"active","role":"admin"}
	encoded := base64.StdEncoding.EncodeToString([]byte(`{"status":"active","role":"admin"}`))

	result, err := ParseFilters[testFilters](encoded)

	require.NoError(t, err)
	assert.Equal(t, "active", result.Status)
	assert.Equal(t, "admin", result.Role)
}

func TestParseFilters_URLSafe(t *testing.T) {
	encoded := base64.URLEncoding.EncodeToString([]byte(`{"status":"active"}`))

	result, err := ParseFilters[testFilters](encoded)

	require.NoError(t, err)
	assert.Equal(t, "active", result.Status)
}

func TestParseFilters_RawNoPadding(t *testing.T) {
	// Browsers often strip padding
	encoded := base64.RawURLEncoding.EncodeToString([]byte(`{"status":"pending"}`))

	result, err := ParseFilters[testFilters](encoded)

	require.NoError(t, err)
	assert.Equal(t, "pending", result.Status)
}

func TestParseFilters_QuotedInput(t *testing.T) {
	// Frontend sends JSON.stringify(base64string) — extra quotes
	raw := base64.StdEncoding.EncodeToString([]byte(`{"role":"user"}`))
	quoted := `"` + raw + `"`

	result, err := ParseFilters[testFilters](quoted)

	require.NoError(t, err)
	assert.Equal(t, "user", result.Role)
}

func TestParseFilters_Empty(t *testing.T) {
	result, err := ParseFilters[testFilters]("")

	require.NoError(t, err)
	assert.Equal(t, testFilters{}, result)
}

func TestParseFilters_InvalidBase64(t *testing.T) {
	_, err := ParseFilters[testFilters]("not-base64!!!")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode filters")
}

func TestParseFilters_InvalidJSON(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte(`{invalid json}`))

	_, err := ParseFilters[testFilters](encoded)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse filters JSON")
}

func TestParseFilters_WithOptionalFields(t *testing.T) {
	search := "john"
	encoded := EncodeFilters(testFilters{Status: "active", Search: &search, Page: 2})

	result, err := ParseFilters[testFilters](encoded)

	require.NoError(t, err)
	assert.Equal(t, "active", result.Status)
	assert.Equal(t, 2, result.Page)
	require.NotNil(t, result.Search)
	assert.Equal(t, "john", *result.Search)
}

// --- MustParseFilters ---

func TestMustParseFilters_Success(t *testing.T) {
	encoded := EncodeFilters(testFilters{Status: "active"})

	result := MustParseFilters[testFilters](encoded)

	assert.Equal(t, "active", result.Status)
}

func TestMustParseFilters_Error_ReturnsZero(t *testing.T) {
	result := MustParseFilters[testFilters]("garbage")

	assert.Equal(t, testFilters{}, result)
}

// --- EncodeFilters ---

func TestEncodeFilters_RoundTrip(t *testing.T) {
	original := testFilters{Status: "shipped", Role: "admin", Page: 3}

	encoded := EncodeFilters(original)
	decoded, err := ParseFilters[testFilters](encoded)

	require.NoError(t, err)
	assert.Equal(t, original.Status, decoded.Status)
	assert.Equal(t, original.Role, decoded.Role)
	assert.Equal(t, original.Page, decoded.Page)
}

// --- StructToMap ---

func TestStructToMap(t *testing.T) {
	type Params struct {
		Name   string `json:"name"`
		Email  string `json:"email"`
		Secret string `json:"secret"`
	}

	m := StructToMap(Params{Name: "John", Email: "j@b.com", Secret: "s3cr3t"}, "secret")

	assert.Equal(t, "John", m["name"])
	assert.Equal(t, "j@b.com", m["email"])
	assert.NotContains(t, m, "secret")
}

func TestStructToMap_NoSkip(t *testing.T) {
	type Params struct {
		A string `json:"a"`
		B int    `json:"b"`
	}

	m := StructToMap(Params{A: "hello", B: 42})

	assert.Equal(t, "hello", m["a"])
	assert.Equal(t, 42, m["b"])
}

func TestStructToMap_Pointer(t *testing.T) {
	type P struct {
		X string `json:"x"`
	}
	m := StructToMap(&P{X: "ptr"})

	assert.Equal(t, "ptr", m["x"])
}

func TestStructToMap_NilPointer(t *testing.T) {
	type P struct{ X string }
	m := StructToMap((*P)(nil))

	assert.Empty(t, m)
}

func TestStructToMap_OmitemptyTag(t *testing.T) {
	type P struct {
		Name string `json:"name,omitempty"`
	}
	m := StructToMap(P{Name: "test"})

	assert.Equal(t, "test", m["name"]) // key is "name" not "name,omitempty"
}

// --- NonZeroFields ---

func TestNonZeroFields(t *testing.T) {
	type UpdateReq struct {
		Name  *string `json:"name"`
		Email *string `json:"email"`
		Role  *string `json:"role"`
		Age   int     `json:"age"`
	}

	role := "admin"
	m := NonZeroFields(UpdateReq{Role: &role})

	assert.Equal(t, "admin", m["role"])
	assert.NotContains(t, m, "name")  // nil pointer = zero
	assert.NotContains(t, m, "email") // nil pointer = zero
	assert.NotContains(t, m, "age")   // 0 = zero
}

func TestNonZeroFields_AllSet(t *testing.T) {
	type Req struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	m := NonZeroFields(Req{Name: "John", Age: 30})

	assert.Len(t, m, 2)
	assert.Equal(t, "John", m["name"])
	assert.Equal(t, 30, m["age"])
}

func TestNonZeroFields_Empty(t *testing.T) {
	type Req struct {
		Name string `json:"name"`
	}

	m := NonZeroFields(Req{})

	assert.Empty(t, m)
}
