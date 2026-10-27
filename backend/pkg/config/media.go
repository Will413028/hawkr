package config

// MediaBaseURLEnvKey is the environment variable backend-payuni services
// read for the public-facing base URL used to render object-storage keys
// into browsable media URLs (49 號設計書 §8.5).
const MediaBaseURLEnvKey = "MEDIA_BASE_URL"

// MediaBaseURLMissing reports whether mediaBaseURL must fail config
// validation. MEDIA_BASE_URL has no default domain — a missing value fails
// fast (backend#2283): dev always uses test-media.example.com, prod
// always uses media.example.com. There is no legacy/new compatibility
// fallback and no domain guessing: an unset value must stop startup rather
// than silently render dead links.
func MediaBaseURLMissing(mediaBaseURL string) bool {
	return mediaBaseURL == ""
}
