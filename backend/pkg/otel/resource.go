package otel

import (
	"context"
	"os"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func newServiceResource(serviceName string) *resource.Resource {
	env := os.Getenv("ENV")
	if env == "" {
		env = "development"
	}

	attrs := []attribute.KeyValue{
		semconv.ServiceNameKey.String(serviceName),
		attribute.String("deployment.environment", env),
	}

	if id := instanceID(); id != "" {
		attrs = append(attrs, semconv.ServiceInstanceIDKey.String(id))
	}

	res, _ := resource.New(context.Background(), resource.WithAttributes(attrs...))
	return res
}

func instanceID() string {
	if v := os.Getenv("POD_NAME"); v != "" {
		return v
	}
	if h, err := os.Hostname(); err == nil {
		return h
	}
	return ""
}
