/**
 * 站外業者 portal 的 client。進入與驗證（preview、exchange、讀 session、step-up）走 Connect：
 * 以 secret 為憑證的 `VendorPortalPublicService`，與以 cookie 為憑證的
 * `VendorPortalInviteeService`（`lib/connect/vendor-portal.ts`）；RFQ 摘要、回價草稿與正式提交
 * 還是舊 gateway 的 REST（`/api/v1/public/vendor_portal/rfqs/…`，到 A9 才改成 Connect）。
 *
 * session 是 `ExchangeInvite` 設的 `__Host-hk_vp_<rfq_recipient_id>` cookie（每張邀請一個；HttpOnly、
 * Secure、SameSite=Lax、`Path=/`），JS 讀不到；REST 的呼叫帶 `credentials: "include"` 讓它隨請求送出。
 * 頁面網址帶收件人 ID（`/vendor/rfq?r=<id>`），重新整理時以它指名要用哪一張邀請的 session。
 *
 * 兩段式刻意分開：`PreviewInvite` 沒有副作用，`ExchangeInvite` 才發 session。頁面因此可以先讓
 * 業者看到「這封信是寄給誰、什麼時候到期」再由人點一下才兌換——email 掃描器或瀏覽器 prerender
 * 打到落地頁時不會發出 session。
 */
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { SessionVerificationMethod } from "@hawkr/api/gen/hawkr/agent/v1alpha1/agent_pb";
import { ApiClientError, apiClient } from "@/lib/api/client";
import {
	vendorPortalClient,
	vendorPortalInviteeClient,
} from "@/lib/connect/vendor-portal";
import {
	normalizeVendorResponseDraft,
	type VendorResponseDraft,
} from "./draft";

/** 邀請落地頁的品牌名稱。顯示文字由 client 決定，PreviewInvite 的回應不帶。 */
export const VENDOR_PORTAL_BRAND = "Hawkr";

export interface VendorInvitePreview {
	brand: string;
	maskedEmail: string;
	expiresAt: string;
}

export interface VendorRfqReferencePriceRange {
	lowerBoundAmount: string;
	upperBoundAmount: string;
	basisQuantity: string;
	basisUnit: string;
}

export interface VendorRfqRequestedPriceRange {
	currency: string;
	priceComponent: string;
	basisQuantity: string;
	basisUnit: string;
	lowerBoundAmount: string;
	upperBoundAmount: string;
}

/** gateway `VendorPortalRFQDTO` 的 deny-by-default 白名單，就這十三欄。 */
export interface VendorRfqSummary {
	/** 空字串＝這筆 approval 建立於後端加這個欄位之前，當「未知」處理、不得補值。 */
	qualityKnowledgeState: string;
	item: string;
	itemLabel: string;
	quantity: string;
	unit: string;
	packaging: string;
	deliveryCity: string;
	deliveryDate: string;
	responseDeadline: string;
	/** 買家自己打字的補充說明，原文照登。空字串＝沒有。⛔ 不是品相宣告。 */
	buyerNote: string;
	referencePriceRange: VendorRfqReferencePriceRange | null;
	buyerRequestedRange: VendorRfqRequestedPriceRange | null;
	/**
	 * 這張 RFQ 的版本（十進位字串）。正式送出報價時原樣帶回去做 CAS：把報價綁在
	 * 「業者當時看到的那一版」上——業者是從 email 連結進來的，可能隔好幾天才回來填。
	 *
	 * ⛔ 前端**不做任何解讀或比較**，原樣搬運即可。判斷版本有沒有過期是後端的事。
	 */
	publicationVersion: string;
}

/** session 已經用 Google 驗證過信箱的 assurance 值（畫面只比對這個值）。 */
export const VENDOR_ASSURANCE_GOOGLE_VERIFIED = "GOOGLE_VERIFIED";

/**
 * 預覽邀請：secret 放在請求訊息（POST），不進 URL。邀請不可用一律是 `not_found`
 * （ConnectError，Code.NotFound），分不出原因；額度用完是 `resource_exhausted`。
 */
export async function previewVendorInvite(
	secret: string,
): Promise<VendorInvitePreview> {
	const res = await vendorPortalClient().previewInvite({ secret });
	return {
		brand: VENDOR_PORTAL_BRAND,
		maskedEmail: res.maskedEmail,
		expiresAt: res.expireTime
			? timestampDate(res.expireTime).toISOString()
			: "",
	};
}

/**
 * session 的驗證狀態與要交給 Google 登入的 nonce（來自 ExchangeInvite 或 GetSession）。
 * `assurance` 是 `VENDOR_ASSURANCE_GOOGLE_VERIFIED` 或 ""（沒有驗證紀錄、方法是 UNSPECIFIED 或不認得
 * 的值都算沒驗證）。`googleNonce` 空字串＝伺服器沒有給（session 不存在），不可以畫 Google 按鈕。
 */
export interface VendorSessionState {
	assurance: string;
	googleNonce: string;
}

function toSessionState(
	verification: { method: SessionVerificationMethod } | undefined,
	googleNonce: string,
): VendorSessionState {
	return {
		assurance:
			verification?.method === SessionVerificationMethod.GOOGLE
				? VENDOR_ASSURANCE_GOOGLE_VERIFIED
				: "",
		googleNonce,
	};
}

/**
 * 以 secret 換 session：回應的 Set-Cookie 設上這張邀請的 `__Host-hk_vp_<rfq_recipient_id>`
 * （`Path=/`），回應內容是收件人 ID 與 session 的驗證狀態、nonce，session token 不進 JS。
 * 請求已帶這張邀請的有效 session 時沿用它（驗證狀態因此會保留）。
 */
export async function exchangeVendorInvite(
	secret: string,
): Promise<{ rfqRecipientId: string; session: VendorSessionState }> {
	const res = await vendorPortalClient().exchangeInvite({ secret });
	return {
		rfqRecipientId: res.rfqRecipientId,
		session: toSessionState(res.verification, res.googleNonce),
	};
}

/**
 * 讀這張邀請的 session 狀態（GET，收件人在 query）。沒有那張邀請的 cookie 或 session 無效回
 * `unauthenticated`（ConnectError）。
 */
export async function fetchVendorSession(
	rfqRecipientId: string,
): Promise<VendorSessionState> {
	const res = await vendorPortalInviteeClient().getSession({ rfqRecipientId });
	return toSessionState(res.verification, res.googleNonce);
}

/** 讀 RFQ 摘要。身分只靠 session cookie，不再帶 secret。 */
export async function fetchVendorRfq(
	rfqRecipientId: string,
): Promise<VendorRfqSummary> {
	const res = await apiClient.public.get<VendorRfqSummary>(
		`/vendor_portal/rfqs/${encodeURIComponent(rfqRecipientId)}`,
		{ credentials: "include" },
	);
	return res.data;
}

/**
 * 回價草稿的一個版本。`draftVersion` 是 CAS 用的樂觀鎖標記，下一次存回去要原樣帶上。
 */
export interface VendorResponseDraftSnapshot {
	draftVersion: string;
	updatedAt: string;
	draft: VendorResponseDraft;
}

function responseDraftPath(rfqRecipientId: string): string {
	return `/vendor_portal/rfqs/${encodeURIComponent(rfqRecipientId)}/response_draft`;
}

function toDraftSnapshot(data: {
	draftVersion?: unknown;
	updatedAt?: unknown;
	draft?: unknown;
}): VendorResponseDraftSnapshot {
	return {
		draftVersion:
			typeof data.draftVersion === "string" ? data.draftVersion : "",
		updatedAt: typeof data.updatedAt === "string" ? data.updatedAt : "",
		// 補成完整 17 欄再交給畫面：受控欄位吃到 undefined 會在使用者第一次輸入時
		// 從 uncontrolled 跳成 controlled。
		draft: normalizeVendorResponseDraft(data.draft),
	};
}

/**
 * 讀回既有草稿。**沒有草稿時後端回 404**（不是 200 空物件），這裡收斂成 `null`
 * ——「還沒存過」是正常狀態不是錯誤，業者第一次進來本來就沒有。
 *
 * 前提：只在 RFQ 摘要已經讀成功之後才呼叫。同一支 session cookie 剛證明過有效，
 * 所以這裡的 404 只可能是「這筆 RFQ 沒有草稿」，不會是 session 過期。
 */
export async function fetchVendorResponseDraft(
	rfqRecipientId: string,
): Promise<VendorResponseDraftSnapshot | null> {
	try {
		const res = await apiClient.public.get<{
			draftVersion?: unknown;
			updatedAt?: unknown;
			draft?: unknown;
		}>(responseDraftPath(rfqRecipientId), { credentials: "include" });
		return toDraftSnapshot(res.data ?? {});
	} catch (error: unknown) {
		if (error instanceof ApiClientError && error.status === 404) return null;
		throw error;
	}
}

/**
 * 存草稿。⛔ **不帶 `Idempotency-Key`**：它的 CAS 對象是 `expectedDraftVersion`，
 * 重送同一個版本本來就會得到同一個結果，再疊一層冪等鍵只是多一個會不同步的狀態。
 *
 * `expectedDraftVersion` 傳 `""` 代表「我這邊還沒有草稿」。不符會回 409，語意是
 * 「有人（多半是業者自己的另一個分頁）先存過了」——呼叫端要提示重新載入，
 * **不得**自動覆蓋。
 */
export async function saveVendorResponseDraft(
	rfqRecipientId: string,
	expectedDraftVersion: string,
	draft: VendorResponseDraft,
): Promise<VendorResponseDraftSnapshot> {
	const res = await apiClient.public.put<{
		draftVersion?: unknown;
		updatedAt?: unknown;
		draft?: unknown;
	}>(responseDraftPath(rfqRecipientId), {
		json: { expectedDraftVersion, draft },
		credentials: "include",
	});
	return toDraftSnapshot(res.data ?? {});
}

/**
 * Google step-up（`VerifySession`）：以 Google 登入證明持有邀請的信箱，session 從只有邀請升到
 * `GOOGLE_VERIFIED`，之後才能**正式提交**報價（存草稿不需要）。
 *
 * ⛔ 只送 `idToken`，**不送 email**。email 只能由後端從驗過的 token 取出——前端送
 * 的 email 是呼叫端說了算的值，拿它去比對 `invited_email` 等於讓對方自己宣告身分。
 *
 * ⚠️ nonce 由伺服器給（ExchangeInvite／GetSession 的 `googleNonce`），呼叫端交給 GSI；
 * `rfqRecipientId` 指名要驗證哪一張邀請的 session。
 *
 * ⚠️ token 無效、nonce 不符、token 太舊、信箱未驗證、信箱不符、session 已用別的信箱驗證一律回同一個
 * `permission_denied`，分不出原因；沒有 session 或 session 失效回 `unauthenticated`。呼叫端
 * **不要**猜成因再分別顯示——後端刻意不說，是為了不給拿到邀請信但不是
 * 本人的人一個逐項試探的介面。
 */
export async function stepUpVendorPortalGoogle(
	rfqRecipientId: string,
	idToken: string,
): Promise<{ assurance: string }> {
	const res = await vendorPortalInviteeClient().verifySession({
		rfqRecipientId,
		googleIdToken: idToken,
	});
	return {
		assurance: toSessionState(res.verification, "").assurance,
	};
}

export interface VendorSubmittedResponse {
	sellerResponseId: string;
	sellerResponseVersion: string;
	status: string;
	submitted: boolean;
}

/**
 * 正式提交報價。
 *
 * 🔴 與存草稿是**兩件不同的事**：這支產生 immutable 的 `sellerResponseVersion`，
 * 買家接受 exact version 即商務 award ⇒ 送出之後不能「改一下再存」，只能撤回重送。
 *
 * ⚠️ **必須帶 `Idempotency-Key`**（草稿刻意不帶，它的 CAS 對象是 draftVersion）。
 * 網路超時重送若換一把鍵，會產生第二個正式版本——而買家那邊會看到兩筆報價。
 * 鍵由呼叫端在「按下送出的那一刻」產生並固定，重試時沿用同一把。
 *
 * ⛔ **沒有 `expectedSellerFacingPayloadHash`**（賣家版有）。那是我方的內部完整性
 * 摘要，不對站外方暴露 ⇒ 這條 lane 的 CAS 是 version-only。
 */
export async function submitVendorResponse(
	rfqRecipientId: string,
	idempotencyKey: string,
	expected: {
		publicationVersion: string;
		sellerResponseVersion: string;
	},
	draft: VendorResponseDraft,
): Promise<VendorSubmittedResponse> {
	const res = await apiClient.public.post<{
		sellerResponseId?: unknown;
		sellerResponseVersion?: unknown;
		status?: unknown;
	}>(
		`/vendor_portal/rfqs/${encodeURIComponent(rfqRecipientId)}/response_submissions`,
		{
			json: {
				expectedPublicationVersion: expected.publicationVersion,
				expectedSellerResponseVersion: expected.sellerResponseVersion,
				quote: draft,
			},
			headers: { "Idempotency-Key": idempotencyKey },
			credentials: "include",
		},
	);
	return {
		sellerResponseId: String(res.data?.sellerResponseId ?? ""),
		sellerResponseVersion: String(res.data?.sellerResponseVersion ?? ""),
		status: String(res.data?.status ?? ""),
		submitted: true,
	};
}
