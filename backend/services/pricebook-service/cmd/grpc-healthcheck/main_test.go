package main

import (
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func TestCheckHealth_AggregateAndNamedServing(t *testing.T) {
	server, addr, stop := startHealthServer(t)
	defer stop()

	server.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	server.SetServingStatus("hawkr.pricebook.PricebookService", healthpb.HealthCheckResponse_SERVING)

	if err := checkHealth(addr, "", time.Second); err != nil {
		t.Fatalf("aggregate health check: %v", err)
	}
	if err := checkHealth(addr, "hawkr.pricebook.PricebookService", time.Second); err != nil {
		t.Fatalf("named health check: %v", err)
	}
}

func TestCheckHealth_ReturnsErrorForNotServing(t *testing.T) {
	server, addr, stop := startHealthServer(t)
	defer stop()

	server.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)
	if err := checkHealth(addr, "", time.Second); err == nil {
		t.Fatal("NOT_SERVING health check returned nil error")
	}
}

func TestCheckHealth_ReturnsErrorForUnavailableAddress(t *testing.T) {
	if err := checkHealth("127.0.0.1:1", "", 50*time.Millisecond); err == nil {
		t.Fatal("unavailable address health check returned nil error")
	}
}

func startHealthServer(t *testing.T) (*health.Server, string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen health test server: %v", err)
	}

	server := grpc.NewServer()
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(server, healthServer)
	go func() { _ = server.Serve(listener) }()

	stop := func() {
		server.GracefulStop()
		_ = listener.Close()
	}
	return healthServer, listener.Addr().String(), stop
}
