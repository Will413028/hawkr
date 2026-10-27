package server

import (
	"sort"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// HealthController owns the process aggregate and generated protobuf service
// health statuses for a gRPC service. A service can advertise process liveness
// after its listener binds while holding its named readiness status until its
// required application dependencies are ready.
type HealthController struct {
	server            *health.Server
	serviceNames      []string
	legacyReadyOnBind bool
}

// ServiceNames returns the sorted generated protobuf service names owned by
// this controller. It is primarily useful to make a service's health contract
// inspectable in graph/config tests.
func (h *HealthController) ServiceNames() []string {
	return append([]string(nil), h.serviceNames...)
}

// NewHealthController creates a final-mode health controller. The aggregate
// status becomes SERVING after the gRPC listener binds; named service statuses
// remain NOT_SERVING until MarkReady is called by the service's readiness hook.
func NewHealthController(serviceNames ...string) *HealthController {
	return newHealthController(serviceNames, false)
}

func newLegacyHealthController(serviceNames []string) *HealthController {
	return newHealthController(serviceNames, true)
}

func newHealthController(serviceNames []string, legacyReadyOnBind bool) *HealthController {
	names := make([]string, 0, len(serviceNames))
	seen := make(map[string]struct{}, len(serviceNames))
	for _, name := range serviceNames {
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	sort.Strings(names)

	controller := &HealthController{
		server:            health.NewServer(),
		serviceNames:      names,
		legacyReadyOnBind: legacyReadyOnBind,
	}
	controller.MarkNotReady()
	return controller
}

// Register adds the standard gRPC health service to registrar. It must happen
// during graph construction, before the server starts accepting connections.
func (h *HealthController) Register(registrar grpc.ServiceRegistrar) {
	healthpb.RegisterHealthServer(registrar, h.server)
}

// MarkBound advertises process liveness after the gRPC listener successfully
// binds. Legacy callers also receive the historical named SERVING transition;
// final-mode callers must explicitly call MarkReady.
func (h *HealthController) MarkBound() {
	h.server.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	if h.legacyReadyOnBind {
		h.MarkReady()
	}
}

// MarkReady advertises all configured protobuf services as ready.
func (h *HealthController) MarkReady() {
	for _, name := range h.serviceNames {
		h.server.SetServingStatus(name, healthpb.HealthCheckResponse_SERVING)
	}
}

// MarkNotReady withdraws both aggregate and named readiness before shutdown
// drain begins.
func (h *HealthController) MarkNotReady() {
	setHealthStatus(h.server, h.serviceNames, healthpb.HealthCheckResponse_NOT_SERVING)
}
