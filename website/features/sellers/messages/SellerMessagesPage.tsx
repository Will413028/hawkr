import type { Metadata } from "next";
import SellerMessageTab from "@/features/sellers/components/SellerMessageTab";
import { getCachedStore } from "@/lib/api/cached";
import { getEnv } from "@/lib/config/env";
import { fetchSellerForum } from "@/lib/forum/repo";
import { resolveStorefront } from "@/lib/seller/storefront-seller";
import {
	localeFromParam,
	sellerSocialImage,
	siteNameFor,
	socialImages,
} from "@/lib/seo/metadata";
import { sellerCanonicalUrl } from "@/lib/seo/urls";
import { sellerMessagesText } from "./text";

interface Props {
	params: Promise<{ locale: string; id: string }>;
	/** 賣家預覽：網址段是 store_id，不是 store_code。 */
	preview?: boolean;
}

const FORUM_PAGE_SIZE = 20;

export async function generateMetadata({ params }: Props): Promise<Metadata> {
	const { id, locale } = await params;
	const s = await getCachedStore(id);
	const text = sellerMessagesText(locale);
	if (!s) return { title: text.notFound() };
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
		title: text.title(s.name),
		description: `查看 ${s.name} 的留言與顧客討論`,
		alternates: { canonical },
		openGraph: {
			title: `${text.title(s.name)} | ${siteNameFor(pageLocale)}`,
			description: `查看 ${s.name} 的留言與顧客討論`,
			url: canonical,
			images: socialImages(socialImage),
		},
	};
}

export default async function MessagesPage({ params, preview = false }: Props) {
	const { id } = await params;

	// Fetch first page on the server so SSR + first paint already have
	// the comment list. useSellerForum will pick this up via its
	// `initialData` argument and skip the bootstrap refetch.
	const storeId = (await resolveStorefront(id, preview))?.storeId ?? "";
	const initialData = storeId
		? await fetchSellerForum(storeId, {
				pageSize: FORUM_PAGE_SIZE,
				signal: AbortSignal.timeout(getEnv().apiTimeoutMs),
			}).catch(() => undefined)
		: undefined;

	// Chrome (profile header / banner / shell) lives in the persistent
	// (storefront) layout — this page renders only the forum column.
	return (
		<section
			className="max-w-[720px] mx-auto py-[var(--hkui-space-6,24px)] px-[var(--hkui-space-5,20px)] pb-[var(--hkui-space-12,48px)]"
			data-hk-page-content-ready="seller-storefront"
		>
			<SellerMessageTab
				sellerId={storeId}
				routeId={id}
				initialData={initialData}
			/>
		</section>
	);
}
