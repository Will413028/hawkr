"use client";

import * as React from "react";
import type { DelegatedMutationMeta } from "@/lib/api/delegated";
import { queryKeys } from "@/lib/query/keys";
import { createDelegatedRepos } from "@/lib/seller-data/delegated-repos";
import {
	type SellerDataArea,
	type SellerDataScope,
	SellerDataScopeProvider,
	type SellerMutationDecision,
	type SellerMutationRequest,
} from "@/lib/seller-data/scope";
import OperationConfirmDrawer from "../components/OperationConfirmDrawer";
import { useDelegatedSession } from "./DelegatedSessionContext";
import { type DelegatedIdentity, delegatedIdentityOf } from "./session";

/**
 * 把代行工作階段接到賣家中心的資料層（56 號 §4.5 L1053／定案第 1 條、第 4 條）。
 *
 * 三樣東西同時換掉，缺一都是靜默失敗：
 * - **傳輸層** → `delegatedFetch`（同源 `/adm/api/*`，憑證是 `Path=/adm` 的 cookie）
 * - **快取鍵** → `queryKeys.delegated.*`（工作階段身分，不是登入身分）。沿用
 *   `QueryIdentity` 時代行的 `accountId`／`activeRole` 恆為 `null` ⇒ 先後代行 A 店與
 *   B 店會共用同一個快取槽，第二家直接看到第一家的資料
 * - **啟用條件** → capability。沿用 `isLoggedIn && activeRole === "seller"` 在代行下
 *   恆為 false ⇒ 一個查詢都不會發，畫面只顯示空清單、不報任何錯
 *
 * 寫入另外多一道 `confirmMutation`：代行的每一筆寫入都必須帶修改原因／目前版本／
 * 通知賣家三欄（§3.4），而賣家中心的表單沒有它們。
 */

/** 面 → 讀取 capability（§6.1.1 端點總表）。 */
const AREA_READ_CAPABILITY: Record<SellerDataArea, string> = {
	storefront: "storefront.read",
	products: "products.read",
	posts: "posts.read",
	orders: "orders.read",
};

export default function DelegatedSellerDataProvider({
	storeId,
	children,
}: {
	storeId: string;
	children: React.ReactNode;
}) {
	const { summary, capabilities, writesAllowed } = useDelegatedSession();
	const [request, setRequest] = React.useState<SellerMutationRequest | null>(
		null,
	);
	// resolver 放 ref 而不是 state：它是一次性的 continuation，進 state 會讓每次
	// setState 都重新渲染抽屜、並在 StrictMode 下被呼叫兩次。
	const resolverRef = React.useRef<
		((decision: SellerMutationDecision) => void) | null
	>(null);

	const bundle = React.useMemo(() => createDelegatedRepos(storeId), [storeId]);

	const identity = React.useMemo<DelegatedIdentity>(
		() =>
			summary
				? delegatedIdentityOf(summary)
				: // 首抓未回時給一個絕不會與任何真實工作階段相同的身分。這條路徑下
					// `canRead` 也是 false，所以不會有查詢真的用到它。
					{ adminId: "", storeId, issuedAt: "", revision: 0 },
		[storeId, summary],
	);

	const settle = React.useCallback((decision: SellerMutationDecision) => {
		const resolve = resolverRef.current;
		resolverRef.current = null;
		setRequest(null);
		resolve?.(decision);
	}, []);

	const confirmMutation = React.useCallback(
		(next: SellerMutationRequest): Promise<SellerMutationDecision> => {
			// 前一筆還開著就先取消它：兩個確認抽屜同時存在時，操作者無從分辨自己正在
			// 為哪一次寫入填理由。
			resolverRef.current?.({ ok: false });
			setRequest(next);
			return new Promise<SellerMutationDecision>((resolve) => {
				resolverRef.current = resolve;
			});
		},
		[],
	);

	const scope = React.useMemo<SellerDataScope>(
		() => ({
			mode: "delegated",
			repos: bundle.repos,
			keys: {
				storefront: () => queryKeys.delegated.storefront(identity),
				products: (params) => queryKeys.delegated.products(identity, params),
				// 賣家路徑用 `sellerId` 分槽（同一瀏覽器可看多個賣家的貼文）；代行的
				// 目標店家已經在 identity 裡，第二個店家鍵沒有意義，忽略它。
				posts: (_sellerId, params) =>
					queryKeys.delegated.posts(identity, params),
				orders: (params) => queryKeys.delegated.orders(identity, params),
				order: (orderNo, params) =>
					queryKeys.delegated.order(identity, orderNo, params),
				orderCounts: (params) =>
					queryKeys.delegated.orders(identity, { counts: true, ...params }),
			},
			canRead: (area) =>
				Boolean(summary) && capabilities.includes(AREA_READ_CAPABILITY[area]),
			ownerStoreId: storeId,
			ownerStoreName: summary?.target.storeName ?? null,
			confirmMutation,
			hasCapability: (capability) => capabilities.includes(capability),
			delegatedCommands: {
				batchUnpublishProducts: (items, meta) =>
					bundle.products.batchUnpublish(items, meta),
				publishPost: (postId, meta, schedule) =>
					bundle.posts.publish(postId, meta, schedule),
			},
		}),
		[bundle, capabilities, confirmMutation, identity, storeId, summary],
	);

	// 工作階段轉終態（到期／撤銷／結束結果不明）時，開著的確認抽屜必須關掉：那一筆
	// 寫入此刻必然被拒，留著它等於邀請操作者把理由打完再收到一個沒有 message 的錯誤。
	React.useEffect(() => {
		if (writesAllowed) return;
		if (resolverRef.current) settle({ ok: false });
	}, [settle, writesAllowed]);

	return (
		<SellerDataScopeProvider scope={scope}>
			{children}
			<OperationConfirmDrawer
				request={request}
				onCancel={() => settle({ ok: false })}
				onConfirm={(meta: DelegatedMutationMeta) => settle({ ok: true, meta })}
			/>
		</SellerDataScopeProvider>
	);
}
