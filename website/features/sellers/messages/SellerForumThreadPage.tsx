import { Code, ConnectError } from "@connectrpc/connect";
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import SellerMessageTab from "@/features/sellers/components/SellerMessageTab";
import { getCachedSeller } from "@/lib/api/cached";
import { getEnv } from "@/lib/config/env";
import { fetchSellerForumThread } from "@/lib/forum/repo";
import { canonicalThreadId } from "@/lib/forum/thread-id";
import { storeIdOfSeller } from "@/lib/seller/store-id";
import { localeFromParam } from "@/lib/seo/metadata";
import { sellerCanonicalUrl } from "@/lib/seo/urls";
import { sellerMessagesText } from "./text";

interface Props {
	params: Promise<{ locale: string; id: string; threadId: string }>;
}

const FORUM_PAGE_SIZE = 20;

/** 單一討論串的頁面是連結目標，不重複收錄：標題沿用店家留言頁，canonical 指向店家的留言分頁。 */
export async function generateMetadata({ params }: Props): Promise<Metadata> {
	const { id, locale } = await params;
	const seller = await getCachedSeller(id);
	const text = sellerMessagesText(locale);
	if (!seller) return { title: text.notFound() };
	const canonical = sellerCanonicalUrl(
		seller,
		id,
		localeFromParam(locale),
		"/messages",
	);
	return {
		title: text.title(seller.name),
		alternates: { canonical },
		robots: { index: false },
	};
}

/** 單一討論串（`/sellers/[id]/messages/[threadId]`）：伺服器端讀第一份資料，討論串不存在或讀不到回 404。 */
export default async function ForumThreadPage({ params }: Props) {
	const { id, threadId: rawThreadId, locale } = await params;
	const threadId = canonicalThreadId(rawThreadId);
	if (!threadId) notFound();
	const storeId = storeIdOfSeller(await getCachedSeller(id));
	const thread = storeId
		? await fetchSellerForumThread(storeId, threadId, {
				pageSize: FORUM_PAGE_SIZE,
				locale: localeFromParam(locale),
				signal: AbortSignal.timeout(getEnv().apiTimeoutMs),
			}).catch((e: unknown) => {
				if (ConnectError.from(e).code === Code.NotFound) notFound();
				return undefined;
			})
		: undefined;

	return (
		<section
			className="max-w-[720px] mx-auto py-[var(--hkui-space-6,24px)] px-[var(--hkui-space-5,20px)] pb-[var(--hkui-space-12,48px)]"
			data-hk-page-content-ready="seller-storefront"
		>
			<SellerMessageTab
				sellerId={storeId}
				routeId={id}
				threadId={threadId}
				initialData={thread?.top}
				initialReplies={thread?.replies}
			/>
		</section>
	);
}
