package middleware

import (
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// normalizePathForSpan converts high-cardinality paths to low-cardinality span names
// e.g., "/v1/scores/player_12345" -> "/v1/scores/{user_id}"
// Returns the normalized path and the extracted user_id (if any)
func normalizePathForSpan(path string) (normalizedPath string, userID string) {
	// Match pattern: /v1/scores/{anything}
	if len(path) > 11 && path[:11] == "/v1/scores/" {
		userID = path[11:] // Extract user_id from path
		return "/v1/scores/{user_id}", userID
	}
	// Match pattern: /v2/scores/{anything}
	if len(path) > 11 && path[:11] == "/v2/scores/" {
		userID = path[11:] // Extract user_id from path
		return "/v2/scores/{user_id}", userID
	}
	return path, ""
}

// TracingMiddleware adds OpenTelemetry tracing to HTTP requests
func TracingMiddleware(next http.Handler) http.Handler {
	tracer := otel.Tracer("leaderboard-http")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract trace context from incoming request headers
		ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))

		// Normalize path to avoid high cardinality
		normalizedPath, userID := normalizePathForSpan(r.URL.Path)
		spanName := r.Method + " " + normalizedPath

		ctx, span := tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.route", normalizedPath),
				attribute.String("http.target", r.URL.Path), // Keep original for debugging
				attribute.String("http.scheme", r.URL.Scheme),
				attribute.String("http.host", r.Host),
				attribute.String("http.user_agent", r.UserAgent()),
			),
		)
		defer span.End()

		// Add user_id as span event if present (keeps it queryable without high cardinality span names)
		if userID != "" {
			span.AddEvent("request.user_id", trace.WithAttributes(
				attribute.String("user_id", userID),
			))
		}

		// Create a response writer wrapper to capture status code
		rw := &tracingResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// Call the next handler with the new context
		next.ServeHTTP(rw, r.WithContext(ctx))

		// Record the response status
		span.SetAttributes(
			attribute.Int("http.status_code", rw.statusCode),
		)

		// Mark span as error if status code indicates an error
		if rw.statusCode >= 400 {
			span.SetAttributes(attribute.Bool("error", true))
		}
	})
}

type tracingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *tracingResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
