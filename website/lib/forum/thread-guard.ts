import { Code, ConnectError } from "@connectrpc/connect";
import { socialPublicClient } from "@/lib/connect/social";
import { defaultLocale } from "@/lib/intl/locales";
import { canonicalThreadId } from "./thread-id";

/**
 * Request-shape decision for `/sellers/{id}/messages/{threadId}`, for `middleware.ts`.
 *
 * The thread page is statically prerendered, so a missing thread would be served
 * as a 200 soft-404 (`notFound()` inside a prerender cannot set the status; same
 * reason as the storefront decisions in lib/seller/canonical-redirect.ts). A real
 * 404 has to come from the middleware, which asks the backend whether the thread exists.
 */
const THREAD_PATH = /^\/sellers\/([^/]+)\/messages\/([^/]+)\/?$/;

const LOOKUP_TIMEOUT_MS = 1_500;
const MEMO_TTL_MS = 30 * 60_000;
const MEMO_MAX_ENTRIES = 500;

/** Only a thread the backend confirmed is remembered; an absent one is asked again. */
const presentMemo = new Map<string, number>();

/** Test-only: drops the per-isolate memo so cases cannot leak into each other. */
export function __resetForumThreadMemoForTest(): void {
	presentMemo.clear();
}

export type ForumThreadLookup =
	| { kind: "present" }
	| { kind: "absent" }
	/** No usable answer: timeout, network error, 5xx. */
	| { kind: "unknown" };

export function parseThreadPath(
	pathname: string,
): { sellerId: string; threadId: string } | null {
	const match = THREAD_PATH.exec(pathname);
	return match ? { sellerId: match[1], threadId: match[2] } : null;
}

/** Asks `GetForumThread` anonymously; NotFound and InvalidArgument settle as absent. */
export async function lookupForumThread(
	storeId: string,
	threadId: string,
	visitorIp?: string | null,
): Promise<ForumThreadLookup> {
	const key = `${storeId}:${threadId}`;
	const at = presentMemo.get(key);
	if (at !== undefined && Date.now() - at < MEMO_TTL_MS) {
		return { kind: "present" };
	}
	try {
		await socialPublicClient({
			locale: defaultLocale,
			visitorIp: visitorIp ?? undefined,
		}).getForumThread(
			{ storeId, threadId, pageSize: 1 },
			{ timeoutMs: LOOKUP_TIMEOUT_MS },
		);
	} catch (err) {
		const code = ConnectError.from(err).code;
		if (code === Code.NotFound || code === Code.InvalidArgument) {
			return { kind: "absent" };
		}
		return { kind: "unknown" };
	}
	if (presentMemo.size >= MEMO_MAX_ENTRIES) presentMemo.clear();
	presentMemo.set(key, Date.now());
	return { kind: "present" };
}

/**
 * True when the request is for a thread that does not exist. A non-UUID thread id
 * is absent without a call; an unknown seller or an unusable answer passes through
 * (the page then renders as it would without this check).
 */
export async function isMissingForumThread(
	pathname: string,
	storeIdOf: (sellerId: string) => Promise<string | null>,
	lookup: (
		storeId: string,
		threadId: string,
	) => Promise<ForumThreadLookup> = lookupForumThread,
): Promise<boolean> {
	const parsed = parseThreadPath(pathname);
	if (!parsed) return false;
	const threadId = canonicalThreadId(parsed.threadId);
	if (!threadId) return true;
	const storeId = await storeIdOf(parsed.sellerId);
	if (!storeId) return false;
	return (await lookup(storeId, threadId)).kind === "absent";
}
