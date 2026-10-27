import SellerPostsPage, {
	generateMetadata,
} from "@/features/sellers/posts/SellerPostsPage";
import { getCachedSeller } from "@/lib/api/cached";
import { sellerStaticParams } from "@/lib/api/static-params";

export { generateMetadata };

interface Props {
	params: Promise<{ locale: string; id: string }>;
}

// Seller-profile freshness tier (on-demand ISR; long-tail seller routes resolve
// on first visit).
export const revalidate = 1800;
export const generateStaticParams = sellerStaticParams;

export default async function Page(props: Props) {
	const { id } = await props.params;
	const seller = await getCachedSeller(id);
	const name = seller?.name ?? "商家";
	return (
		<>
			{/* Keep exactly one semantic heading in the atomic route subtree.
			    SellerProfileV2 renders the same visible name as a non-heading. */}
			<h1 className="sr-only">{name}</h1>
			<SellerPostsPage params={props.params} />
		</>
	);
}
