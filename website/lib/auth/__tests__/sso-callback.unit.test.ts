import { describe, expect, it } from "vitest";
import {
	parseLoginProvider,
	SSO_CALLBACK_PATH,
	ssoCallbackPath,
} from "../sso-callback";

describe("ssoCallbackPath", () => {
	it("carries the login page's workspace, destination and provider through the OAuth round-trip", () => {
		const url = new URL(
			ssoCallbackPath({
				role: "seller",
				redirect: "/seller-center/orders?tab=new",
				provider: "line",
			}),
			"https://hawkr.local",
		);
		expect(url.pathname).toBe(SSO_CALLBACK_PATH);
		expect(url.searchParams.get("role")).toBe("seller");
		expect(url.searchParams.get("redirect")).toBe(
			"/seller-center/orders?tab=new",
		);
		expect(url.searchParams.get("provider")).toBe("line");
	});

	it("omits an empty destination and an unknown provider", () => {
		expect(
			ssoCallbackPath({ role: "buyer", redirect: "", provider: null }),
		).toBe(`${SSO_CALLBACK_PATH}?role=buyer`);
	});
});

describe("parseLoginProvider", () => {
	it("accepts only the three enabled providers", () => {
		expect(parseLoginProvider("google")).toBe("google");
		expect(parseLoginProvider("apple")).toBe("apple");
		expect(parseLoginProvider("line")).toBe("line");
		expect(parseLoginProvider("facebook")).toBeNull();
		expect(parseLoginProvider(null)).toBeNull();
	});
});
