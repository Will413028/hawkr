import { describe, expect, it } from "vitest";
import { resolveChatViewerId } from "../chatIdentity";

// Chat wire ids are CURRENT-stint surrogates, and since backend commit
// 2269417e3 (2026-07-18, "never grandfather buyer_id/store_id to account_id")
// they are freshly minted 64-bit TSIDs — 18-19 digits, well past 2^53. The
// fixtures below use realistic full-width values on purpose: a fixture that
// fits in a JS number would hide the precision half of this bug, and a fixture
// reusing the same value for accountId and buyer_id would make every assertion
// pass regardless of which id the code actually picked (exactly how the
// original bug stayed invisible on grandfathered pre-2026-07-18 accounts).
// Annotated as `string` (not literal types) so the comparisons below stay
// genuine runtime checks — tsc narrows bare literals and rejects them as
// provably-disjoint.
const RAW_ACCOUNT_ID: string = "2623900520";
const RESOLVED_BUYER_ID: string = "7303333333333333333";
const RESOLVED_STORE_ID: string = "857045288972403572";

describe("resolveChatViewerId", () => {
	it("buyer surface resolves to the buyer_id, not the raw accountId", () => {
		const myId = resolveChatViewerId({
			isSeller: false,
			storeId: RESOLVED_STORE_ID,
			buyerId: RESOLVED_BUYER_ID,
		});
		expect(myId).toBe(RESOLVED_BUYER_ID);
		expect(myId).not.toBe(RAW_ACCOUNT_ID);
	});

	it("seller surface resolves to the current store_id, not the buyer_id", () => {
		const myId = resolveChatViewerId({
			isSeller: true,
			storeId: RESOLVED_STORE_ID,
			buyerId: RESOLVED_BUYER_ID,
		});
		expect(myId).toBe(RESOLVED_STORE_ID);
		expect(myId).not.toBe(RESOLVED_BUYER_ID);
	});

	it("preserves full 64-bit TSID precision (Number() would corrupt it)", () => {
		const myId = resolveChatViewerId({
			isSeller: false,
			storeId: null,
			buyerId: RESOLVED_BUYER_ID,
		});
		// The exact decimal string must survive — this is what `senderId`
		// (lib/chat/repo.ts normalizeMessage) is compared against with `===`.
		expect(myId).toBe("7303333333333333333");
		expect(myId).not.toBe(String(Number(RESOLVED_BUYER_ID)));
	});

	it("accepts a numeric id and still yields its decimal string", () => {
		expect(
			resolveChatViewerId({ isSeller: false, storeId: null, buyerId: 42 }),
		).toBe("42");
	});

	it("is empty while the profile fetch is still in flight", () => {
		expect(
			resolveChatViewerId({
				isSeller: false,
				storeId: RESOLVED_STORE_ID,
				buyerId: undefined,
			}),
		).toBe("");
		expect(
			resolveChatViewerId({ isSeller: true, storeId: null, buyerId: 1 }),
		).toBe("");
	});

	it("treats an empty-string id as unresolved rather than a real id", () => {
		expect(
			resolveChatViewerId({ isSeller: true, storeId: "", buyerId: 1 }),
		).toBe("");
	});
});

describe("regression: own messages must not render as the peer's", () => {
	// The shipped bug: myId was String(identity.accountId) while the wire
	// carries the resolved surrogate, so `m.senderId === myId` was false for
	// the viewer's OWN messages (bubble alignment flips) and the useChat
	// inbound filter `m.receiverId === myId` dropped every live peer message.
	it("a buyer's own message (senderId = their buyer_id) is detected as mine", () => {
		const myId = resolveChatViewerId({
			isSeller: false,
			storeId: null,
			buyerId: RESOLVED_BUYER_ID,
		});
		expect(RESOLVED_BUYER_ID === myId).toBe(true);
	});

	it("old buggy behaviour reproduced for contrast", () => {
		const buggyMyId = RAW_ACCOUNT_ID; // what identity.accountId alone produced
		expect(RESOLVED_BUYER_ID === buggyMyId).toBe(false);
	});
});
