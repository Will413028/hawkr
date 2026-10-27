package usecase

import (
	"testing"

	"github.com/Will413028/hawkr/pricebook-service/internal/config"
)

func TestProvideModelArtifact_LoadsValidFile(t *testing.T) {
	// Reuse the same fixture internal/domain/forecast/artifact_test.go
	// already uses (confirmed via `grep -n "LoadArtifact(" internal/domain/forecast/artifact_test.go`
	// during plan-writing: that test loads
	// "../../../testdata/parity/model-demo.json" relative to
	// internal/domain/forecast/, i.e. services/pricebook-service/testdata/parity/model-demo.json
	// relative to the service root). From internal/usecase/ (this file's
	// package), the same fixture is two directories up instead of three.
	cfg := &config.Config{ModelArtifactPath: "../../testdata/parity/model-demo.json"}
	artifact, err := provideModelArtifact(cfg)
	if err != nil {
		t.Fatalf("provideModelArtifact: %v", err)
	}
	if artifact.ArtifactSchemaVersion == "" {
		t.Error("ArtifactSchemaVersion is empty, expected a real parsed artifact")
	}
}

func TestProvideModelArtifact_MissingFileFailsFast(t *testing.T) {
	cfg := &config.Config{ModelArtifactPath: "/nonexistent/model.json"}
	_, err := provideModelArtifact(cfg)
	if err == nil {
		t.Fatal("expected error for missing model file, got nil (must fail fast, not silently skip)")
	}
}
