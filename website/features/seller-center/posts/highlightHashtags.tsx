import type { ReactNode } from "react";

/** Same token shape as extractPostTags (#letters/digits/underscore, CJK-safe). */
const HASHTAG_TOKEN_RE = /#[\p{L}\p{N}_]+/gu;

/** Preserve the source text and glyph metrics while tinting post hashtags. */
export function highlightHashtags(text: string): ReactNode {
	const nodes: ReactNode[] = [];
	let cursor = 0;
	let key = 0;
	for (const match of text.matchAll(HASHTAG_TOKEN_RE)) {
		const start = match.index ?? 0;
		if (start > cursor) nodes.push(text.slice(cursor, start));
		nodes.push(
			<span key={`h${key++}`} className="post-compose__hashtag-token">
				{match[0]}
			</span>,
		);
		cursor = start + match[0].length;
	}
	if (cursor < text.length) nodes.push(text.slice(cursor));
	return nodes;
}
