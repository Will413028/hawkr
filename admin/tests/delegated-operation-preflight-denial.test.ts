import assert from "node:assert/strict";
import test, { afterEach, beforeEach, mock } from "node:test";
import { NextRequest } from "next/server";
import {
	isDelegatedOperationPreflightDenied,
	parseDelegatedOperationPreflightDenial,
} from "../src/features/delegated-operations/model";
import { adminSession } from "../src/lib/session";

/** Access JWT（測試以 mock 取代來源）；null 是沒有憑證。 */
let adminToken: string | null = "qa-admin-session-token";
beforeEach(() => {
	adminToken = "qa-admin-session-token";
	mock.method(adminSession, "token", async () => adminToken);
});
afterEach(() => {
	mock.restoreAll();
});

/**
 * preflight 被拒時，「為什麼」必須一路走到畫面上。
 *
 * 修前：BFF 把 `deniedCode`／`deniedLabel` 整個丟掉，一律回同一句「此店家目前不可
 * 建立代行工作階段」。而 9269（別的管理員正在代行）、9264/9265（尚未開通）、
 * 9272（店家已停權）三者管理員的下一步完全不同 —— 等後者結束、去開通、或根本不該
 * 代行。後端已經逐碼下發中文，前端沒接。
 */

const storeId = "2685914682";
const target = { storeId, sellerAccountId: storeId, storeName: "陳記" };
const actor = { adminId: "2661027599", displayName: "alexischen" };
const reasonOptions = [
	{ code: "seller_request", label: "賣家請求", caseReferenceRequired: false },
];
// ⚠️ 拒絕時 reasonOptions／capabilities 仍必須非空（56 §6.1.2）：parser 在讀到
// allowed 之前就對這兩欄做非空校驗，空陣列會讓整份回應被判契約錯誤。
const capabilities = ["storefront.read"];

function deniedPayload(deniedCode: number, deniedLabel: unknown) {
	return {
		allowed: false,
		target,
		actor,
		reasonOptions,
		capabilities,
		ttlSeconds: 600,
		idleSeconds: 900,
		caseReferenceRequired: false,
		deniedCode,
		deniedLabel,
	};
}

async function callPreflight(upstream: () => Response) {
	const { GET } = await import(
		"../src/app/api/admin/merchants/[storeId]/delegated-operation/preflight/route"
	);
	const realFetch = globalThis.fetch;
	globalThis.fetch = async () => upstream();
	try {
		const request = new NextRequest(
			`http://localhost/api/admin/merchants/${storeId}/delegated-operation/preflight`,
		);
		const response = await GET(request, {
			params: Promise.resolve({ storeId }),
		});
		return { status: response.status, body: await response.json() };
	} finally {
		globalThis.fetch = realFetch;
	}
}

test("parser 保留 deniedCode／deniedLabel，並擋掉不合形狀的標籤", () => {
	const parsed = parseDelegatedOperationPreflightDenial(
		deniedPayload(9269, "此店家已有其他管理員正在代行"),
	);
	assert.deepEqual(parsed, {
		allowed: false,
		deniedCode: 9269,
		deniedLabel: "此店家已有其他管理員正在代行",
	});

	// 上游來的自由文字會直接進畫面，形狀不對就不採用（退回通用句，而不是照印）。
	for (const badLabel of [
		"",
		"  前後有空白  ",
		"含\u0007控制字元",
		"長".repeat(121),
		42,
		null,
	]) {
		assert.equal(
			parseDelegatedOperationPreflightDenial(deniedPayload(9269, badLabel)),
			null,
			`deniedLabel=${JSON.stringify(badLabel)} 不該被採用`,
		);
	}
	// 缺碼也不採用：只有標籤沒有碼，客服無從追。
	assert.equal(
		parseDelegatedOperationPreflightDenial(deniedPayload(0, "沒有碼")),
		null,
	);

	// 但「這是不是一次拒絕」在上面每一種輸入下都必須仍然成立 —— 兩支分開的理由。
	for (const badLabel of ["", 42, null]) {
		assert.equal(
			isDelegatedOperationPreflightDenied(deniedPayload(9269, badLabel)),
			true,
		);
	}
});

test("BFF 把後端下發的拒絕原因原樣帶到前端（三種碼各自不同句）", async () => {
	const cases = [
		{ code: 9269, label: "此店家已有其他管理員正在代行" },
		{ code: 9265, label: "店家已關閉，無法代行。" },
		{ code: 9272, label: "帳號停權中，無法代行。" },
		{ code: 9264, label: "此店家尚未開通代行營運" },
	];
	const seen = new Set<string>();
	for (const { code, label } of cases) {
		const { status, body } = await callPreflight(() =>
			Response.json({ code: 0, data: deniedPayload(code, label) }),
		);
		assert.equal(status, 403);
		assert.equal(body.code, "DELEGATED_OPERATION_DENIED");
		assert.equal(body.message, label);
		assert.equal(body.deniedCode, code);
		seen.add(body.message);
	}
	// 鑑別力：四個碼必須產生四句不同的話。全部壓成同一句正是修前的行為。
	assert.equal(seen.size, cases.length);
});

test("拒絕原因缺席時退回通用句，仍然是 403 而不是契約錯誤", async () => {
	const { status, body } = await callPreflight(() =>
		Response.json({ code: 0, data: deniedPayload(0, undefined) }),
	);
	assert.equal(status, 403);
	assert.equal(body.code, "DELEGATED_OPERATION_DENIED");
	assert.equal(body.message, "此店家目前不可建立代行工作階段");
	assert.equal(body.deniedCode, undefined);
});

test("允許時不得把 deniedCode／deniedLabel 洩進 data", async () => {
	const { status, body } = await callPreflight(() =>
		Response.json({
			code: 0,
			data: {
				allowed: true,
				target,
				actor,
				reasonOptions,
				capabilities,
				ttlSeconds: 600,
				idleSeconds: 900,
				caseReferenceRequired: false,
				deniedCode: 0,
				deniedLabel: "",
			},
		}),
	);
	assert.equal(status, 200);
	assert.equal(body.code, 0);
	assert.equal(body.data.allowed, true);
	assert.equal("deniedCode" in body.data, false);
	assert.equal("deniedLabel" in body.data, false);
});

test("service 把 deniedCode 一路帶到瀏覽器端的例外物件上", async () => {
	const { getDelegatedOperationPreflight, DelegatedOperationClientError } =
		await import("../src/services/delegated-operations");
	const realFetch = globalThis.fetch;
	globalThis.fetch = async () =>
		Response.json(
			{
				code: "DELEGATED_OPERATION_DENIED",
				message: "此店家已有其他管理員正在代行",
				deniedCode: 9269,
			},
			{ status: 403 },
		);
	try {
		await assert.rejects(
			() => getDelegatedOperationPreflight(storeId),
			(error: unknown) => {
				assert.ok(error instanceof DelegatedOperationClientError);
				assert.equal(error.status, 403);
				// 這兩件是 UI 分別要顯示的東西：句子給人看，碼給客服追。
				assert.equal(error.message, "此店家已有其他管理員正在代行");
				assert.equal(error.deniedCode, 9269);
				return true;
			},
		);
		// 不合形狀的碼不得被當成碼帶上去（0／負數／非整數都不是業務碼）。
		for (const bad of [0, -1, 1.5, "9269"]) {
			globalThis.fetch = async () =>
				Response.json(
					{
						code: "DELEGATED_OPERATION_DENIED",
						message: "被拒",
						deniedCode: bad,
					},
					{ status: 403 },
				);
			await assert.rejects(
				() => getDelegatedOperationPreflight(storeId),
				(error: unknown) => {
					assert.ok(error instanceof DelegatedOperationClientError);
					assert.equal(error.deniedCode, undefined, `deniedCode=${bad}`);
					return true;
				},
			);
		}
	} finally {
		globalThis.fetch = realFetch;
	}
});
