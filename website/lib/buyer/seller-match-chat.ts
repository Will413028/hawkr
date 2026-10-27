/**
 * SSE client for the buyer「Hawkr Agent」seller-match 找貨能力。
 *   POST /app/buyers/me/assistant/seller-match
 *   body: { message, sessionId? }   — camelCase，coordinated with ai-service
 *   stream: text/event-stream
 *     event: text         data: "<chunk>"
 *     event: tool_result  data: {"type":"seller_candidates","candidates":[...]}
 *       一輪對話可能觸發多次 tool_result（LLM 把複合查詢拆成多次搜尋）——
 *       onCandidates 收到的是累積去重後的完整清單，不是單次事件的原始批次。
 *     event: done         data: { sessionId }
 *     event: error        data: "<msg>"
 */

import { streamAiChat } from "@/lib/ai-chat/stream-client";
import { parseJsonSafe } from "@/lib/api/json";

export interface SellerCandidate {
	/** Backend TSID (string-on-the-wire)。禁止改回 number — 64-bit TSID
	 *  超出 JS Number 安全範圍 (2^53)，見 lib/api/json.ts 的 preserveBigInts。 */
	id: string;
	brandName: string;
	category: string;
	productSummary: string;
	region: string;
	positioning: string;
	researchSummary: string;
	publicWebsite: string;
}

export interface SellerMatchChatHandlers {
	onText: (chunk: string) => void;
	onCandidates: (candidates: SellerCandidate[]) => void;
	onDone: (sessionId: string | null) => void;
	onError: (message: string) => void;
}

export interface SellerMatchChatRequest {
	message: string;
	sessionId?: string | null;
	signal?: AbortSignal;
}

const ENDPOINT = "/app/buyers/me/assistant/seller-match";

/**
 * Candidate websites come from a research/development list, not a platform
 * seller profile. Only turn ordinary public http(s) destinations into links;
 * malformed values and executable/custom schemes stay plain text in the UI.
 */
export function safeSellerWebsiteUrl(value: string): string | null {
	const raw = value.trim();
	if (!raw) return null;
	const withScheme = /^[a-z][a-z\d+.-]*:/i.test(raw) ? raw : `https://${raw}`;
	try {
		const url = new URL(withScheme);
		if (url.protocol !== "https:" && url.protocol !== "http:") return null;
		if (!url.hostname || url.username || url.password) return null;
		return url.href;
	} catch {
		return null;
	}
}

export async function streamSellerMatchChat(
	req: SellerMatchChatRequest,
	handlers: SellerMatchChatHandlers,
): Promise<void> {
	// One turn can trigger more than one search_candidate_sellers tool call
	// — the LLM sometimes splits a compound query (e.g. "雞肉或蛋品") into
	// two narrower searches, each arriving as its own tool_result event.
	// Accumulate across events instead of forwarding each batch as-is, or a
	// later call (including one that legitimately finds nothing) silently
	// overwrites an earlier call's real results in the caller's state.
	const seenCandidates = new Map<string, SellerCandidate>();
	let noIdFallbackCounter = 0;
	return streamAiChat(
		{
			endpoint: ENDPOINT,
			body: {
				message: req.message,
				...(req.sessionId ? { sessionId: req.sessionId } : {}),
			},
			signal: req.signal,
		},
		{
			onText: handlers.onText,
			onNamedEvent: (eventName, data) => {
				if (eventName !== "tool_result") return;
				try {
					// parseJsonSafe (not raw JSON.parse) — candidate.id is a 64-bit
					// TSID that exceeds Number.MAX_SAFE_INTEGER; a bare JSON.parse
					// silently rounds it (see lib/favorites/repo.ts for the exact
					// symptom: 841713639471484333 → …330).
					const parsed = parseJsonSafe<{
						type?: string;
						candidates?: unknown[];
					}>(data);
					if (parsed.type !== "seller_candidates") return;
					for (const c of parseSellerCandidates(parsed.candidates ?? [])) {
						// id defaults to "" when a candidate is malformed — fall back
						// to a unique key so several such candidates don't collapse
						// into one another in the map.
						const key = c.id || `__no-id-${noIdFallbackCounter++}`;
						seenCandidates.set(key, c);
					}
					handlers.onCandidates(Array.from(seenCandidates.values()));
				} catch {
					/* swallow malformed tool_result — onText already received narration */
				}
			},
			onDone: (data) => {
				let sessionId: string | null = null;
				try {
					const obj = parseJsonSafe<{ sessionId?: string }>(data);
					sessionId = obj?.sessionId ?? null;
				} catch {
					/* allow plain "done" with no payload */
				}
				handlers.onDone(sessionId);
			},
			onError: handlers.onError,
		},
	);
}

function parseSellerCandidates(raw: unknown[]): SellerCandidate[] {
	return raw
		.filter(
			(item): item is Record<string, unknown> =>
				typeof item === "object" && item !== null,
		)
		.map((j) => ({
			// Explicit public-safe projection for browser state. The source stream
			// may still contain contact fields until the backend ships a narrowed
			// DTO, but they must not cross this parser into React state.
			// String(), never Number() — j.id survived parseJsonSafe as either a
			// small JSON number or an already-quoted 16+ digit string; String()
			// renders both back to the exact decimal digits with no precision loss.
			id: String(j.id ?? ""),
			brandName: typeof j.brand_name === "string" ? j.brand_name : "",
			category: typeof j.category === "string" ? j.category : "",
			productSummary:
				typeof j.product_summary === "string" ? j.product_summary : "",
			region: typeof j.region === "string" ? j.region : "",
			positioning: typeof j.positioning === "string" ? j.positioning : "",
			researchSummary:
				typeof j.research_summary === "string" ? j.research_summary : "",
			publicWebsite:
				typeof j.public_website === "string" ? j.public_website : "",
		}));
}
