import { HttpResponse, http } from "msw";
import { describe, expect, it } from "vitest";
import { getEnv } from "@/lib/config/env";
import { server } from "@/tests/mocks/server";
import {
	fetchProductCertificationApplications,
	submitProductCertificationApplication,
	withdrawProductCertificationApplication,
} from "../seller-repo";

/**
 * 認證申請三支端點的 wire contract（19 號設計書 §9.1）。
 *
 * 為什麼要打到 msw 而不是只測純函式：這三支唯一會出錯的地方就是**路徑組得對不對**
 * 與**欄位名對不對**，而那兩件事在 TypeScript 端一路沉默——路徑打錯拿到 404、
 * 欄位名打錯後端就當沒送，兩者在型別上都完全合法。
 */

const BASE = getEnv().serverApiBase;
const PRODUCT_ID = "843856384063471755";

/** 19 §9.1 的回應範例，逐字照抄（含字串型的 `version`）。 */
const APPROVED_RECORD = {
	id: "843856384063471760",
	productId: PRODUCT_ID,
	certificationType: "3701",
	certificateNo: "1-009-300022",
	status: "APPROVED",
	govVerifyState: "VERIFIED",
	govVerifiedAt: "2026-08-26T04:12:07Z",
	submittedAt: "2026-08-26T04:12:05Z",
	reviewedAt: "2026-08-26T04:12:07Z",
	reasonCode: null,
	publicReason: null,
	version: "1",
};

describe("fetchProductCertificationApplications", () => {
	it("讀 data.records，並把 id 與 version 當字串帶回", async () => {
		server.use(
			http.get(
				`${BASE}/app/products/:id/certification_applications`,
				({ params }) => {
					expect(String(params.id)).toBe(PRODUCT_ID);
					return HttpResponse.json({
						code: 0,
						message: "success",
						// ⚠️ total/page/pageSize 在 wire 上是**字串**（pkg/envelope.Page
						// 的簽章就是 string）。照數字解讀會全錯，所以這裡照原樣給。
						data: {
							records: [APPROVED_RECORD],
							total: "1",
							page: "1",
							pageSize: "20",
						},
					});
				},
			),
		);

		const apps = await fetchProductCertificationApplications(PRODUCT_ID);
		expect(apps).toHaveLength(1);
		expect(apps[0].id).toBe("843856384063471760");
		expect(apps[0].version).toBe("1");
		expect(apps[0].status).toBe("APPROVED");
		// 18 位 TSID 一旦被當成 number 解析就會掉精度；這裡直接釘住原字串。
		expect(typeof apps[0].id).toBe("string");
	});

	it("端點不存在時回空陣列，不丟例外（表單少徽章仍可編輯）", async () => {
		server.use(
			http.get(`${BASE}/app/products/:id/certification_applications`, () =>
				HttpResponse.json(
					{ code: 404, message: "not found", data: null },
					{ status: 404 },
				),
			),
		);
		await expect(
			fetchProductCertificationApplications(PRODUCT_ID),
		).resolves.toEqual([]);
	});
});

describe("submitProductCertificationApplication", () => {
	it("POST 到申請集合，body 帶 certificationType / certificateNo / idempotencyKey", async () => {
		const seen: { body?: Record<string, unknown> } = {};
		server.use(
			http.post(
				`${BASE}/app/products/:id/certification_applications`,
				async ({ request }) => {
					seen.body = (await request.json()) as Record<string, unknown>;
					return HttpResponse.json({
						code: 0,
						message: "success",
						data: { ...APPROVED_RECORD, status: "SUBMITTED", version: "2" },
					});
				},
			),
		);

		const app = await submitProductCertificationApplication(PRODUCT_ID, {
			certificationType: "3701",
			certificateNo: "1-009-300022",
		});

		expect(app?.status).toBe("SUBMITTED");
		expect(seen.body).toMatchObject({
			certificationType: "3701",
			certificateNo: "1-009-300022",
		});
		expect(String(seen.body?.idempotencyKey)).toMatch(/^cert-/);
	});

	it("非法輸入的 typed 業務碼往上丟，不吞掉（19 §6.2.10）", async () => {
		server.use(
			http.post(`${BASE}/app/products/:id/certification_applications`, () =>
				HttpResponse.json(
					{ code: 3002, message: "認證編號格式不正確", data: null },
					{ status: 400 },
				),
			),
		);
		await expect(
			submitProductCertificationApplication(PRODUCT_ID, {
				certificationType: "3701",
				certificateNo: "ISO 22000:2018",
			}),
		).rejects.toThrow();
	});

	it("重試同一申請時保留呼叫端的 idempotency key", async () => {
		const keys: unknown[] = [];
		server.use(
			http.post(
				`${BASE}/app/products/:id/certification_applications`,
				async ({ request }) => {
					keys.push(
						((await request.json()) as Record<string, unknown>).idempotencyKey,
					);
					return HttpResponse.json({ code: 0, data: APPROVED_RECORD });
				},
			),
		);
		const input = {
			certificationType: "3701",
			certificateNo: "1-009-300022",
			idempotencyKey: "cert-create-retry",
		};
		await submitProductCertificationApplication(PRODUCT_ID, input);
		await submitProductCertificationApplication(PRODUCT_ID, input);
		expect(keys).toEqual(["cert-create-retry", "cert-create-retry"]);
	});
});

describe("withdrawProductCertificationApplication", () => {
	it("POST 到 /{applicationId}/withdraw 並帶 expectedVersion", async () => {
		const seen: { path?: string; body?: Record<string, unknown> } = {};
		server.use(
			http.post(
				`${BASE}/app/products/:id/certification_applications/:applicationId/withdraw`,
				async ({ request }) => {
					seen.path = new URL(request.url).pathname;
					seen.body = (await request.json()) as Record<string, unknown>;
					return HttpResponse.json({ code: 0, message: "success", data: null });
				},
			),
		);

		await withdrawProductCertificationApplication(
			PRODUCT_ID,
			"843856384063471761",
			"2",
		);

		expect(seen.path).toContain(
			`/app/products/${PRODUCT_ID}/certification_applications/843856384063471761/withdraw`,
		);
		expect(seen.body).toMatchObject({ expectedVersion: "2" });
		expect(String(seen.body?.idempotencyKey)).toMatch(/^cert-wd-/);
	});
});
