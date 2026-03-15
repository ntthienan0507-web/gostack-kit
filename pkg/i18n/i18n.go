package i18n

import (
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// Locale represents a language code.
type Locale string

const (
	EN Locale = "en"
	VI Locale = "vi"
)

const translatorKey = "i18n_translator"

// Translator maps error codes to localized messages.
type Translator struct {
	mu       sync.RWMutex
	messages map[Locale]map[string]string // locale → code → message
	fallback Locale                       // default locale if requested not found
}

// New creates a Translator with the given fallback locale and registers
// default EN + VI messages for all common.* error codes.
func New(fallback Locale) *Translator {
	t := &Translator{
		messages: make(map[Locale]map[string]string),
		fallback: fallback,
	}
	t.registerDefaults()
	return t
}

// Register adds a message for a locale and error code.
func (t *Translator) Register(locale Locale, code string, message string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.messages[locale] == nil {
		t.messages[locale] = make(map[string]string)
	}
	t.messages[locale][code] = message
}

// RegisterBatch adds multiple messages at once.
func (t *Translator) RegisterBatch(locale Locale, messages map[string]string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.messages[locale] == nil {
		t.messages[locale] = make(map[string]string)
	}
	for code, msg := range messages {
		t.messages[locale][code] = msg
	}
}

// Translate returns the localized message for a code.
// Falls back to fallback locale, then the code itself.
func (t *Translator) Translate(locale Locale, code string) string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Try requested locale
	if msgs, ok := t.messages[locale]; ok {
		if msg, ok := msgs[code]; ok {
			return msg
		}
	}

	// Try fallback locale
	if locale != t.fallback {
		if msgs, ok := t.messages[t.fallback]; ok {
			if msg, ok := msgs[code]; ok {
				return msg
			}
		}
	}

	// Return code itself as last resort
	return code
}

// TranslateFromHeader reads Accept-Language header value and translates.
func (t *Translator) TranslateFromHeader(acceptLanguage string, code string) string {
	locale := parseAcceptLanguage(acceptLanguage)
	return t.Translate(locale, code)
}

// Middleware injects the translator into gin context so handlers can access it.
func (t *Translator) Middleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Set(translatorKey, t)
		ctx.Next()
	}
}

// GetTranslator retrieves translator from gin context.
func GetTranslator(ctx *gin.Context) *Translator {
	v, exists := ctx.Get(translatorKey)
	if !exists {
		return nil
	}
	t, _ := v.(*Translator)
	return t
}

// GetLocale extracts locale from the "lang" query param first,
// then falls back to the Accept-Language header. Defaults to EN.
func GetLocale(ctx *gin.Context) Locale {
	// Check query param first
	if lang := ctx.Query("lang"); lang != "" {
		return normalizeLocale(lang)
	}

	// Fall back to Accept-Language header
	return parseAcceptLanguage(ctx.GetHeader("Accept-Language"))
}

// parseAcceptLanguage extracts the primary locale from an Accept-Language value.
// It takes the first language tag and normalises it.
func parseAcceptLanguage(header string) Locale {
	header = strings.TrimSpace(header)
	if header == "" {
		return EN
	}

	// Accept-Language can be "en-US,en;q=0.9,vi;q=0.8" — take first tag.
	tag := header
	if idx := strings.IndexAny(tag, ",;"); idx != -1 {
		tag = tag[:idx]
	}

	return normalizeLocale(tag)
}

// normalizeLocale maps a language tag (e.g. "vi", "vi-VN", "en-US") to a Locale.
func normalizeLocale(tag string) Locale {
	tag = strings.TrimSpace(strings.ToLower(tag))

	// Extract primary subtag (before any hyphen).
	if idx := strings.IndexByte(tag, '-'); idx != -1 {
		tag = tag[:idx]
	}

	switch tag {
	case "vi":
		return VI
	default:
		return EN
	}
}

// registerDefaults adds EN + VI messages for all common.* error codes.
func (t *Translator) registerDefaults() {
	t.RegisterBatch(EN, map[string]string{
		"common.bad_request":            "Invalid request",
		"common.invalid_params":         "Invalid request parameters",
		"common.required_field_missing":  "A required field is missing",
		"common.validation_failed":      "Request validation failed",
		"common.unauthorized":           "Authentication required",
		"common.token_missing":          "Missing bearer token",
		"common.token_invalid":          "Invalid or expired token",
		"common.forbidden":             "Insufficient permissions",
		"common.record_not_found":       "Record not found",
		"common.route_not_found":        "Route not found",
		"common.record_already_exists":  "Record already exists",
		"common.stale_version":          "The record was modified by another request. Please retry with the latest version.",
		"common.related_record_not_found": "Related record not found",
		"common.rate_limited":           "Too many requests, please try again later",
		"common.internal_error":         "An internal error occurred",
	})

	t.RegisterBatch(VI, map[string]string{
		"common.bad_request":            "Yeu cau khong hop le",
		"common.invalid_params":         "Tham so yeu cau khong hop le",
		"common.required_field_missing":  "Thieu truong bat buoc",
		"common.validation_failed":      "Xac thuc yeu cau that bai",
		"common.unauthorized":           "Yeu cau xac thuc",
		"common.token_missing":          "Thieu token xac thuc",
		"common.token_invalid":          "Token khong hop le hoac da het han",
		"common.forbidden":             "Khong du quyen truy cap",
		"common.record_not_found":       "Khong tim thay ban ghi",
		"common.route_not_found":        "Khong tim thay duong dan",
		"common.record_already_exists":  "Ban ghi da ton tai",
		"common.stale_version":          "Ban ghi da bi thay doi boi yeu cau khac. Vui long thu lai voi phien ban moi nhat.",
		"common.related_record_not_found": "Khong tim thay ban ghi lien quan",
		"common.rate_limited":           "Qua nhieu yeu cau, vui long thu lai sau",
		"common.internal_error":         "Da xay ra loi he thong",
	})
}
