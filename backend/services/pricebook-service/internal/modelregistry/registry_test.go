package modelregistry

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadRegistry_ParsesValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.json")
	content := `{
		"model_version": "direct-ridge-v2-86b2d298a9f27cb5",
		"r2_object_key": "pricebook-models/direct-ridge-v2-86b2d298a9f27cb5/model.json",
		"sha256": "abc123",
		"trained_at": "2026-08-01T04:55:29.452949Z",
		"dataset_digest": "c765bc2a7213e85c4a7a89b7813a8e97357d907c5d4217465d40c13b8020526e",
		"series_count": 10321
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if got.ModelVersion != "direct-ridge-v2-86b2d298a9f27cb5" {
		t.Errorf("ModelVersion = %q", got.ModelVersion)
	}
	if got.R2ObjectKey != "pricebook-models/direct-ridge-v2-86b2d298a9f27cb5/model.json" {
		t.Errorf("R2ObjectKey = %q", got.R2ObjectKey)
	}
	if got.SHA256 != "abc123" {
		t.Errorf("SHA256 = %q", got.SHA256)
	}
	if got.SeriesCount != 10321 {
		t.Errorf("SeriesCount = %d", got.SeriesCount)
	}
	wantTrainedAt, _ := time.Parse(time.RFC3339Nano, "2026-08-01T04:55:29.452949Z")
	if !got.TrainedAt.Equal(wantTrainedAt) {
		t.Errorf("TrainedAt = %v, want %v", got.TrainedAt, wantTrainedAt)
	}
}

func TestLoadRegistry_MissingFile(t *testing.T) {
	_, err := LoadRegistry("/nonexistent/registry.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadRegistry_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err := LoadRegistry(path)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}
