import type { Metadata } from "next";
import InfiniteFeed from "@/features/feed/components/InfiniteFeed";
import { getCachedStore, getCachedStorePosts } from "@/lib/api/cached";
import { prerenderSafe } from "@/lib/api/prerender";
import { sellerNotFoundTitle } from "@/lib/seller/not-found-title";
import { resolveStorefront } from "@/lib/seller/storefront-seller";
import {
	localeFromParam,
	sellerSocialImage,
	siteNameFor,
	socialImages,
} from "@/lib/seo/metadata";
import { sellerCanonicalUrl } from "@/lib/seo/urls";

interface Props {
	params: Promise<{ locale: string; id: string }>;
}

const POSTS_PAGE_SIZE = 12;

export async function generateMetadata({ params }: Props): Promise<Metadata> {
	const { id, locale } = await params;
	const s = await getCachedStore(id);
	if (!s) return { title: sellerNotFoundTitle(locale) };
	const socialImage = sellerSocialImage(s);
	const pageLocale = localeFromParam(locale);
	const canonical = sellerCanonicalUrl(s, id, pageLocale, "/posts");
	return {
		title: `${s.name} 的貼文`,
		description: `查看 ${s.name} 的最新貼文`,
		alternates: { canonical },
		openGraph: {
			title: `${s.name} 的貼文 | ${siteNameFor(pageLocale)}`,
			description: `查看 ${s.name} 的最新貼文與動態`,
			url: canonical,
			images: socialImages(socialImage),
		},
	};
}

export default async function PostsPage({ params }: Props) {
	const { id } = await params;

	const emptyPosts = {
		records: [],
		nextPageToken: "",
		pageSize: POSTS_PAGE_SIZE,
	};
	const storeId = (await resolveStorefront(id))?.storeId ?? "";
	const postsData = storeId
		? await getCachedStorePosts(storeId, POSTS_PAGE_SIZE).catch(
				prerenderSafe(emptyPosts),
			)
		: emptyPosts;

	const posts = postsData.records;
	return (
		<section
			className="seller-posts-board"
			data-hk-page-content-ready="seller-storefront"
		>
			{posts.length > 0 ? (
				<InfiniteFeed
					initialPosts={posts}
					initialPage={1}
					initialNextPageToken={postsData.nextPageToken}
					pageSize={POSTS_PAGE_SIZE}
					sellerId={storeId}
					initialPostsMatchQuery
				/>
			) : (
				<div className="seller-posts-empty">
					<h3>還沒有貼文</h3>
					<p>等賣家分享新的採收、出貨或食材故事後，這裡會自動更新。</p>
				</div>
			)}
		</section>
	);
}
