import SellerForumThreadPage, {
	generateMetadata,
} from "@/features/sellers/messages/SellerForumThreadPage";
import { getCachedStore } from "@/lib/api/cached";
import {
	SENTINEL_ENTITY_ID,
	SENTINEL_THREAD_ID,
} from "@/lib/api/static-params";

export { generateMetadata };

interface Props {
	params: Promise<{ locale: string; id: string; threadId: string }>;
}

export function generateStaticParams() {
	return [{ id: SENTINEL_ENTITY_ID, threadId: SENTINEL_THREAD_ID }];
}

export default async function Page(props: Props) {
	const { id } = await props.params;
	const seller = await getCachedStore(id);
	const name = seller?.name ?? "商家";
	return (
		<>
			{/* Keep exactly one semantic heading in the atomic route subtree.
			    SellerProfileV2 renders the same visible name as a non-heading. */}
			<h1 className="sr-only">{name}</h1>
			<SellerForumThreadPage params={props.params} />
		</>
	);
}
