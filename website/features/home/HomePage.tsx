"use client";

import { useQuery } from "@tanstack/react-query";
import ErrorState from "@/components/shared/ErrorState";
import HomeCompactCategoryNav from "@/features/home/components/HomeCompactCategoryNav";
import HomeCompactMarketStage from "@/features/home/components/HomeCompactMarketStage";
import HomeCompactMerchants from "@/features/home/components/HomeCompactMerchants";
import HomeCompactNewArrivals from "@/features/home/components/HomeCompactNewArrivals";
import HomeMemberMobileSearch from "@/features/home/components/HomeMemberMobileSearch";
// market.css + feed-components.css carry the real card visuals;
// without them the cards on the home page render as unstyled blocks
// (this route is outside the (public)/market and (public)/feed layouts
// that normally load them).
import "@/styles/features/market.css";
import "@/styles/features/feed-components.css";
import "@/styles/features/home.css";
import {
	buildHomeSnapshot,
	type HomeSnapshot,
} from "@/features/home/homeSnapshot";
import {
	fetchHomeTabBootstrap,
	HOME_TAB_BOOTSTRAP_FRESH_MS,
} from "@/lib/api/tab-bootstrap";
import { homeSnapshotDeadlineMs } from "@/lib/config/env";
import { openhuninn } from "@/lib/fonts/openhuninn";
import { zenMaruGothic } from "@/lib/fonts/zenMaruGothic";
import { queryKeys } from "@/lib/query/keys";

export const HOME_SNAPSHOT_STALE_MS = HOME_TAB_BOOTSTRAP_FRESH_MS;
const HOME_SNAPSHOT_GC_MS = 30 * 60 * 1000;

/** ErrorState expects a real Error (it logs + shows the digest). */
function toError(value: unknown): Error & { digest?: string } {
	return value instanceof Error
		? value
		: new Error(String(value ?? "home snapshot failed"));
}

export async function loadHomeSnapshot(): Promise<HomeSnapshot> {
	// Browser-owned refresh. It collapses five cross-origin gateway requests into
	// one same-origin request, then reshapes the payload with the same pure
	// `buildHomeSnapshot` the server used — so this background reconcile produces
	// a snapshot structurally identical to the server-rendered one.
	// Deadline comes from lib/config/env so it can never drift below the server's
	// own bootstrap budget: a shorter wait here would silently turn the tail of
	// that budget into dead time (an answer nobody is left listening for).
	const controller = new AbortController();
	const deadline = window.setTimeout(
		() => controller.abort(),
		homeSnapshotDeadlineMs(),
	);
	const bootstrap = await fetchHomeTabBootstrap(controller.signal).finally(() =>
		window.clearTimeout(deadline),
	);
	return buildHomeSnapshot(bootstrap);
}

interface HomeContentProps {
	initialSnapshot: HomeSnapshot;
}

function HomeContent({ initialSnapshot }: HomeContentProps) {
	const isBrowser = typeof window !== "undefined";
	const homeQuery = useQuery({
		queryKey: queryKeys.home.snapshot(),
		queryFn: loadHomeSnapshot,
		// The server built this snapshot and passed it down, so the crawlable HTML
		// and first paint already carry real content. Seeding it as initialData
		// (fresh for staleTime) means the browser query does NOT re-fire the public
		// gateway wave on mount; it only takes over to reconcile price/stock on
		// window focus / reconnect / a later stale remount. `enabled: isBrowser`
		// keeps the queryFn from ever running during SSR.
		initialData: initialSnapshot,
		enabled: isBrowser,
		staleTime: HOME_SNAPSHOT_STALE_MS,
		gcTime: HOME_SNAPSHOT_GC_MS,
		// A cached snapshot stays rendered while TanStack runs the query in the
		// background. Returning to Home or foregrounding an old tab therefore
		// never re-opens the loader, but price/stock/sale fields reconcile instead
		// of living in browser memory for the full fallback window.
		refetchOnMount: true,
		refetchOnWindowFocus: true,
		refetchOnReconnect: true,
		retry: 1,
	});

	if (!homeQuery.data) {
		// With server-provided initialData this branch is a defensive fallback: it
		// can only be reached if the cached snapshot is evicted and a refetch fails
		// before any data exists. Surface the standard error state with retry; its
		// terminal-ready marker also resolves the hard-load splash instead of
		// letting the watchdog time out.
		if (homeQuery.isError) {
			return (
				<ErrorState
					error={toError(homeQuery.error)}
					reset={() => void homeQuery.refetch()}
					homeHref="/market/products"
					homeLabel="逛逛市集"
					// 快照失敗=gateway down/斷網的預期空態：5xx transport 已上報，
					// boundary 再報會分裂 grouping，且每次重試點擊都重複 capture。
					reportToSentry={false}
				/>
			);
		}
		return null;
	}

	const {
		editorRecommendedProducts,
		newArrivalProducts,
		dealProducts,
		originProducts,
		mappedSellers,
		compactUpdates,
		discoveryShelves,
	} = homeQuery.data;

	return (
		<>
			{/* Emitted with the complete snapshot. Because the server renders this
			    subtree into the initial HTML, the marker ships in the document itself
			    — InitialPageLoader's DOMContentLoaded/content check finds it right
			    away and hands first paint straight to real content. */}
			<span data-hk-initial-content-ready="home" hidden />
			<HomeCompactCategoryNav />
			<HomeCompactMarketStage
				posts={compactUpdates}
				products={editorRecommendedProducts}
			/>

			<HomeCompactNewArrivals
				title="新品上市"
				href="/market/products?sort=最新上架"
				products={newArrivalProducts}
			/>

			<HomeCompactNewArrivals
				title="優惠專區"
				href="/market/products"
				products={dealProducts}
			/>

			<HomeCompactNewArrivals
				title="產地專區"
				href="/market/products"
				products={originProducts}
			/>

			<HomeCompactMerchants sellers={mappedSellers} />

			{discoveryShelves.slice(0, 2).map((shelf) => (
				<HomeCompactNewArrivals
					key={shelf.key}
					title={shelf.title.join("")}
					href={shelf.href}
					ariaLabel={shelf.eyebrow}
					products={shelf.products}
				/>
			))}

			{discoveryShelves.slice(2, 4).map((shelf) => (
				<HomeCompactNewArrivals
					key={shelf.key}
					title={shelf.title.join("")}
					href={shelf.href}
					ariaLabel={shelf.eyebrow}
					products={shelf.products}
				/>
			))}

			{discoveryShelves.slice(4).map((shelf) => (
				<HomeCompactNewArrivals
					key={shelf.key}
					title={shelf.title.join("")}
					href={shelf.href}
					ariaLabel={shelf.eyebrow}
					products={shelf.products}
				/>
			))}
		</>
	);
}

interface HomePageProps {
	snapshot: HomeSnapshot;
}

export default function HomePage({ snapshot }: HomePageProps) {
	// The server built `snapshot` and renders this whole tree into the initial
	// HTML (cacheComponents is off — traditional ISR), so product / seller / post
	// cards are crawlable rather than fetched client-side into an empty shell.
	// HomeContent seeds `snapshot` as react-query initialData: first paint uses
	// it directly, and the browser query only reconciles price/stock afterwards.
	return (
		<main
			className={`hkh-page home-page ${zenMaruGothic.variable} ${openhuninn.variable}`}
		>
			{/* sr-only h1 gives the visually heading-less home hero a single,
			    meaningful zh-TW page heading for SEO without changing the layout. */}
			<h1 className="sr-only">Hawkr — 在地小農與職人的生鮮市集</h1>

			<HomeMemberMobileSearch />
			<HomeContent initialSnapshot={snapshot} />
		</main>
	);
}
