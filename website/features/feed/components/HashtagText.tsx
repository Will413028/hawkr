"use client";

import * as React from "react";
import Link from "@/components/shared/Link";
import { normalizePostTag } from "@/lib/post-tags/repo";

interface HashtagTextProps {
	content: string;
	className?: string;
	as?: "p" | "div" | "span";
	onNavigate?: () => void;
}

const HASHTAG_PATTERN = /#[\p{L}\p{N}_]+/gu;

export default function HashtagText({
	content,
	className,
	as = "span",
	onNavigate,
}: HashtagTextProps) {
	const children = React.useMemo(() => {
		const parts: React.ReactNode[] = [];
		let last = 0;
		for (const match of content.matchAll(HASHTAG_PATTERN)) {
			const raw = match[0];
			const index = match.index ?? 0;
			if (index > last) parts.push(content.slice(last, index));
			const tag = normalizePostTag(raw);
			parts.push(
				<Link
					key={`${tag}-${index}`}
					href={`/feed?hashtag=${encodeURIComponent(tag)}`}
					className="hkui-hashtag-link"
					onClick={(event) => {
						event.stopPropagation();
						onNavigate?.();
					}}
				>
					#{tag}
				</Link>,
			);
			last = index + raw.length;
		}
		if (last < content.length) parts.push(content.slice(last));
		return parts;
	}, [content, onNavigate]);

	return React.createElement(as, { className }, children);
}
