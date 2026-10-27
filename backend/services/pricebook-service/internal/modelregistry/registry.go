// Package modelregistry loads the pointer file that tells pricebook-service
// which production model.json to fetch from R2, and (fetch.go) verifies and
// caches the actual 154MB payload. The pointer file mirrors the shape of
// Hawkr_Transformer_Pricebook's own local registry.py convention
// (content-addressed model_version + checksummed pointer) — see
// docs/superpowers/specs/2026-08-01-pricebook-model-artifact-loading-design.md.
package modelregistry

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// Registry is the parsed content of model/registry.json — a small,
// git-committed pointer to the real model artifact in R2. It never contains
// the model payload itself.
type Registry struct {
	ModelVersion  string    `json:"model_version"`
	R2ObjectKey   string    `json:"r2_object_key"`
	SHA256        string    `json:"sha256"`
	TrainedAt     time.Time `json:"trained_at"`
	DatasetDigest string    `json:"dataset_digest"`
	SeriesCount   int       `json:"series_count"`
}

// LoadRegistry reads and parses registry.json. It does not touch R2 or
// validate that the referenced object actually exists — that's FetchModel's
// job (fetch.go).
func LoadRegistry(path string) (Registry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Registry{}, fmt.Errorf("modelregistry: reading %s: %w", path, err)
	}
	var reg Registry
	if err := json.Unmarshal(raw, &reg); err != nil {
		return Registry{}, fmt.Errorf("modelregistry: parsing %s: %w", path, err)
	}
	return reg, nil
}
