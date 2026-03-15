package middleware

import "github.com/gin-gonic/gin"

// SecurityHeaders adds security headers to all responses.
// Protects against MIME sniffing, clickjacking, XSS, and other common attacks.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Prevent browser from guessing Content-Type (e.g. treating .txt as .js → code execution)
		c.Header("X-Content-Type-Options", "nosniff")

		// Block site from being embedded in <iframe> → prevents clickjacking attacks
		c.Header("X-Frame-Options", "DENY")

		// Enable XSS filter in legacy browsers; modern browsers use CSP instead
		c.Header("X-XSS-Protection", "1; mode=block")

		// Force HTTPS for 1 year including subdomains → prevents MITM/protocol downgrade
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		// Cross-origin links only send origin (https://site.com), not full URL with path/query
		// Prevents leaking sensitive data (tokens, IDs) in Referer header
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

		// Block 3rd party scripts from accessing device hardware (camera, mic, GPS)
		c.Header("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		// Block Flash/PDF from loading cross-domain policy files (legacy, but defense-in-depth)
		c.Header("X-Permitted-Cross-Domain-Policies", "none")

		// CSP: only allow resources from same origin → strongest XSS defense
		// - script/font/connect: self only (blocks injected <script src="evil.com">)
		// - style: self + inline (for framework compatibility)
		// - img: self + data: URIs (for base64 images)
		c.Header("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'")

		c.Next()
	}
}
