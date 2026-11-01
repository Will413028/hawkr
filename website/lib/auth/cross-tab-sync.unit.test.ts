import { describe, expect, it } from "vitest";
import {
	parseSessionAnnouncement,
	shouldRefreshForPeer,
} from "./cross-tab-sync";

describe("parseSessionAnnouncement", () => {
	it.each([
		[{ accountId: "7" }, { accountId: "7" }],
		[{ accountId: null }, { accountId: null }],
	])("accepts %j", (data, expected) => {
		expect(parseSessionAnnouncement(data)).toEqual(expected);
	});

	it.each([
		undefined,
		null,
		"7",
		{},
		{ accountId: "" },
		{ accountId: 7 },
		{ accountId: undefined },
	])("rejects %j: an unknown or empty value is never an account", (data) => {
		expect(parseSessionAnnouncement(data)).toBeNull();
	});
});

describe("shouldRefreshForPeer", () => {
	it("refreshes when the peer sees a different account or a guest", () => {
		expect(
			shouldRefreshForPeer(
				{ accountId: null },
				{ status: "signedIn", accountId: "7" },
			),
		).toBe(true);
		expect(
			shouldRefreshForPeer(
				{ accountId: "7" },
				{ status: "guest", accountId: undefined },
			),
		).toBe(true);
		expect(
			shouldRefreshForPeer(
				{ accountId: "8" },
				{ status: "signedIn", accountId: "7" },
			),
		).toBe(true);
	});

	it("does nothing when both tabs already agree", () => {
		expect(
			shouldRefreshForPeer(
				{ accountId: "7" },
				{ status: "signedIn", accountId: "7" },
			),
		).toBe(false);
		expect(
			shouldRefreshForPeer(
				{ accountId: null },
				{ status: "guest", accountId: undefined },
			),
		).toBe(false);
	});

	it("ignores signals while the first server answer is still on its way", () => {
		expect(
			shouldRefreshForPeer(
				{ accountId: "7" },
				{ status: "unknown", accountId: undefined },
			),
		).toBe(false);
	});

	it("re-asks while the server could not answer", () => {
		expect(
			shouldRefreshForPeer(
				{ accountId: null },
				{ status: "unavailable", accountId: undefined },
			),
		).toBe(true);
	});
});
