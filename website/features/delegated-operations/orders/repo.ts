import { delegatedFetch, type RepoFetcher } from "@/lib/api/delegated";
import { taipeiDayEndUtc, taipeiDayStartUtc } from "@/lib/date/taipei";
import type {
	DelegatedOrderFilters,
	DelegatedOrderSearchFilters,
} from "./filters";

/**
 * 代行訂單面的資料層（56 號 §6.1.10，端點恰三支且**全部唯讀**）。
 *
 * - `GET  /adm/api/orders/{storeId}`                              列表
 * - `GET  /adm/api/orders/{storeId}/{orderNo}`                    明細（收件資訊已遮罩）
 * - `GET  /adm/api/orders/{storeId}/{orderNo}/recipient-contact`  揭露（必落稽核）
 *
 * ⚠️ **為什麼不沿用 `lib/orders/repo.ts` 的 `createOrderRepo` ＋ `normalizeOrder`**
 * （定案第 1 條 ② 的同一個判準：「工廠必須帶自己的 normalizer」）：代行 DTO 與
 * `/app/orders` 是兩份不同形狀，而 `normalizeOrder` 對讀不到的欄位一律回
 * `undefined`、**不報錯**。逐欄實查 `gateway/internal/handler/admin_delegated_order.go`
 * 之後，沿用會靜默丟掉的欄位是：
 *   `buyer.displayName`（它嵌在 `buyer` 之下，`normalizeOrder` 的買家別名鏈只掃頂層）
 *   `recipient.{name,city,district}`（明細另有 `maskedPhone`；`recipient` 不在 shipping 別名鏈內）
 *   `itemCount`／`statusLabel`／`shippingStatusLabel`／`timeline[]`／`orderRemark`／`shippingNote`
 *     （`Order` 型別根本沒有這些欄位）
 * 而那幾欄正好就是這一面畫面上的全部內容 —— 沿用的結果是「買家欄空白、地址整段
 * 不見、時間軸不見」，且沒有任何東西會轉紅。
 *
 * ⚠️ 本檔**不得**出現任何履約動作（取號／託運單／叫件／確認出貨／取消／退款）。
 * 代行訂單面沒有這些端點，寫出來只能是死路徑。
 */

/** 買家顯示名。代行 DTO 只給這一欄，沒有買家電話（賣家列表 RPC 從不回）。 */
export interface DelegatedOrderBuyer {
	displayName: string;
}

/**
 * 遮罩後的收件資訊（**明細**）。遮罩發生在 order-service，不在 Gateway、更不在這裡。
 */
export interface DelegatedOrderRecipient {
	name: string;
	maskedPhone: string;
	city: string;
	district: string;
}

/**
 * 列表版的收件資訊，比明細少一個 `maskedPhone`。
 *
 * ⚠️ 這不是漏抄：`OrderSummaryView` 沒有收件人電話欄位，Gateway 的列表 DTO 因此不宣告
 * 它（`admin_delegated_order.go: DelegatedOrderListRecipient`）。這裡跟著分離，呼叫端才
 * 不會寫出一條永遠不執行的「有電話就顯示」分支。要電話請開明細。
 */
export interface DelegatedOrderListRecipient {
	name: string;
	city: string;
	district: string;
}

/**
 * 列表一列。**沒有** `paymentMethod`／`updatedAt`／`recipient.maskedPhone`：上游沒有這幾個
 * 來源欄位。
 *
 * `firstProductName` 是第一件品項的名稱（Gateway 在 `product_name` 為空時已退回 `sku_name`）。
 * 它與 `itemCount` 一起讓商品欄說得出搜尋命中的是哪一件 —— keyword 比對範圍含
 * `order_items.product_name`／`sku_name`，而這一欄先前只印得出件數。
 */
export interface DelegatedOrderListItem {
	orderNo: string;
	createdAt: string;
	status: string;
	statusLabel: string;
	itemCount: string;
	firstProductName: string;
	totalAmount: string;
	shippingStatus: string;
	shippingStatusLabel: string;
	shippingMethod: string;
	buyer: DelegatedOrderBuyer;
	recipient: DelegatedOrderListRecipient;
}

export interface DelegatedOrderItem {
	productId: string;
	productName: string;
	skuId: string;
	skuName: string;
	quantity: string;
	unitPrice: string;
	listPrice: string;
	actualPayAmount: string;
	imageKey: string;
}

/**
 * 狀態時間軸的一個點。
 *
 * ⚠️ 後端**只送真的發生過的節點**，且**最後一個節點恆為訂單當下的狀態**
 * （`buildOrderTimeline`：里程碑不得越過當下狀態，離開正向流程的狀態則以時間戳為證據）。
 * 呼叫端不得為了湊出 mockup 的四節點而自己補上未發生的節點，那會渲染成一個訂單從來
 * 不曾處於的狀態 —— 這正是先前「已取消的訂單顯示配送中」的成因。
 *
 * ⚠️ `occurredAt` **可能是空字串**：待出貨／待取貨／物流異常／部分退款／全額退款在 orders
 * 表沒有專屬的時間欄，後端不拿 `updated_at` 充數（那一欄會被任何後續寫入推走）。呼叫端
 * 據此不渲染時間那一行，不要顯示成空白或 Invalid Date。
 * 送達／完成／取消三個結案節點**有**自己的時間欄，會帶著時間送過來。
 */
export interface DelegatedOrderTimelineEntry {
	status: string;
	statusLabel: string;
	occurredAt: string;
}

export interface DelegatedOrderDetail {
	orderNo: string;
	createdAt: string;
	status: string;
	statusLabel: string;
	shippingStatus: string;
	shippingStatusLabel: string;
	paymentMethod: string;
	shippingMethod: string;
	itemCount: string;
	/** 商品小計（上游 `total_original_amount`）。運費／折扣／訂單金額少了它湊不起來。 */
	itemsSubtotal: string;
	totalAmount: string;
	shippingFee: string;
	discountAmount: string;
	/**
	 * 取消三欄。時間軸說得出「已取消」，說不出「為什麼」，而代行者要協助賣家正是需要
	 * 那個為什麼。`cancelRemark` 的寫入者只有系統排程與賣家（買家自願取消時兩欄皆
	 * NULL，見 CodeMeta `OrderCancelReason`），所以它是賣家自己的內容。
	 * 非取消訂單三欄皆為空字串。
	 */
	cancelReason: string;
	cancelRemark: string;
	cancelByRole: string;
	buyer: DelegatedOrderBuyer;
	recipient: DelegatedOrderRecipient;
	items: DelegatedOrderItem[];
	timeline: DelegatedOrderTimelineEntry[];
	orderRemark: string;
	shippingNote: string;
}

/** 全平台唯一會回傳未遮罩收件資訊的形狀。**不得寫進任何快取。** */
export interface DelegatedRecipientContact {
	name: string;
	phone: string;
	fullAddress: string;
	zipCode: string;
}

export interface DelegatedRecipientContactResult {
	/** 這次揭露落下的稽核列 id，是它可被追溯的唯一憑據。 */
	auditLogId: string;
	recipient: DelegatedRecipientContact;
}

export interface DelegatedOrderListResult {
	records: DelegatedOrderListItem[];
	total: number;
	page: number;
	pageSize: number;
}

/**
 * 列表參數。篩選條件的形狀與判準在 `./filters`（清單、KPI 卡、零筆文案共用同一份），
 * 這裡只負責把它展開成 wire 參數。
 */
export interface DelegatedOrderListParams
	extends Partial<DelegatedOrderFilters> {
	page?: number;
	pageSize?: number;
	signal?: AbortSignal;
}

type UnknownRecord = Record<string, unknown>;

function isRecord(value: unknown): value is UnknownRecord {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

function str(value: unknown): string {
	return typeof value === "string" ? value : "";
}

/** `response.Page` 的 `total`／`page`／`pageSize` 都是數字字串（§4 值域）。 */
function num(value: unknown, fallback: number): number {
	const parsed = Number(value);
	return Number.isFinite(parsed) ? parsed : fallback;
}

function readBuyer(value: unknown): DelegatedOrderBuyer {
	const raw = isRecord(value) ? value : {};
	return { displayName: str(raw.displayName) };
}

function readRecipient(value: unknown): DelegatedOrderRecipient {
	const raw = isRecord(value) ? value : {};
	return {
		name: str(raw.name),
		maskedPhone: str(raw.maskedPhone),
		city: str(raw.city),
		district: str(raw.district),
	};
}

function readListRecipient(value: unknown): DelegatedOrderListRecipient {
	const raw = isRecord(value) ? value : {};
	return {
		name: str(raw.name),
		city: str(raw.city),
		district: str(raw.district),
	};
}

function readListItem(value: unknown): DelegatedOrderListItem {
	const raw = isRecord(value) ? value : {};
	return {
		orderNo: str(raw.orderNo),
		createdAt: str(raw.createdAt),
		status: str(raw.status),
		statusLabel: str(raw.statusLabel),
		itemCount: str(raw.itemCount),
		firstProductName: str(raw.firstProductName),
		totalAmount: str(raw.totalAmount),
		shippingStatus: str(raw.shippingStatus),
		shippingStatusLabel: str(raw.shippingStatusLabel),
		shippingMethod: str(raw.shippingMethod),
		buyer: readBuyer(raw.buyer),
		recipient: readListRecipient(raw.recipient),
	};
}

function readItem(value: unknown): DelegatedOrderItem {
	const raw = isRecord(value) ? value : {};
	return {
		productId: str(raw.productId),
		productName: str(raw.productName),
		skuId: str(raw.skuId),
		skuName: str(raw.skuName),
		quantity: str(raw.quantity),
		unitPrice: str(raw.unitPrice),
		listPrice: str(raw.listPrice),
		actualPayAmount: str(raw.actualPayAmount),
		imageKey: str(raw.imageKey),
	};
}

function readTimelineEntry(value: unknown): DelegatedOrderTimelineEntry {
	const raw = isRecord(value) ? value : {};
	return {
		status: str(raw.status),
		statusLabel: str(raw.statusLabel),
		occurredAt: str(raw.occurredAt),
	};
}

function readDetail(value: unknown): DelegatedOrderDetail {
	const raw = isRecord(value) ? value : {};
	return {
		orderNo: str(raw.orderNo),
		createdAt: str(raw.createdAt),
		status: str(raw.status),
		statusLabel: str(raw.statusLabel),
		shippingStatus: str(raw.shippingStatus),
		shippingStatusLabel: str(raw.shippingStatusLabel),
		paymentMethod: str(raw.paymentMethod),
		shippingMethod: str(raw.shippingMethod),
		itemCount: str(raw.itemCount),
		itemsSubtotal: str(raw.itemsSubtotal),
		totalAmount: str(raw.totalAmount),
		shippingFee: str(raw.shippingFee),
		discountAmount: str(raw.discountAmount),
		cancelReason: str(raw.cancelReason),
		cancelRemark: str(raw.cancelRemark),
		cancelByRole: str(raw.cancelByRole),
		buyer: readBuyer(raw.buyer),
		recipient: readRecipient(raw.recipient),
		items: Array.isArray(raw.items) ? raw.items.map(readItem) : [],
		timeline: Array.isArray(raw.timeline)
			? raw.timeline.map(readTimelineEntry)
			: [],
		orderRemark: str(raw.orderRemark),
		shippingNote: str(raw.shippingNote),
	};
}

function readContact(value: unknown): DelegatedRecipientContactResult {
	const raw = isRecord(value) ? value : {};
	const recipient = isRecord(raw.recipient) ? raw.recipient : {};
	return {
		auditLogId: str(raw.auditLogId),
		recipient: {
			name: str(recipient.name),
			phone: str(recipient.phone),
			fullAddress: str(recipient.fullAddress),
			zipCode: str(recipient.zipCode),
		},
	};
}

export interface DelegatedOrderRepo {
	list(params?: DelegatedOrderListParams): Promise<DelegatedOrderListResult>;
	detail(
		orderNo: string,
		opts?: { signal?: AbortSignal },
	): Promise<DelegatedOrderDetail>;
	revealRecipientContact(
		orderNo: string,
		operationReason: string,
		opts?: { signal?: AbortSignal },
	): Promise<DelegatedRecipientContactResult>;
}

/**
 * 綁定單一店家的訂單資料層。
 *
 * `storeId` 進的是路徑而不是 query：代行的店家界線由路徑決定，代行端點**沒有**
 * `/app/orders` 那個 `view=seller` 參數。
 */
export function createDelegatedOrderRepo(
	storeId: string,
	fetcher: RepoFetcher = delegatedFetch,
): DelegatedOrderRepo {
	const root = `/orders/${encodeURIComponent(storeId)}`;
	return {
		async list({
			page = 1,
			pageSize = 10,
			status,
			keyword,
			dateFrom,
			dateTo,
			signal,
		}: DelegatedOrderListParams = {}) {
			// ⚠️ 只送 Gateway handler 真的讀的鍵。BFF 對未列入的 key 回 400 而不是靜默
			// 丟棄，所以這裡多送一個鍵會當場現形，而不是變成「篩選沒作用也沒錯誤」。
			const query: Record<string, string> = {
				page: String(page),
				page_size: String(pageSize),
			};
			if (status) query.status = status;
			// trim 之後為空就不送：只打了幾個空白鍵，送出去會變成「搜尋了空字串」而
			// 命中全部，與使用者以為的「我還沒搜」剛好相反。
			const trimmed = (keyword ?? "").trim();
			if (trimmed) query.keyword = trimmed;
			// 日期在這裡才從台北曆日展開成 UTC instant（§4：wire 上一律 ISO 8601 UTC）。
			const from = dateFrom ? taipeiDayStartUtc(dateFrom) : "";
			const to = dateTo ? taipeiDayEndUtc(dateTo) : "";
			if (from) query.date_from = from;
			if (to) query.date_to = to;
			const env = await fetcher<unknown>(root, { query, signal });
			const data = isRecord(env.data) ? env.data : {};
			const records = Array.isArray(data.records)
				? data.records.map(readListItem)
				: [];
			return {
				records,
				total: num(data.total, records.length),
				page: num(data.page, page),
				pageSize: num(data.pageSize, pageSize),
			};
		},

		async detail(orderNo, opts = {}) {
			const env = await fetcher<unknown>(
				`${root}/${encodeURIComponent(orderNo)}`,
				{ signal: opts.signal },
			);
			return readDetail(env.data);
		},

		async revealRecipientContact(orderNo, operationReason, opts = {}) {
			// GET 沒有 body，理由走 query（snake_case，trim 後 1–100 字，後端必填）。
			const env = await fetcher<unknown>(
				`${root}/${encodeURIComponent(orderNo)}/recipient-contact`,
				{ query: { operation_reason: operationReason }, signal: opts.signal },
			);
			return readContact(env.data);
		},
	};
}

/**
 * KPI 卡的數字：每個 status 一次 `page_size=1` 的列表，只讀它的 `total`。
 *
 * ⚠️ **這不是第四支端點**，是同一支 `list` 打 N 次（B15 明列的唯一合法作法：放寬一支
 * 統計 RPC 不碰 registry、不觸發 boot 斷言、不需新 capability，CI 全綠而攻擊面已擴大）。
 *
 * ⚠️ **`filters` 必須是清單當下用的那一份。** 抽成獨立函式而不是寫在 hook 的 `queryFn`
 * 裡，就是為了讓「每一張卡都吃到同一組條件」變成一條可執行的斷言 —— 少展開一個條件，
 * `list-filters.unit.test.ts` 當場轉紅，而畫面上只會是一組看起來很正常、卻與清單矛盾
 * 的數字。
 */
export async function fetchDelegatedOrderStatusTotals(
	repo: DelegatedOrderRepo,
	statuses: readonly string[],
	filters: DelegatedOrderSearchFilters,
	signal?: AbortSignal,
): Promise<Record<string, number>> {
	const entries = await Promise.all(
		statuses.map(async (status) => {
			const page = await repo.list({
				page: 1,
				pageSize: 1,
				status,
				keyword: filters.keyword,
				dateFrom: filters.dateFrom,
				dateTo: filters.dateTo,
				signal,
			});
			return [status, page.total] as const;
		}),
	);
	return Object.fromEntries(entries);
}
