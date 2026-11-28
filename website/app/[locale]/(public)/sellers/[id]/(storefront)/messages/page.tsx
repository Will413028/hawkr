import SellerMessagesPage, {
	generateMetadata,
} from "@/features/sellers/messages/SellerMessagesPage";
import { getCachedStore } from "@/lib/api/cached";
import { sellerStaticParams } from "@/lib/api/static-params";

export { generateMetadata };

interface Props {
	params: Promise<{ locale: string; id: string }>;
}

export const generateStaticParams = sellerStaticParams;

export default async function Page(props: Props) {
	const { id } = await props.params;
	const seller = await getCachedStore(id);
	const name = seller?.name ?? "商家";
	return (
		<>
			<h1 className="sr-only">{name}</h1>
			<SellerMessagesPage params={props.params} />
		</>
	);
}
