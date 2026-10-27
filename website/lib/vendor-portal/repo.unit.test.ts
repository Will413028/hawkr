import { Code, ConnectError } from "@connectrpc/connect";
import { HttpResponse, http } from "msw";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ApiClientError } from "@/lib/api/client";
import { getEnv } from "@/lib/config/env";
import { connectBaseUrl } from "@/lib/connect/transport";
import { connectError, connectOk } from "@/tests/mocks/connect-response";
import { server } from "@/tests/mocks/server";
import type { VendorResponseDraft } from "./draft";
import {
	exchangeVendorInvite,
	fetchVendorResponseDraft,
	fetchVendorRfq,
	fetchVendorSession,
	previewVendorInvite,
	saveVendorResponseDraft,
	stepUpVendorPortalGoogle,
	submitVendorResponse,
} from "./repo";

/**
 * 站外業者 portal 的 wire contract：preview、exchange、step-up 是 Connect
 * （`VendorPortalPublicService`，stub 掉 fetch 看 connect-web 真正送出的請求）；RFQ 與草稿
 * 還是 REST（msw）。
 *
 * 為什麼看請求而不是只測純函式：這幾支會靜默出錯的是**路徑**、**secret 放在 URL 還是訊息**與
 * **有沒有帶 cookie**——打錯都只會拿到錯誤，型別上完全合法。
 */
const BASE = getEnv().serverApiBase;
const SECRET = "b".repeat(64);
const RECIPIENT_ID = "843856384063471755";

const SERVICE_PATH = `${connectBaseUrl(false)}/hawkr.agent.v1alpha1.VendorPortalPublicService`;
const INVITEE_PATH = `${connectBaseUrl(false)}/hawkr.agent.v1alpha1.VendorPortalInviteeService`;

const fetchMock = vi.fn<typeof fetch>();

// 這些呼叫在瀏覽器裡跑（cookie 憑證），語系取自頁面；這個 test 在 node 裡跑 transport，所以把頁面語系釘住。
vi.mock("@/lib/intl/active-locale", () => ({
	activeLocale: () => "zh-TW",
	setActiveLocale: () => undefined,
}));

/** 第 n 次 Connect 呼叫實際送出的請求：method、URL、cookie 設定與解碼後的 JSON 訊息。 */
function sentRequest(n = 0) {
	const [url, init] = fetchMock.mock.calls[n];
	return {
		url: String(url),
		method: init?.method,
		credentials: init?.credentials,
		headers: new Headers(init?.headers),
		body: init?.body
			? JSON.parse(new TextDecoder().decode(init.body as Uint8Array))
			: undefined,
		/** GET 的訊息在 query（`message`，JSON）。 */
		query: JSON.parse(
			new URL(String(url)).searchParams.get("message") ?? "null",
		),
	};
}

describe("VendorPortal 的 Connect 呼叫（Public 與 Invitee service）", () => {
	beforeEach(() => {
		vi.stubGlobal("fetch", fetchMock);
	});
	afterEach(() => {
		vi.unstubAllGlobals();
		fetchMock.mockReset();
	});

	describe("previewVendorInvite", () => {
		it("POST PreviewInvite，secret 在訊息裡不在 URL，品牌是 client 常數", async () => {
			fetchMock.mockResolvedValue(
				connectOk({
					maskedEmail: "w***@example.com",
					expireTime: "2026-09-19T04:12:07Z",
				}),
			);

			await expect(previewVendorInvite(SECRET)).resolves.toEqual({
				brand: "Hawkr",
				maskedEmail: "w***@example.com",
				expiresAt: "2026-09-19T04:12:07.000Z",
			});

			expect(fetchMock).toHaveBeenCalledTimes(1);
			const req = sentRequest();
			expect(req.url).toBe(`${SERVICE_PATH}/PreviewInvite`);
			expect(req.url).not.toContain(SECRET);
			expect(req.method).toBe("POST");
			expect(req.body).toEqual({ secret: SECRET });
			expect(req.credentials).toBe("include");
			expect(req.headers.has("Authorization")).toBe(false);
		});

		it("邀請不可用一律 not_found，原樣拋出讓畫面收斂成同一個狀態", async () => {
			fetchMock.mockResolvedValue(connectError(Code.NotFound));

			await expect(previewVendorInvite(SECRET)).rejects.toMatchObject({
				code: Code.NotFound,
			});
			await expect(previewVendorInvite(SECRET)).rejects.toBeInstanceOf(
				ConnectError,
			);
		});
	});

	describe("exchangeVendorInvite", () => {
		it("POST ExchangeInvite 以 secret 換 session，回收件人 ID 與 session 的驗證狀態、nonce", async () => {
			fetchMock.mockResolvedValue(
				connectOk({ rfqRecipientId: RECIPIENT_ID, googleNonce: "nonce-1" }),
			);

			await expect(exchangeVendorInvite(SECRET)).resolves.toEqual({
				rfqRecipientId: RECIPIENT_ID,
				session: { assurance: "", googleNonce: "nonce-1" },
			});

			const req = sentRequest();
			expect(req.url).toBe(`${SERVICE_PATH}/ExchangeInvite`);
			expect(req.url).not.toContain(SECRET);
			expect(req.method).toBe("POST");
			expect(req.body).toEqual({ secret: SECRET });
			expect(req.credentials).toBe("include");
			expect(req.headers.has("Authorization")).toBe(false);
		});

		it("沿用既有的已驗證 session 時，回應帶 GOOGLE 的驗證紀錄 → assurance 是 GOOGLE_VERIFIED", async () => {
			fetchMock.mockResolvedValue(
				connectOk({
					rfqRecipientId: RECIPIENT_ID,
					googleNonce: "n",
					verification: { method: "SESSION_VERIFICATION_METHOD_GOOGLE" },
				}),
			);
			const { session } = await exchangeVendorInvite(SECRET);
			expect(session.assurance).toBe("GOOGLE_VERIFIED");
		});

		it("額度用完回 resource_exhausted，原樣拋出（畫面要說『稍後再試』而不是『連結失效』）", async () => {
			fetchMock.mockResolvedValue(connectError(Code.ResourceExhausted));

			await expect(exchangeVendorInvite(SECRET)).rejects.toMatchObject({
				code: Code.ResourceExhausted,
			});
		});
	});

	describe("fetchVendorSession", () => {
		it("GET GetSession，收件人在 query、不在 body，帶 cookie；回驗證狀態與 nonce", async () => {
			fetchMock.mockResolvedValue(
				connectOk({
					googleNonce: "nonce-2",
					verification: {
						method: "SESSION_VERIFICATION_METHOD_GOOGLE",
						verifyTime: "2026-09-19T04:12:07Z",
					},
				}),
			);

			await expect(fetchVendorSession(RECIPIENT_ID)).resolves.toEqual({
				assurance: "GOOGLE_VERIFIED",
				googleNonce: "nonce-2",
			});

			const req = sentRequest();
			expect(req.url.startsWith(`${INVITEE_PATH}/GetSession?`)).toBe(true);
			expect(req.method).toBe("GET");
			expect(req.query).toEqual({ rfqRecipientId: RECIPIENT_ID });
			expect(req.body).toBeUndefined();
			expect(req.credentials).toBe("include");
			expect(req.headers.has("Authorization")).toBe(false);
		});

		it("沒有驗證紀錄、或方法是 UNSPECIFIED ＝ 沒驗證", async () => {
			fetchMock.mockResolvedValueOnce(connectOk({ googleNonce: "n" }));
			await expect(fetchVendorSession(RECIPIENT_ID)).resolves.toEqual({
				assurance: "",
				googleNonce: "n",
			});
			fetchMock.mockResolvedValueOnce(
				connectOk({
					googleNonce: "n",
					verification: {
						method: "SESSION_VERIFICATION_METHOD_UNSPECIFIED",
					},
				}),
			);
			await expect(fetchVendorSession(RECIPIENT_ID)).resolves.toEqual({
				assurance: "",
				googleNonce: "n",
			});
		});

		it("沒有這張邀請的有效 session 回 unauthenticated，原樣拋出", async () => {
			fetchMock.mockResolvedValue(connectError(Code.Unauthenticated));
			await expect(fetchVendorSession(RECIPIENT_ID)).rejects.toMatchObject({
				code: Code.Unauthenticated,
			});
		});
	});

	describe("stepUpVendorPortalGoogle", () => {
		it("POST VerifySession 送 rfqRecipientId 與 googleIdToken，不得送 email；GOOGLE 對應 GOOGLE_VERIFIED", async () => {
			fetchMock.mockResolvedValue(
				connectOk({
					rfqRecipientId: RECIPIENT_ID,
					verification: {
						method: "SESSION_VERIFICATION_METHOD_GOOGLE",
						verifyTime: "2026-09-19T04:12:07Z",
					},
				}),
			);

			const result = await stepUpVendorPortalGoogle(
				RECIPIENT_ID,
				"id-token-xyz",
			);

			expect(result.assurance).toBe("GOOGLE_VERIFIED");
			const req = sentRequest();
			expect(req.url).toBe(`${INVITEE_PATH}/VerifySession`);
			expect(req.method).toBe("POST");
			expect(req.body).toEqual({
				rfqRecipientId: RECIPIENT_ID,
				googleIdToken: "id-token-xyz",
			});
			expect(req.url).not.toContain("id-token-xyz");
			// 前端送的 email 是呼叫端說了算的值；email 只能由後端從驗過的 token 取出。
			expect(Object.keys(req.body)).not.toContain("email");
			expect(req.credentials).toBe("include");
		});

		it("回應沒有已知的驗證方式時，assurance 是空字串（不升級）", async () => {
			fetchMock.mockResolvedValue(connectOk({ rfqRecipientId: RECIPIENT_ID }));

			await expect(
				stepUpVendorPortalGoogle(RECIPIENT_ID, "t"),
			).resolves.toEqual({ assurance: "" });
		});

		it("沒有 session 回 unauthenticated、Google 驗證沒過回 permission_denied，原樣拋出", async () => {
			fetchMock.mockResolvedValueOnce(connectError(Code.Unauthenticated));
			await expect(
				stepUpVendorPortalGoogle(RECIPIENT_ID, "t"),
			).rejects.toMatchObject({ code: Code.Unauthenticated });
			fetchMock.mockResolvedValueOnce(connectError(Code.PermissionDenied));
			await expect(
				stepUpVendorPortalGoogle(RECIPIENT_ID, "t"),
			).rejects.toMatchObject({ code: Code.PermissionDenied });
		});
	});
});

describe("fetchVendorRfq", () => {
	it("GET rfqs/{id}，回九欄白名單", async () => {
		server.use(
			http.get(
				`${BASE}/public/vendor_portal/rfqs/:id`,
				({ params, request }) => {
					expect(String(params.id)).toBe(RECIPIENT_ID);
					// session 走 cookie，這支不得再帶 Invite secret。
					expect(request.headers.get("Authorization")).toBeNull();
					return HttpResponse.json({
						code: 0,
						message: "success",
						data: {
							qualityKnowledgeState: "BUYER_QUALITY_UNKNOWN",
							item: "LETTUCE",
							itemLabel: "結球萵苣",
							quantity: "300",
							unit: "KG",
							packaging: "紙箱",
							deliveryCity: "台中市",
							deliveryDate: "2026-09-20",
							responseDeadline: "2026-09-18T16:00:00Z",
						},
					});
				},
			),
		);

		await expect(fetchVendorRfq(RECIPIENT_ID)).resolves.toEqual({
			qualityKnowledgeState: "BUYER_QUALITY_UNKNOWN",
			item: "LETTUCE",
			itemLabel: "結球萵苣",
			quantity: "300",
			unit: "KG",
			packaging: "紙箱",
			deliveryCity: "台中市",
			deliveryDate: "2026-09-20",
			responseDeadline: "2026-09-18T16:00:00Z",
		});
	});

	it("session 過期或 id 與 session 綁定的不同時 404", async () => {
		server.use(
			http.get(`${BASE}/public/vendor_portal/rfqs/:id`, () =>
				HttpResponse.json(
					{ code: 404, message: "invite not found" },
					{ status: 404 },
				),
			),
		);

		await expect(fetchVendorRfq(RECIPIENT_ID)).rejects.toMatchObject({
			status: 404,
		});
	});
});

/**
 * 回價草稿的兩支端點（#2538）。
 *
 * ⚠️ 這兩支在 dev 上還沒部署，所以這裡是**對契約驗**而不是對實作驗。會靜默出錯
 * 的是：路徑尾巴、CAS 欄位 `expectedDraftVersion` 的名字、以及「沒有草稿回 404」
 * 被當成錯誤而不是空狀態——三者在型別上全部合法。
 */
const FULL_DRAFT: VendorResponseDraft = {
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

describe("fetchVendorResponseDraft", () => {
	it("GET rfqs/{id}/response_draft，回版本＋更新時間＋17 欄回聲", async () => {
		server.use(
			http.get(
				`${BASE}/public/vendor_portal/rfqs/:id/response_draft`,
				({ params }) => {
					expect(String(params.id)).toBe(RECIPIENT_ID);
					return HttpResponse.json({
						code: 0,
						message: "success",
						data: {
							draftVersion: "1",
							updatedAt: "2026-09-13T15:08:59Z",
							draft: FULL_DRAFT,
						},
					});
				},
			),
		);

		await expect(fetchVendorResponseDraft(RECIPIENT_ID)).resolves.toEqual({
			draftVersion: "1",
			updatedAt: "2026-09-13T15:08:59Z",
			draft: FULL_DRAFT,
		});
	});

	it("沒有草稿時後端回 404，收斂成 null 而不是丟例外", async () => {
		// 「還沒存過」是正常狀態：業者第一次進來本來就沒有草稿，這不是錯誤畫面。
		server.use(
			http.get(`${BASE}/public/vendor_portal/rfqs/:id/response_draft`, () =>
				HttpResponse.json(
					{ code: 404, message: "draft not found" },
					{ status: 404 },
				),
			),
		);

		await expect(fetchVendorResponseDraft(RECIPIENT_ID)).resolves.toBeNull();
	});

	it("404 以外的失敗照樣拋出，不得被當成「沒有草稿」吃掉", async () => {
		server.use(
			http.get(`${BASE}/public/vendor_portal/rfqs/:id/response_draft`, () =>
				HttpResponse.json({ code: 500, message: "boom" }, { status: 500 }),
			),
		);

		await expect(fetchVendorResponseDraft(RECIPIENT_ID)).rejects.toBeInstanceOf(
			ApiClientError,
		);
	});

	it("後端漏掉某些欄位時補成完整 17 欄，不讓受控欄位吃到 undefined", async () => {
		server.use(
			http.get(`${BASE}/public/vendor_portal/rfqs/:id/response_draft`, () =>
				HttpResponse.json({
					code: 0,
					message: "success",
					data: {
						draftVersion: "7",
						updatedAt: "2026-09-13T15:08:59Z",
						draft: { goodsAmount: "100" },
					},
				}),
			),
		);

		const snapshot = await fetchVendorResponseDraft(RECIPIENT_ID);
		expect(snapshot?.draft.goodsAmount).toBe("100");
		expect(snapshot?.draft.termsNote).toBe("");
		expect(snapshot?.draft.offeredQuality).toEqual({
			grade: "",
			inspectionNotes: "",
		});
	});
});

describe("saveVendorResponseDraft", () => {
	it("PUT 帶 expectedDraftVersion ＋ 17 欄，回新版本", async () => {
		server.use(
			http.put(
				`${BASE}/public/vendor_portal/rfqs/:id/response_draft`,
				async ({ params, request }) => {
					expect(String(params.id)).toBe(RECIPIENT_ID);
					expect(await request.json()).toEqual({
						expectedDraftVersion: "1",
						draft: FULL_DRAFT,
					});
					// ⛔ 草稿的 CAS 對象就是 expectedDraftVersion，重送同一版本本來
					// 就是同一個結果——再疊一個冪等鍵只是多一個會不同步的狀態。
					expect(request.headers.get("Idempotency-Key")).toBeNull();
					return HttpResponse.json({
						code: 0,
						message: "success",
						data: {
							draftVersion: "2",
							updatedAt: "2026-09-13T15:10:00Z",
							draft: FULL_DRAFT,
						},
					});
				},
			),
		);

		await expect(
			saveVendorResponseDraft(RECIPIENT_ID, "1", FULL_DRAFT),
		).resolves.toMatchObject({ draftVersion: "2" });
	});

	it("第一次存草稿帶空字串版本，不是省略這個欄位", async () => {
		let body: unknown;
		server.use(
			http.put(
				`${BASE}/public/vendor_portal/rfqs/:id/response_draft`,
				async ({ request }) => {
					body = await request.json();
					return HttpResponse.json({
						code: 0,
						message: "success",
						data: {
							draftVersion: "1",
							updatedAt: "2026-09-13T15:08:59Z",
							draft: FULL_DRAFT,
						},
					});
				},
			),
		);

		await saveVendorResponseDraft(RECIPIENT_ID, "", FULL_DRAFT);
		expect(body).toMatchObject({ expectedDraftVersion: "" });
	});

	it("版本不符回 409，原樣拋出讓畫面提示重新載入而不是自動覆蓋", async () => {
		server.use(
			http.put(`${BASE}/public/vendor_portal/rfqs/:id/response_draft`, () =>
				HttpResponse.json(
					{ code: 409, message: "draft version conflict" },
					{ status: 409 },
				),
			),
		);

		await expect(
			saveVendorResponseDraft(RECIPIENT_ID, "1", FULL_DRAFT),
		).rejects.toMatchObject({ status: 409 });
	});

	it("草稿階段一律不擋：金額對不上、品相空白、日期不是 RFC3339 都照樣送得出去", async () => {
		// 這三項都是正式提交才驗的（422 QUOTE_AMOUNT_INCONSISTENT／
		// QUOTE_QUALITY_REQUIRED，以及平台側 isRfc3339 那條）。把它們搬到草稿上
		// 等於業者算到一半就存不了，草稿的意義就沒了。
		let saved = false;
		server.use(
			http.put(`${BASE}/public/vendor_portal/rfqs/:id/response_draft`, () => {
				saved = true;
				return HttpResponse.json({
					code: 0,
					message: "success",
					data: {
						draftVersion: "1",
						updatedAt: "2026-09-13T15:08:59Z",
						draft: FULL_DRAFT,
					},
				});
			}),
		);

		let body: { draft?: Record<string, unknown> } | undefined;
		server.use(
			http.put(
				`${BASE}/public/vendor_portal/rfqs/:id/response_draft`,
				async ({ request }) => {
					body = (await request.json()) as { draft?: Record<string, unknown> };
					saved = true;
					return HttpResponse.json({
						code: 0,
						message: "success",
						data: {
							draftVersion: "1",
							updatedAt: "2026-09-13T15:08:59Z",
							draft: FULL_DRAFT,
						},
					});
				},
			),
		);

		await saveVendorResponseDraft(RECIPIENT_ID, "", {
			...FULL_DRAFT,
			offeredQuality: { grade: "", inspectionNotes: "" },
			validUntil: "9/20 早上",
			promisedDeliveryAt: "",
			totalAmount: "1",
		});
		expect(saved).toBe(true);
		// 非法日期原樣送上去，不得被前端清空或改寫。
		expect(body?.draft?.validUntil).toBe("9/20 早上");
	});
});

describe("submitVendorResponse", () => {
	const draft = { responseType: "3901" } as unknown as VendorResponseDraft;

	it("POST response_submissions，帶 Idempotency-Key 與 version-only CAS", async () => {
		let key: string | null = null;
		let body: Record<string, unknown> = {};
		server.use(
			http.post(
				`${BASE}/public/vendor_portal/rfqs/${RECIPIENT_ID}/response_submissions`,
				async ({ request }) => {
					key = request.headers.get("Idempotency-Key");
					body = (await request.json()) as Record<string, unknown>;
					return HttpResponse.json({
						code: 0,
						message: "success",
						data: {
							sellerResponseId: "7507680176017244160",
							sellerResponseVersion: "1",
							status: "ACTIVE",
						},
					});
				},
			),
		);

		const result = await submitVendorResponse(
			RECIPIENT_ID,
			"key-once",
			{ publicationVersion: "3", sellerResponseVersion: "" },
			draft,
		);

		// 🔴 少了這個 header，超時重送會產生第二個正式版本，買家會看到兩筆報價。
		expect(key).toBe("key-once");
		expect(result.sellerResponseVersion).toBe("1");
		expect(body.expectedPublicationVersion).toBe("3");
		// ⛔ 站外這條 lane 的 CAS 是 version-only：hash 是我方的內部完整性摘要，
		// 不對站外方暴露，所以這個欄位**不該存在**（不是存在但為空）。
		expect(Object.keys(body)).not.toContain("expectedSellerFacingPayloadHash");
	});

	it("409 原樣拋出，呼叫端自己決定怎麼提示（不得自動用新版本重送）", async () => {
		server.use(
			http.post(
				`${BASE}/public/vendor_portal/rfqs/${RECIPIENT_ID}/response_submissions`,
				() =>
					HttpResponse.json(
						{ code: 9266, message: "stale publication" },
						{ status: 409 },
					),
			),
		);

		await expect(
			submitVendorResponse(
				RECIPIENT_ID,
				"key-once",
				{ publicationVersion: "2", sellerResponseVersion: "" },
				draft,
			),
		).rejects.toBeInstanceOf(ApiClientError);
	});
});
