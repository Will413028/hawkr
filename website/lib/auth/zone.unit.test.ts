import { describe, expect, it } from "vitest";
import type { RequiredAccess } from "./redirect";
import { defaultPathForRole } from "./redirect";
import type { SessionState } from "./session-state";
import { decideZone } from "./zone";

const guest: SessionState = { status: "guest" };
const buyer: SessionState = {
	status: "signedIn",
	identity: { accountId: "7", isSeller: false },
};
const seller: SessionState = {
	status: "signedIn",
	identity: { accountId: "8", isSeller: true },
};
const unavailable: SessionState = { status: "unavailable" };

describe("decideZone: who may open a protected zone", () => {
	it.each<[RequiredAccess, string, string]>([
		[
			"buyer",
			"/account/orders?tab=paid",
			"/login?role=buyer&redirect=%2Faccount%2Forders%3Ftab%3Dpaid",
		],
		["buyer", "/checkout", "/login?role=buyer&redirect=%2Fcheckout"],
		[
			"seller",
			"/seller-center/dashboard",
			"/login?role=seller&redirect=%2Fseller-center%2Fdashboard",
		],
		[
			"account",
			"/seller/measures/9",
			"/login?redirect=%2Fseller%2Fmeasures%2F9",
		],
	])("a guest on %s zone %s goes to the login page the middleware would use", (required, returnPath, href) => {
		expect(decideZone(required, guest, returnPath)).toEqual({
			kind: "redirect",
			href,
			locale: "zh-TW",
		});
	});

	it("a guest keeps the visitor's locale on the way to login", () => {
		expect(decideZone("buyer", guest, "/en/account")).toEqual({
			kind: "redirect",
			href: "/login?role=buyer&redirect=%2Fen%2Faccount",
			locale: "en",
		});
	});

	it("a signed-in account without the seller role is sent home from the seller zone", () => {
		expect(decideZone("seller", buyer, "/seller-center/dashboard")).toEqual({
			kind: "redirect",
			href: "/",
			locale: "zh-TW",
		});
		expect(decideZone("seller", buyer, "/en/seller-center")).toEqual({
			kind: "redirect",
			href: "/",
			locale: "en",
		});
	});

	it("a seller opens the seller zone, and every signed-in account opens buyer and account zones", () => {
		expect(decideZone("seller", seller, "/seller-center")).toEqual({
			kind: "allow",
		});
		expect(decideZone("buyer", buyer, "/account")).toEqual({ kind: "allow" });
		expect(decideZone("buyer", seller, "/account")).toEqual({ kind: "allow" });
		expect(decideZone("account", buyer, "/seller/measures")).toEqual({
			kind: "allow",
		});
	});

	it("an unavailable answer is not a logout: no redirect in any zone", () => {
		for (const required of ["buyer", "seller", "account"] as const) {
			expect(decideZone(required, unavailable, "/account")).toEqual({
				kind: "unavailable",
			});
		}
	});

	it("an unsafe return path is dropped instead of becoming a redirect target", () => {
		expect(decideZone("buyer", guest, "//evil.example/account")).toEqual({
			kind: "redirect",
			href: "/login?role=buyer",
			locale: "zh-TW",
		});
		expect(decideZone("buyer", guest, null)).toEqual({
			kind: "redirect",
			href: "/login?role=buyer",
			locale: "zh-TW",
		});
	});
});

describe("decideZone: the fallback of an account without the zone's role", () => {
	it("is the default page of the account's workspace, the old roleZoneFallbackPath target", () => {
		const decision = decideZone("seller", buyer, "/seller-center");
		expect(decision).toEqual({
			kind: "redirect",
			href: defaultPathForRole("buyer"),
			locale: "zh-TW",
		});
	});

	it("a guest never takes the fallback: it goes to login", () => {
		const decision = decideZone("seller", guest, "/seller-center");
		expect(decision).toMatchObject({ kind: "redirect" });
		expect((decision as { href: string }).href).toMatch(/^\/login\?/);
	});
});
