package middleware

import (
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nathanyu/digital-wallet/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var (
	// Pattern to normalize paths with account IDs
	accountIDPattern = regexp.MustCompile(`/balance/[^/]+`)
)

// normalizePath converts high-cardinality paths to low-cardinality patterns
func normalizePath(path string) string {
	return accountIDPattern.ReplaceAllString(path, "/balance/{account_id}")
}

// Tracing middleware adds OpenTelemetry tracing to requests
func Tracing() gin.HandlerFunc {
	return func(c *gin.Context) {
		if telemetry.Tracer == nil {
			c.Next()
			return
		}

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		normalizedPath := normalizePath(path)

		ctx, span := telemetry.Tracer.Start(c.Request.Context(), "HTTP "+c.Request.Method+" "+normalizedPath,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("http.method", c.Request.Method),
				attribute.String("http.route", normalizedPath),
				attribute.String("http.target", c.Request.URL.Path),
				attribute.String("http.host", c.Request.Host),
				attribute.String("http.user_agent", c.Request.UserAgent()),
			),
		)
		defer span.End()

		// Update context
		c.Request = c.Request.WithContext(ctx)

		// Process request
		start := time.Now()
		c.Next()
		duration := time.Since(start)

		// Add response attributes
		statusCode := c.Writer.Status()
		span.SetAttributes(
			attribute.Int("http.status_code", statusCode),
			attribute.Float64("http.duration_ms", float64(duration.Milliseconds())),
		)

		// Set error status if needed
		if statusCode >= 400 {
			span.SetStatus(codes.Error, "HTTP error")
		} else {
			span.SetStatus(codes.Ok, "")
		}
	}
}
