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
 * - 開店進度卡讀 `useSellerSelf`／`usePayuniStatus`／`useShippingOnboardings`，
 *   後兩支是金流與履約禁區。
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

	// 這支 hook 自己就判 `orders.read`（缺 capability 時一個請求都不發），所以
	// 這裡不再包一層條件 —— 包了反而換成另一把鍵，與訂單頁的槽位就分開了。
	// ⚠️ 這一頁沒有篩選列，數字恆為**未篩選**的總數，所以傳空條件（也因此與訂單頁
	// 尚未篩選時共用同一個快取槽）。訂單頁的卡片會跟著它的搜尋與日期範圍走，兩邊在
	// 那個當下不相等是對的 —— 下面那句「與清單頁逐格相同」講的是未篩選時。
	const { totals: orderTotals } = useDelegatedOrderStatusTotals(
		ORDER_STAT_STATUSES,
		EMPTY_DELEGATED_ORDER_FILTERS,
	);
	// 四張卡與商品頁的 KPI 取自同一組參數（總數／4004／4003／4001），數字因此逐格
	// 相等。⚠️ 4003 那一張不可省：省掉之後這一區會畫成「總數 9、已上架 1、草稿 1」，
	// 剩下的 7 件在畫面上無從解釋，而使用者的下一步正是點「前往商品管理」——那一頁
	// 有四張卡，於是同一批商品在兩個畫面上算出不同的帳。
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
	// 說明列講的區塊數與下面 `canRead*` 三個條件渲染的區塊數是同一件事，所以由同一
	// 組布林推導出來，不另寫一個會漂移的字面值。
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
					{/* `{" "}` 不可省：JSX 會把「運算式 → 換行 → 文字」之間的空白整段吃掉，
					    少了它兩句話會黏成「…逐格相同。營收、瀏覽數…」。原本兩句都是文字節點，
					    換行本身就會被折成一個空格。 */}
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
						{/* 走 CodeMeta Label 而不是「上架中」：商品列表那一列的 pill 直接顯示
						    後端下發的 `statusLabel`，而這張卡與那顆 pill 只隔一次點擊（右上角
						    「前往商品管理」），同一個狀態出現兩種說法會讓人以為是兩種狀態。
						    賣家中心那一側維持自己的操作語（`上架中`／`下架中`）—— 那是既有
						    契約，而且那邊的卡與 pill 本來就彼此一致。

						    ⚠️ **讀 `PRODUCT_STATUS_LABEL`，不抄字面值**：抄過來的字在 CodeMeta
						    改 Label 時不會轉紅，於是卡片與它一步之遙的 pill 各說各話。 */}
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
						{/* 「庫存預警」不渲染：它要一個安全庫存門檻，而代行的列表 DTO 沒有
						    這個來源（賣家路徑用的 ≤5 是賣家自己的視角，不該替另一家店代為
						    定義）。與商品頁同一條裁決。 */}
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
						{/* 只有一張卡是刻意的：貼文列表的 `status` 值域是 `published`／
						    `hidden`，而 `published` 在 social-service 的實際述詞是
						    `hidden_at IS NULL`（`posts.sql` 的 `ListMyPosts`）—— 草稿也
						    算在內。拿它當「已發布」會是一個看起來很具體的假數字。 */}
					</div>
				</section>
			) : null}
		</SellerPageShell>
	);
}

/**
 * 貼文總數。
 *
 * 與訂單／商品不同，貼文走自己的 repo 實例而不是 `useSellerDataScope()`：賣家中心
 * 的 `SellerPostRepo` 只回 `PostListItem[]`，`total` 在映射時就沒了（`count()` 是
 * 代行專屬命令）。
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
			return repo.count();
		},
		enabled: enabled && repo !== null,
	});
	return enabled ? (query.data ?? null) : null;
}
