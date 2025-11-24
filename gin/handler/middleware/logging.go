// Package middleware provides HTTP middleware for Gin framework.
//
// Logging Middleware Usage:
//
//	// Basic usage with default settings
//	router.Use(middleware.LoggingRequestWithConfig(&middleware.LoggingConfig{
//	    AppEnv: "prd",
//	}))
//
//	// With custom sensitive fields
//	router.Use(middleware.LoggingRequestWithConfig(&middleware.LoggingConfig{
//	    AppEnv:                   "prd",
//	    ExtraSensitiveHeaders:    []string{"x-custom-token"},
//	    ExtraSensitiveBodyFields: []string{"otp", "ssn", "bank_account"},
//	}))
//
//	// Skip logging for specific paths (health checks, metrics, etc.)
//	router.Use(middleware.LoggingRequestWithConfig(&middleware.LoggingConfig{
//	    AppEnv:    "prd",
//	    SkipPaths: []string{"/health", "/metrics", "/readiness"},
//	}))
//
//	// Disable default sensitive fields (use only custom)
//	router.Use(middleware.LoggingRequestWithConfig(&middleware.LoggingConfig{
//	    AppEnv:                      "prd",
//	    DisableDefaultSensitiveData: true,
//	    ExtraSensitiveBodyFields:    []string{"my_secret_field"},
//	}))
package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gitlab.com/goxp/cloud0/logger"
)

const (
	DefaultMaxBodyLogSize = 1 << 20 // 1MB
	redactedStr           = "[REDACTED]"
)

// DefaultSensitiveHeaders contains header names that will be masked by default.
var DefaultSensitiveHeaders = []string{
	"authorization", "x-api-key", "cookie", "x-auth-token",
}

// DefaultSensitiveBodyFields contains JSON field names that will be masked by default.
var DefaultSensitiveBodyFields = []string{
	"password", "token", "secret", "access_token", "refresh_token", "api_key",
}

// LoggingConfig holds configuration for logging middleware.
type LoggingConfig struct {
	AppEnv                      string   // Environment: "dev", "stg", "prd". Dev/stg logs headers, prd does not.
	ExtraSensitiveHeaders       []string // Additional headers to mask (merged with defaults)
	ExtraSensitiveBodyFields    []string // Additional body fields to mask (merged with defaults)
	DisableDefaultSensitiveData bool     // If true, only use Extra* fields, skip defaults
	MaxBodyLogSize              int64    // Max body size to log in bytes (default: 1MB)
	SkipPaths                   []string // Paths to skip logging (e.g., "/health", "/metrics")
}

// LoggingRequest logs request details with sensitive data masked.
// Deprecated: Use LoggingRequestWithConfig for more control.
//
// Example:
//
//	router.Use(middleware.LoggingRequest(true)) // production mode
func LoggingRequest(isProduction bool) gin.HandlerFunc {
	env := "dev"
	if isProduction {
		env = "prd"
	}
	return LoggingRequestWithConfig(&LoggingConfig{AppEnv: env})
}

// LoggingRequestWithConfig logs request details with sensitive data masked.
// It reads the request body, masks sensitive fields, logs the request, and restores
// the body for downstream handlers.
//
// Behavior by environment:
//   - dev/stg: logs URI, headers (masked), and body (masked)
//   - prd: logs URI, body (masked), and x-current-version header only
//
// Example:
//
//	router.Use(middleware.LoggingRequestWithConfig(&middleware.LoggingConfig{
//	    AppEnv:                   "prd",
//	    ExtraSensitiveBodyFields: []string{"otp"},
//	}))
func LoggingRequestWithConfig(config *LoggingConfig) gin.HandlerFunc {
	if config == nil {
		config = &LoggingConfig{}
	}

	// Build lookup maps once
	sensitiveHeaders := buildSensitiveMap(DefaultSensitiveHeaders, config.ExtraSensitiveHeaders, config.DisableDefaultSensitiveData)
	sensitiveBodyFields := buildSensitiveMap(DefaultSensitiveBodyFields, config.ExtraSensitiveBodyFields, config.DisableDefaultSensitiveData)

	// Build skip paths map for O(1) lookup
	skipPaths := make(map[string]struct{}, len(config.SkipPaths))
	for _, p := range config.SkipPaths {
		skipPaths[p] = struct{}{}
	}

	maxBodySize := config.MaxBodyLogSize
	if maxBodySize <= 0 {
		maxBodySize = DefaultMaxBodyLogSize
	}

	isDevOrStg := config.AppEnv == "dev" || config.AppEnv == "stg"

	return func(c *gin.Context) {
		// Skip logging for whitelisted paths
		if _, skip := skipPaths[c.Request.URL.Path]; skip {
			c.Next()
			return
		}

		log := logger.WithCtx(c, "Request Detail")
		defer recoverPanic(log)

		// Read body
		bodyBytes, truncated, err := readBody(c.Request.Body, maxBodySize)
		if err != nil {
			log.WithError(err).Error("Error reading request body")
			http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
			return
		}

		// Sanitize and log
		logBody := sanitizeAndFormatBody(bodyBytes, sensitiveBodyFields, truncated)

		if isDevOrStg {
			header := sanitizeHeaders(c.Request.Header, sensitiveHeaders)
			log.Infof("uri: %s, header: %v, body: %s", c.Request.RequestURI, header, logBody)
		} else {
			log.Infof("uri: %s, body: %s, x-current-version: %s", c.Request.RequestURI, logBody, c.Request.Header.Get("x-current-version"))
		}

		// Restore body for downstream handlers
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		c.Next()
	}
}

// --- Helper functions ---

// buildSensitiveMap creates a map for O(1) lookup of sensitive field names.
// It merges default fields with extra fields, all converted to lowercase.
func buildSensitiveMap(defaults, extras []string, disableDefaults bool) map[string]struct{} {
	m := make(map[string]struct{})
	if !disableDefaults {
		for _, s := range defaults {
			m[strings.ToLower(s)] = struct{}{}
		}
	}
	for _, s := range extras {
		m[strings.ToLower(s)] = struct{}{}
	}
	return m
}

// readBody reads the request body up to maxSize bytes.
// Returns the body bytes, whether it was truncated, and any error.
func readBody(body io.ReadCloser, maxSize int64) ([]byte, bool, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxSize+1))
	if err != nil {
		return nil, false, err
	}
	truncated := int64(len(data)) > maxSize
	if truncated {
		data = data[:maxSize]
	}
	return data, truncated, nil
}

// sanitizeAndFormatBody masks sensitive fields in JSON body and returns formatted string.
// If body is not valid JSON, returns as-is. Appends "[TRUNCATED]" if body was truncated.
func sanitizeAndFormatBody(body []byte, sensitiveFields map[string]struct{}, truncated bool) string {
	if len(body) == 0 {
		return ""
	}

	var result string
	if len(sensitiveFields) > 0 {
		var obj interface{}
		if json.Unmarshal(body, &obj) == nil {
			obj = sanitizeBody(obj, sensitiveFields)
			data, _ := json.Marshal(obj)
			result = string(data)
		} else {
			result = string(body)
		}
	} else {
		result = string(body)
	}

	if truncated {
		result += "...[TRUNCATED]"
	}
	return result
}

// sanitizeHeaders returns a copy of headers with sensitive values replaced by "[REDACTED]".
func sanitizeHeaders(headers http.Header, sensitive map[string]struct{}) http.Header {
	if len(sensitive) == 0 {
		return headers
	}
	sanitized := make(http.Header, len(headers))
	for key, values := range headers {
		if _, ok := sensitive[strings.ToLower(key)]; ok {
			sanitized[key] = []string{redactedStr}
		} else {
			sanitized[key] = values
		}
	}
	return sanitized
}

// sanitizeBody recursively masks sensitive fields in JSON object/array.
// Returns a new object with sensitive field values replaced by "[REDACTED]".
func sanitizeBody(obj interface{}, sensitive map[string]struct{}) interface{} {
	switch v := obj.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(v))
		for key, value := range v {
			if _, ok := sensitive[strings.ToLower(key)]; ok {
				result[key] = redactedStr
			} else {
				result[key] = sanitizeBody(value, sensitive)
			}
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = sanitizeBody(item, sensitive)
		}
		return result
	default:
		return obj
	}
}

// recoverPanic handles panic in middleware, logs it, and re-panics.
func recoverPanic(log *logrus.Entry) {
	if r := recover(); r != nil {
		log.Error(r)
		debug.PrintStack()
		panic(r)
	}
}
