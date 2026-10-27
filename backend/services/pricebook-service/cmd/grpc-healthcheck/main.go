package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:10039", "gRPC address to check")
	service := flag.String("service", "", "optional fully-qualified gRPC service name")
	timeout := flag.Duration("timeout", 5*time.Second, "health check timeout")
	flag.Parse()

	if err := checkHealth(*addr, *service, *timeout); err != nil {
		fail("health check failed: %v", err)
	}
}

func checkHealth(addr, service string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := grpc.NewClient(
		"passthrough:///"+addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("create gRPC client: %w", err)
	}
	defer func() { _ = conn.Close() }()

	response, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{
		Service: service,
	})
	if err != nil {
		return err
	}
	if response.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		return fmt.Errorf("health check returned %s", response.GetStatus().String())
	}
	return nil
}

func fail(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
