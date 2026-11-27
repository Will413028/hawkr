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
}

const FORUM_PAGE_SIZE = 20;

export async function generateMetadata({ params }: Props): Promise<Metadata> {
	const { id, locale } = await params;
	const s = await getCachedStore(id);
	const text = sellerMessagesText(locale);
	if (!s) return { title: text.notFound() };
	const socialImage = sellerSocialImage(s);
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

export default async function MessagesPage({ params }: Props) {
	const { id } = await params;

	const storeId = (await resolveStorefront(id))?.storeId ?? "";
	const initialData = storeId
		? await fetchSellerForum(storeId, {
				pageSize: FORUM_PAGE_SIZE,
				signal: AbortSignal.timeout(getEnv().apiTimeoutMs),
			}).catch(() => undefined)
		: undefined;

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
