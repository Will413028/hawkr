"use client";

import type * as React from "react";
import { useUrlSearchParams } from "@/lib/hooks/useUrlSearchParams";
import { MAIN_TAB_STALE_MS } from "@/lib/navigation/main-tab-freshness";
import SearchForm from "./components/SearchForm";
import SearchFreshnessBridge from "./components/SearchFreshnessBridge";
import SearchHashtagNav from "./components/SearchHashtagNav";
import SearchResultsClient from "./components/SearchResultsClient";

/**
 * Search page client shell.
 *
 * `/search` is noindex and ISR: the server renders the query-less landing
 * (`landing`, passed as a server-rendered node so its editorial rails stay in
 * the static HTML) and this component reads `?q` / `?sort` from the URL —
 * without `useSearchParams` (which would bail the route out of static
 * rendering). With no query it shows the landing; with a query it fetches and
 * renders results client-side. Both the search box and the hashtag rail live
 * here so a submit/sort/tag click re-reads the URL and swaps the body in place,
 * with no per-request dynamic server render.
 */
export default function SearchClient({
	landing,
}: {
	landing: React.ReactNode;
}) {
	const params = useUrlSearchParams();
	const query = params.get("q") ?? "";
	const sort = params.get("sort") || "推薦";
	const isLanding = !query;

	return (
		<main className="search-page pb-20" data-hk-page-content-ready="search">
			<SearchFreshnessBridge
				scope="search:landing"
				staleMs={MAIN_TAB_STALE_MS.search}
				enabled={isLanding}
			/>
			{/* Slim header — search box is the primary surface; the editorial
			 *  headline shrinks so results dominate the page. */}
			<section className="w-[min(1440px,calc(100%-48px))] mx-auto max-md:w-[min(100%-28px,680px)] pt-[28px] pb-6 max-[840px]:pt-[38px]">
				<span className="inline-flex w-fit mb-3.5 text-hk-primary-strong text-xs font-bold tracking-[0.12em] uppercase">
					搜尋
				</span>
				<h1 className="mt-1.5 mb-[18px] text-[clamp(1.625rem,3.4vw,2.375rem)] leading-[1.18] max-w-[640px]">
					找找在地好物
				</h1>

				<SearchForm query={query} sort={sort} />

				<SearchHashtagNav query={query} />
			</section>

			{isLanding ? landing : <SearchResultsClient query={query} sort={sort} />}
		</main>
	);
}
