package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nathanyu/digital-wallet/internal/telemetry"
)

// Metrics middleware collects Prometheus metrics for requests
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		normalizedPath := normalizePath(path)

		start := time.Now()
		c.Next()
		duration := time.Since(start)

		status := strconv.Itoa(c.Writer.Status())

		// Record metrics
		telemetry.HTTPRequestsTotal.WithLabelValues(
			c.Request.Method,
			normalizedPath,
			status,
		).Inc()

		telemetry.HTTPRequestDuration.WithLabelValues(
			c.Request.Method,
			normalizedPath,
		).Observe(duration.Seconds())
	}
}
