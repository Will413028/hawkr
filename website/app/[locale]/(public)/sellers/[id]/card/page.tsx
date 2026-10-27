import type { Metadata } from "next";
import { notFound } from "next/navigation";
import JsonLd from "@/components/seo/JsonLd";
import SellerCardPage, {
	type SellerCardSeller,
} from "@/features/sellers/card/SellerCardPage";
import { getCachedSeller } from "@/lib/api/cached";
import { sellerSchema } from "@/lib/seo/jsonld";
import {
	localeFromParam,
	sellerSocialImage,
	socialImages,
} from "@/lib/seo/metadata";
import { sellerCanonicalUrl } from "@/lib/seo/urls";
import "@/styles/features/seller-card-page.css";

interface Props {
	params: Promise<{ locale: string; id: string }>;
}

export const revalidate = 300;

function cardTitle(name?: string): string {
	return name?.trim()
		? `${name.trim()} 的商家名片 | Hawkr`
		: "商家名片 | Hawkr";
}

function cardDescription(seller: SellerCardSeller): string {
	return (
		seller.description?.trim() ||
		`認識 ${seller.name?.trim() || "這間商家"}，查看商家介紹與 Hawkr 商品頁面。`
	);
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
	const { id, locale } = await params;
	const seller = (await getCachedSeller(id)) as SellerCardSeller | null;
	if (!seller) return { title: "找不到賣家" };

	const canonical = sellerCanonicalUrl(
		seller,
		id,
		localeFromParam(locale),
		"/card",
	);
	const title = cardTitle(seller.name);
	const description = cardDescription(seller);
	return {
		title: { absolute: title },
		description,
		alternates: { canonical },
		openGraph: {
			type: "profile",
			title,
			description,
			url: canonical,
			images: socialImages(sellerSocialImage(seller)),
		},
	};
}

export default async function Page({ params }: Props) {
	const { id, locale: rawLocale } = await params;
	const seller = (await getCachedSeller(id)) as SellerCardSeller | null;
	if (!seller) notFound();

	const locale = localeFromParam(rawLocale);
	const canonical = sellerCanonicalUrl(seller, id, locale, "/card");
	return (
		<>
			<JsonLd
				data={sellerSchema({
					name: seller.name?.trim() || "商家",
					url: canonical,
					description: seller.description?.trim(),
					image: sellerSocialImage(seller),
				})}
			/>
			<SellerCardPage seller={seller} sellerId={id} />
		</>
	);
}
