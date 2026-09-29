// Package observability wires the logging, tracing and metrics every service shares.
package observability

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Init sets a JSON slog default logger (the std log package included) and the global
// tracer provider. Spans are exported over OTLP/HTTP only when OTEL_EXPORTER_OTLP_ENDPOINT
// is set; trace context is propagated either way.
func Init(ctx context.Context, service string) (shutdown func(context.Context) error) {
	slog.SetDefault(slog.New(newTraceHandler(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel()}),
	)).With("service", service))

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))

	noop := func(context.Context) error { return nil }
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return noop
	}

	exp, err := otlptracehttp.New(ctx)
	if err != nil {
		slog.Error("tracing disabled: creating otlp exporter", "error", err)
		return noop
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(resource.NewSchemaless(attribute.String("service.name", service))),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown
}

func logLevel() slog.Level {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(strings.TrimSpace(os.Getenv("LOG_LEVEL")))); err != nil {
		return slog.LevelInfo
	}
	return lvl
}
