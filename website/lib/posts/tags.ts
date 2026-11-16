export const POST_TAG_LIMIT = 10;

export function normalizePostTag(raw: string | null | undefined): string {
	return String(raw ?? "")
		.trim()
		.replace(/^#+/, "")
		.trim();
}

export function formatPostTag(raw: string): string {
	const tag = normalizePostTag(raw);
	return tag ? `#${tag}` : "";
}

export function uniquePostTags(
	tags: readonly string[] | null | undefined,
): string[] {
	const seen = new Set<string>();
	const result: string[] = [];
	for (const raw of tags ?? []) {
		const tag = normalizePostTag(raw);
		if (!tag) continue;
		const key = tag.toLowerCase();
		if (seen.has(key)) continue;
		seen.add(key);
		result.push(tag);
	}
	return result;
}

export function extractPostTags(
	text: string,
	limit = POST_TAG_LIMIT,
): string[] {
	const result: string[] = [];
	const seen = new Set<string>();
	for (const match of text.matchAll(/#([\p{L}\p{N}_]+)/gu)) {
		const tag = normalizePostTag(match[1]);
		if (!tag) continue;
		const key = tag.toLowerCase();
		if (seen.has(key)) continue;
		seen.add(key);
		result.push(tag);
		if (result.length >= limit) break;
	}
	return result;
}

export interface PostTaggingMode {
	hashAt: number;
	prefix: string;
}

export function detectPostTaggingMode(
	textBeforeCaret: string,
	triggerAfterChars = 1,
): PostTaggingMode | null {
	if (!textBeforeCaret) return null;
	for (let i = textBeforeCaret.length - 1; i >= 0; i -= 1) {
		const ch = textBeforeCaret[i];
		if (ch === "#") {
			if (i > 0) {
				const prev = textBeforeCaret[i - 1];
				if (prev === "#" || isPostTagChar(prev)) return null;
			}
			const prefix = textBeforeCaret.slice(i + 1);
			if (prefix.length < triggerAfterChars) return null;
			return { hashAt: i, prefix };
		}
		if (!isPostTagChar(ch)) return null;
	}
	return null;
}

function isPostTagChar(ch: string): boolean {
	return /^[\p{L}\p{N}_]$/u.test(ch);
}
