"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Link from "@/components/shared/Link";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import { targetStoreLabel } from "@/features/delegated-operations/confirm-content";
import { delegatedBatchFailureText } from "@/features/delegated-operations/load-failure";
import { PostImageIcon } from "@/features/feed/components/PostMetricIcons";
import { ApiClientError } from "@/lib/api/client";
import type { DelegatedMutationMeta } from "@/lib/api/delegated";
import { PRODUCT_STATUS_LABEL, ProductStatus } from "@/lib/codemeta/enums";
import { formatTaipeiDate } from "@/lib/date/taipei";
import { useCategoryTree } from "@/lib/hooks/useCategoryTree";
import { useInvalidateSellerProducts } from "@/lib/hooks/useSellerProducts";
import { ossImageUrl } from "@/lib/media/cdn";
import {
	fetchSellerSalesCounts,
	isProcessedFoodCategory,
	type SellerProductListItem,
} from "@/lib/products/seller-repo";
import { useCertGate } from "@/lib/seller/cert-gate";
import { formatNullableCount } from "@/lib/seller/order-dashboard-counts";
import { delegatedProductKpi } from "@/lib/seller-data/delegated-product-kpi";
import { useSellerDataScope } from "@/lib/seller-data/scope";
import { SellerEmptyState } from "../components/SellerEmptyState";
import {
	SellerMobileDisclosureList,
	type SellerMobileDisclosureSummary,
} from "../components/SellerMobileDisclosureList";
import { SellerProductsSkeletonRows } from "../components/SellerPageSkeletons";
import ShelfStatusControl, {
	resolveShelfState,
} from "../components/ShelfStatusControl";
import {
	nextSummaryTabIndex,
	SummaryStatCard,
} from "../components/SummaryStatCard";
import ProductEditDrawer, { openDrawerOnPlainClick } from "./ProductEditDrawer";
import ProductReviewsDrawer from "./ProductReviewsDrawer";
import {
	delegatedProductsSubtitle,
	productRowActionHint,
	productRowActionLabel,
} from "./productsCapabilityCopy";
import { useSellerProductsUiStore } from "./productsUiStore";
import { useProductEditDrawer } from "./useProductEditDrawer";
import { useProductReviewsDrawer } from "./useProductReviewsDrawer";

const PAGE_SIZE = 20;

/** 加工食品沒有核准的食品業者登錄時，禁止上架的提示（與商品表單共用同一句）。 */
const FOOD_REG_REQUIRED_HINT = "加工食品需先通過食品業者登錄審核才能上架";

export default function ProductsPage() {
	// 賣家中心與代行工作區渲染的是同一支元件；傳輸層、快取鍵、寫入前的確認關卡
	// 全部由 scope 提供（沒有 provider ⇒ 賣家路徑，行為逐位元不變）。
	const scope = useSellerDataScope();
	const delegated = scope.mode === "delegated";
	/** 代行的商品路由根（`/adm/{storeId}/products`）。賣家路徑用不到。 */
	const delegatedProductRoot = `/adm/${encodeURIComponent(
		scope.ownerStoreId ?? "",
	)}/products`;
	/**
	 * 確認抽屜的提示卡要指名店家（§6.5），單筆與批次共用同一個稱呼。
	 *
	 * 賣家路徑不會開抽屜（`confirmMutation` 立刻放行），這個值在那條路徑上算完就
	 * 沒人讀。
	 */
	const targetStore = targetStoreLabel(
		scope.ownerStoreName,
		scope.ownerStoreId,
	);
	/**
	 * 上／下架（單筆膠囊與批次列）的渲染條件。
	 *
	 * 賣家路徑恆為 true（那裡沒有 capability 這個概念，`hasCapability` 也恆回 true，
	 * 但仍以 `!delegated` 短路，避免哪天賣家路徑的實作改了就憑空少掉功能）。代行
	 * 缺 `products.publish` 時**整個不渲染**而不是 disabled（L3857）—— disabled 的
	 * 按鈕仍在宣稱「這件事本來做得到」，而這個工作階段從頭到尾都做不到。
	 */
	const canPublish = !delegated || scope.hasCapability("products.publish");
	/** 建立商品的 capability（頂部導覽的 `＋ 上架新商品` 用同一個條件）。 */
	const canWrite = !delegated || scope.hasCapability("products.write");
	const productRepo = scope.repos.products;
	const certGate = useCertGate();
	// This page owns its own paginated/optimistic list; it only invalidates the
	// shared seller-products family so the nav low-stock badge + dashboard count
	// refetch after a create/update/status/delete.
	const invalidateSellerProducts = useInvalidateSellerProducts();

	const [items, setItems] = React.useState<SellerProductListItem[]>([]);
	const [summaryItems, setSummaryItems] = React.useState<
		SellerProductListItem[]
	>([]);
	const [total, setTotal] = React.useState(0);
	const [summaryTotal, setSummaryTotal] = React.useState(0);
	const [page, setPage] = React.useState(1);
	const [loading, setLoading] = React.useState(false);
	const [loadingMore, setLoadingMore] = React.useState(false);
	const [error, setError] = React.useState<string | null>(null);
	const statusFilter = useSellerProductsUiStore((state) => state.statusFilter);
	const setStatusFilter = useSellerProductsUiStore(
		(state) => state.setStatusFilter,
	);
	const search = useSellerProductsUiStore((state) => state.search);
	const setSearch = useSellerProductsUiStore((state) => state.setSearch);
	const {
		editProductId,
		expanded: editExpanded,
		openEdit,
		closeEdit,
		toggleExpanded: toggleEditExpanded,
	} = useProductEditDrawer();
	const { reviewsTarget, openReviews, closeReviews } =
		useProductReviewsDrawer();
	// 銷量 isn't on the seller list endpoint (B-192) but IS on the public list.
	// Backfill it in one extra request keyed by product id; merged at render.
	const [salesMap, setSalesMap] = React.useState<Map<string, number>>(
		new Map(),
	);
	// 分類樹（上架閘記用）：加工食品商品在店家有核准的食品業者登錄前不可上架。
	// 表單已擋，這裡補上列表 row 的上架切換，避免從列表繞過。
	const { tree: categoryTree } = useCategoryTree();
	// 店家有核准的食品業者登錄就完全不需閘記；否則加工食品商品擋上架。
	const upshelfBlockedFor = React.useCallback(
		(product: SellerProductListItem): boolean =>
			!certGate.hasFoodBusinessReg &&
			isProcessedFoodCategory(categoryTree, product.category_id),
		[certGate.hasFoodBusinessReg, categoryTree],
	);

	/**
	 * 代行的四個 KPI 數字（總數／已上架／未上架／草稿）。
	 *
	 * B15 明列的唯一合法作法：**一張卡＝一次帶 `status` 的列表，只讀它的 `total`**。
	 * 賣家路徑那份「抓 100 筆回來自己數」在代行下是兩層錯誤：代行列表的
	 * `page_size` 上限是 50（Gateway `delegatedPaging`），送 100 直接 9260／400；
	 * 而 catch 之後畫面退回用第一頁的 20 筆去數，於是數字看起來仍然「有值」，
	 * 只是靜默少報。
	 *
	 * ⚠️ `off`（4003 未上架）是後補的：少了它，一件已下架的商品在列表看得到、在卡片
	 * 無處可歸、也沒有任何控制項篩得到它（這一頁沒有狀態下拉，卡片就是唯一的篩選器）。
	 * 於是「總商品數 ≠ 其餘卡片之和」這個落差在畫面上無從解釋。
	 */
	const [delegatedTotals, setDelegatedTotals] = React.useState<{
		all: number;
		active: number;
		off: number;
		draft: number;
	} | null>(null);

	const refreshSummary = React.useCallback(async () => {
		if (delegated) {
			try {
				const [all, active, off, draft] = await Promise.all([
					productRepo.fetchSellerProducts({ page: 1, pageSize: 1 }),
					productRepo.fetchSellerProducts({
						page: 1,
						pageSize: 1,
						status: "4004",
					}),
					productRepo.fetchSellerProducts({
						page: 1,
						pageSize: 1,
						status: "4003",
					}),
					productRepo.fetchSellerProducts({
						page: 1,
						pageSize: 1,
						status: "4001",
					}),
				]);
				setDelegatedTotals({
					all: all.total,
					active: active.total,
					off: off.total,
					draft: draft.total,
				});
			} catch {
				setDelegatedTotals(null);
			}
			return;
		}
		try {
			const r = await productRepo.fetchSellerProducts({
				page: 1,
				pageSize: 100,
			});
			setSummaryItems(r.records);
			setSummaryTotal(r.total);
		} catch {
			setSummaryItems([]);
			setSummaryTotal(0);
		}
	}, [delegated, productRepo]);

	/**
	 * 重抓第一頁並回傳那一頁的內容。
	 *
	 * 回傳值是版本衝突（9266）自救路徑要用的：`setItems` 之後 state 還沒更新，
	 * 而合流需要的是**這一次**抓到的新 `version`，不是下一次 render 才看得到的值。
	 */
	const reloadProducts = React.useCallback(async (): Promise<
		SellerProductListItem[]
	> => {
		const r = await productRepo.fetchSellerProducts({
			page: 1,
			pageSize: PAGE_SIZE,
			status:
				statusFilter === "low_stock" || statusFilter === undefined
					? undefined
					: statusFilter,
		});
		setItems(r.records);
		setTotal(r.total);
		setPage(1);
		return r.records;
	}, [productRepo, statusFilter]);

	const refresh = React.useCallback(async () => {
		setLoading(true);
		setError(null);
		try {
			await reloadProducts();
		} catch (e) {
			setError(e instanceof Error ? e.message : "載入商品失敗");
		} finally {
			setLoading(false);
		}
	}, [reloadProducts]);

	const handleRefresh = React.useCallback(async () => {
		await Promise.all([refresh(), refreshSummary()]);
		void invalidateSellerProducts();
	}, [refresh, refreshSummary, invalidateSellerProducts]);

	const loadMore = async () => {
		setLoadingMore(true);
		try {
			const r = await productRepo.fetchSellerProducts({
				page: page + 1,
				pageSize: PAGE_SIZE,
				status:
					statusFilter === "low_stock" || statusFilter === undefined
						? undefined
						: statusFilter,
			});
			setItems((prev) => [...prev, ...r.records]);
			setPage(r.page);
		} catch {
			/* ignore */
		} finally {
			setLoadingMore(false);
		}
	};

	const [busyId, setBusyId] = React.useState<string | null>(null);
	const [deletingId, setDeletingId] = React.useState<string | null>(null);

	/**
	 * 把一次成功的上下架就地反映到清單上。
	 *
	 * ⚠️ `version` 必須跟著 `status` 一起 patch，理由與 `ProductWriteResult` 同一條：
	 * 這一頁在同一次載入內可以對**同一個商品**連按上架、下架，而每一次成功都把版本
	 * 推進一格。只 patch `status` 的話，第二次送出帶的仍是第一次之前的版本 ⇒ 必 9266
	 * 「資料已被更新」，接著彈出版本衝突面板叫人重新載入 —— 而現場只有代行者一個人
	 * 在改，什麼都沒被別人動過。
	 *
	 * ⚠️ 版本只從**本次寫入的回應**來（參數 `version`），不從背景 refetch 抄：後者會
	 * 把賣家這段期間的修改當成自己的版本，樂觀鎖從此永遠不會擋。`void
	 * invalidateSellerProducts()` 只是讓別處的 query 重抓，不回填這裡的 `items`。
	 *
	 * 賣家自助路徑恆為 `undefined`（`/app/products` 沒有版本概念），那時整段退化成
	 * 原本的「只換 status」，行為一格都不變。
	 */
	const patchStatusLocal = React.useCallback(
		(id: string, status: string, version?: string) => {
			const patch = (p: SellerProductListItem) =>
				String(p.id) === id
					? { ...p, status, ...(version ? { version } : {}) }
					: p;
			setItems((prev) => prev.map(patch));
			setSummaryItems((prev) => prev.map(patch));
		},
		[],
	);

	/**
	 * 單筆上下架撞到版本衝突（9266）時的待處理狀態。
	 *
	 * 它與一般錯誤**必須分開呈現**（§3.4 版本衝突契約 L5346）：一般錯誤「再按一次」
	 * 可能會過，9266 再按一次仍然是 9266 —— 賣家或另一位管理員已經改過這一筆，我們
	 * 手上的 `expectedVersion` 永遠不會再相符。
	 *
	 * 存的是**操作者剛剛填過的東西**（修改原因、通知設定），那是「複製我的內容」在
	 * 這條路徑上的全部內容：上下架沒有表單欄位，操作者投入的就是那一段理由。
	 */
	const [statusConflict, setStatusConflict] = React.useState<{
		product: SellerProductListItem;
		target: "4003" | "4004";
		reason: string;
		notifySeller: boolean;
	} | null>(null);

	/** 送出一次上下架。`meta` 由呼叫端決定（首次來自抽屜、合流後帶新版本）。 */
	const runSetStatus = React.useCallback(
		async (
			product: SellerProductListItem,
			target: "4003" | "4004",
			meta: DelegatedMutationMeta | undefined,
		) => {
			const id = String(product.id);
			setBusyId(id);
			setError(null);
			try {
				const written = await productRepo.updateProductStatus(id, target, meta);
				patchStatusLocal(id, target, written.version);
				setStatusConflict(null);
				void invalidateSellerProducts();
			} catch (e) {
				if (e instanceof ApiClientError && e.code === 9266 && meta) {
					setStatusConflict({
						product,
						target,
						reason: meta.operationReason,
						notifySeller: meta.notifySeller ?? true,
					});
					return;
				}
				setError(e instanceof Error ? e.message : "更新狀態失敗");
			} finally {
				setBusyId(null);
			}
		},
		[patchStatusLocal, invalidateSellerProducts, productRepo],
	);

	const confirmSetStatus = React.useCallback(
		async (
			product: SellerProductListItem,
			target: "4003" | "4004",
			initial?: { reason: string; notifySeller: boolean },
		) => {
			const publish = target === "4004";
			// ⚠️ 代行的上／下架是兩支**命令端點**（body 只收共通三欄），不是「傳 status
			// 值」；`/publish` 的變更恆為單一欄位，所以不畫三欄表（L3704）。
			const decision = await scope.confirmMutation({
				title: publish ? "檢視變更並上架" : "檢視變更並下架",
				intent: `以 ADM 身分替「${targetStore}」${publish ? "上架" : "下架"}「${
					product.name || "未命名商品"
				}」`,
				hint: `請確認以下資訊無誤後執行${publish ? "上架" : "下架"}。`,
				reasonPlaceholder: publish
					? "例：協助完成新品上架"
					: "例：依賣家來信暫停販售",
				expectedVersion: product.version,
				confirmLabel: publish ? "確認上架" : "確認下架",
				...(initial
					? {
							initialReason: initial.reason,
							initialNotifySeller: initial.notifySeller,
						}
					: {}),
			});
			if (!decision.ok) return;
			await runSetStatus(product, target, decision.meta);
		},
		[runSetStatus, scope, targetStore],
	);

	const handleSetStatus = React.useCallback(
		(product: SellerProductListItem, target: "4003" | "4004") =>
			confirmSetStatus(product, target),
		[confirmSetStatus],
	);

	/**
	 * 版本衝突的出口①：**重新載入最新版本**。丟掉這一次的意圖，只把列表對齊賣家
	 * 現在的內容 —— 對「看到別人已經改過就不打算再動它」的情境，這就是正解。
	 */
	const handleConflictReload = React.useCallback(async () => {
		setStatusConflict(null);
		setError(null);
		await handleRefresh();
	}, [handleRefresh]);

	/**
	 * 版本衝突的出口②：**複製我的內容**。
	 *
	 * 它不是「強制覆蓋」：先重抓最新版本，拿**新的** `expectedVersion` 再開一次確認
	 * 抽屜，並把操作者剛才打的理由與通知設定帶進去（不必重打一次）。真正要不要執行
	 * 仍由操作者在抽屜裡決定，而且那個抽屜上顯示的是賣家最新的版本號。
	 *
	 * ⚠️ 沿用舊 `Idempotency-Key` 是 9267（重用同一把打不同 payload）—— 資料層每次
	 * 呼叫都產生新的一把，這裡不重試舊請求、而是走一次新的送出。
	 */
	const handleConflictKeepMine = React.useCallback(async () => {
		const pending = statusConflict;
		if (!pending) return;
		setStatusConflict(null);
		setError(null);
		let latest: SellerProductListItem | undefined;
		try {
			const records = await reloadProducts();
			latest = records.find(
				(item) => String(item.id) === String(pending.product.id),
			);
		} catch (e) {
			setError(e instanceof Error ? e.message : "載入商品失敗");
			return;
		}
		if (!latest) {
			setError("這個商品已不在目前的清單中，請重新整理後再試。");
			return;
		}
		// 對方剛才做的可能**正是**我要做的事。此時再送一次會被 product-service 以
		// 「狀態轉換不合法」擋下（400），而那個訊息完全看不出真正的原因是「已經做完
		// 了」。合流的正解是把結果講出來，不是再撞一次牆。
		if (
			resolveShelfState(latest.status) === resolveShelfState(pending.target)
		) {
			setError(
				`「${latest.name || "未命名商品"}」目前已經是${
					pending.target === "4004" ? "上架" : "下架"
				}狀態 —— 賣家本人或另一位管理員剛完成了同樣的操作，這次不需要再送出。`,
			);
			return;
		}
		await confirmSetStatus(latest, pending.target, {
			reason: pending.reason,
			notifySeller: pending.notifySeller,
		});
	}, [confirmSetStatus, reloadProducts, statusConflict]);

	const handleDelete = React.useCallback(
		async (id: string, name: string) => {
			if (
				typeof window !== "undefined" &&
				!window.confirm(
					`確定要刪除「${name || "未命名商品"}」嗎？此動作無法復原。`,
				)
			) {
				return;
			}
			setBusyId(id);
			setDeletingId(id);
			setError(null);
			try {
				await productRepo.deleteProduct(id);
				setItems((prev) => prev.filter((p) => String(p.id) !== id));
				setSummaryItems((prev) => prev.filter((p) => String(p.id) !== id));
				setTotal((t) => Math.max(0, t - 1));
				setSummaryTotal((t) => Math.max(0, t - 1));
				void invalidateSellerProducts();
			} catch (e) {
				setError(e instanceof Error ? e.message : "刪除商品失敗");
			} finally {
				setBusyId(null);
				setDeletingId(null);
			}
		},
		[invalidateSellerProducts, productRepo],
	);

	React.useEffect(() => {
		void refresh();
	}, [refresh]);

	React.useEffect(() => {
		void refreshSummary();
	}, [refreshSummary]);

	// All of a seller's products share the same ownerId; grab it from whichever
	// list has loaded and backfill 銷量 from the public endpoint once.
	const sellerStoreId =
		items.find((p) => p.owner_id)?.owner_id ??
		summaryItems.find((p) => p.owner_id)?.owner_id;
	React.useEffect(() => {
		// 代行下不做這次回填：它打的是公開列表（`/public/products`，賣家傳輸層之外的
		// 另一條路），而代行的商品列表 DTO 本來就帶 `salesCount`。多打一次不會更準，
		// 只會在工作區裡多一條沒有必要的跨店查詢。
		if (delegated || !sellerStoreId) return;
		let alive = true;
		void fetchSellerSalesCounts(sellerStoreId).then((m) => {
			if (alive && m.size > 0) setSalesMap(m);
		});
		return () => {
			alive = false;
		};
	}, [delegated, sellerStoreId]);

	const visibleItems = React.useMemo(() => {
		const q = search.trim().toLowerCase();
		const filtered =
			statusFilter === "low_stock"
				? items.filter((p) => p.stock != null && p.stock <= 5)
				: items;
		if (!q) return filtered;
		return filtered.filter((p) => {
			const name = p.name?.toLowerCase() ?? "";
			// SKU codes aren't returned by the list endpoint (need
			// `/app/products/{id}/skus`), so search falls back to name only.
			return name.includes(q);
		});
	}, [items, search, statusFilter]);

	/**
	 * 批次下架（mockup §4.7.6／L3126）——**代行專屬**：賣家自助端沒有這支端點，
	 * 所以賣家路徑的選取框維持既有的非受控裝飾狀態，一個 prop 都不多給。
	 */
	const [selectedIds, setSelectedIds] = React.useState<string[]>([]);
	const [batchBusy, setBatchBusy] = React.useState(false);
	const [batchResult, setBatchResult] = React.useState<{
		succeeded: number;
		failed: number;
		// `name` 在送出當下就記下來：批次成功後會 `handleRefresh()`，那批列可能已經
		// 換頁或改了狀態，事後拿 id 回列表查名字會查到 undefined 而退回 TSID。
		failures: { productId: string; name: string; code: number }[];
	} | null>(null);

	const toggleSelected = React.useCallback((id: string) => {
		setSelectedIds((prev) =>
			prev.includes(id) ? prev.filter((v) => v !== id) : [...prev, id],
		);
	}, []);

	const handleBatchUnpublish = React.useCallback(async () => {
		const commands = scope.delegatedCommands;
		if (!commands) return;
		// `expectedVersion` 逐項必填：沒有版本的列一律不送，而不是送一個空字串讓
		// 後端回 9260 —— 那會讓整批看起來像「後端壞了」。
		const selected = items.filter(
			(p) => selectedIds.includes(String(p.id)) && p.version,
		);
		const targets = selected.map((p) => ({
			productId: String(p.id),
			expectedVersion: String(p.version),
		}));
		if (targets.length === 0) return;
		const decision = await scope.confirmMutation({
			title: "批次下架",
			intent: `以 ADM 身分替「${targetStore}」下架 ${targets.length} 項商品`,
			hint: "批次下架需先確認原因；整批共用同一個修改原因與通知設定。",
			reasonPlaceholder: "例：依賣家來信暫停整批販售",
			// 批次的版本是逐項送的，沒有單一 `expectedVersion` 可放 —— 逐項列出來，
			// 才有辦法在部分失敗（9266）時對得上是哪幾筆的版本被別人先動過。
			versionedItems: selected.map((p) => ({
				label: p.name || "未命名商品",
				version: String(p.version),
			})),
			confirmLabel: "確認批次下架",
		});
		if (!decision.ok || !decision.meta) return;
		setBatchBusy(true);
		setError(null);
		try {
			const result = await commands.batchUnpublishProducts(
				targets,
				decision.meta,
			);
			const nameById = new Map(
				selected.map((p) => [String(p.id), p.name || "未命名商品"]),
			);
			// ⚠️ 只要批次被受理，頂層 `code` 恆為 0（部分失敗也是）—— 成敗看
			// `failed`，而逐項結果必須畫得出來，否則操作者看到的是「成功」。
			setBatchResult({
				succeeded: result.succeeded,
				failed: result.failed,
				failures: result.results
					.filter((entry) => !entry.ok)
					.map((entry) => ({
						productId: entry.productId,
						name: nameById.get(entry.productId) ?? entry.productId,
						code: entry.code,
					})),
			});
			setSelectedIds([]);
			await handleRefresh();
		} catch (e) {
			setError(e instanceof Error ? e.message : "批次下架失敗");
		} finally {
			setBatchBusy(false);
		}
	}, [handleRefresh, items, scope, selectedIds, targetStore]);

	/**
	 * 摘要列的卡片數。四張兩邊共用（總數／已上架／未上架／草稿），「庫存預警」只有
	 * 賣家側算得出來（代行的列表 DTO 沒有安全庫存門檻，見 B15）⇒ 賣家 5、代行 4。
	 *
	 * 共用的 `.seller-inventory__stats` 是寫死的四欄格線（庫存／訂單／客戶頁同用），
	 * 五張卡會讓第五張自己佔滿一整列；`--5` 變體只換那一列的欄數，其餘頁面不動。
	 */
	const statCardCount = delegated ? 4 : 5;

	const hasMore = items.length < total;
	// Header counts must never flash 0→N while loading: the subtitle prefers
	// the summary total (loaded once, unaffected by status filters) and both
	// counters show an em dash until their first data arrives.
	const listBooting = loading && items.length === 0;
	const knownTotal = summaryTotal || total;
	const totalKnown = !listBooting || summaryItems.length > 0;
	const summaryStats = React.useMemo(() => {
		if (delegated) {
			// 未知一律 null → 渲染破折號。`?? 0` 會把「四支探詢全部失敗」講成
			// 「這家店一件上架商品都沒有」，而那是斷路器 half-open 時的實測畫面
			// （成因記錄在 lib/seller-data/delegated-product-kpi.ts）。
			return delegatedProductKpi(delegatedTotals);
		}
		const source = summaryItems.length > 0 ? summaryItems : items;
		return {
			total: summaryTotal || total,
			active: source.filter((p) => isActiveStatus(p.status)).length,
			off: source.filter((p) => isOffStatus(p.status)).length,
			// `stock` isn't returned by the list endpoint yet (pending backend
			// ticket), so only count rows that actually carry a number — never
			// treat "missing" as 0, which would flag every product.
			lowStock: source.filter((p) => p.stock != null && p.stock <= 5).length,
			draft: source.filter((p) => isDraftStatus(p.status)).length,
		};
	}, [delegated, delegatedTotals, items, summaryItems, summaryTotal, total]);

	const handleSummaryTabKeyDown = React.useCallback(
		(event: React.KeyboardEvent<HTMLDivElement>) => {
			const currentTab = (
				event.target as HTMLElement
			).closest<HTMLButtonElement>('button[role="tab"]');
			if (!currentTab || !event.currentTarget.contains(currentTab)) return;

			const tabs = Array.from(
				event.currentTarget.querySelectorAll<HTMLButtonElement>(
					'button[role="tab"]',
				),
			);
			const nextIndex = nextSummaryTabIndex(
				event.key,
				tabs.indexOf(currentTab),
				tabs.length,
			);
			if (nextIndex === null) return;

			event.preventDefault();
			const nextTab = tabs[nextIndex];
			nextTab.focus();
			nextTab.click();
		},
		[],
	);

	const renderMobileProductDetails = React.useCallback(
		(p: SellerProductListItem) => {
			const id = String(p.id);
			const productName = p.name || "未命名商品";
			const tags = normaliseHashtags(p.hashtags);
			const score = ratingValue(p);
			const sales = salesMap.get(id) ?? p.sales_count ?? "—";

			return (
				<>
					<div className="seller-mobile-disclosure__body">
						{delegated && (
							<div className="seller-mobile-disclosure__detail-row">
								<span>商品 ID</span>
								<Link
									href={`${delegatedProductRoot}/${encodeURIComponent(id)}/edit`}
								>
									{id}
								</Link>
							</div>
						)}
						{tags.length > 0 && (
							<ul
								className="seller-mobile-disclosure__facts m-0 list-none p-0"
								aria-label="商品標籤"
							>
								{tags.map((tag) => (
									<li className="seller-mobile-disclosure__fact" key={tag}>
										#{tag}
									</li>
								))}
							</ul>
						)}
						{!delegated && (
							<div className="seller-mobile-disclosure__detail-row">
								<span>評分</span>
								<RatingCell
									delegated={false}
									productName={productName}
									onOpen={() =>
										openReviews({
											id,
											name: productName,
											image: p.main_image,
										})
									}
								>
									{score != null ? (
										<span className="inline-flex items-center gap-1">
											<AppIcon
												name="star"
												size={13}
												fill
												className="shrink-0 text-[var(--hkui-warning)]"
											/>
											<span className="font-medium tabular-nums">
												{score.toFixed(1)}
											</span>
										</span>
									) : (
										<span className="tabular-nums">—</span>
									)}
									<AppIcon name="chat" size={14} className="shrink-0" />
								</RatingCell>
							</div>
						)}
						<div className="seller-mobile-disclosure__detail-row">
							<span>分類</span>
							<strong>{p.category_name || "—"}</strong>
						</div>
						<div className="seller-mobile-disclosure__detail-row">
							<span>庫存</span>
							<strong>{p.stock ?? "—"}</strong>
						</div>
						{!delegated && (
							<div className="seller-mobile-disclosure__detail-row">
								<span>收藏</span>
								<strong>{p.favorite_count ?? "—"}</strong>
							</div>
						)}
						<div className="seller-mobile-disclosure__detail-row">
							<span>銷量</span>
							<strong>{sales}</strong>
						</div>
						{!delegated && (
							<div className="seller-mobile-disclosure__detail-row">
								<span>上架時間</span>
								<strong>{formatListedDate(p.created_at)}</strong>
							</div>
						)}
						<div className="seller-mobile-disclosure__detail-row">
							<span>上架狀態</span>
							<ShelfStatusControl
								status={p.status}
								statusLabel={p.status_label}
								busy={busyId === id && deletingId !== id}
								upshelfDisabledReason={
									!delegated && upshelfBlockedFor(p)
										? FOOD_REG_REQUIRED_HINT
										: undefined
								}
								onToggle={
									deletingId === id || !canPublish
										? undefined
										: (target) => {
												if (
													!delegated &&
													target === "4004" &&
													upshelfBlockedFor(p)
												) {
													return;
												}
												handleSetStatus(p, target);
											}
								}
							/>
						</div>
					</div>

					<div className="seller-mobile-disclosure__actions">
						{delegated && canPublish && (
							<label className="inline-flex min-h-11 min-w-max flex-none cursor-pointer items-center gap-2 rounded-[var(--hkui-radius-md)] border border-[var(--hkui-divider)] px-3 text-sm font-semibold">
								<input
									type="checkbox"
									className="size-5"
									aria-label={`選取 ${productName}`}
									checked={selectedIds.includes(id)}
									onChange={() => toggleSelected(id)}
								/>
								<span>選取</span>
							</label>
						)}
						{delegated ? (
							<>
								<Link
									href={`${delegatedProductRoot}/${encodeURIComponent(id)}/edit`}
									className="seller-table__action"
									title={productRowActionHint(
										canWrite,
										resolveShelfState(p.status) === "active",
									)}
								>
									{productRowActionLabel(
										canWrite,
										resolveShelfState(p.status) === "active",
									)}
								</Link>
								<Link
									href={`${delegatedProductRoot}/${encodeURIComponent(id)}/preview`}
									className="seller-table__action"
									title="以買家視角預覽這個商品（不會送出任何變更）"
								>
									預覽
								</Link>
							</>
						) : (
							<>
								<RowEditButton
									id={p.id}
									status={p.status}
									productName={productName}
									onOpen={() => openEdit(id)}
								/>
								<RowDeleteButton
									status={p.status}
									busy={busyId === id}
									deleting={deletingId === id}
									productName={productName}
									onDelete={() => handleDelete(id, p.name || "")}
								/>
							</>
						)}
					</div>
				</>
			);
		},
		[
			busyId,
			canPublish,
			canWrite,
			delegated,
			delegatedProductRoot,
			deletingId,
			handleDelete,
			handleSetStatus,
			openEdit,
			openReviews,
			salesMap,
			selectedIds,
			toggleSelected,
			upshelfBlockedFor,
		],
	);

	return (
		<SellerPageShell
			className="seller-products"
			title="商品管理"
			subtitle={
				delegated
					? delegatedProductsSubtitle(canWrite, canPublish)
					: `總共 ${totalKnown ? knownTotal.toLocaleString() : "—"} 件商品`
			}
			actions={
				<>
					<Button
						type="button"
						className="seller-orders__btn"
						onClick={() => void handleRefresh()}
						disabled={loading}
						isLoading={loading}
						loadingLabel="重新整理中…"
					>
						<AppIcon name="refresh" size={14} />
						<span className="inline-block min-w-[4em] text-center">
							重新整理
						</span>
					</Button>
					{/* 代行的新增商品入口（`POST .../products`，capability
					    `products.write`）。與頂部導覽的 `＋ 上架新商品` 是**同一顆
					    按鈕的兩個落點**，條件必須一致 —— 兩處各說各話正是它上一輪
					    被移除的原因。缺 capability 時不渲染，不是 disabled（L3857）。 */}
					{delegated && canWrite && (
						<Link
							href={`${delegatedProductRoot}/new`}
							// ⚠️ `data-hkui-button` 是「這是一顆按鈕、前景色由它自己的表面決定」
							// 的既有宣告（`Button.tsx:404`／`01-tokens-theme.css:761`）。少了它，
							// dark 下 `body a:not([data-hkui-button])` 的 `--color-ink-200`（0-2-2）壓過
							// `.seller-orders__btn--primary` 的白字（0-1-0），實心森林綠底就配上
							// 一層橄欖綠字（dev 實測最亮字素 --color-ink-200，約 4.4:1）。
							data-hkui-button=""
							className="seller-orders__btn seller-orders__btn--primary"
						>
							＋ 上架新商品
						</Link>
					)}
					{/* 代行不渲染批次匯入：封閉九值不含任何 import 能力，端點總表也沒有
					    import 路徑；要做是新增第 10 個 capability，另一張票。 */}
					{!delegated && (
						<>
							<Link
								href="/seller-center/products/import"
								className="seller-orders__btn"
							>
								<IconUpload />
								批次匯入
							</Link>
							<Link
								href="/seller-center/products/new"
								// 同上：實心表面自己擁有前景色，否則 dark 下退成橄欖綠字。
								data-hkui-button=""
								className="seller-orders__btn seller-orders__btn--primary"
							>
								＋ 新增商品
							</Link>
						</>
					)}
				</>
			}
		>
			<div
				className={`seller-inventory__stats seller-summary-rail seller-summary-rail--compact-phone${
					statCardCount === 5 ? " seller-inventory__stats--5" : ""
				}`}
				role="tablist"
				aria-label="商品狀態篩選"
				aria-orientation="horizontal"
				onKeyDown={handleSummaryTabKeyDown}
			>
				<SummaryStatCard
					label="總商品數"
					value={formatNullableCount(summaryStats.total)}
					tint="primary"
					icon={<IconBoxes />}
					active={statusFilter === undefined}
					tabIndex={statusFilter === undefined ? 0 : -1}
					onClick={() => setStatusFilter(undefined)}
				/>
				{/* 卡片是篩選器，點下去列表那一列的狀態 pill 必須是同一個詞。代行的 pill
				    走後端下發的 CodeMeta Label，賣家中心走自己的操作語（`上架中`／
				    `下架中`）—— 兩邊都對，但同一個畫面裡混用就會讓人以為是兩種狀態，
				    所以卡片跟著各自的 pill 走。

				    ⚠️ 代行那一側**讀 `PRODUCT_STATUS_LABEL` 而不是抄字面值**：抄過來
				    的字在 CodeMeta 改 Label 時不會轉紅，於是卡片說「未啟用」、它篩出來
				    的每一列 pill 說「未上架」，而這條分界要守的正是這件事。

				    ⚠️ 這條分界只管**狀態標籤**（渲染某個碼的值：pill、KPI 卡、篩選器）。
				    **動作用語**不在其內：`上架`／`下架` 是 ADM 自己按鈕上的動詞
				    （「檢視變更並下架」），所以「上架中無法編輯內容，請先下架」這類敘述句
				    兩邊共用同一份文案是對的，改成「啟用中…」反而與它旁邊的按鈕對不上。
				    代行儀表板那張同名卡片也照這條走（已由「上架中」收斂為 CodeMeta Label）。 */}
				<SummaryStatCard
					label={
						delegated ? PRODUCT_STATUS_LABEL[ProductStatus.ACTIVE] : "上架中"
					}
					value={formatNullableCount(summaryStats.active)}
					tint="info"
					icon={<IconCheck />}
					active={statusFilter === "4004"}
					tabIndex={statusFilter === "4004" ? 0 : -1}
					onClick={() => setStatusFilter("4004")}
				/>
				{/* 未上架（4003）。這一頁沒有狀態下拉，卡片就是唯一的篩選器，少了它就有
				    一種狀態看得到、篩不到、也不計入任何一張卡，於是「總商品數 ≠ 其餘卡片
				    之和」在畫面上無從解釋。

				    ⚠️ 兩邊都要渲染，**不得再包回 `delegated &&`**（#2725）：代行建立的
				    商品一律落 4003，而賣家在自己的商品頁完全找不到它們 —— 四張卡沒有一格
				    收得下 4003，於是那批商品只能在清單裡逐頁翻。這條由
				    `tests/seller-products-kpi-cards.test.ts` 守住。

				    圖示用 `hide`（未陳列於店面）而不是 `danger`：賣家側「庫存預警」就在
				    隔壁且同為 accent，兩張卡再共用同一個驚嘆號就分不出誰是誰。 */}
				<SummaryStatCard
					label={
						delegated ? PRODUCT_STATUS_LABEL[ProductStatus.INACTIVE] : "下架中"
					}
					value={formatNullableCount(summaryStats.off)}
					tint="accent"
					icon={<IconHidden />}
					active={statusFilter === "4003"}
					tabIndex={statusFilter === "4003" ? 0 : -1}
					onClick={() => setStatusFilter("4003")}
				/>
				{/* B15：算不出來的那張卡不渲染。「庫存預警」需要安全庫存門檻，代行的
				    列表 DTO 沒有這個來源（賣家路徑用的是 ≤5 的本地啟發式，那是賣家
				    自己的視角，不該替另一家店代為定義）。 */}
				{!delegated && (
					<SummaryStatCard
						label="庫存預警"
						value={formatNullableCount(summaryStats.lowStock)}
						tint="accent"
						icon={<IconAlert />}
						active={statusFilter === "low_stock"}
						tabIndex={statusFilter === "low_stock" ? 0 : -1}
						onClick={() => setStatusFilter("low_stock")}
					/>
				)}
				<SummaryStatCard
					label={delegated ? PRODUCT_STATUS_LABEL[ProductStatus.DRAFT] : "草稿"}
					value={formatNullableCount(summaryStats.draft)}
					tint="secondary"
					icon={<IconDraft />}
					active={statusFilter === "4001"}
					tabIndex={statusFilter === "4001" ? 0 : -1}
					onClick={() => setStatusFilter("4001")}
				/>
			</div>
			<section className="seller-products__panel">
				<div className="seller-products__toolbar">
					<div className="seller-products__toolbar-title" aria-live="polite">
						顯示 {listBooting ? "—" : visibleItems.length.toLocaleString()} 件
					</div>
					<div className="seller-orders__search">
						<IconSearch />
						<input
							className="seller-products__search-input"
							type="search"
							value={search}
							onChange={(e) => setSearch(e.target.value)}
							placeholder="搜尋商品名稱或 SKU..."
							aria-label="搜尋商品"
						/>
					</div>
				</div>

				{delegated && canPublish && selectedIds.length > 0 && (
					<div className="adm-batch-bar">
						<div className="adm-batch-bar__lead">
							<AppIcon name="check" size={18} fill />
							<strong>已選取 {selectedIds.length} 項</strong>
							<span>以管理員身分操作</span>
						</div>
						<div className="adm-batch-bar__actions">
							<span>批次下架需先確認原因</span>
							<Button
								type="button"
								color="primary"
								size="sm"
								onClick={() => void handleBatchUnpublish()}
								disabled={batchBusy}
								isLoading={batchBusy}
								loadingLabel="批次下架中…"
							>
								批次下架
							</Button>
						</div>
					</div>
				)}

				{/* 失敗必須比成功更顯眼。這一段曾是一行純黑 13px 文字直接躺在頁面上，
				    而成功的批次列反而有綠框 —— 於是「2 筆中 0 成功、2 失敗」讀起來像
				    一句補充說明。失敗時改走既有的 danger 面（`.adm-batch-result--failed`
				    沿用 `--hkui-danger-*`，與結束摘要的 `adm-end-dialog__alert` 同一組
				    色票）＋ `role="alert"`，逐項碼也換成人看得懂的話。 */}
				{delegated && batchResult && (
					<div
						className={`adm-batch-result${
							batchResult.failed > 0 ? " adm-batch-result--failed" : ""
						}`}
						role={batchResult.failed > 0 ? "alert" : "status"}
					>
						{batchResult.failed > 0 ? (
							<AppIcon name="danger" size={18} className="shrink-0" />
						) : (
							<AppIcon name="check" size={18} className="shrink-0" />
						)}
						<div className="adm-batch-result__text">
							<strong>
								{batchResult.succeeded + batchResult.failed} 筆中{" "}
								{batchResult.succeeded} 成功、{batchResult.failed} 失敗
							</strong>
							{batchResult.failures.length > 0 && (
								<ul className="adm-batch-result__failures">
									{batchResult.failures.map((entry) => (
										<li key={entry.productId}>
											{entry.name}：{delegatedBatchFailureText(entry.code)}
										</li>
									))}
								</ul>
							)}
						</div>
					</div>
				)}

				{/* 版本衝突（9266）的 UI 契約：兩個出口、不覆寫賣家較新的修改，且與
				    一般欄位驗證分開呈現（§3.4／L5346）。少了它，這條路徑只會吐一行
				    「代行請求失敗（代碼 9266）」，而操作者唯一想得到的動作是再按一次
				    —— 那必然仍是 9266。 */}
				{statusConflict && (
					<div className="adm-conflict" role="alert">
						<div className="adm-conflict__text">
							<strong>
								「{statusConflict.product.name || "未命名商品"}
								」已被更新（版本衝突）
							</strong>
							<span>
								{
									"賣家本人或另一位管理員在這段期間改過它，系統不會覆寫較新的內容。你可以只把清單對齊最新版本，或保留剛才填的原因、以最新版本再確認一次。"
								}
							</span>
						</div>
						<div className="adm-conflict__actions">
							<Button
								type="button"
								variant="bordered"
								size="sm"
								className="adm-quiet-button"
								onClick={() => void handleConflictReload()}
							>
								重新載入最新版本
							</Button>
							<Button
								type="button"
								color="primary"
								size="sm"
								onClick={() => void handleConflictKeepMine()}
							>
								複製我的內容
							</Button>
						</div>
					</div>
				)}

				{error && (
					<p className="seller-products__error" role="alert">
						{error}
					</p>
				)}

				<div className="seller-orders__card seller-products__card seller-mobile-disclosure-desktop">
					{/* 卡片比表格窄時才顯示（容器查詢）。被裁掉時畫面上沒有任何痕跡——
					    macOS 的覆蓋式捲軸不算提示。賣家路徑不渲染它：那張表的橫捲是它
					    自己推導過的取捨（≤960px 另有凍結首欄那一套）。
					    ⚠️ 文案跟著「操作」欄已釘在右側一起改：叫人去捲一個根本不會動的
					    欄位，等於把提示變成另一個假訊息。 */}
					{delegated && (
						<p className="adm-products__scroll-hint">
							<AppIcon name="arrow-right" size={14} className="shrink-0" />
							表格比畫面寬，請左右捲動查看「狀態」欄；「操作」已固定在右側
						</p>
					)}
					<div className="seller-orders__table-wrap">
						<table
							className={`seller-orders__table seller-products-table${
								delegated ? " seller-products-table--delegated" : ""
							}${canPublish ? "" : " seller-products-table--noselect"}`}
							data-mobile-layout="cards"
						>
							<thead>
								<tr>
									{/* 缺 `products.publish` 時整欄不渲染（§6.4：批次操作只在具備
									    該 capability 時出現）。停用的框仍在宣稱「勾選這件事有意義」，
									    而批次列在這個工作階段從頭到尾不會出現 —— 勾了沒有任何去處。 */}
									{canPublish && (
										<th className="seller-products-table__check">
											<label className="seller-products-table__check-hit">
												<input
													type="checkbox"
													aria-label="選取全部商品"
													{...(delegated
														? {
																checked:
																	visibleItems.length > 0 &&
																	selectedIds.length === visibleItems.length,
																onChange: (
																	e: React.ChangeEvent<HTMLInputElement>,
																) =>
																	setSelectedIds(
																		e.target.checked
																			? visibleItems.map((p) => String(p.id))
																			: [],
																	),
															}
														: {})}
												/>
											</label>
										</th>
									)}
									{/* mockup §4.7.7 的第一欄。⚠️ D7：它是 TSID 數字字串，不是
									    `P-20250801-001` 那種對外編號，前端不得對它做前綴解析。 */}
									{delegated && <th>商品 ID</th>}
									<th>商品</th>
									{/* 評分／收藏／上架時間：代行的列表 DTO 沒有這三欄（買家側社群
									    數據與 `createdAt` 都不在 allow-list），恆為「—」的欄位不畫。 */}
									{!delegated && <th>評分</th>}
									<th>分類</th>
									<th>價格</th>
									<th>庫存</th>
									{!delegated && <th>收藏</th>}
									<th>銷量</th>
									{!delegated && <th>上架時間</th>}
									<th>狀態</th>
									<th>操作</th>
								</tr>
							</thead>
							<tbody>
								{listBooting ? (
									<SellerProductsSkeletonRows
										rows={6}
										delegated={delegated}
										canPublish={canPublish}
									/>
								) : visibleItems.length === 0 ? (
									<tr>
										{/* 勾選欄在缺 `products.publish` 時整欄不畫，跨欄數要跟著減 ——
										    多算一欄會讓空狀態把表格撐出一格看不見的空白欄。 */}
										<td
											colSpan={(delegated ? 8 : 10) + (canPublish ? 1 : 0)}
											className="seller-orders__empty"
										>
											<SellerEmptyState
												variant="products"
												searchQuery={search}
												seamless
											/>
										</td>
									</tr>
								) : (
									visibleItems.map((p) => (
										<tr
											key={String(p.id)}
											className={
												delegated
													? "seller-products-table__row"
													: "seller-products-table__row cursor-pointer"
											}
											onClick={
												delegated
													? undefined
													: (e) =>
															openDrawerOnPlainClick(e, () =>
																openEdit(String(p.id)),
															)
											}
										>
											{canPublish && (
												// biome-ignore lint/a11y/useKeyWithClickEvents: stops the row click from opening the drawer when toggling the checkbox; the checkbox stays keyboard-accessible.
												<td
													className="seller-products-table__check"
													data-label="選取"
													onClick={(e) => e.stopPropagation()}
												>
													<label className="seller-products-table__check-hit">
														<input
															type="checkbox"
															aria-label={`選取 ${p.name || "未命名商品"}`}
															{...(delegated
																? {
																		checked: selectedIds.includes(String(p.id)),
																		onChange: () =>
																			toggleSelected(String(p.id)),
																	}
																: {})}
														/>
													</label>
												</td>
											)}
											{delegated && (
												<td
													className="seller-products-table__muted"
													data-label="商品 ID"
												>
													<Link
														href={`${delegatedProductRoot}/${encodeURIComponent(String(p.id))}/edit`}
														className="adm-products__id"
													>
														{String(p.id)}
													</Link>
												</td>
											)}
											<td data-label="商品">
												{/* 代行下不是連結：`/seller-center/*` 會把操作者導出
												    工作區（那條路徑吃的是他自己的賣家身分）。代行的
												    編輯入口在「商品 ID」與「操作」兩欄，都指向 /adm。 */}
												<ProductNameCell
													delegated={delegated}
													href={`/seller-center/products/${p.id}/edit`}
													onOpen={() => openEdit(String(p.id))}
												>
													<span className="seller-products-table__thumb">
														{p.main_image ? (
															// biome-ignore lint/performance/noImgElement: Seller product images may be remote API URLs.
															<img
																src={ossImageUrl(p.main_image, "productThumb")}
																alt=""
																loading="eager"
															/>
														) : (
															// 無圖時給一個圖示，不留純色空方塊：44px 放不下說明
															// 文字，而空方塊在 dark 下是整頁最亮的東西，讀起來
															// 像壞了。與動態列表 `.seller-posts__cover-placeholder`
															// 同一支 icon、同一組色值。
															<span aria-hidden="true">
																<PostImageIcon size={20} />
															</span>
														)}
													</span>
													<span>
														<strong>{p.name || "未命名商品"}</strong>
														{(() => {
															const tags = normaliseHashtags(p.hashtags);
															return tags.length > 0 ? (
																<small className="seller-products-table__tags">
																	{tags.map((t) => `#${t}`).join(" ")}
																</small>
															) : null;
														})()}
													</span>
												</ProductNameCell>
											</td>
											{!delegated && (
												// biome-ignore lint/a11y/useKeyWithClickEvents: stops the row click from opening the edit drawer; the 評論 button stays keyboard-accessible.
												<td
													className="seller-products-table__muted"
													data-label="評分"
													onClick={(e) => e.stopPropagation()}
												>
													{/* 代行下這一欄只顯示評分：買家評論走的是賣家自助端的
													    評論端點（代行端點總表零命中），點開只會拿到錯誤。 */}
													<RatingCell
														delegated={delegated}
														productName={p.name || "未命名商品"}
														onOpen={() =>
															openReviews({
																id: String(p.id),
																name: p.name || "未命名商品",
																image: p.main_image,
															})
														}
													>
														{(() => {
															const score = ratingValue(p);
															return score != null ? (
																<span className="inline-flex items-center gap-1">
																	<AppIcon
																		name="star"
																		size={13}
																		fill
																		className="shrink-0 text-[var(--hkui-warning)]"
																	/>
																	<span className="font-medium tabular-nums text-[var(--hkui-foreground)]">
																		{score.toFixed(1)}
																	</span>
																</span>
															) : (
																<span className="tabular-nums">—</span>
															);
														})()}
														{delegated ? null : (
															<AppIcon
																name="chat"
																size={14}
																className="shrink-0"
															/>
														)}
													</RatingCell>
												</td>
											)}
											<td
												className="seller-products-table__muted"
												data-label="分類"
											>
												{p.category_name || "—"}
											</td>
											<td
												className="seller-products-table__price"
												data-label="價格"
											>
												{priceDisplay(p)}
											</td>
											<td
												className="seller-products-table__strong"
												data-label="庫存"
											>
												{p.stock ?? "—"}
											</td>
											{!delegated && (
												<td
													className="seller-products-table__muted"
													data-label="收藏"
												>
													{p.favorite_count ?? "—"}
												</td>
											)}
											<td
												className="seller-products-table__strong"
												data-label="銷量"
											>
												{salesMap.get(String(p.id)) ?? p.sales_count ?? "—"}
											</td>
											{!delegated && (
												<td
													className="seller-products-table__muted"
													data-label="上架時間"
												>
													{formatListedDate(p.created_at)}
												</td>
											)}
											{/* biome-ignore lint/a11y/useKeyWithClickEvents: stops the row click from opening the drawer when toggling 上下架; the toggle stays keyboard-accessible. */}
											<td
												data-label="狀態"
												onClick={(e) => e.stopPropagation()}
											>
												<ShelfStatusControl
													status={p.status}
													statusLabel={p.status_label}
													busy={
														busyId === String(p.id) &&
														deletingId !== String(p.id)
													}
													upshelfDisabledReason={
														// 加工食品閘記讀的是賣家自己的認證狀態，代行下沒有
														// 那份資料；上架資格由後端授權鏈把關。
														!delegated && upshelfBlockedFor(p)
															? FOOD_REG_REQUIRED_HINT
															: undefined
													}
													onToggle={
														deletingId === String(p.id) || !canPublish
															? undefined
															: (target) => {
																	// 停用按鈕已擋，這裡再保險一次：加工食品未補認證不可上架。
																	if (
																		!delegated &&
																		target === "4004" &&
																		upshelfBlockedFor(p)
																	) {
																		return;
																	}
																	handleSetStatus(p, target);
																}
													}
												/>
											</td>
											{delegated ? (
												<td data-label="操作">
													<div className="adm-products__actions">
														{/* 上架中的商品**內容**不可編輯（product-service 3019），與賣家自助端
														    同一條規則。仍然給得進去（右欄要看狀態與版本），但入口要先說清楚
														    為什麼欄位是鎖的。
														    ⚠️ 商品狀態不是唯一的鎖：缺 `products.write` 時每一列都進不了
														    編輯，字面仍寫「編輯」＝拿商品狀態去回答一個權限問題。 */}
														<Link
															href={`${delegatedProductRoot}/${encodeURIComponent(String(p.id))}/edit`}
															className="seller-table__action"
															title={productRowActionHint(
																canWrite,
																resolveShelfState(p.status) === "active",
															)}
														>
															{productRowActionLabel(
																canWrite,
																resolveShelfState(p.status) === "active",
															)}
														</Link>
														{/* 「預覽」＝買家視角，**每一種狀態都渲染**（含草稿與已下架）。
														    它曾經連去 `/products/{id}`，於是只能在已上架時出現 —— 而那
														    是把工具給錯了人：已上架的商品代行者本來就看得到，看不到的
														    正是草稿。改指工作區內的自繪預覽（56 號 §2.2 第 13 列：以代行
														    GET 的 DTO 自繪，不新增端點、不改打 `/api/v1/public/*`）之後，
														    這個條件沒有存在的理由，而且留在 `/adm` 之下也不會把代行工作
														    階段的上下文丟掉。 */}
														<Link
															href={`${delegatedProductRoot}/${encodeURIComponent(String(p.id))}/preview`}
															className="seller-table__action"
															title="以買家視角預覽這個商品（不會送出任何變更）"
														>
															預覽
														</Link>
													</div>
												</td>
											) : (
												// biome-ignore lint/a11y/useKeyWithClickEvents: stops the row click from opening the drawer when using 編輯/刪除; both stay keyboard-accessible.
												<td
													data-label="操作"
													onClick={(e) => e.stopPropagation()}
												>
													<div className="seller-products-table__actions">
														<RowEditButton
															id={p.id}
															status={p.status}
															productName={p.name || "未命名商品"}
															onOpen={() => openEdit(String(p.id))}
														/>
														<RowDeleteButton
															status={p.status}
															busy={busyId === String(p.id)}
															deleting={deletingId === String(p.id)}
															productName={p.name || "未命名商品"}
															onDelete={() =>
																handleDelete(String(p.id), p.name || "")
															}
														/>
													</div>
												</td>
											)}
										</tr>
									))
								)}
							</tbody>
						</table>
					</div>

					{hasMore && !search && statusFilter !== "low_stock" && (
						<div className="seller-orders__more">
							<Button
								variant="bordered"
								color="default"
								size="sm"
								onClick={loadMore}
								disabled={loadingMore}
								isLoading={loadingMore}
								loadingLabel="載入更多商品中…"
							>
								載入更多
							</Button>
						</div>
					)}
				</div>

				{listBooting ? (
					<div className="seller-mobile-disclosure-list">
						<div className="seller-orders__mobile-loading" role="status">
							載入商品中…
						</div>
					</div>
				) : visibleItems.length === 0 ? (
					<div className="seller-mobile-disclosure-list">
						<div className="seller-orders__mobile-empty">
							<SellerEmptyState
								variant="products"
								searchQuery={search}
								seamless
							/>
						</div>
					</div>
				) : (
					<>
						<SellerMobileDisclosureList
							items={visibleItems}
							getKey={productDisclosureKey}
							getSummary={productDisclosureSummary}
							renderDetails={renderMobileProductDetails}
							ariaLabel="商品清單"
							resetKey={`${String(statusFilter ?? "all")}:${search}`}
						/>
						{hasMore && !search && statusFilter !== "low_stock" && (
							<div className="seller-mobile-disclosure-list">
								<div className="seller-orders__more">
									<Button
										variant="bordered"
										color="default"
										size="sm"
										onClick={loadMore}
										disabled={loadingMore}
										isLoading={loadingMore}
										loadingLabel="載入更多商品中…"
									>
										載入更多
									</Button>
								</div>
							</div>
						)}
					</>
				)}
			</section>
			{/* 代行不渲染這兩個抽屜：商品編輯尚未接線，買家評論在代行的端點總表零命中。 */}
			{!delegated && (
				<ProductEditDrawer
					key={editProductId ?? "closed"}
					productId={editProductId}
					expanded={editExpanded}
					onClose={closeEdit}
					onToggleExpanded={toggleEditExpanded}
					onMutated={handleRefresh}
				/>
			)}
			{!delegated && (
				<ProductReviewsDrawer target={reviewsTarget} onClose={closeReviews} />
			)}
		</SellerPageShell>
	);
}

function productDisclosureKey(product: SellerProductListItem): string {
	return String(product.id);
}

function productDisclosureSummary(
	product: SellerProductListItem,
): SellerMobileDisclosureSummary {
	const name = product.name || "未命名商品";
	return {
		label: `商品「${name}」`,
		leading: product.main_image ? (
			// biome-ignore lint/performance/noImgElement: Seller product images may be remote API URLs.
			<img
				src={ossImageUrl(product.main_image, "productThumb")}
				alt=""
				loading="lazy"
			/>
		) : (
			<PostImageIcon size={18} />
		),
		title: name,
		meta: priceSummaryText(product),
		status: (
			<ShelfStatusControl
				status={product.status}
				statusLabel={product.status_label}
			/>
		),
	};
}

/**
 * 商品名稱格。賣家路徑保持既有的「連結 ＋ 平常點擊開抽屜」；代行路徑是純文字。
 *
 * ⚠️ 不能只是把 `href` 換掉：`/seller-center/*` 在代行工作區之下吃的是**操作者
 * 自己**的賣家身分，點進去就離開了工作區。
 */
function ProductNameCell({
	delegated,
	href,
	onOpen,
	children,
}: {
	delegated: boolean;
	href: string;
	onOpen: () => void;
	children: React.ReactNode;
}) {
	if (delegated) {
		return <span className="seller-products-table__product">{children}</span>;
	}
	return (
		<Link
			href={href}
			className="seller-products-table__product"
			onClick={(e) => openDrawerOnPlainClick(e, onOpen)}
		>
			{children}
		</Link>
	);
}

/** 評分格。代行路徑不可點（沒有評論端點），賣家路徑維持既有的評論入口。 */
function RatingCell({
	delegated,
	productName,
	onOpen,
	children,
}: {
	delegated: boolean;
	productName: string;
	onOpen: () => void;
	children: React.ReactNode;
}) {
	if (delegated) {
		return <span className="inline-flex items-center gap-1.5">{children}</span>;
	}
	return (
		<button
			type="button"
			className="inline-flex items-center gap-1.5 rounded-[var(--hkui-radius-sm)] px-1.5 py-1 leading-none text-inherit transition-colors hover:bg-[var(--hkui-content2)] hover:text-[var(--hkui-foreground)] focus-visible:!outline-none focus-visible:![box-shadow:var(--hkui-focus-ring)]"
			onClick={onOpen}
			aria-label={`查看「${productName}」的買家評論`}
			title="查看買家評論"
		>
			{children}
		</button>
	);
}

function priceRange(lo?: string, hi?: string, single?: string) {
	if (lo && hi && lo !== hi) return `NT$ ${lo} - ${hi}`;
	const v = single ?? lo ?? hi;
	return v ? `NT$ ${v}` : null;
}

function priceSummaryText(p: SellerProductListItem): string {
	return priceRange(p.price_min, p.price_max, p.price) ?? "NT$ -";
}

/**
 * `priceMin/Max` are already the SKU-aggregated *effective* prices
 * (salePrice ?? price). When any SKU has an active discount the wire also
 * carries `salePriceMin/Max`, so we strike through the original
 * `listPriceMin/Max` to show the markdown.
 */
function priceDisplay(p: SellerProductListItem) {
	const main = priceSummaryText(p);
	const onSale = Boolean(p.sale_price_min || p.sale_price_max);
	const list = onSale ? priceRange(p.list_price_min, p.list_price_max) : null;
	return (
		<>
			<span>{main}</span>
			{list && list !== main && (
				<small className="seller-products-table__price-strike">{list}</small>
			)}
		</>
	);
}

function normaliseHashtags(
	raw: SellerProductListItem["hashtags"],
	limit = 3,
): string[] {
	if (!Array.isArray(raw)) return [];
	const out: string[] = [];
	const seen = new Set<string>();
	for (const tag of raw) {
		const name = typeof tag === "string" ? tag : (tag?.name ?? "");
		const trimmed = name.trim().replace(/^#/, "");
		if (!trimmed || seen.has(trimmed)) continue;
		seen.add(trimmed);
		out.push(trimmed);
		if (out.length >= limit) break;
	}
	return out;
}

/** Aggregate rating from the list endpoint (`rating`, a string). Returns the
 *  numeric score, or null when the product has no ratings yet (0/empty). */
function ratingValue(p: SellerProductListItem): number | null {
	const n = Number(p.rating);
	return Number.isFinite(n) && n > 0 ? n : null;
}

/** Format the wire `createdAt` (unix seconds, unix ms, or ISO string) as
 *  YYYY/MM/DD. Returns "—" when absent or unparseable. */
function formatListedDate(value?: number | string) {
	if (value == null || value === "") return "—";
	let ms: number;
	if (typeof value === "number") {
		ms = value < 1e12 ? value * 1000 : value;
	} else if (/^\d+$/.test(value)) {
		const n = Number(value);
		ms = n < 1e12 ? n * 1000 : n;
	} else {
		ms = Date.parse(value);
	}
	if (!Number.isFinite(ms)) return "—";
	return formatTaipeiDate(ms) || "—";
}

/**
 * Product lifecycle (handler.SellerProductItem.status is a CodeMeta string):
 *   "4001" 草稿   "4002" 審核中   "4003" 未上架   "4004" 已上架
 * 顯示名一律取 CodeMeta（pill 由後端下發、KPI 卡讀 `PRODUCT_STATUS_LABEL`）；這裡
 * 的中文只是給讀者對照碼值用，不得被當成可以抄進 JSX 的字面值。
 * Numeric fallbacks (1=active, 2=off, 3=review, 0=draft) cover legacy server
 * responses still in the wild during the schema migration. Label / colour /
 * 上下架切換的呈現已收斂到 `ShelfStatusControl`；這裡只留摘要卡計數要用的判定。
 */
function isActiveStatus(status?: string | number) {
	const k = status == null ? "" : String(status);
	return k === "4004" || k === "1";
}

function isDraftStatus(status?: string | number) {
	const k = status == null ? "" : String(status);
	return k === "4001" || k === "0";
}

function isOffStatus(status?: string | number) {
	const k = status == null ? "" : String(status);
	return k === "4003" || k === "2";
}

function IconSearch() {
	return <AppIcon name="search" />;
}

function IconUpload() {
	return <AppIcon name="upload" />;
}

function IconBoxes() {
	return <AppIcon name="category" />;
}

function IconCheck() {
	return <AppIcon name="check" />;
}

function IconAlert() {
	return <AppIcon name="danger" />;
}

function IconHidden() {
	return <AppIcon name="hide" />;
}

function IconDraft() {
	return <AppIcon name="document" />;
}

/**
 * Row 編輯 control. 上架中(4004) 的商品禁止編輯（避免改到正在販售的內容）—
 * 此時不渲染可點的連結，改成被鎖頭蓋住、變淡的靜態 icon，並用 title/aria-label
 * 說明「請先下架再編輯」。其餘狀態（草稿／審核中／下架）照常導到編輯頁。
 */
function RowEditButton({
	id,
	status,
	productName,
	onOpen,
}: {
	id: string | number;
	status?: string | number;
	productName: string;
	onOpen: () => void;
}) {
	const locked = resolveShelfState(status) === "active";

	if (locked) {
		return (
			<span
				className="seller-products-table__edit-locked"
				role="img"
				aria-label={`${productName} 上架中無法編輯，請先下架`}
				title="上架中無法編輯，請先下架"
			>
				<AppIcon name="edit" size={16} />
				<LockGlyph />
			</span>
		);
	}

	return (
		<Link
			href={`/seller-center/products/${id}/edit`}
			aria-label={`編輯 ${productName}`}
			onClick={(e) => openDrawerOnPlainClick(e, onOpen)}
		>
			<AppIcon name="edit" size={16} />
		</Link>
	);
}

function LockGlyph() {
	return <AppIcon name="lock" className="seller-products-table__lock" />;
}

/**
 * Row 刪除 button. 上架/下架 now lives inline as the `ShelfStatusControl`
 * toggle, so the old kebab menu only ever surfaced 刪除 — drop the dropdown and
 * fire delete straight from a single trash-icon button. 刪除 is permitted for
 * 草稿/下架 (4001/4003) only; when unavailable (上架中 or 審核中) the button is
 * hidden entirely rather than offering a dead control. The destructive
 * confirmation lives in `handleDelete` (window.confirm).
 */
function RowDeleteButton({
	status,
	busy,
	deleting,
	productName,
	onDelete,
}: {
	status?: string | number;
	busy: boolean;
	deleting: boolean;
	productName: string;
	onDelete: () => void;
}) {
	const k = status == null ? "" : String(status);
	const canDelete = k === "4001" || k === "0" || k === "4003" || k === "2";

	// 無可用操作（上架中 / 審核中）— drop the trigger entirely.
	if (!canDelete) return null;

	return (
		<Button
			type="button"
			variant="light"
			color="danger"
			size="sm"
			isIconOnly
			className="seller-products-table__delete"
			aria-label={`刪除 ${productName}`}
			title="刪除"
			disabled={busy}
			isLoading={deleting}
			loadingLabel={`刪除「${productName}」中…`}
			onClick={onDelete}
		>
			<AppIcon name="delete" size={16} />
		</Button>
	);
}
