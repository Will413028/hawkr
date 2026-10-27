import type { Metadata } from "next";
import { getCachedSeller } from "@/lib/api/cached";
import {
	localeFromParam,
	sellerSocialImage,
	socialImages,
} from "@/lib/seo/metadata";
import { storeSeoDescription, storeSeoTitle } from "@/lib/seo/store-summary";
import { sellerCanonicalUrl } from "@/lib/seo/urls";

interface SellerStorefrontMetadataProps {
	params: Promise<{ locale: string; id: string }>;
}

export async function generateMetadata({
	params,
}: SellerStorefrontMetadataProps): Promise<Metadata> {
	const { id, locale: rawLocale } = await params;
	const s = await getCachedSeller(id);
	// Bare string, no brand: the root title.template appends it. Currently
	// unreachable (a missing seller 404s before this renders) — kept correct so
	// it stays correct if that ever changes.
	if (!s) return { title: "找不到賣家" };
	const locale = localeFromParam(rawLocale);
	const canonical = sellerCanonicalUrl(s, id, locale);
	const socialImage = sellerSocialImage(s);
	// Everything is generated from what the store page shows (ADR
	// 2026-10-09-store-seo-platform-generated); <meta> and og: share one value.
	// Always `{ absolute }`: storeSeoTitle already carries the brand suffix, and
	// the root title.template ("%s | {站名}") would append a second one.
	const source = {
		name: s.name,
		description: s.description,
		county: s.city,
		town: s.district,
		ratingAverage: s.rating,
	};
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
