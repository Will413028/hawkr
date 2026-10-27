/**
 * Preloaded into every test process (`pnpm test`, `--import`). The media origin is configuration, not code:
 * unset, `buildImageUrl` builds no media URL at all. Tests that assert on image URLs run against this
 * reserved-name (RFC 2606) origin unless they choose their own.
 */
if (!process.env.NEXT_PUBLIC_MEDIA_BASE_URL?.trim()) {
	process.env.NEXT_PUBLIC_MEDIA_BASE_URL = "https://test-media.example.com";
}
