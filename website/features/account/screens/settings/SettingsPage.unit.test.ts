import { describe, expect, it } from "vitest";
import { SECTIONS } from "./SettingsPage";
import SignInMethodsSection from "./SignInMethodsSection";

describe("SettingsPage SECTIONS", () => {
	it("sign-in methods render inside the account-security card, after its item list", () => {
		const hosts = SECTIONS.filter((s) => s.extra === SignInMethodsSection);
		expect(hosts).toHaveLength(1);
		const [host] = hosts;
		expect(host?.items.map((i) => i.label)).toEqual(["個人資料", "聯絡資訊"]);
	});

	it("no other section carries an extra", () => {
		expect(SECTIONS.filter((s) => s.extra).length).toBe(1);
	});
});
