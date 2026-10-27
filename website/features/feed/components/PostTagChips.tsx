"use client";

import Link from "@/components/shared/Link";
import { uniquePostTags } from "@/lib/post-tags/repo";

interface PostTagChipsProps {
	tags?: readonly string[] | null;
	className?: string;
	chipClassName?: string;
	limit?: number;
	onNavigate?: () => void;
}

export default function PostTagChips({
	tags,
	className = "hkui-feed-card__hashtags",
	chipClassName = "hkui-feed-card__hashtag",
	limit,
	onNavigate,
}: PostTagChipsProps) {
	const normalized = uniquePostTags(tags);
	const visible = limit ? normalized.slice(0, limit) : normalized;
	const overflow =
		limit && normalized.length > limit ? normalized.length - limit : 0;
	if (visible.length === 0) return null;
	return (
		<nav className={className} aria-label="貼文 hashtag">
			{visible.map((tag) => (
				<Link
					key={tag}
					href={`/feed?hashtag=${encodeURIComponent(tag)}`}
					className={chipClassName}
					onClick={(event) => {
						event.stopPropagation();
						onNavigate?.();
					}}
				>
					#{tag}
				</Link>
			))}
			{overflow ? <span className={chipClassName}>+{overflow}</span> : null}
		</nav>
	);
}
