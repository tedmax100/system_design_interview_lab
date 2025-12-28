package telemetry

import (
	"context"
	"io"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel/trace"
)

// TracingHandler wraps slog.Handler to add trace context
type TracingHandler struct {
	handler slog.Handler
}

// NewTracingHandler creates a new tracing-aware log handler
func NewTracingHandler(w io.Writer, opts *slog.HandlerOptions) *TracingHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &TracingHandler{
		handler: slog.NewJSONHandler(w, opts),
	}
}

// Enabled implements slog.Handler
func (h *TracingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

// Handle implements slog.Handler
func (h *TracingHandler) Handle(ctx context.Context, record slog.Record) error {
	// Add trace context if available
	spanCtx := trace.SpanContextFromContext(ctx)
	if spanCtx.IsValid() {
		record.AddAttrs(
			slog.String("trace_id", spanCtx.TraceID().String()),
			slog.String("span_id", spanCtx.SpanID().String()),
		)
	}
	return h.handler.Handle(ctx, record)
}

// WithAttrs implements slog.Handler
func (h *TracingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TracingHandler{handler: h.handler.WithAttrs(attrs)}
}

// WithGroup implements slog.Handler
func (h *TracingHandler) WithGroup(name string) slog.Handler {
	return &TracingHandler{handler: h.handler.WithGroup(name)}
}

// Logger is the global structured logger
var Logger *slog.Logger

// InitLogger initializes the structured logger with trace context support
func InitLogger(serviceName string) {
	handler := NewTracingHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	Logger = slog.New(handler).With(
		slog.String("service", serviceName),
	)

	// Set as default logger
	slog.SetDefault(Logger)
}
