import { buildImageUrl } from "./media";

/**
 * Backward-compatible wrapper over `buildImageUrl` (design book 49). Kept so the
 * existing call sites need no edit; new code should call `buildImageUrl(value,
 * preset)` directly. Returns `null` for empty input to preserve the previous
 * nullable contract that callers depend on (`resolveOssUrl(x) ?? fallback`).
 */
export function resolveOssUrl(url: string): string | null {
	return buildImageUrl(url, "original") || null;
}
