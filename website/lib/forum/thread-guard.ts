import { Code, ConnectError } from "@connectrpc/connect";
import { socialPublicClient } from "@/lib/connect/social";
import { defaultLocale } from "@/lib/intl/locales";
import { createPresentMemo } from "@/lib/seller/storefront-guard";
import { canonicalThreadId } from "./thread-id";

/**
 * Request-shape decision for `/sellers/{id}/messages/{threadId}`, for `middleware.ts`.
 *
 * The thread page is statically prerendered, so a missing thread would be served
 * as a 200 soft-404 (`notFound()` inside a prerender cannot set the status; same
 * reason as the storefront decisions in lib/seller/storefront-guard.ts). A real
 * 404 has to come from the middleware, which asks the backend whether the thread exists.
 */
const THREAD_PATH = /^\/sellers\/([^/]+)\/messages\/([^/]+)\/?$/;

const LOOKUP_TIMEOUT_MS = 1_500;
const MEMO_KEY_SEP = ":";

/** Only backend-confirmed "present" answers are kept (see `PRESENT_TTL_MS`); absent and errors never are. */
const presentMemo = createPresentMemo<true>();

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
	const key = `${storeId}${MEMO_KEY_SEP}${threadId}`;
	if (presentMemo.get(key)) return { kind: "present" };
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
			presentMemo.delete(key);
			return { kind: "absent" };
		}
		return { kind: "unknown" };
	}
	presentMemo.set(key, true);
	return { kind: "present" };
}

/**
 * The lowercase canonical path for a thread URL whose id is a UUID in another case, else null
 * (not a thread path, already canonical, or not a UUID). `middleware.ts` answers it with a 308.
 */
export function canonicalThreadPath(pathname: string): string | null {
	const parsed = parseThreadPath(pathname);
	if (!parsed) return null;
	const threadId = canonicalThreadId(parsed.threadId);
	if (!threadId || threadId === parsed.threadId) return null;
	return `/sellers/${parsed.sellerId}/messages/${threadId}`;
}

/**
 * True when the request is for a thread that does not exist. A non-UUID thread id
 * is absent without a call; an unconfirmed store (no `storeId`) or an unusable answer passes through
 * (the page then renders as it would without this check).
 */
export async function isMissingForumThread(
	pathname: string,
	storeId: string | null | undefined,
	lookup: (
		storeId: string,
		threadId: string,
	) => Promise<ForumThreadLookup> = lookupForumThread,
): Promise<boolean> {
	const parsed = parseThreadPath(pathname);
	if (!parsed) return false;
	const threadId = canonicalThreadId(parsed.threadId);
	if (!threadId) return true;
	if (!storeId) return false;
	return (await lookup(storeId, threadId)).kind === "absent";
}
