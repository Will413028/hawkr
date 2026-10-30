import { createTranslator } from "next-intl";
import { describe, expect, it } from "vitest";
import { messagesFor } from "@/lib/intl/config";
import { linkErrorKey, linkStartUrl, loginErrorKey } from "./oauth-start";

const textOf = (locale: "zh-TW" | "en", ns: string, key: string) =>
	(
		createTranslator({
			locale,
			messages: messagesFor(locale),
			namespace: ns as never,
		}) as unknown as (k: string) => string
	)(key);

function loginText(locale: "zh-TW" | "en", code: string): string | null {
	const key = loginErrorKey(code);
	return key ? textOf(locale, "auth.error", key) : null;
}

function linkText(locale: "zh-TW" | "en", code: string): string | null {
	const k = linkErrorKey(code);
	return k ? textOf(locale, k.ns, k.key) : null;
}

describe("login error code -> text", () => {
	it.each([
		[
			"access_denied",
			"你取消了授權，請再試一次",
			"You cancelled the authorization. Please try again.",
		],
		[
			"unavailable",
			"登入服務暫時無法使用，請稍後再試",
			"Sign-in is temporarily unavailable. Please try again later.",
		],
		[
			"invalid_flow",
			"登入失敗，請稍後再試",
			"Sign-in failed. Please try again later.",
		],
		[
			"provider_error",
			"登入失敗，請稍後再試",
			"Sign-in failed. Please try again later.",
		],
		[
			"login_failed",
			"登入失敗，請稍後再試",
			"Sign-in failed. Please try again later.",
		],
		[
			"login_required",
			"登入失敗，請稍後再試",
			"Sign-in failed. Please try again later.",
		],
	])("%s", (code, zh, en) => {
		expect(loginText("zh-TW", code)).toBe(zh);
		expect(loginText("en", code)).toBe(en);
	});

	it.each([
		"whatever",
		"",
		"constructor",
		"__proto__",
		"identity_already_linked",
	])("unknown code %j shows nothing", (code) => {
		expect(loginErrorKey(code)).toBeNull();
	});
	it("missing code shows nothing", () => {
		expect(loginErrorKey(null)).toBeNull();
		expect(loginErrorKey(undefined)).toBeNull();
	});
});

describe("link_error code -> text", () => {
	it("identity_already_linked has its own sentence", () => {
		expect(linkText("zh-TW", "identity_already_linked")).toBe(
			"這個登入帳號已連結到別的帳號",
		);
		expect(linkText("en", "identity_already_linked")).toBe(
			"This sign-in account is already linked to another account.",
		);
	});
	it("login error codes reuse the login sentences", () => {
		expect(linkText("zh-TW", "access_denied")).toBe("你取消了授權，請再試一次");
		expect(linkText("zh-TW", "unavailable")).toBe(
			"登入服務暫時無法使用，請稍後再試",
		);
		expect(linkText("zh-TW", "provider_error")).toBe("登入失敗，請稍後再試");
	});
	it("unknown code shows nothing", () => {
		expect(linkErrorKey("whatever")).toBeNull();
		expect(linkErrorKey(null)).toBeNull();
	});
});

describe("linkStartUrl", () => {
	it("carries link=1 and the encoded return_to", () => {
		const url = linkStartUrl("line", "/en/account/settings");
		expect(url.startsWith("/api/auth/line/start?")).toBe(true);
		const q = new URL(url, "http://x").searchParams;
		expect(q.get("link")).toBe("1");
		expect(q.get("return_to")).toBe("/en/account/settings");
		expect(url).toContain("return_to=%2Fen%2Faccount%2Fsettings");
	});
});
