package config

import "testing"

func TestServerConfig_FieldsHoldPorts(t *testing.T) {
	cfg := ServerConfig{GRPCPort: "10033", HTTPPort: "10032"}
	if cfg.GRPCPort != "10033" {
		t.Errorf("GRPCPort = %q, want %q", cfg.GRPCPort, "10033")
	}
	if cfg.HTTPPort != "10032" {
		t.Errorf("HTTPPort = %q, want %q", cfg.HTTPPort, "10032")
	}
}

func TestServerConfig_ZeroValue(t *testing.T) {
	var cfg ServerConfig
	if cfg.GRPCPort != "" || cfg.HTTPPort != "" {
		t.Errorf("zero value ServerConfig should have empty ports, got %+v", cfg)
	}
}
