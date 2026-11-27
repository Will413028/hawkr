import type { Metadata } from "next";
import { getCachedStore } from "@/lib/api/cached";
import { sellerNotFoundTitle } from "@/lib/seller/not-found-title";
import {
	localeFromParam,
	sellerSocialImage,
	socialImages,
} from "@/lib/seo/metadata";
import {
	storeSeoDescription,
	storeSeoSource,
	storeSeoTitle,
} from "@/lib/seo/store-summary";
import { sellerCanonicalUrl } from "@/lib/seo/urls";

interface SellerStorefrontMetadataProps {
	params: Promise<{ locale: string; id: string }>;
}

export async function generateMetadata({
	params,
}: SellerStorefrontMetadataProps): Promise<Metadata> {
	const { id, locale: rawLocale } = await params;
	const s = await getCachedStore(id);
	if (!s) return { title: sellerNotFoundTitle(rawLocale) };
	const locale = localeFromParam(rawLocale);
	const canonical = sellerCanonicalUrl(s, id, locale);
	const socialImage = sellerSocialImage(s);
	const source = storeSeoSource(s);
	const title = storeSeoTitle(source, locale);
	const description = storeSeoDescription(source);
	return {
		title: { absolute: title },
		description,
		alternates: { canonical },
		openGraph: {
			title,
			description,
			url: canonical,
			images: socialImages(socialImage),
			type: "profile",
		},
	};
}
