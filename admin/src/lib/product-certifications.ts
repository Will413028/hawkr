/**
 * 商品農產品認證審核（backend-payuni 19 號設計書）的 wire 型別與純函式。
 *
 * 為什麼型別與純函式不放在 `services/product-certifications.ts`：那個檔負責打網路（`./api`），
 * 測試不必帶著它。ADM 既有的做法就是把可測的部分放 `src/lib/**`（`lib/disputes/detail.ts`、`lib/posts/adminList.ts`
 * 是同一個形狀），service 只留真正打網路的那幾行。
 *
 * ## 🔴 三個必須寫在型別裡的契約事實
 *
 * 1. **所有 id 系列是 string**。TSID 是 64 位元、超過 `Number.MAX_SAFE_INTEGER`，
 *    宣告成 number 會在 `JSON.parse` 當下就失精，之後拿著錯的 id 去核准另一筆申請。
 * 2. **分頁三欄 `total` / `page` / `pageSize` 在 wire 上是「字串」**。gateway 的
 *    `pkg/envelope.Page(c, records, total, page, pageSize string)` 三個參數都是
 *    string，吐出來就是 `"page":"1"`。照數字宣告 TypeScript 不會報錯（JSON 沒有
 *    型別檢查），錯誤會一路沉默到執行期算出 `NaN`。所以 wire 型別誠實寫成 string，
 *    由 `parsePaged` 在邊界轉一次。
 * 3. **可空欄位在 gateway 是 `*string` → JSON `null`**，不是空字串也不是欄位消失
 *    （`gateway/internal/handler/admin_product_certification.go` 的 `nilIfEmptyStr`）。
 */

import {
	CertificationType,
	codeLabel,
	PRODUCT_CERTIFICATION_GOV_VERIFY_STATE_LABEL,
	PRODUCT_CERTIFICATION_STATUS_LABEL,
	PRODUCT_PUBLICATION_STATE_LABEL,
	ProductPublicationState,
} from "@/lib/codemeta";
import { formatDateTime as formatTaipeiDateTime } from "@/lib/format";

/**
 * 政府開放資料平台的查驗結果（Admin-only；19 §9.2 / §10.2 `GovVerifyResult`）。
 *
 * 🔴 這一整塊就是「ADM 憑什麼核准」的依據。少顯示一個欄位，審核者就少一個判斷條件，
 * 而畫面不會有任何異狀——它只是變成在無依據硬核准。
 */
export interface GovVerifyResultWire {
	/** CertificationType value（"3701"–"3706"） */
	certType: string;
	/** 送查的證書編號；與 `certificateNo` 不同即代表查驗結果對不上這一筆申請 */
	certNo: string;
	queriedAt: string | null;
	/** 政府資料庫是否查得到這個編號。`found=true` 不等於 `validity=VALID`（19 §8.6） */
	found: boolean;
	/**
	 * 政府資料判讀出的證書效力；值域見生成物 `ProductCertificationValidity`，
	 * 顯示一律走 `codeLabel(PRODUCT_CERTIFICATION_VALIDITY_LABEL, …)`。
	 * 🔴 這是 ADM 判斷「能不能核准」的主要依據，不是塌縮後的 `govVerifyState`。
	 * 型別維持 `string` 而非 enum union：後端新增值域時要落到 `codeLabel` 的
	 * 原值 fallback（顯示碼值），不是在 wire 邊界炸掉整頁。
	 */
	validity: string;
	/** 3701 才有（通過／結束／終止／暫終）；其餘型別為 null */
	statusRaw: string | null;
	/** 3703 沒有效期欄位 */
	expiresOn: string | null;
	holderName: string | null;
	holderAddress: string | null;
	issuerName: string | null;
	productSummary: string | null;
	/**
	 * 這個編號在政府資料庫對到幾列（數字字串）。
	 * 🔴 3703 CAS 實測一個 `Emblem_ID` 可以對到 173 列——它是**工廠**標誌編號。
	 */
	matchedCount: string;
	/**
	 * 已由後端修剪過的政府原始回應（19 §8.4 防線 3）。
	 * ⚠️ 3702 的 `ParentTraceCode` 內含未跳脫的 HTML（`<a href="…">`），
	 * 渲染時必須走 React 的文字節點（自動 escape），不得用 dangerouslySetInnerHTML。
	 */
	rawExcerpt: unknown;
}

/** 同類型「現行有效」那筆的精簡投影（19 §9.2 `currentApproved`）。 */
export interface CertificationSummaryWire {
	id: string;
	certificationType: string;
	certificateNo: string;
	/** 恆為 APPROVED（本欄只回現行有效那筆） */
	status: string;
	govVerifyState: string;
	reviewedAt: string | null;
}

/**
 * 這筆申請的商品是否已不在平台上（已軟刪／賣家帳號已刪）。
 *
 * 判斷只認 `REMOVED`：`TEMPORARILY_UNAVAILABLE`（下架中）與 `NEVER_PUBLISHED`（草稿）
 * 的商品都還在，審核照常進行。空字串＝後端查不到該商品列，也視為已不在。
 */
export function isProductGone(state: string): boolean {
	return state === ProductPublicationState.REMOVED || state === "";
}

/**
 * 商品已不在平台上時要顯示的說明。
 *
 * 空字串是「後端連商品列都查不到」（被實體刪除），`codeLabel` 對空值回 "—" 會讓畫面
 * 看起來像沒填，所以這裡自己給一句話。
 */
export function productGoneLabel(state: string): string {
	if (state === "") return "商品已不存在";
	return codeLabel(PRODUCT_PUBLICATION_STATE_LABEL, state);
}

/** ADM 看到的一筆認證申請（列表與詳情共用同一個 DTO）。 */
export interface CertificationApplicationWire {
	id: string;
	productId: string;
	productName: string;
	/**
	 * 商品的公開生命週期（ProductPublicationState value，"4011"–"4014"）。
	 *
	 * 🔴 **認證申請結構上會指向已軟刪的商品**（後端 19 號設計書 §9.2）：legacy migration
	 * 不濾 deleted_at、賣家送件後可以刪掉自己的商品，而後端刻意保留這些申請讓 ADM 能對
	 * 已下架商品撤銷標章。所以「有 productName」**不等於**「這個商品還在平台上」。
	 * 顯示時必須把非 PUBLIC 的狀態標出來，否則審核者會核准一個已經不存在的商品。
	 *
	 * 商品列被實體刪除時為空字串（後端查不到 ⇒ 不設）。
	 */
	productPublicationState: string;
	storeId: string;
	/** 送件者的登入 principal（accounts PK），不是營運鍵 storeId */
	submittedByAccountId: string | null;
	certificationType: string;
	certificateNo: string;
	status: string;
	source: string;
	requiresManualReview: boolean;
	govVerifyState: string;
	govVerifiedAt: string | null;
	submittedAt: string | null;
	reviewedAt: string | null;
	/** 自動核准時為 null（actor 由審核事件的 SYSTEM 標示） */
	reviewedByAccountId: string | null;
	reasonCode: string | null;
	publicReason: string | null;
	govVerifyPayload: GovVerifyResultWire | null;
	/** 首次申請時是 null，代表核准這一筆不會換掉任何既有標章 */
	currentApproved: CertificationSummaryWire | null;
	internalNote: string | null;
	/** 樂觀鎖版本（數字字串），送裁決時原樣回填 expectedVersion */
	version: string;
}

/** append-only 審核事件（19 §4.2）。 */
export interface CertificationAuditLogWire {
	id: string;
	applicationId: string;
	action: string;
	/** 建立事件為空字串 */
	fromStatus: string;
	toStatus: string;
	/**
	 * 事件發起者身分；值域見生成物 `ProductCertificationActorRole`，
	 * 顯示一律走 `codeLabel(PRODUCT_CERTIFICATION_ACTOR_ROLE_LABEL, …)`。
	 * 型別維持 `string` 的理由同 `GovVerifyResultWire.validity`。
	 */
	actorRole: string;
	actorAccountId: string | null;
	certificationType: string;
	certificateNo: string;
	reasonCode: string | null;
	publicReason: string | null;
	internalNote: string | null;
	applicationVersion: string;
	traceId: string | null;
	createdAt: string | null;
}

/** `response.Page` 的 data 形狀——三個數字欄位在 wire 上都是字串。 */
export interface PagedWire<T> {
	records: T[] | null;
	total: string;
	page: string;
	pageSize: string;
}

/** 轉換後給畫面用的形狀：records 保證是陣列，三個數字保證是 number。 */
export interface Paged<T> {
	records: T[];
	total: number;
	page: number;
	pageSize: number;
}

/**
 * 把 wire 的數字字串轉成 number。
 *
 * 不用 `parseInt`：`parseInt("12abc")` 回 12，會把壞掉的欄位變成看似正常的數字。
 * `Number("12abc")` 是 NaN，落回 fallback，至少不會謊報總數。
 */
function toCount(raw: unknown, fallback: number): number {
	if (typeof raw === "number") return Number.isFinite(raw) ? raw : fallback;
	if (typeof raw !== "string" || raw.trim() === "") return fallback;
	const n = Number(raw);
	return Number.isFinite(n) ? n : fallback;
}

/**
 * `response.Page` 的邊界轉換。
 *
 * `records` 為 null（後端回空集合時 proto repeated 欄位可能整個不設）一律轉成 `[]`，
 * 讓呼叫端不必到處寫 `?? []`——空集合是正常狀態，不是錯誤。
 */
export function parsePaged<T>(
	wire: PagedWire<T> | null | undefined,
	fallbackPage = 1,
	fallbackPageSize = 20,
): Paged<T> {
	return {
		records: wire?.records ?? [],
		total: toCount(wire?.total, 0),
		page: toCount(wire?.page, fallbackPage),
		pageSize: toCount(wire?.pageSize, fallbackPageSize),
	};
}

/**
 * `<input type="date">` 的 `YYYY-MM-DD` → gateway 要的 ISO 8601 UTC。
 *
 * 為什麼不是直接接上 `T00:00:00Z`：管理員挑的是**他所在時區的那一天**（台灣 UTC+8），
 * 直接補 `Z` 會把區間整整平移 8 小時，早上 8 點前送的件全部掉出「今天」。
 * 這裡把「本地當日 00:00:00 / 23:59:59」轉成對應的 UTC 瞬間。
 *
 * 毫秒要砍掉：契約寫的是 `YYYY-MM-DDTHH:mm:ssZ`，而 `toISOString()` 會多帶 `.000`。
 * 空字串（未選日期）回空字串，呼叫端據此不帶該 query 參數。
 */
function localDateTimeToIsoUtc(local: string): string {
	const d = new Date(local);
	if (Number.isNaN(d.getTime())) return "";
	return d.toISOString().replace(/\.\d{3}Z$/u, "Z");
}

export function toUtcDayStart(localDate: string): string {
	if (!localDate) return "";
	return localDateTimeToIsoUtc(`${localDate}T00:00:00`);
}

export function toUtcDayEnd(localDate: string): string {
	if (!localDate) return "";
	return localDateTimeToIsoUtc(`${localDate}T23:59:59`);
}

/**
 * 這個認證類型的證書編號是不是「工廠級」而非「商品級」。
 *
 * 🔴 目前只有 3703 CAS：`Emblem_ID` 是工廠標誌編號，實測一個編號對到 173 個產品
 * （19 §8.5 實測）。查得到只證明**該工廠具 CAS 資格**，不證明**這個商品是 CAS 產品**。
 * 所以 3703 一律人工審，而且 ADM 詳情頁必須把這件事講出來——否則審核者看到
 * 「查驗通過」就會按核准，等於讓賣家用工廠編號給任意商品掛 CAS 標章。
 */
export function isFactoryScopedCertification(
	certificationType: string,
): boolean {
	return certificationType === CertificationType.CAS;
}

/** 工廠級證書的審核提示；`matchedCount` 是 wire 上的數字字串。 */
export function factoryScopeHint(
	matchedCount: string | null | undefined,
): string {
	const n = toCount(matchedCount, 0);
	const scope = n > 0 ? `共 ${n.toLocaleString()} 項產品` : "涵蓋多項產品";
	return `CAS 資料僅能證明該工廠具 CAS 資格（${scope}），請確認申請商品是否在清單內。`;
}

/**
 * FNV-1a 32 位元。純粹拿來當「送出內容的指紋」，不是安全雜湊。
 *
 * 用 32 位而非 64：本專案 tsconfig `target: ES2017`，BigInt 字面值不可用（TS2737）。
 * `Math.imul` 才是 32 位環繞乘法，直接用 `*` 會走 IEEE754 double 而失去低位。
 */
function fnv1a32(input: string): string {
	const bytes = new TextEncoder().encode(input);
	let hash = 0x811c9dc5;
	for (let i = 0; i < bytes.length; i++) {
		hash ^= bytes[i];
		hash = Math.imul(hash, 0x01000193);
	}
	return (hash >>> 0).toString(16).padStart(8, "0");
}

/** 裁決請求的冪等鍵輸入——就是會被送到後端的那幾個欄位。 */
export interface ReviewKeyInput {
	applicationId: string;
	action: string;
	expectedVersion: string;
	reasonCode: string;
	publicReason: string;
	internalNote: string;
}

/**
 * 裁決的冪等鍵：由送出內容決定，不含亂數。
 *
 * 🔴 為什麼不能用「開對話框時抽一把 UUID」：後端 `IdempotencyService` 會比對
 * request hash，**同一把鑰匙配不同 payload 會被拒絕**（`IdempotencyConflictException`），
 * 不是靜默覆蓋。管理員填錯原因、改完再送，就會撞在這一關上而看不懂為什麼。
 *
 * 🔴 為什麼也不能「每次點擊都抽新的」：那樣連點兩次就是兩筆裁決請求，冪等鍵等於沒作用。
 *
 * 由內容決定同時滿足兩邊：同一個決定永遠同一把鑰匙（重送安全），改任何一個欄位就換一把
 * （不會撞 hash 比對）。`expectedVersion` 進指紋是必要的——同一份文字在不同版本上是
 * 不同的決定。
 *
 * ⚠️ 長度上限 64：`idempotency_request.key` 是 `VARCHAR(64)`（product-service
 * V1 migration）。這裡的組成最長約 50 字元，但改格式前要重算。
 */
export function reviewIdempotencyKey(input: ReviewKeyInput): string {
	const fingerprint = fnv1a32(
		[
			input.applicationId,
			input.action,
			input.expectedVersion,
			input.reasonCode,
			input.publicReason,
			input.internalNote,
			// 分隔符用 NUL：原因與註記都是自由輸入，拿空白或換行當分隔的話
			// ["a b", ""] 與 ["a", "b"] 會算出同一個指紋——那正是要區分的兩個決定。
		].join("\u0000"),
	);
	return `cert-${input.action}-${input.applicationId}-v${input.expectedVersion}-${fingerprint}`;
}

/**
 * 查驗結果與申請單上的編號是否一致。
 *
 * 對不上代表這份 payload 不是這筆申請的查驗結果（例如政府 API 忽略未知參數回了整個
 * 資料集，19 §8.4 防線 2）。後端已經擋了一層，這裡是顯示層的第二眼——真的對不上時
 * 要讓審核者看見，不是靜靜地把別人的資料當成證據。
 */
export function govPayloadMatchesApplication(
	app: Pick<
		CertificationApplicationWire,
		"certificateNo" | "certificationType"
	>,
	payload: Pick<GovVerifyResultWire, "certNo" | "certType"> | null | undefined,
): boolean {
	if (!payload) return true;
	return (
		payload.certNo === app.certificateNo &&
		payload.certType === app.certificationType
	);
}

/** ISO 8601 → 本地可讀時間；空值回 "-"，無法解析時原樣回傳（不要吞掉壞資料）。 */
export function formatDateTime(iso: string | null | undefined): string {
	if (!iso) return "-";
	const d = new Date(iso);
	if (Number.isNaN(d.getTime())) return iso;
	return formatTaipeiDateTime(d);
}

/**
 * 裁決成功後的就地提示文案（核准／駁回／撤銷共用）。
 *
 * 🔴 **不得寫成 `` `已${codeLabel(PRODUCT_CERTIFICATION_STATUS_LABEL, status)}` ``**。
 * 那些標籤**本身就是完成式**：`APPROVED`→「已通過」、`REVOKED`→「已撤銷」，
 * 前面再補一個「已」就吐出「已已通過」「已已撤銷」；而 `REJECTED`→「未通過」
 * 補上去會變成「已未通過」——三個動作裡唯一不重字、因此最容易通過肉眼檢查的一個，
 * dev 上正是它先被使用者抓到。
 *
 * 標籤是 CodeMeta 生成物（值由後端 Markdown 決定，前端不得對它的字面做假設），
 * 所以正解不是「挑掉那個已」，而是**讓句型不依賴標籤的詞性**：把標籤整個放進
 * 引號當成一個名詞塞進句子，任何標籤字面都成立。
 */
export function reviewResultNoticeText(
	status: string | null | undefined,
): string {
	const label = codeLabel(PRODUCT_CERTIFICATION_STATUS_LABEL, status);
	return `審核結果已更新為「${label}」，並已寫入審核紀錄。`;
}

/**
 * 「重新查驗」成功後的就地提示文案。
 *
 * 兩件必須講出來的事：
 *
 * 1. 🔴 **這一次真的打了農業部**。後端的 reverify 路徑刻意繞過結果快取
 *    （backend-payuni 19 §9.3），因為快取 TTL 對 `VALID` 是 **7 天** ——
 *    賣家在申請之後才補登證書時，若讀快取，ADM 七天內按幾次都拿不到新答案，
 *    而畫面上不會有任何線索。繞過快取的代價是**消耗每日 API 配額**，
 *    所以要讓按的人知道自己剛剛花掉了什麼。
 * 2. **審核狀態沒有被動到**。reverify 只回寫查驗欄位、不改 `status`
 *    （§9.3：ADM 已經在看這一筆，自動放行會讓人工與系統裁決在同一秒打架）。
 *    原本的文案把 `status` 寫成「最新結果」，讀起來像是查驗改變了審核結論。
 */
export function reverifyResultNoticeText(
	govVerifyState: string | null | undefined,
	status: string | null | undefined,
): string {
	const stateLabel = codeLabel(
		PRODUCT_CERTIFICATION_GOV_VERIFY_STATE_LABEL,
		govVerifyState,
	);
	const statusLabel = codeLabel(PRODUCT_CERTIFICATION_STATUS_LABEL, status);
	return (
		`已重新向農業部查詢（不使用先前的查驗快取），查驗狀態：${stateLabel}。` +
		`本動作不會改變審核狀態（目前為「${statusLabel}」）。`
	);
}

/** 「重新查驗」按鈕旁的事前說明——配額是按下去才花的，講在事後就太晚了。 */
export const REVERIFY_HINT =
	"「重新查驗」會直接向農業部重新查詢（不使用先前的查驗快取），會消耗每日 API 配額。";

/**
 * 「裁決者」欄位的顯示值。
 *
 * `reviewedByAccountId` 為空有<b>兩種完全不同</b>的意思，共用一個 `||` fallback
 * 會把它們混成同一句話：
 *
 * 1. **尚未裁決**（`reviewedAt` 也是空）—— 待審案件。應顯示 `-`
 * 2. **自動核准**（`reviewedAt` 有值，但沒有人工介入）—— 政府 API 查得到而自動放行，
 *    這時「（系統自動）」才是正確答案
 *
 * 判斷依據是 `reviewedAt` 而不是 `status`：撤銷／駁回同樣會寫入 `reviewedAt`，
 * 用 status 白名單會在新增狀態時漏掉。
 */
export function reviewerDisplay(
	reviewedAt: string | null | undefined,
	reviewedByAccountId: string | null | undefined,
): string {
	if (!reviewedAt) return "-";
	return reviewedByAccountId || "（系統自動）";
}
