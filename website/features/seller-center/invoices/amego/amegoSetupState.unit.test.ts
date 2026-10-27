import { describe, expect, it } from "vitest";
import { InvoiceProviderAccountStatus } from "@/lib/codemeta/enums";
import { amegoProgressPercent, amegoSetupState } from "./amegoSetupState";

const ALL = Object.values(InvoiceProviderAccountStatus);

describe("amegoSetupState", () => {
	it("13 個值域內的值全部有歸屬，沒有一個落 default", () => {
		// 🔴 鑑別力的來源：default 若寫成把已知值一併吃掉的萬用分支，這條會紅。
		expect(ALL).toHaveLength(13);
		for (const value of ALL) {
			expect(
				amegoSetupState(value, false).unknown,
				`${value} 落到 default`,
			).toBe(false);
		}
	});

	it("值域外落 default 而不是白畫面", () => {
		const s = amegoSetupState("SOME_FUTURE_STATUS", false);
		expect(s.unknown).toBe(true);
		expect(s.pillLabel).toBe("狀態更新中");
		expect(s.canActivate).toBe(false);
	});

	it("ACTIVE 的步驟 4／5 分界看切點不看 status", () => {
		const before = amegoSetupState(InvoiceProviderAccountStatus.ACTIVE, false);
		const after = amegoSetupState(InvoiceProviderAccountStatus.ACTIVE, true);

		expect(before.step).toBe(4);
		expect(before.pillLabel).toBe("測試成功");
		expect(before.canActivate).toBe(true);
		expect(before.activated).toBe(false);

		expect(after.step).toBe(5);
		expect(after.pillLabel).toBe("已啟用");
		expect(after.activated).toBe(true);
		// 同一個 status、兩種呈現 —— 這正是本檔存在的理由。
		expect(before.pillLabel).not.toBe(after.pillLabel);
	});

	it("DEGRADED 沿用 ACTIVE 的兩列，但 pill 是 pending", () => {
		const d = amegoSetupState(InvoiceProviderAccountStatus.DEGRADED, true);
		expect(d.step).toBe(5);
		expect(d.pillTone).toBe("pending");
		expect(d.canActivate).toBe(true);
	});

	it("憑證被拒的兩態把 AppKey 欄位切回可輸入", () => {
		for (const v of [
			InvoiceProviderAccountStatus.INVALID_CREDENTIAL,
			InvoiceProviderAccountStatus.PROVIDER_REJECTED,
		]) {
			const s = amegoSetupState(v, false);
			expect(s.credentialEditable, `${v} 應可重新輸入 AppKey`).toBe(true);
			expect(s.canActivate).toBe(false);
			expect(s.pillTone).toBe("alert");
		}
	});

	it("驗證中的兩態不讓改 AppKey，也不讓啟用", () => {
		for (const v of [
			InvoiceProviderAccountStatus.CREDENTIAL_SUBMITTED,
			InvoiceProviderAccountStatus.CREDENTIAL_VERIFYING,
		]) {
			const s = amegoSetupState(v, false);
			expect(s.step).toBe(3);
			expect(s.credentialEditable).toBe(false);
			expect(s.canActivate).toBe(false);
			// 🔴 兩態都必須讓「測試連線」按得下去：端點 24 只把憑證收下，
			// 真正打 AMEGO 的是端點 25。按鈕鎖住 ⇒ 賣家永遠出不了這個狀態。
			expect(s.canVerify, `${v} 必須能按測試連線`).toBe(true);
		}
	});

	it("CREDENTIAL_VERIFYING 的文案不得暗示系統正在跑", () => {
		// 端點 24 送出憑證後就是這個狀態，而**沒有任何東西在驗**（真正打 AMEGO 的是
		// 端點 25）。寫「測試中」會讓賣家以為只要等 —— dev 實測 4 個帳號因此停在這裡
		// 1~3 天，歷程上只有 SUBMITTED、沒有 VERIFICATION_STARTED。
		const s = amegoSetupState(
			InvoiceProviderAccountStatus.CREDENTIAL_VERIFYING,
			false,
		);
		expect(s.pillLabel).toBe("待測試");
		expect(s.pillLabel).not.toContain("中");
	});

	it("平台端停用／作廢是 terminal，賣家什麼都不能按", () => {
		for (const v of [
			InvoiceProviderAccountStatus.SUSPENDED,
			InvoiceProviderAccountStatus.REVOKED,
		]) {
			const s = amegoSetupState(v, true);
			expect(s.terminal).toBe(true);
			expect(s.canActivate).toBe(false);
			expect(s.credentialEditable).toBe(false);
		}
	});

	it("CONFIG_ERROR 顯示配置錯誤，不是狀態更新中", () => {
		// 投影值有沒有真的進 map —— 58A §S03 驗收 6。
		const s = amegoSetupState(InvoiceProviderAccountStatus.CONFIG_ERROR, false);
		expect(s.pillLabel).toBe("配置錯誤");
		expect(s.unknown).toBe(false);
	});

	it("進度百分比在 1–5 之間線性，且會夾住越界值", () => {
		expect(amegoProgressPercent(1)).toBe(0);
		expect(amegoProgressPercent(3)).toBe(50);
		expect(amegoProgressPercent(5)).toBe(100);
		expect(amegoProgressPercent(0)).toBe(0);
		expect(amegoProgressPercent(99)).toBe(100);
	});
});
