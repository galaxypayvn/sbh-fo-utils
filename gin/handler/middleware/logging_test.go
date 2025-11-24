package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestSanitizeHeaders(t *testing.T) {
	tests := []struct {
		name             string
		headers          http.Header
		sensitiveHeaders map[string]struct{}
		expected         http.Header
	}{
		{
			name: "mask authorization header",
			headers: http.Header{
				"Authorization": []string{"Bearer token123"},
				"Content-Type":  []string{"application/json"},
			},
			sensitiveHeaders: map[string]struct{}{
				"authorization": {},
			},
			expected: http.Header{
				"Authorization": []string{"[REDACTED]"},
				"Content-Type":  []string{"application/json"},
			},
		},
		{
			name: "mask multiple sensitive headers",
			headers: http.Header{
				"Authorization": []string{"Bearer token123"},
				"X-Api-Key":     []string{"secret-key"},
				"Content-Type":  []string{"application/json"},
			},
			sensitiveHeaders: map[string]struct{}{
				"authorization": {},
				"x-api-key":     {},
			},
			expected: http.Header{
				"Authorization": []string{"[REDACTED]"},
				"X-Api-Key":     []string{"[REDACTED]"},
				"Content-Type":  []string{"application/json"},
			},
		},
		{
			name: "no sensitive headers",
			headers: http.Header{
				"Content-Type": []string{"application/json"},
			},
			sensitiveHeaders: map[string]struct{}{},
			expected: http.Header{
				"Content-Type": []string{"application/json"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeHeaders(tt.headers, tt.sensitiveHeaders)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeBody(t *testing.T) {
	tests := []struct {
		name            string
		body            interface{}
		sensitiveFields map[string]struct{}
		expected        interface{}
	}{
		{
			name: "mask password field",
			body: map[string]interface{}{
				"username": "john",
				"password": "secret123",
			},
			sensitiveFields: map[string]struct{}{
				"password": {},
			},
			expected: map[string]interface{}{
				"username": "john",
				"password": "[REDACTED]",
			},
		},
		{
			name: "mask nested sensitive fields",
			body: map[string]interface{}{
				"user": map[string]interface{}{
					"name":     "john",
					"password": "secret123",
				},
			},
			sensitiveFields: map[string]struct{}{
				"password": {},
			},
			expected: map[string]interface{}{
				"user": map[string]interface{}{
					"name":     "john",
					"password": "[REDACTED]",
				},
			},
		},
		{
			name: "mask fields in array",
			body: map[string]interface{}{
				"users": []interface{}{
					map[string]interface{}{
						"name":     "john",
						"password": "secret1",
					},
					map[string]interface{}{
						"name":     "jane",
						"password": "secret2",
					},
				},
			},
			sensitiveFields: map[string]struct{}{
				"password": {},
			},
			expected: map[string]interface{}{
				"users": []interface{}{
					map[string]interface{}{
						"name":     "john",
						"password": "[REDACTED]",
					},
					map[string]interface{}{
						"name":     "jane",
						"password": "[REDACTED]",
					},
				},
			},
		},
		{
			name: "mask multiple sensitive fields",
			body: map[string]interface{}{
				"username":     "john",
				"password":     "secret123",
				"access_token": "token123",
				"email":        "john@example.com",
			},
			sensitiveFields: map[string]struct{}{
				"password":     {},
				"access_token": {},
			},
			expected: map[string]interface{}{
				"username":     "john",
				"password":     "[REDACTED]",
				"access_token": "[REDACTED]",
				"email":        "john@example.com",
			},
		},
		{
			name:            "nil body",
			body:            nil,
			sensitiveFields: map[string]struct{}{"password": {}},
			expected:        nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeBody(tt.body, tt.sensitiveFields)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLoggingRequestWithConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *LoggingConfig
		requestBody map[string]interface{}
		headers     map[string]string
	}{
		{
			name: "production environment with defaults",
			config: &LoggingConfig{
				AppEnv: "prd",
			},
			requestBody: map[string]interface{}{
				"username": "john",
				"password": "secret123",
			},
			headers: map[string]string{
				"Authorization": "Bearer token",
				"Content-Type":  "application/json",
			},
		},
		{
			name: "development environment",
			config: &LoggingConfig{
				AppEnv: "dev",
			},
			requestBody: map[string]interface{}{
				"username": "john",
				"password": "secret123",
			},
			headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
		{
			name: "staging environment",
			config: &LoggingConfig{
				AppEnv: "stg",
			},
			requestBody: map[string]interface{}{
				"username": "john",
				"password": "secret123",
			},
			headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
		{
			name: "with extra sensitive fields",
			config: &LoggingConfig{
				AppEnv:                   "prd",
				ExtraSensitiveHeaders:    []string{"x-custom-token"},
				ExtraSensitiveBodyFields: []string{"otp", "ssn"},
			},
			requestBody: map[string]interface{}{
				"username": "john",
				"otp":      "123456",
				"ssn":      "123-45-6789",
			},
			headers: map[string]string{
				"X-Custom-Token": "custom-secret",
				"Content-Type":   "application/json",
			},
		},
		{
			name: "disable default sensitive data",
			config: &LoggingConfig{
				AppEnv:                      "prd",
				DisableDefaultSensitiveData: true,
				ExtraSensitiveBodyFields:    []string{"custom_field"},
			},
			requestBody: map[string]interface{}{
				"password":     "should-not-be-masked",
				"custom_field": "should-be-masked",
			},
			headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
		{
			name:   "nil config",
			config: nil,
			requestBody: map[string]interface{}{
				"username": "john",
			},
			headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(LoggingRequestWithConfig(tt.config))
			router.POST("/test", func(c *gin.Context) {
				// Read body to verify it's still available after middleware
				bodyBytes, err := io.ReadAll(c.Request.Body)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				var body map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &body); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, body)
			})

			bodyBytes, _ := json.Marshal(tt.requestBody)
			req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBuffer(bodyBytes))
			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			// Verify body is still accessible
			var responseBody map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &responseBody)
			assert.NoError(t, err)
			assert.Equal(t, tt.requestBody, responseBody)
		})
	}
}

func TestLoggingRequest_BackwardCompatible(t *testing.T) {
	tests := []struct {
		name         string
		isProduction bool
	}{
		{
			name:         "production mode",
			isProduction: true,
		},
		{
			name:         "development mode",
			isProduction: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(LoggingRequest(tt.isProduction))
			router.POST("/test", func(c *gin.Context) {
				bodyBytes, err := io.ReadAll(c.Request.Body)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				var body map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &body); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, body)
			})

			requestBody := map[string]interface{}{
				"username": "john",
				"password": "secret123",
			}
			bodyBytes, _ := json.Marshal(requestBody)
			req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewBuffer(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
		})
	}
}

func TestDefaultSensitiveData(t *testing.T) {
	// Verify default sensitive headers
	expectedHeaders := []string{
		"authorization",
		"x-api-key",
		"cookie",
		"x-auth-token",
	}
	assert.Equal(t, expectedHeaders, DefaultSensitiveHeaders)

	// Verify default sensitive body fields
	expectedBodyFields := []string{
		"password",
		"token",
		"secret",
		"access_token",
		"refresh_token",
		"api_key",
	}
	assert.Equal(t, expectedBodyFields, DefaultSensitiveBodyFields)
}

func TestSkipPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Track if handler was called
	var handlerCalled bool

	config := &LoggingConfig{
		AppEnv:    "prd",
		SkipPaths: []string{"/health", "/metrics"},
	}

	router := gin.New()
	router.Use(LoggingRequestWithConfig(config))
	router.GET("/health", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/metrics", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(http.StatusOK, gin.H{"metrics": "data"})
	})
	router.POST("/api/users", func(c *gin.Context) {
		handlerCalled = true
		c.JSON(http.StatusOK, gin.H{"user": "created"})
	})

	tests := []struct {
		name       string
		path       string
		method     string
		shouldSkip bool
	}{
		{
			name:       "skip health endpoint",
			path:       "/health",
			method:     http.MethodGet,
			shouldSkip: true,
		},
		{
			name:       "skip metrics endpoint",
			path:       "/metrics",
			method:     http.MethodGet,
			shouldSkip: true,
		},
		{
			name:       "do not skip api endpoint",
			path:       "/api/users",
			method:     http.MethodPost,
			shouldSkip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerCalled = false

			var req *http.Request
			if tt.method == http.MethodPost {
				req = httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(`{"name":"test"}`))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.True(t, handlerCalled, "handler should be called")
		})
	}
}

// Benchmark tests for performance validation
func BenchmarkLoggingRequestWithConfig(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)

	config := &LoggingConfig{
		AppEnv: "prd",
	}

	router := gin.New()
	router.Use(LoggingRequestWithConfig(config))
	router.POST("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	requestBody := []byte(`{"username":"john","password":"secret123","email":"john@example.com"}`)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer token123")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

func BenchmarkLoggingRequestWithConfig_LargeBody(b *testing.B) {
	gin.SetMode(gin.ReleaseMode)

	config := &LoggingConfig{
		AppEnv: "prd",
	}

	router := gin.New()
	router.Use(LoggingRequestWithConfig(config))
	router.POST("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Create a larger JSON body with nested structures
	largeBody := map[string]interface{}{
		"users": make([]map[string]interface{}, 100),
	}
	for i := 0; i < 100; i++ {
		largeBody["users"].([]map[string]interface{})[i] = map[string]interface{}{
			"id":       i,
			"username": "user" + string(rune(i)),
			"password": "secret",
			"email":    "user@example.com",
			"profile": map[string]interface{}{
				"name":    "John Doe",
				"token":   "secret_token",
				"address": "123 Main St",
			},
		}
	}
	requestBody, _ := json.Marshal(largeBody)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

func BenchmarkSanitizeBody(b *testing.B) {
	sensitiveFields := map[string]struct{}{
		"password":     {},
		"token":        {},
		"secret":       {},
		"access_token": {},
	}

	body := map[string]interface{}{
		"username": "john",
		"password": "secret123",
		"profile": map[string]interface{}{
			"name":  "John Doe",
			"token": "secret_token",
		},
		"items": []interface{}{
			map[string]interface{}{"id": 1, "secret": "s1"},
			map[string]interface{}{"id": 2, "secret": "s2"},
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = sanitizeBody(body, sensitiveFields)
	}
}

func BenchmarkSanitizeHeaders(b *testing.B) {
	sensitiveHeaders := map[string]struct{}{
		"authorization": {},
		"x-api-key":     {},
		"cookie":        {},
	}

	headers := http.Header{
		"Authorization": []string{"Bearer token123"},
		"X-Api-Key":     []string{"secret-key"},
		"Content-Type":  []string{"application/json"},
		"Accept":        []string{"application/json"},
		"User-Agent":    []string{"Mozilla/5.0"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = sanitizeHeaders(headers, sensitiveHeaders)
	}
}
