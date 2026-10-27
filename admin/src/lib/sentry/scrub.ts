type SentryLikeEvent = {
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
	"x-delegated-operation-session",
	"x-hk-operation-session",
	"x-operation-session",
	"x-csrf-token",
	"x-xsrf-token",
]);

const SENSITIVE_BODY_KEYS = new Set([
	"accesstoken",
	"authorization",
	"cookie",
	"operationcredential",
	"operationsession",
	"password",
	"refreshtoken",
	"token",
]);

function normalizedSensitiveKey(value: string): string {
	return value.toLowerCase().replaceAll(/[-_]/gu, "");
}

function isSensitiveHeaderName(value: string): boolean {
	const lower = value.toLowerCase();
	return (
		SENSITIVE_HEADER_NAMES.has(lower) ||
		lower.startsWith("x-operation-") ||
		lower.startsWith("x-hk-operation-") ||
		lower.startsWith("x-delegated-operation-")
	);
}

function isSensitiveBodyKey(value: string): boolean {
	const normalized = normalizedSensitiveKey(value);
	return (
		SENSITIVE_BODY_KEYS.has(normalized) ||
		(normalized.startsWith("operation") &&
			(normalized.includes("session") || normalized.includes("credential")))
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
			if (isSensitiveHeaderName(key)) {
				delete event.request.headers[key];
			}
		}
	}

	if (event.request?.data) {
		event.request.data =
			typeof event.request.data === "string"
				? "[Filtered]"
				: scrubData(event.request.data);
	}

	if (event.user) {
		delete event.user.email;
		delete event.user.ip_address;
	}

	return event;
}
