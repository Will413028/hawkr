"use client";

import { useQuery } from "@tanstack/react-query";
import { Suspense } from "react";
import ErrorState from "@/components/shared/ErrorState";
import HomeCompactCategoryNav from "@/features/home/components/HomeCompactCategoryNav";
import HomeCompactMarketStage from "@/features/home/components/HomeCompactMarketStage";
import HomeCompactMerchants from "@/features/home/components/HomeCompactMerchants";
import HomeCompactNewArrivals from "@/features/home/components/HomeCompactNewArrivals";
import HomeMemberMobileSearch from "@/features/home/components/HomeMemberMobileSearch";
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
		initialData: initialSnapshot,
		enabled: isBrowser,
		staleTime: HOME_SNAPSHOT_STALE_MS,
		gcTime: HOME_SNAPSHOT_GC_MS,
		refetchOnMount: true,
		refetchOnWindowFocus: true,
		refetchOnReconnect: true,
		retry: 1,
	});

	if (!homeQuery.data) {
		if (homeQuery.isError) {
			return (
				<ErrorState
					error={toError(homeQuery.error)}
					reset={() => void homeQuery.refetch()}
					homeHref="/market/products"
					homeLabel="逛逛市集"
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
	return (
		<div
			className={`hkh-page home-page ${zenMaruGothic.variable} ${openhuninn.variable}`}
		>

			<h1 className="sr-only">Hawkr — 在地小農與職人的生鮮市集</h1>

			<HomeMemberMobileSearch />
			<Suspense fallback={null}>
				<HomeContent initialSnapshot={snapshot} />
			</Suspense>
		</div>
	);
}
