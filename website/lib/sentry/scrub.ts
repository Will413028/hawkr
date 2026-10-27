export type SentryLikeEvent = {
	request?: {
		cookies?: unknown;
		headers?: Record<string, unknown>;
		data?: unknown;
	};
	user?: {
		email?: string;
		ip_address?: string | null;
		[key: string]: unknown;
	};
};

const SENSITIVE_HEADER_NAMES = new Set([
	"authorization",
	"cookie",
	"set-cookie",
	"x-csrf-token",
	"x-xsrf-token",
]);

const SENSITIVE_BODY_KEYS = new Set([
	"accessToken",
	"access_token",
	"authorization",
	"cookie",
	"email",
	"password",
	"refreshToken",
	"refresh_token",
	"token",
]);

const SENSITIVE_BODY_KEY_PATTERNS = [
	"authorization",
	"bankaccount",
	"birthday",
	"card",
	"contactphone",
	"cookie",
	"creditcard",
	"csrf",
	"cvv",
	"email",
	"expire",
	"expiry",
	"fullName",
	"idnumber",
	"identity",
	"mobile",
	"nationalid",
	"password",
	"payuni",
	"payment",
	"phone",
	"recipient",
	"receiver",
	"refresh",
	"secret",
	"session",
	"shippingaddress",
	"token",
	"wallet",
	"xsrf",
].map((key) => key.toLowerCase());

const SENSITIVE_BODY_KEY_ALIASES = new Set(
	[
		"address",
		"buyername",
		"cardno",
		"cardnumber",
		"contactname",
		"cvc",
		"cvv",
		"nameoncard",
		"pan",
		"recipientname",
		"receivername",
		"tel",
	].map((key) => key.toLowerCase()),
);

function normalizeKey(key: string): string {
	return key.replace(/[_\-\s]/g, "").toLowerCase();
}

function isSensitiveBodyKey(key: string): boolean {
	const normalized = normalizeKey(key);
	return (
		SENSITIVE_BODY_KEYS.has(key) ||
		SENSITIVE_BODY_KEYS.has(normalized) ||
		SENSITIVE_BODY_KEY_ALIASES.has(normalized) ||
		SENSITIVE_BODY_KEY_PATTERNS.some((pattern) => normalized.includes(pattern))
	);
}

function scrubData(value: unknown): unknown {
	if (!value || typeof value !== "object") return value;
	if (Array.isArray(value)) return value.map(scrubData);

	const out: Record<string, unknown> = {};
	for (const [key, item] of Object.entries(value)) {
		out[key] = isSensitiveBodyKey(key) ? "[Filtered]" : scrubData(item);
	}
	return out;
}

export function scrubSentryEvent<T extends SentryLikeEvent>(event: T): T {
	if (event.request?.cookies) delete event.request.cookies;

	if (event.request?.headers) {
		for (const key of Object.keys(event.request.headers)) {
			if (SENSITIVE_HEADER_NAMES.has(key.toLowerCase())) {
				delete event.request.headers[key];
			}
		}
	}

	if (event.request?.data) {
		event.request.data = scrubData(event.request.data);
	}

	if (event.user) {
		delete event.user.email;
		delete event.user.ip_address;
	}

	return event;
}
