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
 * - **啟用條件** → capability。沿用 `activeRole === "seller"` 在代行下
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
	const resolverRef = React.useRef<
		((decision: SellerMutationDecision) => void) | null
	>(null);

	const bundle = React.useMemo(() => createDelegatedRepos(storeId), [storeId]);

	const identity = React.useMemo<DelegatedIdentity>(
		() =>
			summary
				? delegatedIdentityOf(summary)
				:
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
