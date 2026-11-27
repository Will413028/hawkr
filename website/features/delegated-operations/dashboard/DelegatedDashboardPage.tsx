"use client";

import { useQuery } from "@tanstack/react-query";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { SummaryStatCard } from "@/features/seller-center/components/SummaryStatCard";
import { delegatedFetch } from "@/lib/api/delegated";
import { PRODUCT_STATUS_LABEL, ProductStatus } from "@/lib/codemeta/enums";
import { useSellerProducts } from "@/lib/hooks/useSellerProducts";
import { queryKeys } from "@/lib/query/keys";
import { createDelegatedPostRepo } from "@/lib/seller-data/delegated-repos";
import { DELEGATED_ORDER_STAT_CARDS, orderStatusText } from "../orders/display";
import { EMPTY_DELEGATED_ORDER_FILTERS } from "../orders/filters";
import { useDelegatedOrderStatusTotals } from "../orders/useDelegatedOrders";
import { useDelegatedSession } from "../state/DelegatedSessionContext";
import {
	canRenderDelegatedWorkspace,
	DELEGATED_PENDING_IDENTITY,
	delegatedIdentityOf,
} from "../state/session";

/**
 * 代行工作區・賣家總覽（§6.1.14 第 1 列／定案第 3 條）。
 *
 * ⚠️ **刻意不是 `features/seller-center/dashboard/DashboardPage.tsx`。** 那一支的
 * 三塊主要內容在代行下各自有獨立的否決理由：
 * - `useSellerDashboard()`（本月營收／商品瀏覽／新追蹤者）**沒有代行端點**，接上它
 *   等於放寬一支統計 RPC —— 不碰 `delegatedRoutes` registry、不觸發 boot 斷言、
 *   不需要新 capability，**CI 全綠而攻擊面已經擴大**（§6.1.8 禁令）。
 * - 「快速操作」四顆是**匯出訂單／建立優惠券／新增商品／發布動態** —— 前兩顆分別
 *   落在履約與封閉九值之外，代行按下去只會拿到 403 或打到管理員自己的帳號。
 * - 開店進度卡讀 `useSellerSelf`／`useShippingOnboardings`，
 *   後一支是履約禁區。
 *
 * ⇒ 這一頁只做一件事：把**既有三支唯讀列表端點的 `total`** 排成 KPI（B15 明列的
 * 唯一合法作法，與訂單頁四張卡同一個機制），後端零新增。缺哪一個 read capability
 * 就整塊不渲染 —— 不是渲染 0，那是一個假宣稱。
 */

/** 一張卡＝一次 `page_size=1` 的帶狀態列表，只讀它的 `total`。 */
const TOTAL_PROBE = { page: 1, pageSize: 1 } as const;

/**
 * 訂單卡沿用訂單頁那一組 status（`DELEGATED_ORDER_STAT_CARDS`）。
 *
 * 內容相同 ⇒ 與訂單頁**共用同一個查詢槽**（TanStack 以值而非參考雜湊鍵），總覽與
 * 訂單頁的數字因此不可能各說各話 —— A-10 那類「詳情頁 0、列表 106」正是兩個各自
 * 算數的來源造成的。
 */
const ORDER_STAT_STATUSES = DELEGATED_ORDER_STAT_CARDS.map(
	(card) => card.status,
);

/** 載入中／查詢未啟用時一律破折號，不是 `0`：`0` 是一個宣稱。 */
function statValue(total: number | null | undefined): string {
	return total == null ? "—" : total.toLocaleString();
}

/** 一～三支就夠：來源清單恆為訂單／商品／動態三者的子集。 */
const COUNT_WORD: Record<number, string> = { 1: "一", 2: "兩", 3: "三" };

/**
 * 說明列的第一句。**必須跟著實際渲染出來的區塊走。**
 *
 * ⚠️ 這句話曾寫死「本次代行三支唯讀清單的總數」，而區塊是逐一由 capability 決定
 * 渲染的：只核發 `products.read` 的工作階段畫面上只有「商品」一塊，說明卻仍宣稱三支
 * —— 讀的人會去找另外兩塊，找不到就只能推論是壞了，而實際上是這次沒有核發那兩項
 * 讀取權限。數字與清單各自為政的敘述，正是本輪要消滅的那一類。
 */
export function sectionsSentence(sections: ReadonlyArray<string>): string {
	if (sections.length === 0) return "";
	const names = sections.join("、");
	return `這裡的每一個數字都是本次代行「${names}」${COUNT_WORD[sections.length] ?? sections.length}支唯讀清單的總數，與清單頁逐格相同。`;
}

export default function DelegatedDashboardPage() {
	const { summary, state, hasCapability } = useDelegatedSession();
	const storeId = summary?.target.storeId ?? "";
	const ready = canRenderDelegatedWorkspace(state) && Boolean(summary);
	const canReadOrders = ready && hasCapability("orders.read");
	const canReadProducts = ready && hasCapability("products.read");
	const canReadPosts = ready && hasCapability("posts.read");
	const base = `/adm/${encodeURIComponent(storeId)}`;

	const { totals: orderTotals } = useDelegatedOrderStatusTotals(
		ORDER_STAT_STATUSES,
		EMPTY_DELEGATED_ORDER_FILTERS,
	);
	const productsAll = useSellerProducts(TOTAL_PROBE, {
		enabled: canReadProducts,
	});
	const productsActive = useSellerProducts(
		{ ...TOTAL_PROBE, status: ProductStatus.ACTIVE },
		{ enabled: canReadProducts },
	);
	const productsInactive = useSellerProducts(
		{ ...TOTAL_PROBE, status: ProductStatus.INACTIVE },
		{ enabled: canReadProducts },
	);
	const productsDraft = useSellerProducts(
		{ ...TOTAL_PROBE, status: ProductStatus.DRAFT },
		{ enabled: canReadProducts },
	);
	const postTotal = useDelegatedPostTotal(canReadPosts);
	const grantedSections = [
		canReadOrders ? "訂單" : "",
		canReadProducts ? "商品" : "",
		canReadPosts ? "動態" : "",
	].filter(Boolean);

	if (!canReadOrders && !canReadProducts && !canReadPosts) {
		return (
			<section className="adm-panel">
				<h1 className="adm-panel__title">本次代行沒有可彙總的資料</h1>
				<p className="adm-panel__hint">
					{
						"總覽由訂單、商品與動態三份唯讀清單的總數組成，本次工作階段一項讀取權限都沒有核發。需要查看請返回 ADM 重新建立工作階段。"
					}
				</p>
			</section>
		);
	}

	return (
		<SellerPageShell
			className="adm-dashboard"
			title="賣家總覽"
			subtitle={`以平台管理員身分檢視 ${summary?.target.storeName ?? ""}`.trim()}
		>
			<div className="adm-dashboard__note" role="note">
				<AppIcon name="info" size={20} />
				<span>
					{sectionsSentence(grantedSections)}{" "}
					營收、瀏覽數與追蹤者屬於統計端點，不在代行授權範圍，因此不呈現。
				</span>
			</div>

			{canReadOrders ? (
				<section className="adm-dashboard__section">
					<div className="adm-dashboard__section-head">
						<h2>訂單</h2>
						<Link className="adm-dashboard__link" href={`${base}/orders`}>
							前往訂單管理
							<AppIcon name="arrow-right" size={16} />
						</Link>
					</div>
					<div className="adm-dashboard__stats">
						{DELEGATED_ORDER_STAT_CARDS.map((card) => (
							<SummaryStatCard
								key={card.status || "all"}
								label={
									"label" in card
										? card.label
										: orderStatusText(card.status, "")
								}
								value={statValue(orderTotals[card.status])}
								tint={card.tint}
								icon={<AppIcon name={card.icon} size={20} />}
							/>
						))}
					</div>
				</section>
			) : null}

			{canReadProducts ? (
				<section className="adm-dashboard__section">
					<div className="adm-dashboard__section-head">
						<h2>商品</h2>
						<Link className="adm-dashboard__link" href={`${base}/products`}>
							前往商品管理
							<AppIcon name="arrow-right" size={16} />
						</Link>
					</div>
					<div className="adm-dashboard__stats">
						<SummaryStatCard
							label="總商品數"
							value={statValue(productsAll.data?.total)}
							tint="primary"
							icon={<AppIcon name="category" size={20} />}
						/>

						<SummaryStatCard
							label={PRODUCT_STATUS_LABEL[ProductStatus.ACTIVE]}
							value={statValue(productsActive.data?.total)}
							tint="info"
							icon={<AppIcon name="check" size={20} />}
						/>
						<SummaryStatCard
							label={PRODUCT_STATUS_LABEL[ProductStatus.INACTIVE]}
							value={statValue(productsInactive.data?.total)}
							tint="accent"
							icon={<AppIcon name="danger" size={20} />}
						/>
						<SummaryStatCard
							label={PRODUCT_STATUS_LABEL[ProductStatus.DRAFT]}
							value={statValue(productsDraft.data?.total)}
							tint="secondary"
							icon={<AppIcon name="document" size={20} />}
						/>
					</div>
				</section>
			) : null}

			{canReadPosts ? (
				<section className="adm-dashboard__section">
					<div className="adm-dashboard__section-head">
						<h2>動態</h2>
						<Link className="adm-dashboard__link" href={`${base}/posts`}>
							前往動態管理
							<AppIcon name="arrow-right" size={16} />
						</Link>
					</div>
					<div className="adm-dashboard__stats">
						<SummaryStatCard
							label="動態總數"
							value={statValue(postTotal)}
							tint="accent"
							icon={<AppIcon name="edit" size={20} />}
						/>
					</div>
				</section>
			) : null}
		</SellerPageShell>
	);
}

/**
 * 貼文總數。
 *
 * 與訂單／商品不同，貼文走自己的 repo 實例而不是 `useSellerDataScope()`：這裡直接以
 * 目標店家建立代行的貼文 repo，數字是伺服器的各狀態篇數加總（`fetchPostCount`），
 * 不是列表長度。
 */
function useDelegatedPostTotal(enabled: boolean): number | null {
	const { summary } = useDelegatedSession();
	const storeId = summary?.target.storeId ?? "";
	const identity = summary
		? delegatedIdentityOf(summary)
		: DELEGATED_PENDING_IDENTITY;
	const repo = React.useMemo(
		() => (storeId ? createDelegatedPostRepo(delegatedFetch, storeId) : null),
		[storeId],
	);
	const query = useQuery({
		queryKey: queryKeys.delegated.posts(identity, { kind: "total" }),
		queryFn: () => {
			if (!repo) throw new Error("代行貼文資料層尚未就緒（缺少目標店家）");
			return repo.fetchPostCount();
		},
		enabled: enabled && repo !== null,
	});
	return enabled ? (query.data ?? null) : null;
}
