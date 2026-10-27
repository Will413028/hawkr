import { NextResponse } from "next/server";
import { z } from "zod";

const GOOGLE_FORM_URL =
	"https://docs.google.com/forms/d/e/1FAIpQLSefqVw5BrXD70RksYvw25ks0x4jD-JEb0zUenKk5JYQxlu3ow/formResponse";

const MAX_BODY_BYTES = 12_000;
const UPSTREAM_TIMEOUT_MS = 8_000;
const RATE_LIMIT_WINDOW_MS = 10 * 60 * 1_000;
const RATE_LIMIT_REQUESTS = 5;
const MAX_RATE_LIMIT_BUCKETS = 4_096;

const PRODUCT_TYPES = [
	"蔬果與農產",
	"肉品與水產",
	"加工食品",
	"飲品與甜點",
	"禮盒與其他",
] as const;

const PLAN_LABELS = {
	starter: "Free 免費（STARTER；月費 0 元＋成交服務費 6%）",
	growth: "Growth 成長（舊版提交，需人工確認）",
	pro: "Pro 專業（成交服務費 0%）",
	undecided: "尚未決定",
} as const;

const SERVICE_INTEREST_LABELS = {
	none: "暫不需要",
	managed: "加值代行包（每筆有效交易額外 6%；上架支援與精修電商圖）",
	launch: "開店上線包（NT$2,990／次）",
	content: "成交內容包（NT$5,990／次）",
	full: "完整行銷包（NT$7,990／次）",
} as const;

function billingCycleLabel(
	plan: keyof typeof PLAN_LABELS,
	billingCycle: "monthly" | "annual",
) {
	if (plan === "pro") {
		return billingCycle === "annual"
			? "年繳（NT$29,900／年，省 NT$5,980）"
			: "月繳（NT$2,990／月）";
	}

	if (plan === "starter") {
		return billingCycle === "annual"
			? "Free 無固定方案年費（Pro 年繳優惠不適用）"
			: "Free 無固定月費";
	}

	if (plan === "growth") {
		return `${billingCycle === "annual" ? "年繳" : "月繳"}偏好（舊版方案，需人工確認）`;
	}

	return `${billingCycle === "annual" ? "年繳" : "月繳"}偏好（方案與金額待確認）`;
}

const plainText = (maximum: number) =>
	z
		.string()
		.trim()
		.min(1)
		.max(maximum)
		.refine((value) =>
			Array.from(value).every((character) => {
				const codePoint = character.codePointAt(0) ?? 0;
				return codePoint >= 32 && codePoint !== 127;
			}),
		);

const optionalEmail = z
	.string()
	.trim()
	.max(254)
	.refine(
		(value) => value === "" || z.string().email().safeParse(value).success,
	);

const optionalPhone = z
	.string()
	.trim()
	.max(30)
	.refine(
		(value) =>
			value === "" ||
			(/^[0-9+\-() #]+$/u.test(value) &&
				(value.match(/\d/gu) ?? []).length >= 8),
	);

const sellerLeadSchema = z
	.object({
		brandName: plainText(120),
		contactName: plainText(80),
		phone: optionalPhone,
		email: optionalEmail,
		productType: z.enum(PRODUCT_TYPES),
		plan: z
			.enum(["starter", "growth", "pro", "undecided"])
			.optional()
			.default("undecided"),
		billingCycle: z.enum(["monthly", "annual"]).optional().default("monthly"),
		serviceInterest: z
			.enum(["none", "managed", "launch", "content", "full"])
			.optional()
			.default("none"),
	})
	.strict()
	.refine((lead) => lead.email !== "" || lead.phone !== "", {
		path: ["email"],
	});

type RateLimitBucket = {
	count: number;
	resetAt: number;
};

const rateLimitBuckets = new Map<string, RateLimitBucket>();

function jsonError(
	status: number,
	code: string,
	message: string,
	headers?: HeadersInit,
) {
	return NextResponse.json(
		{ ok: false, code, message },
		{
			status,
			headers: {
				"Cache-Control": "no-store",
				...headers,
			},
		},
	);
}

function isSameOrigin(request: Request): boolean {
	const origin = request.headers.get("origin");
	if (!origin) return false;

	try {
		return new URL(origin).origin === new URL(request.url).origin;
	} catch {
		return false;
	}
}

function clientRateLimitKey(request: Request): string {
	// Cloudflare overwrites this header before the Worker sees the request. Do
	// not fall back to x-forwarded-for, which an untrusted direct caller can
	// supply as an arbitrary string.
	return request.headers.get("cf-connecting-ip")?.trim() || "unknown-client";
}

function consumeRateLimit(request: Request, now = Date.now()) {
	for (const [key, bucket] of rateLimitBuckets) {
		if (bucket.resetAt <= now) rateLimitBuckets.delete(key);
	}

	if (rateLimitBuckets.size >= MAX_RATE_LIMIT_BUCKETS) {
		const oldestKey = rateLimitBuckets.keys().next().value;
		if (typeof oldestKey === "string") rateLimitBuckets.delete(oldestKey);
	}

	const key = clientRateLimitKey(request);
	const current = rateLimitBuckets.get(key);
	if (!current || current.resetAt <= now) {
		rateLimitBuckets.set(key, {
			count: 1,
			resetAt: now + RATE_LIMIT_WINDOW_MS,
		});
		return { allowed: true as const, retryAfterSeconds: 0 };
	}

	if (current.count >= RATE_LIMIT_REQUESTS) {
		return {
			allowed: false as const,
			retryAfterSeconds: Math.max(
				1,
				Math.ceil((current.resetAt - now) / 1_000),
			),
		};
	}

	current.count += 1;
	return { allowed: true as const, retryAfterSeconds: 0 };
}

function googleFormBody(lead: z.infer<typeof sellerLeadSchema>) {
	return new URLSearchParams({
		"entry.70596064": lead.contactName,
		"entry.2093562995": lead.email,
		"entry.80969214": lead.phone,
		"entry.939655911": "賣家方案聯絡需求",
		"entry.409767": [
			`品牌／農場名稱：${lead.brandName}`,
			`商品品類：${lead.productType}`,
			`想了解的方案：${PLAN_LABELS[lead.plan]}`,
			`付款週期：${billingCycleLabel(lead.plan, lead.billingCycle)}`,
			`想了解的人工服務：${SERVICE_INTEREST_LABELS[lead.serviceInterest]}`,
		].join("\n"),
		fvv: "1",
		pageHistory: "0",
	});
}

function isTimeoutError(error: unknown): boolean {
	return (
		typeof error === "object" &&
		error !== null &&
		"name" in error &&
		(error.name === "TimeoutError" || error.name === "AbortError")
	);
}

async function readBodyWithLimit(request: Request) {
	if (!request.body) return { ok: true as const, text: "" };

	const reader = request.body.getReader();
	const decoder = new TextDecoder();
	let byteLength = 0;
	let text = "";

	while (true) {
		const chunk = await reader.read();
		if (chunk.done) break;
		byteLength += chunk.value.byteLength;
		if (byteLength > MAX_BODY_BYTES) {
			await reader.cancel().catch(() => undefined);
			return { ok: false as const };
		}
		text += decoder.decode(chunk.value, { stream: true });
	}

	text += decoder.decode();
	return { ok: true as const, text };
}

/**
 * Public seller-interest intake. The browser talks only to this same-origin
 * boundary; this handler validates and forwards to the existing Google Form.
 * No lead fields are logged or returned.
 */
export async function POST(request: Request) {
	if (!isSameOrigin(request)) {
		return jsonError(
			403,
			"ORIGIN_NOT_ALLOWED",
			"無法驗證送出來源，請重新整理後再試。",
		);
	}

	if (
		request.headers.get("content-type")?.split(";", 1)[0]?.trim() !==
		"application/json"
	) {
		return jsonError(
			415,
			"UNSUPPORTED_MEDIA_TYPE",
			"送出格式不正確，請重新整理後再試。",
		);
	}

	const declaredLength = Number(request.headers.get("content-length"));
	if (Number.isFinite(declaredLength) && declaredLength > MAX_BODY_BYTES) {
		return jsonError(413, "PAYLOAD_TOO_LARGE", "填寫內容過長，請精簡後再試。");
	}

	const limit = consumeRateLimit(request);
	if (!limit.allowed) {
		return jsonError(429, "RATE_LIMITED", "送出次數過多，請稍後再試。", {
			"Retry-After": String(limit.retryAfterSeconds),
		});
	}

	const bodyResult = await readBodyWithLimit(request).catch(() => null);
	if (!bodyResult?.ok) {
		return jsonError(413, "PAYLOAD_TOO_LARGE", "填寫內容過長，請精簡後再試。");
	}

	let parsedBody: unknown;
	try {
		parsedBody = JSON.parse(bodyResult.text);
	} catch {
		return jsonError(400, "INVALID_REQUEST", "請檢查填寫內容後再試。");
	}

	const parsedLead = sellerLeadSchema.safeParse(parsedBody);
	if (!parsedLead.success) {
		return jsonError(400, "INVALID_REQUEST", "請檢查填寫內容後再試。");
	}

	let upstream: Response;
	try {
		upstream = await fetch(GOOGLE_FORM_URL, {
			method: "POST",
			headers: {
				Accept: "text/html",
				"Accept-Language": "zh-TW,zh;q=0.9,en;q=0.7",
				"Content-Type": "application/x-www-form-urlencoded;charset=UTF-8",
			},
			body: googleFormBody(parsedLead.data),
			cache: "no-store",
			redirect: "follow",
			signal: AbortSignal.timeout(UPSTREAM_TIMEOUT_MS),
		});
	} catch (error) {
		if (isTimeoutError(error)) {
			return jsonError(
				504,
				"UPSTREAM_TIMEOUT",
				"聯絡需求暫時無法送出，請稍後再試。",
			);
		}
		return jsonError(
			502,
			"UPSTREAM_UNAVAILABLE",
			"聯絡需求暫時無法送出，請稍後再試。",
		);
	}
	await upstream.body?.cancel().catch(() => undefined);

	// Unlike a browser no-cors request, a server-side response has an observable
	// status. Only an explicit successful response from the fixed Google Forms
	// destination is treated as accepted.
	if (upstream.type === "opaque" || !upstream.ok) {
		return jsonError(
			502,
			"UPSTREAM_UNAVAILABLE",
			"聯絡需求暫時無法送出，請稍後再試。",
		);
	}

	return NextResponse.json(
		{ ok: true, message: "application received" },
		{ headers: { "Cache-Control": "no-store" } },
	);
}
