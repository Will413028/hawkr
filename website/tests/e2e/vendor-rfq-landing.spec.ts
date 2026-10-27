import { expect, type Page, type Route, test } from "@playwright/test";
import { fulfillConnect, fulfillConnectError } from "./helpers/mock-gateway";
import { installProviderMocks } from "./helpers/mock-gsi";

// Mocked gateway only (page.route intercepts every call) — hermetic, `@ui`.
// preview／exchange／step-up 的契約來源是 `api/hawkr/agent/v1alpha1/agent.proto`
// （VendorPortalPublicService，Connect JSON）；RFQ 與草稿還是舊 gateway 的 REST。
const SECRET = "a".repeat(64);
const RECIPIENT_ID = "843856384063471755";

const PREVIEW_BODY = {
	maskedEmail: "we***@example.com",
	expireTime: "2026-09-19T04:12:07Z",
};

const PUBLIC_SERVICE = "hawkr.agent.v1alpha1.VendorPortalPublicService";
const INVITEE_SERVICE = "hawkr.agent.v1alpha1.VendorPortalInviteeService";

type VendorMethod =
	| "PreviewInvite"
	| "ExchangeInvite"
	| "GetSession"
	| "VerifySession";

const SERVICE_OF: Record<VendorMethod, string> = {
	PreviewInvite: PUBLIC_SERVICE,
	ExchangeInvite: PUBLIC_SERVICE,
	GetSession: INVITEE_SERVICE,
	VerifySession: INVITEE_SERVICE,
};

/** 沒標 NO_SIDE_EFFECTS 的方法 POST、憑證在訊息裡；GetSession 是 GET、訊息在 query。 */
const HTTP_METHOD_OF: Record<VendorMethod, string> = {
	PreviewInvite: "POST",
	ExchangeInvite: "POST",
	GetSession: "GET",
	VerifySession: "POST",
};

/** 這張邀請的 session nonce（伺服器給，交給 GSI）。 */
const NONCE = "nonce-for-session-A";

/** 兌換發的 session cookie：每張邀請一個，`__Host-` 前綴要求 Secure、`Path=/`、沒有 Domain。 */
function sessionCookie(rfqRecipientId: string): string {
	return `__Host-hk_vp_${rfqRecipientId}=opaque; Path=/; Secure; HttpOnly; SameSite=Lax`;
}

/** 兌換之後（或重新整理、書籤）的頁面網址：帶收件人 ID。 */
const ENTRY_URL = `/vendor/rfq?r=${RECIPIENT_ID}`;

interface VendorCall {
	method: string;
	url: string;
	body: Record<string, unknown>;
}

/**
 * 攔 VendorPortal 兩個 service 的一個方法，並記下實際送出的請求。
 * 形狀由 `expectVendorWire` 斷言：HTTP method、路徑是 `/<service>/<method>`、
 * 憑證（secret／Google token）在訊息裡而不在 URL。
 */
async function routeVendor(
	page: Page,
	method: VendorMethod,
	respond: (route: Route, call: VendorCall) => Promise<void>,
): Promise<VendorCall[]> {
	const calls: VendorCall[] = [];
	await page.route(`**/${SERVICE_OF[method]}/${method}*`, (route) => {
		const req = route.request();
		const message = new URL(req.url()).searchParams.get("message");
		const call: VendorCall = {
			method: req.method(),
			url: req.url(),
			body:
				req.method() === "GET"
					? JSON.parse(message ?? "{}")
					: (req.postDataJSON() ?? {}),
		};
		calls.push(call);
		return respond(route, call);
	});
	return calls;
}

function expectVendorWire(
	call: VendorCall | undefined,
	method: VendorMethod,
	body: Record<string, unknown>,
): void {
	expect(call).toBeDefined();
	if (!call) return;
	const url = new URL(call.url);
	expect(call.method).toBe(HTTP_METHOD_OF[method]);
	expect(url.pathname.endsWith(`/${SERVICE_OF[method]}/${method}`)).toBe(true);
	if (call.method === "POST") expect(url.search).toBe("");
	expect(call.url).not.toContain(SECRET);
	expect(call.url).not.toContain("vendor-id-token");
	expect(call.body).toEqual(body);
}

const UNVERIFIED_SESSION = { googleNonce: NONCE };

const VERIFIED_SESSION = {
	googleNonce: NONCE,
	verification: {
		method: "SESSION_VERIFICATION_METHOD_GOOGLE",
		verifyTime: "2026-09-19T04:12:07Z",
	},
};

/** 這張邀請已有有效 session：GetSession 回給定的狀態（預設未驗證）。 */
async function routeInviteeSession(
	page: Page,
	session: Record<string, unknown> = UNVERIFIED_SESSION,
) {
	return routeVendor(page, "GetSession", (route) =>
		fulfillConnect(route, session),
	);
}

/** preview 成功、exchange 發 session cookie 的標準兩步。 */
async function routeInviteHappyPath(page: Page) {
	const preview = await routeVendor(page, "PreviewInvite", (route) =>
		fulfillConnect(route, PREVIEW_BODY),
	);
	const exchange = await routeVendor(page, "ExchangeInvite", (route) =>
		fulfillConnect(
			route,
			{ rfqRecipientId: RECIPIENT_ID, googleNonce: NONCE },
			{ "set-cookie": sessionCookie(RECIPIENT_ID) },
		),
	);
	return { preview, exchange };
}

const VERIFIED_BODY = {
	rfqRecipientId: RECIPIENT_ID,
	verification: VERIFIED_SESSION.verification,
};

const RFQ_BODY = {
	code: 0,
	message: "success",
	data: {
		qualityKnowledgeState: "BUYER_QUALITY_UNKNOWN",
		// 白名單第 13 欄：正式送出時原樣帶回去做 version-only CAS。
		publicationVersion: "7",
		item: "LETTUCE",
		itemLabel: "結球萵苣",
		quantity: "300",
		unit: "KG",
		packaging: "紙箱",
		deliveryCity: "台中市",
		deliveryDate: "2026-09-20",
		responseDeadline: "2026-09-18",
		buyerNote: "",
		referencePriceRange: {
			lowerBoundAmount: "32000",
			upperBoundAmount: "40000",
			basisQuantity: "1",
			basisUnit: "BOX",
		},
		buyerRequestedRange: {
			currency: "TWD",
			priceComponent: "GOODS_ONLY",
			basisQuantity: "1",
			basisUnit: "BOX",
			lowerBoundAmount: "38000",
			upperBoundAmount: "43000",
		},
	},
};

const DRAFT_ROUTE = "**/api/v1/public/vendor_portal/rfqs/*/response_draft";

/**
 * 從 `hawkr_seller`
 * `lib/features/procurement/presentation/seller_quote_validation.dart:91` 的
 * `isRfc3339()` 逐字抄過來。這一頁存的草稿就是那次正式提交的預填，格式不同的話
 * 業者存的東西永遠變不成合法提交。
 */
const SELLER_APP_RFC3339 =
	/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$/;

const SAVED_DRAFT = {
	responseType: "MANUAL_QUOTE",
	offeredQuality: { grade: "特級", inspectionNotes: "抽驗 3 箱" },
	unitBasis: { quantity: "1", unit: "KG" },
	offeredQuantity: "300",
	offeredQuantityUnit: "KG",
	conversionBasis: "1 箱 = 10 KG",
	currency: "TWD",
	goodsAmount: "30000",
	shippingAmount: "800",
	taxAmount: "1540",
	taxInclusionBasis: "TAX_EXCLUDED",
	totalAmount: "32340",
	leadTimeDays: "3",
	promisedDeliveryAt: "2026-09-20T09:00:00+08:00",
	validUntil: "2026-09-18T16:00:00+08:00",
	termsNote: "雨天順延一日",
	deviationDisclosure: "包裝改用塑膠籃",
};

/**
 * 還沒存過草稿。契約是 404 而不是 200 空物件，所以「第一次進來」這條路本身就要
 * 被當成正常狀態走一遍——把它當錯誤處理的話，業者第一眼看到的會是一則警告。
 *
 * 一律在 `rfqs/*` 之後註冊：Playwright 由後往前比對，這樣就算 glob 的 `*` 在某
 * 個版本吃掉了 `/`，草稿這條仍然先命中。
 */
async function routeDraftAbsent(page: Page): Promise<void> {
	await page.route(DRAFT_ROUTE, (route) =>
		route.fulfill({
			status: 404,
			contentType: "application/json",
			body: JSON.stringify({ code: 404, message: "draft not found" }),
		}),
	);
}

test.describe("站外業者 RFQ 落地頁", { tag: ["@ui"] }, () => {
	// 刻意把瀏覽器釘在**非台北**的時區。這一頁的時間一律以台北牆上時間呈現
	// （`lib/vendor-portal/draft.ts` 走 `taipeiParts()` 而不是 `getHours` 家族），
	// 所以下面對日期欄的斷言在這個設定下仍然要看到台北的鐘點——釘成 Asia/Taipei
	// 反而會讓「有人改回跟著裝置時區跑」這種回歸完全看不出來。
	test.use({ timezoneId: "America/New_York" });

	test.skip(
		({ browserName }) => browserName !== "chromium",
		"single representative browser is enough for this mocked-gateway flow",
	);

	test("帶邀請連結進來只預覽，不發 session", async ({ page }) => {
		const { preview, exchange } = await routeInviteHappyPath(page);

		await page.goto(`/vendor/rfq#invite=${SECRET}`);

		await expect(page.getByText("we***@example.com")).toBeVisible();
		expect(preview).toHaveLength(1);
		expectVendorWire(preview[0], "PreviewInvite", { secret: SECRET });
		// session 必須由人點擊才發——email 掃描器與瀏覽器 prerender 打到落地頁時不能替業者兌換。
		expect(exchange).toHaveLength(0);
	});

	test("點「查看詢價內容」才兌換，成功後八欄落地且 secret 從網址消失", async ({
		page,
	}) => {
		const { exchange } = await routeInviteHappyPath(page);
		await page.route("**/api/v1/public/vendor_portal/rfqs/*", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify(RFQ_BODY),
			}),
		);
		await routeDraftAbsent(page);

		await page.goto(`/vendor/rfq#invite=${SECRET}`);
		await page.getByRole("button", { name: "查看詢價內容" }).click();

		await expect(page.getByText("結球萵苣")).toBeVisible();
		await expect(page.getByText("台中市")).toBeVisible();
		expect(exchange).toHaveLength(1);
		expectVendorWire(exchange[0], "ExchangeInvite", { secret: SECRET });
		// secret 留在網址上會隨截圖、分享與瀏覽器歷史外流，兌換完就該清掉。
		expect(page.url()).not.toContain("invite=");
	});

	test("沒有邀請連結時不打任何端點，只顯示連結無效", async ({ page }) => {
		let apiCalls = 0;
		for (const glob of [
			"**/api/v1/public/vendor_portal/**",
			`**/${PUBLIC_SERVICE}/**`,
			`**/${INVITEE_SERVICE}/**`,
		]) {
			await page.route(glob, () => {
				apiCalls += 1;
				throw new Error("no endpoint may be called without an invite");
			});
		}

		await page.goto("/vendor/rfq");

		await expect(page.getByText("連結無效")).toBeVisible();
		expect(apiCalls).toBe(0);
	});

	test("草稿可存；正式提交在完成 Google 驗證之前不得出現", async ({ page }) => {
		await routeInviteeSession(page);
		await page.route("**/api/v1/public/vendor_portal/rfqs/*", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify(RFQ_BODY),
			}),
		);
		await routeDraftAbsent(page);

		await page.goto(ENTRY_URL);

		await expect(page.getByText("結球萵苣")).toBeVisible();
		// 存草稿只要 INVITE_ONLY，業者點邀請進來就有——這一塊是可以填的。
		await expect(page.getByLabel("貨款金額")).toBeEditable();
		await expect(page.getByRole("button", { name: "儲存草稿" })).toBeEnabled();
		// 🔴 建立不可變的正式回價版本要 GOOGLE_VERIFIED。在那之前「正式提交報價」
		// 這顆按鈕**不該存在**——先前它是畫出來但 disabled，那個形狀會讓業者一直找
		// 「為什麼按不下去」；現在改成該位置直接放驗證入口，路徑是明確的。
		await expect(
			page.getByRole("button", { name: "正式提交報價" }),
		).toHaveCount(0);
		await expect(
			page.getByText("送出前請先用收到邀請信的那個 Google 帳號完成驗證"),
		).toBeVisible();
		await expect(
			page.getByText(
				"要讓買家看到、可以成交，請在下方完成 Google 帳號驗證後正式提交",
			),
		).toBeVisible();
		// 第 3 格「正式提交」不得因為中欄變成表單就跟著點亮。
		// 斷言綁在那一格的容器上：第 3～7 格的狀態字完全相同，在整頁上找「尚未開始」
		// 只會證明「至少有一格是」，第 3 格被改成別的字它照樣會綠。
		const submitStep = page.getByTestId("progress-step-3");
		await expect(submitStep).toContainText("正式提交");
		await expect(submitStep).toContainText("尚未開始");
		// V.18 的誠實性紀律，稿上逐字寫在右欄。
		await expect(page.getByText("供應商回價不是成交")).toBeVisible();
	});

	test("開頁把既有草稿帶回來預填，業者關掉分頁再回來看得到自己填過的東西", async ({
		page,
	}) => {
		await routeInviteeSession(page);
		await page.route("**/api/v1/public/vendor_portal/rfqs/*", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify(RFQ_BODY),
			}),
		);
		await page.route(DRAFT_ROUTE, (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					message: "success",
					data: {
						draftVersion: "3",
						updatedAt: "2026-09-13T15:08:59Z",
						draft: SAVED_DRAFT,
					},
				}),
			}),
		);

		await page.goto(ENTRY_URL);

		await expect(page.getByLabel("貨款金額")).toHaveValue("30000");
		await expect(page.getByLabel("品相等級")).toHaveValue("特級");
		await expect(page.getByLabel("條款備註")).toHaveValue("雨天順延一日");
		// 瀏覽器被釘在紐約（-04:00），這兩欄仍然要顯示台北的鐘點。跟著裝置時區跑
		// 的話這裡會變成 04:00／21:00。
		await expect(page.getByLabel("報價有效期限")).toHaveValue(
			"2026-09-18T16:00",
		);
		await expect(page.getByLabel("承諾送達時間")).toHaveValue(
			"2026-09-20T09:00",
		);
		await expect(page.getByText("已帶回您上次儲存的草稿")).toBeVisible();
		// 已經有草稿了，進度軌第 2 格要說得出來。
		await expect(page.getByTestId("progress-step-2")).toContainText("已存草稿");
	});

	test("存草稿帶 expectedDraftVersion、不帶冪等鍵，成功後明說尚未正式提交", async ({
		page,
	}) => {
		// 收進陣列而不是單一變數：只在 callback 裡賦值的 `let` 會被 TS 窄化成
		// `null`，後面的屬性存取就變成 never。
		const puts: Array<{
			expectedDraftVersion?: string;
			draft?: Record<string, unknown>;
		}> = [];
		let idempotencyKey: string | undefined;
		await routeInviteeSession(page);
		await page.route("**/api/v1/public/vendor_portal/rfqs/*", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify(RFQ_BODY),
			}),
		);
		await page.route(DRAFT_ROUTE, (route) => {
			if (route.request().method() !== "PUT") {
				return route.fulfill({
					status: 404,
					contentType: "application/json",
					body: JSON.stringify({ code: 404, message: "draft not found" }),
				});
			}
			puts.push(route.request().postDataJSON());
			idempotencyKey = route.request().headers()["idempotency-key"];
			return route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					message: "success",
					data: {
						draftVersion: "1",
						updatedAt: "2026-09-13T15:08:59Z",
						draft: { ...SAVED_DRAFT, goodsAmount: "12345" },
					},
				}),
			});
		});

		await page.goto(ENTRY_URL);
		await page.getByLabel("貨款金額").fill("12345");
		await page.getByRole("button", { name: "儲存草稿" }).click();

		await expect(page.getByText("已儲存草稿，尚未正式提交")).toBeVisible();
		// 第一次存草稿帶空字串版本，不是省略這個欄位——後端的 CAS 靠它分辨
		// 「我還沒有草稿」與「我以為版本是 X」。
		expect(puts).toHaveLength(1);
		expect(puts[0]?.expectedDraftVersion).toBe("");
		expect(puts[0]?.draft?.goodsAmount).toBe("12345");
		// 草稿的 CAS 對象就是 expectedDraftVersion，不該再疊一個冪等鍵。
		expect(idempotencyKey).toBeUndefined();
		// 🔴 存了草稿**不會**讓正式提交變可用：兩者的 assurance 需求不同，存草稿只要
		// INVITE_ONLY。少了這條，把 step-up 判斷寫成「存過草稿就算數」也會全綠。
		await expect(
			page.getByRole("button", { name: "正式提交報價" }),
		).toHaveCount(0);
		// 步驟軌第 3 格同樣不准因為存了草稿就點亮——草稿買家看不到，點亮等於告訴
		// 業者他送出了，他會一直等一個不會來的回應。
		await expect(page.getByTestId("progress-step-3")).toContainText("尚未開始");
	});

	test("日期選擇器產出的字串，就是平台賣家 App 收的那個 RFC3339", async ({
		page,
	}) => {
		const puts: Array<{ draft?: Record<string, unknown> }> = [];
		await routeInviteeSession(page);
		await page.route("**/api/v1/public/vendor_portal/rfqs/*", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify(RFQ_BODY),
			}),
		);
		await page.route(DRAFT_ROUTE, (route) => {
			if (route.request().method() !== "PUT") {
				return route.fulfill({
					status: 404,
					contentType: "application/json",
					body: JSON.stringify({ code: 404, message: "draft not found" }),
				});
			}
			puts.push(route.request().postDataJSON());
			return route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					message: "success",
					data: {
						draftVersion: "1",
						updatedAt: "2026-09-13T15:08:59Z",
						draft: SAVED_DRAFT,
					},
				}),
			});
		});

		await page.goto(ENTRY_URL);
		await page.getByLabel("報價有效期限").fill("2026-09-18T16:00");
		await page.getByLabel("承諾送達時間").fill("2026-09-20T09:00");
		await page.getByRole("button", { name: "儲存草稿" }).click();
		await expect(page.getByText("已儲存草稿，尚未正式提交")).toBeVisible();

		// 送上去的必須是含位移的 RFC3339，不是選擇器那串本地時間。
		expect(puts).toHaveLength(1);
		const validUntil = String(puts[0]?.draft?.validUntil);
		const promised = String(puts[0]?.draft?.promisedDeliveryAt);
		expect(SELLER_APP_RFC3339.test(validUntil)).toBe(true);
		expect(SELLER_APP_RFC3339.test(promised)).toBe(true);
		expect(validUntil).toBe("2026-09-18T16:00:00+08:00");
		expect(promised).toBe("2026-09-20T09:00:00+08:00");
	});

	test("既有草稿的日期不是 RFC3339 時原樣留著、照樣存得進去", async ({
		page,
	}) => {
		const puts: Array<{ draft?: Record<string, unknown> }> = [];
		await routeInviteeSession(page);
		await page.route("**/api/v1/public/vendor_portal/rfqs/*", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify(RFQ_BODY),
			}),
		);
		await page.route(DRAFT_ROUTE, (route) => {
			if (route.request().method() !== "PUT") {
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						message: "success",
						data: {
							draftVersion: "1",
							updatedAt: "2026-09-13T15:08:59Z",
							draft: { ...SAVED_DRAFT, validUntil: "9/20 早上" },
						},
					}),
				});
			}
			puts.push(route.request().postDataJSON());
			return route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					code: 0,
					message: "success",
					data: {
						draftVersion: "2",
						updatedAt: "2026-09-13T15:10:00Z",
						draft: { ...SAVED_DRAFT, validUntil: "9/20 早上" },
					},
				}),
			});
		});

		await page.goto(ENTRY_URL);

		// 原樣顯示，不清空也不報錯——那是業者自己填的東西。
		await expect(page.getByText("目前存著的是「9/20 早上」")).toBeVisible();
		await expect(page.getByLabel("報價有效期限")).toHaveValue("");
		// 草稿階段不擋存：格式不對照樣按得下去，而且送上去的還是原字串。
		await page.getByRole("button", { name: "儲存草稿" }).click();
		await expect(page.getByText("已儲存草稿，尚未正式提交")).toBeVisible();
		expect(puts).toHaveLength(1);
		expect(puts[0]?.draft?.validUntil).toBe("9/20 早上");
	});

	test("版本衝突回 409 時要求重新載入，絕不自動覆蓋", async ({ page }) => {
		let putCount = 0;
		await routeInviteeSession(page);
		await page.route("**/api/v1/public/vendor_portal/rfqs/*", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify(RFQ_BODY),
			}),
		);
		await page.route(DRAFT_ROUTE, (route) => {
			if (route.request().method() !== "PUT") {
				return route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						message: "success",
						data: {
							draftVersion: "1",
							updatedAt: "2026-09-13T15:08:59Z",
							draft: SAVED_DRAFT,
						},
					}),
				});
			}
			putCount += 1;
			return route.fulfill({
				status: 409,
				contentType: "application/json",
				body: JSON.stringify({ code: 409, message: "draft version conflict" }),
			});
		});

		await page.goto(ENTRY_URL);
		await expect(page.getByLabel("貨款金額")).toHaveValue("30000");
		await page.getByLabel("貨款金額").fill("99999");
		await page.getByRole("button", { name: "儲存草稿" }).click();

		await expect(page.getByText("不會自動覆蓋")).toBeVisible();
		await expect(page.getByRole("button", { name: "重新載入" })).toBeVisible();
		// 自動重試等於拿本分頁的內容蓋掉另一個分頁剛存進去的那份。
		expect(putCount).toBe(1);
		// 業者填的內容還在畫面上，沒有被清掉也沒有被伺服器版本蓋回去。
		await expect(page.getByLabel("貨款金額")).toHaveValue("99999");
	});
	test("兌換拿到的 portal session cookie，真的會跟著 RFQ 讀取一起送出", async ({
		page,
	}) => {
		let rfqCookie: string | undefined;
		await routeInviteHappyPath(page);
		await page.route("**/api/v1/public/vendor_portal/rfqs/*", (route) => {
			rfqCookie = route.request().headers().cookie;
			return route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify(RFQ_BODY),
			});
		});
		await routeDraftAbsent(page);

		await page.goto(`/vendor/rfq#invite=${SECRET}`);
		await page.getByRole("button", { name: "查看詢價內容" }).click();
		await expect(page.getByText("結球萵苣")).toBeVisible();

		// apiClient.public 預設 credentials:"omit"。漏了 include，這一頁的
		// session 就完全沒有作用——而畫面不會有任何徵兆，只會一路 404。
		// cookie 每張邀請一個、`Path=/`：要同時送到 Connect 與 gateway 的 RFQ 路由。
		expect(rfqCookie).toContain(`__Host-hk_vp_${RECIPIENT_ID}=opaque`);
		// 兌換之後網址帶收件人 ID（secret 已從 fragment 移除）。
		expect(new URL(page.url()).search).toBe(`?r=${RECIPIENT_ID}`);
		expect(new URL(page.url()).hash).toBe("");
	});

	test("邀請已不可用（secret 已換新、到期或撤銷）：顯示連結無效，沒有退路", async ({
		page,
	}) => {
		const session = await routeInviteeSession(page);
		await routeVendor(page, "PreviewInvite", (route) =>
			fulfillConnectError(route, "not_found", "invite not found"),
		);

		await page.goto(`/vendor/rfq#invite=${SECRET}`);

		await expect(page.getByText("連結無效")).toBeVisible();
		// 沒有收件人 ID 可用（網址只有 fragment），不會去讀任何邀請的 session。
		expect(session).toHaveLength(0);
	});

	test("重新整理在 ?r=<id>：以 GetSession 讀這張邀請的 session，驗證狀態來自伺服器", async ({
		page,
	}) => {
		await installProviderMocks(page, "vendor-id-token");
		const session = await routeInviteeSession(page, VERIFIED_SESSION);
		await page.route("**/api/v1/public/vendor_portal/rfqs/*", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify(RFQ_BODY),
			}),
		);
		await routeDraftAbsent(page);

		await page.goto(ENTRY_URL);

		await expect(page.getByText("結球萵苣")).toBeVisible();
		expect(session).toHaveLength(1);
		expectVendorWire(session[0], "GetSession", {
			rfqRecipientId: RECIPIENT_ID,
		});
		// 已驗證：直接是「正式提交報價」，不再要業者重做 Google 驗證。
		await expect(
			page.getByRole("button", { name: "正式提交報價" }),
		).toBeVisible();
		await expect(page.getByTestId("mock-google")).toHaveCount(0);
	});

	test("RFQ 讀取回 401（gateway：沒有有效的 vendor session）：顯示連結無效", async ({
		page,
	}) => {
		await routeInviteeSession(page);
		await page.route("**/api/v1/public/vendor_portal/rfqs/*", (route) =>
			route.fulfill({
				status: 401,
				contentType: "application/json",
				body: JSON.stringify({ code: 401, message: "unauthorized" }),
			}),
		);

		await page.goto(ENTRY_URL);

		await expect(page.getByText("連結無效")).toBeVisible();
	});

	test("GetSession 回 unauthenticated（沒有這張邀請的有效 session）：顯示連結無效", async ({
		page,
	}) => {
		await routeVendor(page, "GetSession", (route) =>
			fulfillConnectError(route, "unauthenticated"),
		);

		await page.goto(ENTRY_URL);

		await expect(page.getByText("連結無效")).toBeVisible();
	});

	test("同一個瀏覽器兌換兩張邀請：兩個 cookie 並存，每個請求都用自己那張的收件人 ID 與 cookie", async ({
		page,
		context,
	}) => {
		const ID_A = "1001";
		const ID_B = "2002";
		const SECRET_A = "a".repeat(64);
		const SECRET_B = "b".repeat(64);
		/** 每個請求的收件人 ID 都要有對應的 `__Host-hk_vp_<id>` cookie；不符的記在這裡。 */
		const cookieMismatches: string[] = [];
		const requireCookie = (route: Route, id: string, what: string) => {
			const cookie = route.request().headers().cookie ?? "";
			if (!cookie.includes(`__Host-hk_vp_${id}=opaque`)) {
				cookieMismatches.push(`${what} id=${id} cookie=${cookie}`);
			}
		};
		await routeVendor(page, "PreviewInvite", (route) =>
			fulfillConnect(route, PREVIEW_BODY),
		);
		await routeVendor(page, "ExchangeInvite", (route, call) => {
			const id = call.body.secret === SECRET_A ? ID_A : ID_B;
			return fulfillConnect(
				route,
				{ rfqRecipientId: id, googleNonce: `nonce-${id}` },
				{ "set-cookie": sessionCookie(id) },
			);
		});
		const sessions = await routeVendor(page, "GetSession", (route, call) => {
			requireCookie(route, String(call.body.rfqRecipientId), "GetSession");
			return fulfillConnect(route, UNVERIFIED_SESSION);
		});
		const rfqIds: string[] = [];
		await page.route("**/api/v1/public/vendor_portal/rfqs/*", (route) => {
			const id =
				new URL(route.request().url()).pathname.split("/").at(-1) ?? "";
			rfqIds.push(id);
			requireCookie(route, id, "RFQ");
			return route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify(RFQ_BODY),
			});
		});
		await routeDraftAbsent(page);

		for (const secret of [SECRET_A, SECRET_B]) {
			await page.goto(`/vendor/rfq#invite=${secret}`);
			await page.getByRole("button", { name: "查看詢價內容" }).click();
			await expect(page.getByText("結球萵苣")).toBeVisible();
		}
		expect(new URL(page.url()).search).toBe(`?r=${ID_B}`);

		await page.goto(`/vendor/rfq?r=${ID_A}`);
		await expect(page.getByText("結球萵苣")).toBeVisible();

		// 兌換時 session 已知，不另外讀；回到 A 的網址才讀，而且指名 A。
		expect(sessions).toHaveLength(1);
		expectVendorWire(sessions[0], "GetSession", { rfqRecipientId: ID_A });
		// RFQ 讀取依序是 A、B、A，路徑上的收件人 ID 與頁面當時的邀請一致。
		expect(rfqIds).toEqual([ID_A, ID_B, ID_A]);
		expect(cookieMismatches).toEqual([]);

		// 兌換 B 沒有蓋掉 A 的 cookie：兩張邀請的 session 各自存在。
		const names = (await context.cookies()).map((c) => c.name);
		expect(names).toContain(`__Host-hk_vp_${ID_A}`);
		expect(names).toContain(`__Host-hk_vp_${ID_B}`);
	});

	test("?r 不是合法的收件人 ID：顯示連結無效，不問伺服器", async ({ page }) => {
		let calls = 0;
		for (const glob of [
			`**/${INVITEE_SERVICE}/**`,
			"**/api/v1/public/vendor_portal/**",
		]) {
			await page.route(glob, () => {
				calls += 1;
				throw new Error("no request may be sent for an invalid recipient id");
			});
		}

		await page.goto("/vendor/rfq?r=abc");

		await expect(page.getByText("連結無效")).toBeVisible();
		expect(calls).toBe(0);
	});

	test("被節流時說稍後再試，不說連結失效", async ({ page }) => {
		await routeVendor(page, "PreviewInvite", (route) =>
			fulfillConnectError(route, "resource_exhausted", "too many requests"),
		);

		await page.goto(`/vendor/rfq#invite=${SECRET}`);

		await expect(page.getByText("請稍後再試")).toBeVisible();
		await expect(page.getByText("連結無效")).toHaveCount(0);
	});
	test("左欄不畫買家品相，那件事由中欄的實際品相承擔", async ({ page }) => {
		await routeInviteeSession(page);
		await page.route("**/api/v1/public/vendor_portal/rfqs/*", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify(RFQ_BODY),
			}),
		);
		await routeDraftAbsent(page);

		await page.goto(ENTRY_URL);

		await expect(page.getByText("結球萵苣")).toBeVisible();
		await expect(page.getByText("買家品相")).toHaveCount(0);
		await expect(page.getByText("尚未指定")).toHaveCount(0);
		await expect(page.getByText("實際品相")).toBeVisible();
	});

	test("Pricebook 行情與買家希望區間都顯示，並各自標明計價基準", async ({
		page,
	}) => {
		await routeInviteeSession(page);
		await page.route("**/api/v1/public/vendor_portal/rfqs/*", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify(RFQ_BODY),
			}),
		);
		await routeDraftAbsent(page);

		await page.goto(ENTRY_URL);

		await expect(page.getByText("Pricebook 市場行情")).toBeVisible();
		await expect(page.getByText("32000 ～ 40000")).toBeVisible();
		await expect(page.getByText("買家希望區間")).toBeVisible();
		await expect(page.getByText("TWD 38000 ～ 43000")).toBeVisible();
		await expect(page.getByText("每 1 BOX")).toHaveCount(2);
		await expect(
			page.getByText("這兩個區間只是報價脈絡，不是成交價格"),
		).toBeVisible();
	});

	test("買家自行出價、沒有 Pricebook 參考時，不畫一個空的行情區塊", async ({
		page,
	}) => {
		await routeInviteeSession(page);
		await page.route("**/api/v1/public/vendor_portal/rfqs/*", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					...RFQ_BODY,
					data: { ...RFQ_BODY.data, referencePriceRange: null },
				}),
			}),
		);
		await routeDraftAbsent(page);

		await page.goto(ENTRY_URL);

		await expect(page.getByText("Pricebook 市場行情")).toHaveCount(0);
		await expect(page.getByText("買家希望區間")).toBeVisible();
		await expect(page.getByText("TWD 38000 ～ 43000")).toBeVisible();
	});

	test("兩個區間都沒有時，連那句脈絡說明都不出現", async ({ page }) => {
		await routeInviteeSession(page);
		await page.route("**/api/v1/public/vendor_portal/rfqs/*", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					...RFQ_BODY,
					data: {
						...RFQ_BODY.data,
						referencePriceRange: null,
						buyerRequestedRange: null,
					},
				}),
			}),
		);
		await routeDraftAbsent(page);

		await page.goto(ENTRY_URL);

		await expect(page.getByText("結球萵苣")).toBeVisible();
		await expect(page.getByText("Pricebook 市場行情")).toHaveCount(0);
		await expect(page.getByText("買家希望區間")).toHaveCount(0);
		await expect(
			page.getByText("這兩個區間只是報價脈絡，不是成交價格"),
		).toHaveCount(0);
	});

	test("買家的補充說明原文顯示；沒填時整個區塊不出現", async ({ page }) => {
		await routeInviteeSession(page);
		await page.route("**/api/v1/public/vendor_portal/rfqs/*", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify({
					...RFQ_BODY,
					data: { ...RFQ_BODY.data, buyerNote: "中大果，不要有壓傷" },
				}),
			}),
		);
		await routeDraftAbsent(page);

		await page.goto(ENTRY_URL);

		await expect(page.getByText("買家的補充說明")).toBeVisible();
		// 原文照登：不截斷、不改寫、不加工成品相等級。
		await expect(page.getByText("中大果，不要有壓傷")).toBeVisible();
	});

	test("沒有補充說明時不畫一個空區塊", async ({ page }) => {
		await routeInviteeSession(page);
		await page.route("**/api/v1/public/vendor_portal/rfqs/*", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify(RFQ_BODY),
			}),
		);
		await routeDraftAbsent(page);

		await page.goto(ENTRY_URL);

		await expect(page.getByText("結球萵苣")).toBeVisible();
		// ⛔ 空字串不得畫成一個標著「買家的補充說明」的空框——供應商會以為載入失敗。
		await expect(page.getByText("買家的補充說明")).toHaveCount(0);
	});
});

test.describe("站外業者正式提交報價", () => {
	/** 進到「已驗證、可提交」那一步之前的共同前置。 */
	async function arriveAtWorkspace(
		page: Page,
		session: Record<string, unknown> = UNVERIFIED_SESSION,
	): Promise<VendorCall[]> {
		await installProviderMocks(page, "vendor-id-token");
		const sessionCalls = await routeInviteeSession(page, session);
		await page.route("**/api/v1/public/vendor_portal/rfqs/*", (route) =>
			route.fulfill({
				status: 200,
				contentType: "application/json",
				body: JSON.stringify(RFQ_BODY),
			}),
		);
		await routeDraftAbsent(page);
		await page.goto(ENTRY_URL);
		await expect(page.getByText("結球萵苣")).toBeVisible();
		return sessionCalls;
	}

	test("驗證 → 二次確認 → 送出：帶冪等鍵與 version-only CAS，送出後鎖定 @ui", async ({
		page,
	}) => {
		const verify = await routeVendor(page, "VerifySession", (route) =>
			fulfillConnect(route, VERIFIED_BODY),
		);

		const submissions: {
			key: string | undefined;
			body: Record<string, unknown>;
		}[] = [];
		await page.route(
			"**/api/v1/public/vendor_portal/rfqs/*/response_submissions",
			async (route) => {
				submissions.push({
					key: route.request().headers()["idempotency-key"],
					body: route.request().postDataJSON(),
				});
				await route.fulfill({
					status: 200,
					contentType: "application/json",
					body: JSON.stringify({
						code: 0,
						message: "success",
						data: {
							sellerResponseId: "7507680176017244160",
							sellerResponseVersion: "1",
							status: "ACTIVE",
						},
					}),
				});
			},
		);

		await arriveAtWorkspace(page);
		await page.getByLabel("貨款金額").fill("12345");

		// ① 驗證之前沒有提交按鈕。
		await expect(
			page.getByRole("button", { name: "正式提交報價" }),
		).toHaveCount(0);

		// ② 完成 Google 驗證——⛔ 只送 idToken，不得夾帶 email。
		// GSI 初始化時帶的 nonce 必須就是伺服器（GetSession）給的那一個，token 才綁得到這個 session。
		await expect(page.getByTestId("mock-google")).toBeVisible();
		expect(await page.evaluate(() => window.__gsiNonces)).toEqual([NONCE]);
		await page.getByTestId("mock-google").click();
		await expect(page.getByText("已完成 Google 帳號驗證")).toBeVisible();
		expect(verify).toHaveLength(1);
		// 只送收件人與 Google 的 ID token，不送 email：email 由伺服器從驗過的 token 取出。
		// 指名這一頁的收件人，Google token 在訊息裡。
		expectVendorWire(verify[0], "VerifySession", {
			rfqRecipientId: RECIPIENT_ID,
			googleIdToken: "vendor-id-token",
		});

		// ③ 🔴 一次點擊不得送出：送出建立的是 immutable 版本，按錯沒有「改一下再存」
		//    可以救，所以中間必須隔一個明確的再確認。
		await page.getByRole("button", { name: "正式提交報價" }).click();
		expect(submissions).toHaveLength(0);
		await expect(page.getByText("確定要送出嗎？")).toBeVisible();

		// ④ 確認後才真的送。
		await page.getByRole("button", { name: "確定送出" }).click();
		await expect(page.getByText("報價已正式提交（第 1 版）")).toBeVisible();

		expect(submissions).toHaveLength(1);
		// 少了冪等鍵，超時重送會產生第二個正式版本，買家會看到兩筆報價。
		expect(submissions[0]?.key).toBeTruthy();
		expect(submissions[0]?.body.expectedPublicationVersion).toBe("7");
		// ⛔ 站外這條 lane 的 CAS 是 version-only：hash 不對站外方暴露，這個欄位
		//    **不該存在**（不是存在但為空）。
		expect(Object.keys(submissions[0]?.body ?? {})).not.toContain(
			"expectedSellerFacingPayloadHash",
		);

		// ⑤ 送出後鎖定：這一版不能再改，畫面不該再給一個看起來能重送的按鈕。
		await expect(page.getByRole("button", { name: "確定送出" })).toHaveCount(0);
		await expect(page.getByTestId("progress-step-3")).toContainText("已完成");
	});

	test("「再想一下」退回未送出狀態，不留下任何請求 @ui", async ({ page }) => {
		let submitted = 0;
		await routeVendor(page, "VerifySession", (route) =>
			fulfillConnect(route, VERIFIED_BODY),
		);
		await page.route(
			"**/api/v1/public/vendor_portal/rfqs/*/response_submissions",
			(route) => {
				submitted += 1;
				return route.fulfill({ status: 200, body: "{}" });
			},
		);

		await arriveAtWorkspace(page);
		await page.getByTestId("mock-google").click();
		await page.getByRole("button", { name: "正式提交報價" }).click();
		await page.getByRole("button", { name: "再想一下" }).click();

		await expect(page.getByText("確定要送出嗎？")).toHaveCount(0);
		await expect(
			page.getByRole("button", { name: "正式提交報價" }),
		).toBeVisible();
		expect(submitted).toBe(0);
	});

	test("驗證時 session 已失效（unauthenticated）：說清楚要重新開啟信件連結 @ui", async ({
		page,
	}) => {
		await routeVendor(page, "VerifySession", (route) =>
			fulfillConnectError(route, "unauthenticated"),
		);

		await arriveAtWorkspace(page);
		await page.getByTestId("mock-google").click();

		await expect(
			page.getByText("連結的使用期限已過，請重新開啟信件中的連結。"),
		).toBeVisible();
		await expect(page.getByText("驗證沒有通過")).toHaveCount(0);
	});

	test("驗證失效回 403 時退回未驗證狀態，並說清楚報價沒有送出 @ui", async ({
		page,
	}) => {
		await routeVendor(page, "VerifySession", (route) =>
			fulfillConnect(route, VERIFIED_BODY),
		);
		await page.route(
			"**/api/v1/public/vendor_portal/rfqs/*/response_submissions",
			(route) =>
				route.fulfill({
					status: 403,
					contentType: "application/json",
					body: JSON.stringify({ code: 9346, message: "step-up required" }),
				}),
		);

		const sessionCalls = await arriveAtWorkspace(page);
		await page.getByTestId("mock-google").click();
		await page.getByRole("button", { name: "正式提交報價" }).click();
		expect(sessionCalls).toHaveLength(1);
		await page.getByRole("button", { name: "確定送出" }).click();

		// ⛔ 「沒有送出」必須說出口：業者看不到請求，只看得到畫面。
		await expect(page.getByText("報價沒有送出")).toBeVisible();
		// 退回未驗證，讓他知道下一步要做什麼。
		await expect(
			page.getByText("送出前請先用收到邀請信的那個 Google 帳號完成驗證"),
		).toBeVisible();
		// 閘道說沒驗證時以伺服器現在的狀態為準：重讀 GetSession，而不是只清掉記憶體裡的狀態。
		expect(sessionCalls).toHaveLength(2);
		expectVendorWire(sessionCalls[1], "GetSession", {
			rfqRecipientId: RECIPIENT_ID,
		});
	});

	test("驗證失效回 403、重讀 session 得到 unauthenticated：丟掉舊 nonce，說清楚要重新開啟信件連結 @ui", async ({
		page,
	}) => {
		await routeVendor(page, "VerifySession", (route) =>
			fulfillConnect(route, VERIFIED_BODY),
		);
		await page.route(
			"**/api/v1/public/vendor_portal/rfqs/*/response_submissions",
			(route) =>
				route.fulfill({
					status: 403,
					contentType: "application/json",
					body: JSON.stringify({ code: 9346, message: "step-up required" }),
				}),
		);
		await arriveAtWorkspace(page);
		// 第一次 GetSession 之後 session 就不在了：後續的讀取回 unauthenticated。
		await page.unroute(`**/${INVITEE_SERVICE}/GetSession*`);
		await routeVendor(page, "GetSession", (route) =>
			fulfillConnectError(route, "unauthenticated"),
		);
		await page.getByTestId("mock-google").click();
		await page.getByRole("button", { name: "正式提交報價" }).click();
		await page.getByRole("button", { name: "確定送出" }).click();

		await expect(page.getByText("報價沒有送出")).toBeVisible();
		await expect(
			page.getByText("連結的使用期限已過，請重新開啟信件中的連結。"),
		).toBeVisible();
		await expect(page.getByTestId("mock-google")).toHaveCount(0);
	});

	test("session 沒有 nonce（伺服器沒有這張邀請的 session）：不畫 Google 按鈕，說清楚要重新開啟信件連結 @ui", async ({
		page,
	}) => {
		await arriveAtWorkspace(page, {});

		await expect(
			page.getByText("連結的使用期限已過，請重新開啟信件中的連結。"),
		).toBeVisible();
		await expect(page.getByTestId("mock-google")).toHaveCount(0);
		expect(await page.evaluate(() => window.__gsiNonces ?? [])).toEqual([]);
	});
});
