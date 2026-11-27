import JsonLd from "@/components/seo/JsonLd";
import { StorefrontPage } from "@/features/sellers/storefront";
import { getCachedStore } from "@/lib/api/cached";
import { sellerStaticParams } from "@/lib/api/static-params";
import { breadcrumbSchema, sellerSchema } from "@/lib/seo/jsonld";
import {
	localeFromParam,
	type SellerSocialImageSource,
	sellerSocialImage,
} from "@/lib/seo/metadata";
import { absoluteUrl, sellerCanonicalUrl } from "@/lib/seo/urls";

export { generateMetadata } from "@/features/sellers/storefront/metadata";

type SellerLike = SellerSocialImageSource & {
	name: string;
	description?: string;
	storeId?: string;
};

interface Props {
	params: Promise<{ locale: string; id: string }>;
}

export const generateStaticParams = sellerStaticParams;

export default async function Page({ params }: Props) {
	const [{ id, locale: rawLocale }, seller] = await Promise.all([
		params,
		params.then(({ id: sellerId }) => getCachedStore(sellerId)),
	]);
	const locale = localeFromParam(rawLocale);
	const s = seller as SellerLike | null | undefined;
	const canonical = sellerCanonicalUrl(seller, id, locale);

	const jsonLd = s ? (
		<JsonLd
			data={[
				sellerSchema({
					name: s.name,
					url: canonical,
					description: s.description,
					image: sellerSocialImage(s),
				}),
				breadcrumbSchema([
					{ name: "Hawkr 市集", url: absoluteUrl("/market/products", locale) },
					{ name: s.name, url: canonical },
				]),
			]}
		/>
	) : null;

	return (
		<>
			{jsonLd}

			<h1 className="sr-only">{s?.name ?? "商家"}</h1>
			{s?.storeId ? <StorefrontPage sellerId={s.storeId} /> : null}
		</>
	);
}
