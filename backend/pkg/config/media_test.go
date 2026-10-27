package config

import "testing"

func TestMediaBaseURLMissing(t *testing.T) {
	tests := []struct {
		name         string
		mediaBaseURL string
		want         bool
	}{
		{"empty is missing", "", true},
		{"set is not missing", "https://test-media.example.com", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MediaBaseURLMissing(tt.mediaBaseURL); got != tt.want {
				t.Errorf("MediaBaseURLMissing(%q) = %v, want %v", tt.mediaBaseURL, got, tt.want)
			}
		})
	}
}

func TestMediaBaseURLEnvKey(t *testing.T) {
	if MediaBaseURLEnvKey != "MEDIA_BASE_URL" {
		t.Errorf("MediaBaseURLEnvKey = %q, want %q", MediaBaseURLEnvKey, "MEDIA_BASE_URL")
	}
}
