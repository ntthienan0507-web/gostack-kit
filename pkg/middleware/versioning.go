package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
)

const apiVersionKey = "api_version"

// APIVersion extracts the API version from the request and stores it in context.
// Checks in order:
//  1. URL prefix: /api/v2/users -> version "v2"
//  2. Header: X-API-Version: v2 -> version "v2"
//  3. Query param: ?api_version=v2 -> version "v2"
//  4. Default: defaultVersion
func APIVersion(defaultVersion string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		version := ""

		// 1. Check URL prefix — look for /api/vN/ pattern.
		path := ctx.Request.URL.Path
		if idx := strings.Index(path, "/api/"); idx != -1 {
			rest := path[idx+len("/api/"):]
			if seg := strings.SplitN(rest, "/", 2); len(seg) > 0 && strings.HasPrefix(seg[0], "v") {
				version = seg[0]
			}
		}

		// 2. Check X-API-Version header.
		if version == "" {
			if h := ctx.GetHeader("X-API-Version"); h != "" {
				version = h
			}
		}

		// 3. Check query param.
		if version == "" {
			if q := ctx.Query("api_version"); q != "" {
				version = q
			}
		}

		// 4. Default.
		if version == "" {
			version = defaultVersion
		}

		ctx.Set(apiVersionKey, version)
		ctx.Next()
	}
}

// GetAPIVersion retrieves the API version from gin context.
func GetAPIVersion(ctx *gin.Context) string {
	v, _ := ctx.Get(apiVersionKey)
	s, _ := v.(string)
	return s
}

// DeprecatedVersion returns middleware that adds Deprecation and Sunset headers.
// Warns clients that this API version will be removed.
//
//	v1 := router.Group("/api/v1")
//	v1.Use(middleware.DeprecatedVersion("2025-06-01", "Use /api/v2 instead"))
func DeprecatedVersion(sunsetDate string, message string) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Header("Deprecation", "true")
		ctx.Header("Sunset", sunsetDate)
		ctx.Header("Link", fmt.Sprintf(`; rel="successor-version"; title="%s"`, message))
		ctx.Next()
	}
}
