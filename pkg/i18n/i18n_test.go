package i18n

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// --- Translate ---

func TestTranslate_EN(t *testing.T) {
	tr := New(EN)
	msg := tr.Translate(EN, "common.bad_request")
	assert.Equal(t, "Invalid request", msg)
}

func TestTranslate_VI(t *testing.T) {
	tr := New(EN)
	msg := tr.Translate(VI, "common.bad_request")
	assert.Equal(t, "Yeu cau khong hop le", msg)
}

func TestTranslate_FallbackToDefault(t *testing.T) {
	tr := New(EN)
	// Request a locale that has no messages — should fall back to EN.
	msg := tr.Translate(Locale("fr"), "common.unauthorized")
	assert.Equal(t, "Authentication required", msg)
}

func TestTranslate_FallbackToCode(t *testing.T) {
	tr := New(EN)
	msg := tr.Translate(EN, "unknown.code")
	assert.Equal(t, "unknown.code", msg)
}

func TestTranslate_CustomMessage(t *testing.T) {
	tr := New(EN)
	tr.Register(EN, "user.not_found", "User not found")
	msg := tr.Translate(EN, "user.not_found")
	assert.Equal(t, "User not found", msg)
}

// --- RegisterBatch ---

func TestRegisterBatch(t *testing.T) {
	tr := New(EN)
	tr.RegisterBatch(VI, map[string]string{
		"user.not_found": "Khong tim thay nguoi dung",
		"user.exists":    "Nguoi dung da ton tai",
	})

	assert.Equal(t, "Khong tim thay nguoi dung", tr.Translate(VI, "user.not_found"))
	assert.Equal(t, "Nguoi dung da ton tai", tr.Translate(VI, "user.exists"))
}

// --- TranslateFromHeader ---

func TestTranslateFromHeader_VI(t *testing.T) {
	tr := New(EN)
	msg := tr.TranslateFromHeader("vi", "common.forbidden")
	assert.Equal(t, "Khong du quyen truy cap", msg)
}

func TestTranslateFromHeader_ENWithQuality(t *testing.T) {
	tr := New(EN)
	msg := tr.TranslateFromHeader("en-US,en;q=0.9,vi;q=0.8", "common.forbidden")
	assert.Equal(t, "Insufficient permissions", msg)
}

func TestTranslateFromHeader_VIRegion(t *testing.T) {
	tr := New(EN)
	msg := tr.TranslateFromHeader("vi-VN,vi;q=0.9", "common.forbidden")
	assert.Equal(t, "Khong du quyen truy cap", msg)
}

func TestTranslateFromHeader_Empty(t *testing.T) {
	tr := New(EN)
	msg := tr.TranslateFromHeader("", "common.forbidden")
	assert.Equal(t, "Insufficient permissions", msg)
}

// --- GetLocale ---

func TestGetLocale_FromQueryParam(t *testing.T) {
	r := gin.New()
	r.GET("/test", func(ctx *gin.Context) {
		locale := GetLocale(ctx)
		ctx.String(200, string(locale))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test?lang=vi", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, "vi", w.Body.String())
}

func TestGetLocale_FromAcceptLanguage(t *testing.T) {
	r := gin.New()
	r.GET("/test", func(ctx *gin.Context) {
		locale := GetLocale(ctx)
		ctx.String(200, string(locale))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Language", "vi-VN")
	r.ServeHTTP(w, req)

	assert.Equal(t, "vi", w.Body.String())
}

func TestGetLocale_DefaultEN(t *testing.T) {
	r := gin.New()
	r.GET("/test", func(ctx *gin.Context) {
		locale := GetLocale(ctx)
		ctx.String(200, string(locale))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, "en", w.Body.String())
}

// --- Middleware + GetTranslator ---

func TestMiddleware_InjectsTranslator(t *testing.T) {
	tr := New(EN)
	r := gin.New()
	r.Use(tr.Middleware())
	r.GET("/test", func(ctx *gin.Context) {
		got := GetTranslator(ctx)
		if got == nil {
			ctx.String(500, "nil")
			return
		}
		ctx.String(200, got.Translate(EN, "common.forbidden"))
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Insufficient permissions", w.Body.String())
}

func TestGetTranslator_ReturnsNilWhenNotSet(t *testing.T) {
	r := gin.New()
	r.GET("/test", func(ctx *gin.Context) {
		got := GetTranslator(ctx)
		if got == nil {
			ctx.String(200, "nil")
			return
		}
		ctx.String(500, "unexpected")
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "nil", w.Body.String())
}
