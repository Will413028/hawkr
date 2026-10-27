import type { Metadata } from "next";
import SellerMessageTab from "@/features/sellers/components/SellerMessageTab";
import { getCachedSeller } from "@/lib/api/cached";
import { fetchSellerForum } from "@/lib/seller/forum-repo";
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

const FORUM_PAGE_SIZE = 20;

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
	const canonical = sellerCanonicalUrl(s, id, pageLocale, "/messages");
	return {
		title: `${s.name} 的留言`,
		description: `查看 ${s.name} 的留言與顧客討論`,
		alternates: { canonical },
		openGraph: {
			title: `${s.name} 的留言 | ${siteNameFor(pageLocale)}`,
			description: `查看 ${s.name} 的留言與顧客討論`,
			url: canonical,
			images: socialImages(socialImage),
		},
	};
}

export default async function MessagesPage({ params }: Props) {
	const { id } = await params;

	// Fetch first page on the server so SSR + first paint already have
	// the comment list. useSellerForum will pick this up via its
	// `initialData` argument and skip the bootstrap refetch.
	const initialData = await fetchSellerForum(id, {
		page: 1,
		pageSize: FORUM_PAGE_SIZE,
	}).catch(() => undefined);

	// Chrome (profile header / banner / shell) lives in the persistent
	// (storefront) layout — this page renders only the forum column.
	return (
		<section
			className="max-w-[720px] mx-auto py-[var(--hkui-space-6,24px)] px-[var(--hkui-space-5,20px)] pb-[var(--hkui-space-12,48px)]"
			data-hk-page-content-ready="seller-storefront"
		>
			<SellerMessageTab sellerId={id} initialData={initialData} />
		</section>
	);
}
