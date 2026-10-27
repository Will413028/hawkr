import { afterEach, describe, expect, it, vi } from "vitest";
import { POST } from "../route";

const ENDPOINT = "https://example.com/api/public/seller-leads";
const GOOGLE_FORM_URL =
	"https://docs.google.com/forms/d/e/1FAIpQLSefqVw5BrXD70RksYvw25ks0x4jD-JEb0zUenKk5JYQxlu3ow/formResponse";

const VALID_LEAD = {
	brandName: "山腳農園",
	contactName: "王小明",
	phone: "0912-345-678",
	email: "seller@example.com",
	productType: "蔬果與農產",
};

let requestSequence = 0;

function sellerLeadRequest(
	body: unknown = VALID_LEAD,
	options: {
		contentType?: string;
		ip?: string;
		origin?: string | null;
		rawBody?: string;
		extraHeaders?: Record<string, string>;
	} = {},
) {
	requestSequence += 1;
	const headers = new Headers({
		"content-type": options.contentType ?? "application/json;charset=UTF-8",
		"cf-connecting-ip": options.ip ?? `203.0.113.${requestSequence}`,
		...options.extraHeaders,
	});
	if (options.origin !== null) {
		headers.set("origin", options.origin ?? "https://example.com");
	}

	return new Request(ENDPOINT, {
		method: "POST",
		headers,
		body: options.rawBody ?? JSON.stringify(body),
	});
}

function stubUpstream(
	response: Response = new Response("recorded", { status: 200 }),
) {
	const mock = vi.fn(
		async (_input: RequestInfo | URL, _init?: RequestInit): Promise<Response> =>
			response,
	);
	vi.stubGlobal("fetch", mock);
	return mock;
}

afterEach(() => {
	vi.unstubAllGlobals();
});

describe("POST /api/public/seller-leads", () => {
	it("validates and maps a same-origin lead to the existing Google Form", async () => {
		const fetchMock = stubUpstream();

		const response = await POST(sellerLeadRequest());

		expect(response.status).toBe(200);
		expect(response.headers.get("cache-control")).toBe("no-store");
		expect(await response.json()).toEqual({
			ok: true,
			message: "application received",
		});
		expect(fetchMock).toHaveBeenCalledTimes(1);

		const [url, init] = fetchMock.mock.calls[0] ?? [];
		expect(url).toBe(GOOGLE_FORM_URL);
		expect(init).toMatchObject({
			method: "POST",
			cache: "no-store",
			redirect: "follow",
		});
		expect(init?.headers).toMatchObject({
			"Content-Type": "application/x-www-form-urlencoded;charset=UTF-8",
		});

		const form = init?.body;
		expect(form).toBeInstanceOf(URLSearchParams);
		const fields = form as URLSearchParams;
		expect(fields.get("entry.70596064")).toBe(VALID_LEAD.contactName);
		expect(fields.get("entry.2093562995")).toBe(VALID_LEAD.email);
		expect(fields.get("entry.80969214")).toBe(VALID_LEAD.phone);
		expect(fields.get("entry.939655911")).toBe("賣家方案聯絡需求");
		expect(fields.get("entry.409767")).toBe(
			[
				"品牌／農場名稱：山腳農園",
				"商品品類：蔬果與農產",
				"想了解的方案：尚未決定",
				"付款週期：月繳偏好（方案與金額待確認）",
				"想了解的人工服務：暫不需要",
			].join("\n"),
		);
	});

	it.each([
		["email only", { ...VALID_LEAD, phone: "" }],
		["phone only", { ...VALID_LEAD, email: "" }],
	] as const)("accepts a valid %s contact method", async (_case, lead) => {
		const fetchMock = stubUpstream();

		const response = await POST(sellerLeadRequest(lead));

		expect(response.status).toBe(200);
		expect(fetchMock).toHaveBeenCalledTimes(1);
		const body = fetchMock.mock.calls[0]?.[1]?.body as URLSearchParams;
		expect(body.get("entry.2093562995")).toBe(lead.email);
		expect(body.get("entry.80969214")).toBe(lead.phone);
	});

	it.each([
		["starter", "Free 免費（STARTER；月費 0 元＋成交服務費 6%）"],
		["growth", "Growth 成長（舊版提交，需人工確認）"],
		["pro", "Pro 專業（成交服務費 0%）"],
		["undecided", "尚未決定"],
	] as const)("accepts the explicit %s plan value", async (plan, label) => {
		const fetchMock = stubUpstream();
		const response = await POST(sellerLeadRequest({ ...VALID_LEAD, plan }));

		expect(response.status).toBe(200);
		const body = fetchMock.mock.calls[0]?.[1]?.body as URLSearchParams;
		expect(body.get("entry.409767")).toContain(`想了解的方案：${label}`);
	});

	it.each([
		["pro", "monthly", "月繳（NT$2,990／月）"],
		["pro", "annual", "年繳（NT$29,900／年，省 NT$5,980）"],
		["starter", "monthly", "Free 無固定月費"],
		["starter", "annual", "Free 無固定方案年費（Pro 年繳優惠不適用）"],
		["undecided", "monthly", "月繳偏好（方案與金額待確認）"],
		["undecided", "annual", "年繳偏好（方案與金額待確認）"],
		["growth", "monthly", "月繳偏好（舊版方案，需人工確認）"],
		["growth", "annual", "年繳偏好（舊版方案，需人工確認）"],
	] as const)("maps %s with %s billing to the plan-aware label", async (plan, billingCycle, label) => {
		const fetchMock = stubUpstream();
		const response = await POST(
			sellerLeadRequest({ ...VALID_LEAD, plan, billingCycle }),
		);

		expect(response.status).toBe(200);
		const body = fetchMock.mock.calls[0]?.[1]?.body as URLSearchParams;
		const summary = body.get("entry.409767") ?? "";
		expect(summary).toContain(`付款週期：${label}`);
		if (plan !== "pro") {
			expect(summary).not.toContain("NT$29,900");
			expect(summary).not.toContain("NT$5,980");
		}
	});

	it.each([
		"starter",
		"undecided",
	] as const)("never describes the %s annual preference as the Pro annual fee", async (plan) => {
		const fetchMock = stubUpstream();
		const response = await POST(
			sellerLeadRequest({ ...VALID_LEAD, plan, billingCycle: "annual" }),
		);

		expect(response.status).toBe(200);
		const body = fetchMock.mock.calls[0]?.[1]?.body as URLSearchParams;
		const summary = body.get("entry.409767") ?? "";
		expect(summary).not.toContain("付款週期：年繳（NT$29,900／年");
		expect(summary).not.toContain("省 NT$5,980");
	});

	it.each([
		["none", "暫不需要"],
		["managed", "加值代行包（每筆有效交易額外 6%；上架支援與精修電商圖）"],
		["launch", "開店上線包（NT$2,990／次）"],
		["content", "成交內容包（NT$5,990／次）"],
		["full", "完整行銷包（NT$7,990／次）"],
	] as const)("accepts the explicit %s service interest value", async (serviceInterest, label) => {
		const fetchMock = stubUpstream();
		const response = await POST(
			sellerLeadRequest({ ...VALID_LEAD, serviceInterest }),
		);

		expect(response.status).toBe(200);
		const body = fetchMock.mock.calls[0]?.[1]?.body as URLSearchParams;
		expect(body.get("entry.409767")).toContain(`想了解的人工服務：${label}`);
	});

	it.each([
		["missing required field", { ...VALID_LEAD, brandName: "" }],
		["missing contact method", { ...VALID_LEAD, email: "", phone: "" }],
		["unknown plan", { ...VALID_LEAD, plan: "enterprise" }],
		["unknown billing cycle", { ...VALID_LEAD, billingCycle: "quarterly" }],
		["unknown service interest", { ...VALID_LEAD, serviceInterest: "vip" }],
		[
			"removed feature interest",
			{ ...VALID_LEAD, featureInterest: "growth_tools" },
		],
		["retired plan id", { ...VALID_LEAD, plan: "scale" }],
		[
			"retired service interest",
			{ ...VALID_LEAD, serviceInterest: "one_time" },
		],
		["invalid email", { ...VALID_LEAD, email: "not-an-email" }],
		["invalid phone", { ...VALID_LEAD, phone: "call me now" }],
		["unknown product type", { ...VALID_LEAD, productType: "anything" }],
		["removed sales channel", { ...VALID_LEAD, salesChannel: "社群販售" }],
		["removed monthly orders", { ...VALID_LEAD, monthlyOrders: "51–300" }],
		["unexpected keys", { ...VALID_LEAD, admin: true }],
	])("rejects %s without contacting Google", async (_case, body) => {
		const fetchMock = stubUpstream();
		const response = await POST(sellerLeadRequest(body));

		expect(response.status).toBe(400);
		expect(await response.json()).toEqual({
			ok: false,
			code: "INVALID_REQUEST",
			message: "請檢查填寫內容後再試。",
		});
		expect(fetchMock).not.toHaveBeenCalled();
	});

	it.each([
		null,
		"https://evil.example",
		"null",
	])("rejects the untrusted Origin %s before parsing or forwarding", async (origin) => {
		const fetchMock = stubUpstream();
		const response = await POST(sellerLeadRequest(VALID_LEAD, { origin }));

		expect(response.status).toBe(403);
		expect(fetchMock).not.toHaveBeenCalled();
	});

	it("rejects non-JSON and oversized bodies before forwarding", async () => {
		const fetchMock = stubUpstream();

		const wrongType = await POST(
			sellerLeadRequest(VALID_LEAD, { contentType: "text/plain" }),
		);
		const declaredOversize = await POST(
			sellerLeadRequest(VALID_LEAD, {
				extraHeaders: { "content-length": "12001" },
			}),
		);
		const actualOversize = await POST(
			sellerLeadRequest(VALID_LEAD, {
				rawBody: JSON.stringify({
					...VALID_LEAD,
					brandName: "農".repeat(12_001),
				}),
			}),
		);

		expect(wrongType.status).toBe(415);
		expect(declaredOversize.status).toBe(413);
		expect(actualOversize.status).toBe(413);
		expect(fetchMock).not.toHaveBeenCalled();
	});

	it("returns a recoverable privacy-safe error unless Google explicitly succeeds", async () => {
		const fetchMock = stubUpstream(new Response("failed", { status: 503 }));

		const response = await POST(sellerLeadRequest());
		const body = await response.json();

		expect(response.status).toBe(502);
		expect(body).toEqual({
			ok: false,
			code: "UPSTREAM_UNAVAILABLE",
			message: "聯絡需求暫時無法送出，請稍後再試。",
		});
		expect(JSON.stringify(body)).not.toContain(VALID_LEAD.brandName);
		expect(JSON.stringify(body)).not.toContain(VALID_LEAD.email);
		expect(fetchMock).toHaveBeenCalledTimes(1);
	});

	it("distinguishes an upstream timeout without leaking the exception", async () => {
		const fetchMock = vi.fn(async () => {
			throw new DOMException("private upstream detail", "TimeoutError");
		});
		vi.stubGlobal("fetch", fetchMock);

		const response = await POST(sellerLeadRequest());
		const body = await response.json();

		expect(response.status).toBe(504);
		expect(body.code).toBe("UPSTREAM_TIMEOUT");
		expect(JSON.stringify(body)).not.toContain("private upstream detail");
	});

	it("throttles repeated submissions before a sixth upstream write", async () => {
		const fetchMock = stubUpstream();
		const statuses: number[] = [];

		for (let index = 0; index < 6; index += 1) {
			const response = await POST(
				sellerLeadRequest(VALID_LEAD, { ip: "198.51.100.77" }),
			);
			statuses.push(response.status);
			if (index === 5) {
				expect(response.headers.get("retry-after")).toMatch(/^\d+$/u);
			}
		}

		expect(statuses).toEqual([200, 200, 200, 200, 200, 429]);
		expect(fetchMock).toHaveBeenCalledTimes(5);
	});
});
