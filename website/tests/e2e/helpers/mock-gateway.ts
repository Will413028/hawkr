import type { Page, Route } from "@playwright/test";
import { ACTIVE_ROLE_STORAGE_KEY } from "../../../lib/auth/active-role";
import { signInWithSessionCookie } from "./session-cookie";

/**
 * Server mocks for the e2e.
 *
 * Every server call is intercepted so tests run without a real backend. The
 * signed-in account comes from the monolith's identity Connect API
 * (hawkr.identity.v1alpha1, ADR 2026-10-30-identity-self-hosted-federated-login); the
 * not-yet-migrated REST stays under `/api/v1`. `API` is a `**`-prefixed glob
 * so it matches whether the browser base is the dev same-origin proxy
 * (`/api/v1`) or an absolute origin.
 *
 * The credential is the HttpOnly `__Host-hk_session` cookie: the middleware only lets a
 * guarded path through when it is present, and the browser asks GetSession on every
 * load (it cannot see the cookie). `session: "identity"` therefore sets the cookie
 * (helpers/session-cookie.ts) and answers GetSession with the account; every other
 * mode sets no cookie and answers GetSession `unauthenticated`, like the monolith does
 * for a visitor with no (or a revoked) session.
 */
const API = "**/api/v1";

/**
 * Seed the localStorage active role (the workspace a signed-in browser last
 * used; lib/auth/active-role.ts).
 *
 * Must run before any app code, hence addInitScript rather than an evaluate.
 */
export async function seedStoredActiveRole(
	page: Page,
	role: "buyer" | "seller",
): Promise<void> {
	await page.addInitScript(
		([key, value]) => {
			try {
				const seededKey = `${key}:e2e-seeded`;
				if (sessionStorage.getItem(seededKey) === "1") return;
				localStorage.setItem(key, value);
				sessionStorage.setItem(seededKey, "1");
			} catch {
				// A browser that refuses storage simply has no stored role.
			}
		},
		[ACTIVE_ROLE_STORAGE_KEY, role] as const,
	);
}

export interface MockIdentity {
	/** 帳號 ID：opaque 的字串（GetSession 的 `account.id`）。 */
	accountId: string;
	/** 帳號的角色：有 "seller" 時 GetSession 的 `account.roles` 列 ROLE_SELLER（每個帳號都是買家，"buyer" 只是說明）。 */
	roles: ReadonlyArray<"buyer" | "seller">;
}

/**
 * identity 的 Connect 方法。e2e 的 browser base 是與頁面同源的 `http://localhost:PORT/api/v1`，
 * Connect 打同源的第一方路徑（lib/connect/transport.ts 的 FIRST_PARTY_CONNECT_PATH）：
 * `/api/connect/{package.Service}/{Method}`（NO_SIDE_EFFECTS 的 GetSession 以 GET 呼叫、帶 query）。
 */
function identityMethod(service: string, method: string): RegExp {
	return new RegExp(
		`/api/connect/hawkr\\.identity\\.v1alpha1\\.${service}/${method}(?:\\?|$)`,
	);
}
export const GET_SESSION_URL = identityMethod(
	"IdentityAppService",
	"GetSession",
);

export const LIST_LINKED_IDENTITIES_URL = identityMethod(
	"IdentityAppService",
	"ListLinkedIdentities",
);

/** 這個 URL 是 session 的探測（GetSession）嗎。 */
export function isGetSessionUrl(url: string): boolean {
	return GET_SESSION_URL.test(url);
}

/**
 * proto3 JSON 的 GetSessionResponse（identity.proto）。工作區是 client 的狀態，回應不帶；Connect 也不送 X-Active-Role。
 */
export function sessionJson(identity: MockIdentity) {
	const isSeller = identity.roles.includes("seller");
	return {
		account: {
			id: identity.accountId,
			roles: isSeller ? ["ROLE_SELLER"] : [],
		},
	};
}

/**
 * SellerAppService 的 Connect 方法：與 identity 同樣打 `/api/connect/{package.Service}/{Method}`，
 * NO_SIDE_EFFECTS 的方法（GetMyStore、GetMyKYC）以 GET 呼叫、帶 query。
 */
function sellerMethod(method: string): RegExp {
	return new RegExp(
		`/api/connect/hawkr\\.seller\\.v1alpha1\\.SellerAppService/${method}(?:\\?|$)`,
	);
}
export const GET_MY_STORE_URL = sellerMethod("GetMyStore");
export const GET_MY_KYC_URL = sellerMethod("GetMyKYC");
export const UPDATE_MY_STORE_URL = sellerMethod("UpdateMyStore");
export const GET_MY_DASHBOARD_URL = sellerMethod("GetMyDashboard");
export const LIST_MY_MEASURES_URL = sellerMethod("ListMyMeasures");
export const GET_MY_STORE_OPENING_URL = sellerMethod("GetMyStoreOpening");

/** 健康的店家（營業中、沒有停權）此刻擁有的全部能力（seller.proto 的 SellerCapability）。 */
export const ALL_SELLER_CAPABILITIES = [
	"ACCOUNT_SELF_READ",
	"STOREFRONT_WRITE",
	"PRODUCT_MANAGE",
	"ORDER_FULFILL",
	"ORDER_CONTRACT",
	"MARKETING",
	"CONTENT_CONTRACT",
	"CHAT_SEND",
	"CHAT_READ",
	"NOTIFICATION",
	"ACCOUNT_CLOSE",
	"INVOICE_MANAGE",
	"INVOICE_TAX_REMEDY",
	"SHIPPING_SETUP",
] as const;
export type MockSellerCapability = (typeof ALL_SELLER_CAPABILITIES)[number];

/** 停權時伺服器仍授予的能力（backend `grantedWhenSuspended`）。 */
export const SUSPENDED_SELLER_CAPABILITIES: readonly MockSellerCapability[] = [
	"ACCOUNT_SELF_READ",
	"ORDER_FULFILL",
	"ORDER_CONTRACT",
	"CHAT_SEND",
	"CHAT_READ",
	"NOTIFICATION",
	"CONTENT_CONTRACT",
	"ACCOUNT_CLOSE",
	"INVOICE_TAX_REMEDY",
];

/** 等待刪除時：沒有經營類（店面設定、商品、行銷、發票管理）與關店。 */
export const PENDING_DELETION_SELLER_CAPABILITIES: readonly MockSellerCapability[] =
	[
		"ACCOUNT_SELF_READ",
		"ORDER_FULFILL",
		"ORDER_CONTRACT",
		"CONTENT_CONTRACT",
		"CHAT_SEND",
		"CHAT_READ",
		"NOTIFICATION",
		"INVOICE_TAX_REMEDY",
	];

export type MockSellerRequirement =
	| "STORE_ACTIVE"
	| "NOT_SUSPENDED"
	| "KYC_APPROVED"
	| "STORE_NAME"
	| "LOGO"
	| "TOWN_CODE"
	| "NOT_PENDING_DELETION";

/** SellerRequirement 的數字順序（seller.proto）；伺服器的 `missing` 依它排列。 */
const SELLER_REQUIREMENT_ORDER: readonly MockSellerRequirement[] = [
	"STORE_ACTIVE",
	"NOT_SUSPENDED",
	"KYC_APPROVED",
	"STORE_NAME",
	"LOGO",
	"TOWN_CODE",
	"NOT_PENDING_DELETION",
];

function inRequirementOrder(
	requirements: readonly MockSellerRequirement[],
): MockSellerRequirement[] {
	return [...requirements].sort(
		(a, b) =>
			SELLER_REQUIREMENT_ORDER.indexOf(a) - SELLER_REQUIREMENT_ORDER.indexOf(b),
	);
}

export interface MockStore {
	id: string;
	storeCode?: string;
	storeName?: string;
	description?: string;
	contactPhone?: string;
	contactName?: string;
	contactMobile?: string;
	supportEmail?: string;
	address?: { townCode: string; addressDetail: string };
	prize?: string;
	websiteUrl?: string;
	/** 預設 true（已完成新手引導）。 */
	onboardingCompleted?: boolean;
	status?: "ACTIVE" | "PENDING_DELETION";
	/** 預設 "e1"。 */
	etag?: string;
	logoUriTemplate?: string;
	/** 有生效中的停權措施（`GetMyStoreResponse.suspended`）；原因與期間看 `ListMyMeasures`。 */
	suspended?: boolean;
	/** 啟用的能力；預設是全部（物流建店在缺 KYC_APPROVED 時不啟用）。 */
	capabilities?: readonly MockSellerCapability[];
	/** 店家狀態 PUBLIC_LISTING 還沒滿足的條件；預設沒有缺。 */
	missingRequirements?: readonly MockSellerRequirement[];
}

/**
 * 沒有啟用的能力缺的條件，照伺服器的規則（backend `model.CapabilitiesOf`）：停權缺 NOT_SUSPENDED，物流建店
 * 缺 KYC_APPROVED，等待刪除缺 NOT_PENDING_DELETION；測試單獨拿掉某個能力、其他狀態都正常時，缺 NOT_SUSPENDED
 * 當作代表（契約要求沒有啟用的能力至少缺一項）。
 */
function missingOfInactive(
	store: MockStore,
	capability: MockSellerCapability,
	publicListingMissing: readonly MockSellerRequirement[],
): MockSellerRequirement[] {
	const missing: MockSellerRequirement[] = [];
	if (store.suspended) missing.push("NOT_SUSPENDED");
	if (
		capability === "SHIPPING_SETUP" &&
		publicListingMissing.includes("KYC_APPROVED")
	) {
		missing.push("KYC_APPROVED");
	}
	if (store.status === "PENDING_DELETION") missing.push("NOT_PENDING_DELETION");
	return inRequirementOrder(missing.length > 0 ? missing : ["NOT_SUSPENDED"]);
}

/** proto3 JSON 的 GetMyStoreResponse（seller.proto）。 */
export function myStoreJson(store: MockStore) {
	const missing = inRequirementOrder(store.missingRequirements ?? []);
	const granted = new Set(store.capabilities ?? ALL_SELLER_CAPABILITIES);
	if (missing.includes("KYC_APPROVED")) granted.delete("SHIPPING_SETUP");
	return {
		store: {
			id: store.id,
			...(store.storeCode ? { storeCode: store.storeCode } : {}),
			storeName: store.storeName ?? "",
			description: store.description ?? "",
			contactPhone: store.contactPhone ?? "",
			contactName: store.contactName ?? "",
			contactMobile: store.contactMobile ?? "",
			supportEmail: store.supportEmail ?? "",
			...(store.address ? { address: store.address } : {}),
			prize: store.prize ?? "",
			websiteUrl: store.websiteUrl ?? "",
			onboardingCompleted: store.onboardingCompleted ?? true,
			...(store.logoUriTemplate
				? { logo: { uriTemplate: store.logoUriTemplate } }
				: {}),
			status: `STORE_STATUS_${store.status ?? "ACTIVE"}`,
			etag: store.etag ?? "e1",
		},
		// proto3 JSON 不輸出預設值 false。
		...(store.suspended ? { suspended: true } : {}),
		capabilities: [
			...ALL_SELLER_CAPABILITIES.map((c) => ({
				capability: `SELLER_CAPABILITY_${c}`,
				active: granted.has(c),
				missing: granted.has(c)
					? []
					: missingOfInactive(store, c, missing).map(
							(r) => `SELLER_REQUIREMENT_${r}`,
						),
			})),
			{
				capability: "SELLER_CAPABILITY_PUBLIC_LISTING",
				active: missing.length === 0,
				missing: missing.map((r) => `SELLER_REQUIREMENT_${r}`),
			},
		],
	};
}

export interface MockMeasureFinding {
	id: string;
	/** 政策分類中認定可用的值（common.PolicyCategory 去掉 POLICY_CATEGORY_ 前綴；OTHER 只給檢舉人選，認定沒有）。 */
	category: "FOOD_SAFETY" | "FAKE_CERTIFICATION" | "FRAUD" | "MISLEADING_INFO";
	severity: "LOW" | "MEDIUM" | "HIGH" | "CRITICAL";
	createTime: string;
}

export interface MockMeasureRequest {
	id: string;
	/** 申覆或解除請求（seller.proto MeasureRequest.kind）。 */
	kind: "APPEAL" | "REMEDIATION";
	state: "PENDING" | "ACCEPTED" | "REJECTED" | "CLOSED";
	reason: string;
	submitTime: string;
	decisionReason?: string;
	decideTime?: string;
	/** 只有 state 是 CLOSED 時設（seller.proto MeasureRequest.close_cause）。 */
	closeCause?: "REVOKED" | "ENDED";
}

/** proto3 JSON 的 `seller.v1alpha1.MeasureRequest`；列舉用完整名稱，沒設的欄位不輸出。 */
function measureRequestJson(request: MockMeasureRequest) {
	return {
		id: request.id,
		kind: `MEASURE_REQUEST_KIND_${request.kind}`,
		state: `MEASURE_REQUEST_STATE_${request.state}`,
		reason: request.reason,
		submitTime: request.submitTime,
		...(request.decisionReason
			? { decisionReason: request.decisionReason }
			: {}),
		...(request.decideTime ? { decideTime: request.decideTime } : {}),
		...(request.closeCause
			? { closeCause: `MEASURE_REQUEST_CLOSE_CAUSE_${request.closeCause}` }
			: {}),
	};
}

export interface MockMeasure {
	id: string;
	kind: "WARNING" | "SUSPENSION" | "TERMINATION";
	basis: "VIOLATION" | "RISK" | "CONTRACT";
	findings?: readonly MockMeasureFinding[];
	riskGround?:
		| "TRANSACTION"
		| "FOOD_SAFETY"
		| "FRAUD"
		| "PERSONAL_DATA"
		| "INFRINGEMENT"
		| "SECURITY"
		| "FULFILLMENT"
		| "EXTERNAL_ORDER";
	statement: string;
	state: "ISSUED" | "SCHEDULED" | "ACTIVE" | "ENDED" | "REVOKED";
	/** 警告不設；沒有 endTime 表示由平台定期複審。 */
	period?: { startTime: string; endTime?: string };
	/** 申覆與解除請求的紀錄，新到舊；state 是伺服器算好的生效狀態。 */
	requests?: readonly MockMeasureRequest[];
	createTime: string;
	/** 預設 "m1"。 */
	etag?: string;
	canAppeal: boolean;
	/** 預設 false（proto3 預設值不輸出）。 */
	canRequestRemediation?: boolean;
	/**
	 * 不能請求解除的原因與伺服器組好的句子，依優先序排好；能請求時是空的（proto3 空 repeated 不輸出）。
	 * 形狀是 `RemediationUnavailability { reason, message: google.rpc.LocalizedMessage }`。
	 */
	remediationUnavailabilities?: readonly MockUnavailability<
		| "REMEDIATION_PENDING"
		| "APPEAL_PENDING"
		| "COOLDOWN"
		| "NOT_RISK"
		| "NOT_ACTIVE"
	>[];
	/** 冷卻期的結束時間（RFC 3339）；只在冷卻期內設。 */
	nextRemediationTime?: string;
	/** 下一個請求若被駁回會開始的冷卻期長度（秒）；不設表示沒有冷卻。proto3 JSON 的 Duration 是 "604800s"。 */
	rejectionCooldownSeconds?: number;
	revokeReason?: "APPEAL_ACCEPTED" | "FINDING_REVOKED" | "SOURCE_REVOKED";
	/** 終止合作（kind 是 TERMINATION）的事由；其他種類不設。 */
	terminationGround?:
		| "NOTICE"
		| "MATERIAL_BREACH"
		| "CEASED_BUSINESS"
		| "DISSOLUTION"
		| "BANKRUPTCY"
		| "LICENSE_REVOKED"
		| "FOOD_SAFETY_INCIDENT"
		| "FRAUD"
		| "PLATFORM_SECURITY"
		| "OTHER_SERIOUS"
		| "LINKED_ACCOUNT";
	/** 終止合作的申覆期限（RFC 3339）；其他種類不設。 */
	appealDeadlineTime?: string;
	/**
	 * 不能申覆的原因與伺服器組好的句子，依優先序排好；能申覆時是空的（proto3 空 repeated 不輸出）。
	 * 形狀是 `AppealUnavailability { reason, message: google.rpc.LocalizedMessage }`。
	 */
	appealUnavailabilities?: readonly MockUnavailability<
		"REVOKED" | "ALREADY_APPEALED" | "APPEAL_WINDOW_ENDED" | "REQUEST_PENDING"
	>[];
}

/**
 * 伺服器回的一筆「不能做」的原因（`*Unavailability`）：`reason` 給 client 分支，`message` 是伺服器組好的一句話
 * （`google.rpc.LocalizedMessage`，locale 是實際用的語系）。不給 `message` 就不輸出該欄位（伺服器沒帶句子）。
 */
export interface MockUnavailability<R extends string> {
	reason: R;
	message?: string;
}

/** 轉成 proto3 JSON；`prefix` 是列舉值的完整名稱前綴（例如 `APPEAL_UNAVAILABLE_REASON_`）。 */
function unavailabilityJson<R extends string>(
	prefix: string,
	u: MockUnavailability<R>,
) {
	return {
		reason: `${prefix}${u.reason}`,
		...(u.message === undefined
			? {}
			: { message: { locale: "zh-TW", message: u.message } }),
	};
}

/** proto3 JSON 的 `seller.v1alpha1.Measure`（seller.proto）；列舉用完整名稱，沒設的欄位不輸出。 */
export function measureJson(measure: MockMeasure) {
	return {
		id: measure.id,
		kind: `MEASURE_KIND_${measure.kind}`,
		basis: `MEASURE_BASIS_${measure.basis}`,
		...(measure.findings && measure.findings.length > 0
			? {
					findings: measure.findings.map((finding) => ({
						id: finding.id,
						category: `POLICY_CATEGORY_${finding.category}`,
						severity: `VIOLATION_SEVERITY_${finding.severity}`,
						createTime: finding.createTime,
					})),
				}
			: {}),
		...(measure.riskGround
			? { riskGround: `RISK_GROUND_${measure.riskGround}` }
			: {}),
		...(measure.statement ? { statement: measure.statement } : {}),
		state: `MEASURE_STATE_${measure.state}`,
		...(measure.period ? { period: measure.period } : {}),
		createTime: measure.createTime,
		etag: measure.etag ?? "m1",
		...(measure.canAppeal ? { canAppeal: true } : {}),
		...(measure.revokeReason
			? { revokeReason: `REVOKE_REASON_${measure.revokeReason}` }
			: {}),
		...(measure.requests && measure.requests.length > 0
			? { requests: measure.requests.map(measureRequestJson) }
			: {}),
		...(measure.remediationUnavailabilities &&
		measure.remediationUnavailabilities.length > 0
			? {
					remediationUnavailabilities: measure.remediationUnavailabilities.map(
						(u) => unavailabilityJson("REMEDIATION_UNAVAILABLE_REASON_", u),
					),
				}
			: {}),
		...(measure.canRequestRemediation ? { canRequestRemediation: true } : {}),
		...(measure.nextRemediationTime
			? { nextRemediationTime: measure.nextRemediationTime }
			: {}),
		...(measure.rejectionCooldownSeconds
			? { rejectionCooldown: `${measure.rejectionCooldownSeconds}s` }
			: {}),
		...(measure.terminationGround
			? { terminationGround: `TERMINATION_GROUND_${measure.terminationGround}` }
			: {}),
		...(measure.appealDeadlineTime
			? { appealDeadlineTime: measure.appealDeadlineTime }
			: {}),
		...(measure.appealUnavailabilities &&
		measure.appealUnavailabilities.length > 0
			? {
					appealUnavailabilities: measure.appealUnavailabilities.map((u) =>
						unavailabilityJson("APPEAL_UNAVAILABLE_REASON_", u),
					),
				}
			: {}),
	};
}

export type MockKycFlowStatus =
	| "NOT_STARTED"
	| "DRAFT"
	| "SUBMITTED"
	| "REJECTED"
	| "BINDING"
	| "BIND_FAILED"
	| "FORWARDED"
	| "UNDER_REVIEW"
	| "PENDING_INFO"
	| "APPROVED"
	| "PAYUNI_REJECTED";

/**
 * proto3 JSON 的 GetMyKYCResponse：只放流程狀態（`application.flow`）。NOT_STARTED 在真實契約上是
 * not_found，這裡用同樣的錯誤回應（client 把它讀成「還沒開始」）。
 */
export interface MockSellerPayUni {
	state:
		| "REVIEWING"
		| "PENDING_INFO"
		| "FIXABLE"
		| "FIX_VIA_SUPPORT"
		| "CONTACT_SUPPORT"
		| "RETRY_LATER"
		| "TERMINATED"
		| "APPROVED";
	/** 伺服器的狀態說明（APPROVED 沒有）。 */
	explanation?: string;
	reason?: string;
	fixFields?: string[];
	fixStoreFields?: string[];
}

export async function mockMyKyc(
	page: Page,
	flow: {
		status: MockKycFlowStatus;
		identityLocked?: boolean;
		canResubmit?: boolean;
		payuni?: MockSellerPayUni;
		resubmitUnavailabilities?: { reason: string; message: string }[];
	},
): Promise<void> {
	await page.route(GET_MY_KYC_URL, (route) => {
		if (flow.status === "NOT_STARTED") {
			void fulfillConnectError(route, "not_found", "尚未申請");
			return;
		}
		void fulfillConnect(route, {
			application: {
				id: "1",
				flow: {
					status: `KYC_FLOW_STATUS_${flow.status}`,
					identityLocked: flow.identityLocked ?? false,
					// 伺服器的值：草稿與被退回一律能重送。
					canResubmit:
						flow.canResubmit ??
						(flow.status === "DRAFT" || flow.status === "REJECTED"),
					...(flow.payuni
						? {
								payuni: {
									state: `PAY_UNI_STATE_${flow.payuni.state}`,
									...(flow.payuni.explanation
										? {
												explanation: {
													locale: "zh-TW",
													message: flow.payuni.explanation,
												},
											}
										: {}),
									reason: flow.payuni.reason ?? "",
									fixFields: flow.payuni.fixFields ?? [],
									fixStoreFields: flow.payuni.fixStoreFields ?? [],
								},
							}
						: {}),
					resubmitUnavailabilities: (flow.resubmitUnavailabilities ?? []).map(
						(u) => ({
							reason: `KYC_RESUBMIT_UNAVAILABLE_REASON_${u.reason}`,
							message: { locale: "zh-TW", message: u.message },
						}),
					),
				},
			},
		});
	});
}

/**
 * proto3 JSON 的 `Dashboard`：int64 是字串、沒有給的區塊就是沒設（不是 0）。
 * 期間固定為 2026-09-30 至 2026-10-06（前期 9/23 至 9/29）。
 */
export interface MockDashboard {
	revenue?: { current: number; previous: number };
	productViews?: { current: number; previous: number };
	newFollowers?: { current: number; previous: number };
	orderStatusCounts?: {
		pendingShipmentCount: number;
		inProgressCount: number;
		closedCount: number;
		aftersalesCount: number;
	};
}

function dashboardJson(d: MockDashboard) {
	return {
		dashboard: {
			currentPeriod: {
				firstDate: { year: 2026, month: 9, day: 30 },
				lastDate: { year: 2026, month: 10, day: 6 },
			},
			previousPeriod: {
				firstDate: { year: 2026, month: 9, day: 23 },
				lastDate: { year: 2026, month: 9, day: 29 },
			},
			...(d.revenue
				? {
						sales: {
							revenue: {
								current: String(d.revenue.current),
								previous: String(d.revenue.previous),
							},
						},
					}
				: {}),
			...(d.newFollowers
				? { followers: { total: 0, newFollowers: d.newFollowers } }
				: {}),
			...(d.productViews ? { productViews: d.productViews } : {}),
			...(d.orderStatusCounts
				? { orderStatusCounts: d.orderStatusCounts }
				: {}),
		},
	};
}

/** 以固定的儀表板回應 GetMyDashboard。 */
export async function mockMyDashboard(
	page: Page,
	dashboard: MockDashboard,
): Promise<void> {
	await page.route(GET_MY_DASHBOARD_URL, (route) => {
		void fulfillConnect(route, dashboardJson(dashboard));
	});
}

/** ListMyMeasures 的 `state` 篩選：NO_SIDE_EFFECTS 以 GET 帶 query `message`（JSON），也接受 POST 的 JSON body。 */
function listMeasuresStateOf(route: Route): string {
	const request = route.request();
	let raw: string | null = null;
	if (request.method() === "GET") {
		raw = new URL(request.url()).searchParams.get("message");
	} else {
		raw = request.postData();
	}
	if (!raw) return "MEASURE_STATE_UNSPECIFIED";
	const parsed = JSON.parse(raw) as { state?: string };
	return parsed.state ?? "MEASURE_STATE_UNSPECIFIED";
}

export interface MeasuresMock {
	/** 換掉之後讀到的措施（已載入的頁面要重新整理或重新查詢才會看到）。 */
	set: (measures: readonly MockMeasure[]) => void;
	/** 每次 ListMyMeasures 帶的 `state`（完整列舉名）。 */
	readonly requestedStates: () => readonly string[];
}

/**
 * ListMyMeasures：預設空；依請求的 `state` 篩選（UNSPECIFIED 不篩），與伺服器一樣只回狀態相符的措施。
 * 後註冊的 route 會蓋過 setupGatewayMocks 的預設（空）。
 */
export async function mockMyMeasures(
	page: Page,
	initial: readonly MockMeasure[] = [],
): Promise<MeasuresMock> {
	let measures = initial;
	const states: string[] = [];
	await page.route(LIST_MY_MEASURES_URL, (route) => {
		const state = listMeasuresStateOf(route);
		states.push(state);
		const matched = measures.filter(
			(m) =>
				state === "MEASURE_STATE_UNSPECIFIED" ||
				`MEASURE_STATE_${m.state}` === state,
		);
		void fulfillConnect(route, { measures: matched.map(measureJson) });
	});
	return {
		set: (next) => {
			measures = next;
		},
		requestedStates: () => states,
	};
}

/** seller.proto 的 StoreOpeningUnavailableReason（proto3 JSON 用完整列舉名）。 */
export type MockStoreOpeningReason =
	| "TERMINATED"
	| "SUSPENDED"
	| "ACCOUNT_DELETING"
	| "STORE_EXISTS";

/**
 * GetMyStoreOpening（NO_SIDE_EFFECTS，GET）：預設可以開店（can_apply_store=true、沒有原因）；給原因就是不能開店，
 * 原因與伺服器組好的句子照給的順序（伺服器已排好優先序）。形狀是 `StoreOpeningUnavailability { reason, message }`。
 * proto3 JSON 省略預設值：false 的 canApplyStore 與空陣列都不出現。後註冊的 route 會蓋過 setupGatewayMocks 的預設。
 */
export async function mockStoreOpening(
	page: Page,
	unavailabilities: readonly MockUnavailability<MockStoreOpeningReason>[] = [],
): Promise<{ readonly getCount: () => number }> {
	let count = 0;
	await page.route(GET_MY_STORE_OPENING_URL, (route) => {
		count += 1;
		void fulfillConnect(
			route,
			unavailabilities.length === 0
				? { canApplyStore: true }
				: {
						unavailabilities: unavailabilities.map((u) =>
							unavailabilityJson("STORE_OPENING_UNAVAILABLE_REASON_", u),
						),
					},
		);
	});
	return { getCount: () => count };
}

/** 以固定的店家回應 GetMyStore；回傳目前的 GetMyStore 呼叫次數。 */
export async function mockMyStore(
	page: Page,
	store: MockStore,
): Promise<{ readonly getCount: () => number }> {
	let count = 0;
	await page.route(GET_MY_STORE_URL, (route) => {
		count += 1;
		void fulfillConnect(route, myStoreJson(store));
	});
	return { getCount: () => count };
}

const CONNECT_ERROR_STATUS = {
	invalid_argument: 400,
	failed_precondition: 400,
	unauthenticated: 401,
	permission_denied: 403,
	not_found: 404,
	already_exists: 409,
	aborted: 409,
	resource_exhausted: 429,
	internal: 500,
	unavailable: 503,
} as const;

/** 單一長度前綴字串欄位的 protobuf 編碼（欄位號小於 16、字串短於 128 位元組）。 */
function stringField(fieldNumber: number, text: string): Buffer {
	return Buffer.concat([
		Buffer.from([(fieldNumber << 3) | 2, Buffer.byteLength(text)]),
		Buffer.from(text),
	]);
}

function errorDetail(type: string, ...fields: Buffer[]) {
	return {
		type,
		value: Buffer.concat(fields).toString("base64").replace(/=+$/, ""),
	};
}

/**
 * `google.rpc.ErrorInfo{reason, domain: "hawkr"}` 的 Connect JSON 錯誤細節（`type` 加 base64 的
 * protobuf 編碼）。reason 是 `ErrorReason` 的名稱（error_reason.proto）。
 */
export function errorInfoDetail(reason: string) {
	return errorDetail(
		"google.rpc.ErrorInfo",
		stringField(1, reason),
		stringField(2, "hawkr"),
	);
}

/** `google.rpc.LocalizedMessage{locale, message}`：伺服器給使用者看的文字。 */
export function localizedMessageDetail(message: string) {
	return errorDetail(
		"google.rpc.LocalizedMessage",
		stringField(1, "zh-TW"),
		stringField(2, message),
	);
}

/**
 * BuyerAppService 的 Connect 方法：與 identity 同樣打 `/api/connect/{package.Service}/{Method}`，
 * NO_SIDE_EFFECTS 的方法（Get*、List*）以 GET 呼叫、帶 query。
 */
function buyerMethod(method: string): RegExp {
	return new RegExp(
		`/api/connect/hawkr\\.buyer\\.v1alpha1\\.BuyerAppService/${method}(?:\\?|$)`,
	);
}
export const GET_PROFILE_URL = buyerMethod("GetProfile");
export const LIST_ADDRESSES_URL = buyerMethod("ListAddresses");

/** proto3 JSON 的買家 `Address`（buyer.proto）；`townCode` 是內政部 8 碼鄉鎮市區代碼。 */
export interface MockBuyerAddress {
	id: string;
	receiverName: string;
	receiverPhone: string;
	townCode: string;
	addressDetail: string;
	defaultAddress?: boolean;
}

/** 以固定的收貨地址清單回應 ListAddresses（單頁，不帶 next_page_token）。 */
export async function mockBuyerAddresses(
	page: Page,
	addresses: readonly MockBuyerAddress[],
): Promise<void> {
	await page.route(LIST_ADDRESSES_URL, (route) => {
		void fulfillConnect(route, {
			addresses: addresses.map((a) => ({
				id: a.id,
				receiverName: a.receiverName,
				receiverPhone: a.receiverPhone,
				postalAddress: {
					townCode: a.townCode,
					addressDetail: a.addressDetail,
				},
				defaultAddress: a.defaultAddress ?? false,
				etag: `etag-${a.id}`,
			})),
			nextPageToken: "",
		});
	});
}

/** 店家卡片（common.v1alpha1.StoreCard）的 JSON 線上形狀；status 用列舉名稱字串。 */
export interface MockStoreCard {
	id: string;
	storeCode?: string;
	name?: string;
	status?:
		| "STORE_CARD_STATUS_OPEN"
		| "STORE_CARD_STATUS_PAUSED"
		| "STORE_CARD_STATUS_CLOSED";
}

/**
 * 公開的店家卡片讀取（SellerPublicService.BatchGetStores 與 DiscoveryPublicService.SuggestRecommendedSellers）：
 * 預設都沒有卡片，沒有被個別 mock 蓋過時不會打到沒有後端的同源代理而 502。
 * BatchGetStores 是 NO_SIDE_EFFECTS 的 GET，ids 在 query 的 `message`（JSON）裡；只回請求到的、有給卡片的店，
 * 與伺服器「沒有卡片的店不在結果裡」一致。較晚註冊的 route 會蓋過這裡。
 */
export async function mockStoreCards(
	page: Page,
	cards: readonly MockStoreCard[] = [],
) {
	await page.route(
		/\/api\/connect\/hawkr\.seller\.v1alpha1\.SellerPublicService\/BatchGetStores(?:\?|$)/,
		(route) => {
			const raw = new URL(route.request().url()).searchParams.get("message");
			const ids: string[] = raw ? (JSON.parse(raw).ids ?? []) : [];
			void fulfillConnect(route, {
				stores: cards.filter((card) => ids.includes(card.id)),
			});
		},
	);
	await page.route(
		/\/api\/connect\/hawkr\.discovery\.v1alpha1\.DiscoveryPublicService\/SuggestRecommendedSellers(?:\?|$)/,
		(route) => {
			void fulfillConnect(route, { sellers: [] });
		},
	);
}

/**
 * 登入頁面共用的買家資料讀取（個人資料與帳號選單、收藏、追蹤、造訪、會員卡、地址）：
 * 預設都回空清單，沒有被個別 mock 蓋過時不會打到沒有後端的同源代理而 502。
 * 較晚註冊的 route 會蓋過這裡。
 */
async function mockBuyerDefaults(page: Page, accountId: string) {
	await page.route(GET_PROFILE_URL, (route) => {
		void fulfillConnect(route, {
			profile: { id: accountId, displayName: "測試買家", etag: "etag-profile" },
		});
	});
	const empty: ReadonlyArray<readonly [string, Record<string, unknown>]> = [
		["ListAddresses", { addresses: [], nextPageToken: "" }],
		["ListMemberCards", { memberCards: [], nextPageToken: "" }],
		["ListFavorites", { favorites: [], nextPageToken: "", totalSize: 0 }],
		["ListFollows", { follows: [], nextPageToken: "", totalSize: 0 }],
		["ListVisits", { visits: [], nextPageToken: "" }],
	];
	for (const [method, body] of empty) {
		await page.route(buyerMethod(method), (route) => {
			void fulfillConnect(route, body);
		});
	}
}

/** 以 Connect 的錯誤（JSON）回應：HTTP 狀態碼依 code，body 是 `{code, message}`（加上 details）。 */
export function fulfillConnectError(
	route: Route,
	code: keyof typeof CONNECT_ERROR_STATUS,
	message: string = code,
	headers?: Record<string, string>,
	details?: ReadonlyArray<{ type: string; value: string }>,
): Promise<void> {
	return route.fulfill({
		status: CONNECT_ERROR_STATUS[code],
		contentType: "application/json",
		headers,
		body: JSON.stringify({ code, message, ...(details ? { details } : {}) }),
	});
}

/** 以 Connect 的成功（JSON）回應。 */
export function fulfillConnect(
	route: Route,
	body: unknown = {},
	headers?: Record<string, string>,
): Promise<void> {
	return route.fulfill({
		status: 200,
		contentType: "application/json",
		headers,
		body: JSON.stringify(body),
	});
}

export interface GatewayMockOptions {
	/** Account GetSession answers with. */
	identity?: MockIdentity;
	/**
	 * GetSession's answer. "identity" sets the session cookie and answers with
	 * `identity`; default (omitted) and "guest" set no cookie and answer
	 * unauthenticated (a visitor, or a session the monolith refuses).
	 */
	session?: "identity" | "guest";
	/**
	 * 賣家登入後的開店判斷：App 讀 GetMyStore。設定時以它 mock 那支（true → 店家還沒完成新手引導，
	 * false → 已完成）；呼叫端自己註冊的 GetMyStore route 較晚加入，會蓋過這裡。
	 */
	needsOnboarding?: boolean;
	/** GetMyStoreOpening 的不能開店原因與伺服器句子（依序）；預設空 = can_apply_store 為 true。 */
	storeOpeningUnavailabilities?: readonly MockUnavailability<MockStoreOpeningReason>[];
	/** ListMyMeasures 的措施；預設沒有。要中途換用 `mockMyMeasures`。 */
	measures?: readonly MockMeasure[];
	/** HTTP status for the generic /app/** catch-all. Default 200; use 401 to drive AUTH_EXPIRED. */
	appStatus?: number;
}

export interface GatewayMockState {
	/** X-Active-Role seen on the most recent generic /app/** request. */
	appActiveRole: string | null;
}

/**
 * Intercept the server. Route order matters: Playwright runs the LAST-added
 * matching handler first, so the generic /app/** catch-all is registered FIRST
 * and the specific routes AFTER it (they win for exact paths).
 */
export async function setupGatewayMocks(
	page: Page,
	opts: GatewayMockOptions = {},
): Promise<GatewayMockState> {
	const identity: MockIdentity = opts.identity ?? {
		accountId: "1",
		roles: ["buyer"],
	};
	const appStatus = opts.appStatus ?? 200;
	const state: GatewayMockState = { appActiveRole: null };

	// 1. Generic authed catch-all (registered first → lowest priority).
	await page.route(`${API}/app/**`, (route) => {
		state.appActiveRole = route.request().headers()["x-active-role"] ?? null;
		route.fulfill({
			status: appStatus,
			contentType: "application/json",
			body: JSON.stringify(
				appStatus === 401
					? { code: 401, message: "missing or invalid credentials" }
					: { code: 0, data: {} },
			),
		});
	});

	// 2. 賣家登入後的開店判斷（GetMyStore）。
	if (opts.needsOnboarding !== undefined) {
		const needsOnboarding = opts.needsOnboarding;
		await page.route(GET_MY_STORE_URL, (route) => {
			void fulfillConnect(
				route,
				myStoreJson({
					id: identity.accountId,
					onboardingCompleted: !needsOnboarding,
				}),
			);
		});
	}

	// 2a. 買家資料與賣家儀表板的讀取：預設回空，個別 mock 較晚註冊會蓋過。
	await mockBuyerDefaults(page, identity.accountId);
	await mockStoreCards(page);
	await mockMyDashboard(page, {});

	// 2b. 措施與申覆（ListMyMeasures）：預設空。
	await mockMyMeasures(page, opts.measures ?? []);

	// 2c. 開店可用性（GetMyStoreOpening）：預設可以開店。
	await mockStoreOpening(page, opts.storeOpeningUnavailabilities ?? []);

	// 2d. 已連結的登入方式（ListLinkedIdentities，設定頁的「登入方式」）：預設只連結 Google。
	await page.route(LIST_LINKED_IDENTITIES_URL, (route) => {
		void fulfillConnect(route, {
			identities: [{ provider: "LOGIN_PROVIDER_GOOGLE" }],
		});
	});

	// 3. The signed-in account (GetSession), and the session cookie behind it.
	if (opts.session === "identity") {
		await signInWithSessionCookie(page, identity.accountId);
	}
	await page.route(GET_SESSION_URL, (route) => {
		if (opts.session !== "identity") {
			void fulfillConnectError(route, "unauthenticated");
			return;
		}
		void fulfillConnect(route, sessionJson(identity));
	});

	return state;
}
