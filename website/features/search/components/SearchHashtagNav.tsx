"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "@/components/shared/Link";
import { type SearchHashtagHit, searchHashtags } from "@/lib/search/repo";

/**
 * Client hashtag rail for `/search`. The route is ISR, so the query-dependent
 * hashtag set is fetched here rather than during a per-request server render.
 * With no query it shows trending tags; with a query it shows related ones —
 * same `/public/search/*` transport and markup as the previous server render.
 * Clicking a chip turns the hashtag into a normal search query (stays on the
 * search-page mental model instead of jumping to /feed).
 */
export default function SearchHashtagNav({ query }: { query: string }) {
	const { data } = useQuery<SearchHashtagHit[]>({
		queryKey: ["search", "hashtags", query],
		queryFn: () => searchHashtags(query).catch(() => [] as SearchHashtagHit[]),
		staleTime: 60_000,
		gcTime: 5 * 60 * 1000,
		placeholderData: (previous) => previous,
	});
	const hashtags = data ?? [];
	if (!hashtags.length) return null;
	return (
		<nav
			className="flex flex-wrap gap-2 mb-[18px]"
			aria-label={query ? "相關 hashtag" : "熱門 hashtag"}
		>
			{hashtags.slice(0, 10).map((tag) => (
				<Link
					href={`/search?q=${encodeURIComponent(tag.name)}`}
					key={tag.name}
					className="px-3.5 py-2 text-[0.8125rem] rounded-full bg-[var(--surface-card,rgba(255,255,255,0.7))] border border-[var(--border-soft,rgb(var(--color-ink-950-rgb)/0.1))] text-[var(--ink-body)] transition-all duration-[var(--hkui-dur-fast)] ease-[var(--hkui-ease)] hover:bg-hk-primary hover:border-hk-primary hover:text-hk-inverse dark:bg-white/6 dark:border-[rgba(245,222,165,0.18)] dark:text-[rgba(245,222,165,0.92)] dark:hover:bg-hk-primary dark:hover:border-hk-primary dark:hover:text-hk-inverse"
				>
					<span aria-hidden="true">#</span>
					{tag.label?.replace(/^#/, "") || tag.name}
				</Link>
			))}
		</nav>
	);
}
