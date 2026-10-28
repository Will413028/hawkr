package config

// ServerConfig holds the gRPC and HTTP listen ports read from GRPC_PORT /
// HTTP_PORT by every backend-payuni service that exposes both protocols
// side by side. Before this type existed, buyer-service, content-service,
// crm-service, notification-service, review-service,
// seller-service and social-service each declared their own byte-identical
// {GRPCPort, HTTPPort string} struct in internal/config/config.go.
//
// pkg/server.ServerConfig embeds this type to add its own fx-bootstrap-
// specific fields (Env, EnableStreamInterceptors) on top: this is the
// single source of truth for the port fields, consumed directly by
// services and via embedding by pkg/server.
type ServerConfig struct {
	GRPCPort string
	HTTPPort string
}
