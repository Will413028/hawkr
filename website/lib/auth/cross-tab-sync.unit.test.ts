import { describe, expect, it } from "vitest";
import { parseSessionAnnouncement, peerAction } from "./cross-tab-sync";

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

describe("peerAction", () => {
	it("reloads a signed-in tab when the peer sees another account or a guest", () => {
		expect(
			peerAction({ accountId: null }, { status: "signedIn", accountId: "7" }),
		).toBe("reload");
		expect(
			peerAction({ accountId: "8" }, { status: "signedIn", accountId: "7" }),
		).toBe("reload");
	});

	it("only asks again when a guest tab hears about a sign-in", () => {
		expect(
			peerAction({ accountId: "7" }, { status: "guest", accountId: undefined }),
		).toBe("refresh");
	});

	it("does nothing when both tabs already agree", () => {
		expect(
			peerAction({ accountId: "7" }, { status: "signedIn", accountId: "7" }),
		).toBe("ignore");
		expect(
			peerAction(
				{ accountId: null },
				{ status: "guest", accountId: undefined },
			),
		).toBe("ignore");
	});

	it("ignores signals while the first server answer is still on its way", () => {
		expect(
			peerAction(
				{ accountId: "7" },
				{ status: "unknown", accountId: undefined },
			),
		).toBe("ignore");
	});

	it("re-asks while the server could not answer", () => {
		expect(
			peerAction(
				{ accountId: null },
				{ status: "unavailable", accountId: undefined },
			),
		).toBe("refresh");
	});
});
