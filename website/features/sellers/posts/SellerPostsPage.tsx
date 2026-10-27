import type { Metadata } from "next";
import InfiniteFeed from "@/features/feed/components/InfiniteFeed";
import { getCachedSeller } from "@/lib/api/cached";
import type { PostListItem } from "@/lib/api/openapi-types";
import { fetchPublicPostsTyped } from "@/lib/api/posts";
import { prerenderSafe } from "@/lib/api/prerender";
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
	const seller = await getCachedSeller(id);
	const s = seller;
	if (!s) return { title: "找不到賣家" };
	const socialImage = sellerSocialImage(s);
	// Self-canonical on the store_code, the same way the storefront index does it.
	// Middleware already 308s a store_id URL here, but the redirect and this tag
	// are two independent signals and this tab used to emit neither: the file set
	// no `alternates`, so the only canonicalization a crawler could see on a tab
	// URL was the redirect. One signal is a single point of failure.
	//
	// The tab suffix is carried on purpose: this consolidates the two ids for the
	// SAME tab. Pointing the tab at `/sellers/{code}` instead would additionally
	// ask search engines to drop this tab's URL from the index — a separate
	// decision, and not one #365 asked for.
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

	// Posts is an infinite-scroll feed with no `?page` pagination UI, so the
	// first page is the SEO/ISR body and InfiniteFeed loads more in place. The
	// route therefore reads no request-time input and stays statically renderable.
	const postsData = await fetchPublicPostsTyped({
		page: 1,
		pageSize: POSTS_PAGE_SIZE,
		sellerId: id,
	}).catch(prerenderSafe({ records: [], total: 0, totalPages: 0 }));

	const posts = postsData.records as PostListItem[];

	// Chrome (profile header / banner / shell) lives in the persistent
	// (storefront) layout — this page renders only the posts board.
	return (
		<section
			className="seller-posts-board"
			data-hk-page-content-ready="seller-storefront"
		>
			{posts.length > 0 ? (
				<InfiniteFeed
					initialPosts={posts}
					initialPage={1}
					pageSize={POSTS_PAGE_SIZE}
					sellerId={id}
					// This page fetched page one for this exact seller, so the feed
					// keeps that seed. Without the declaration the scoped query would
					// discard it, refetch page one on the client, and flash a
					// full-page loading skeleton over content already rendered.
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
