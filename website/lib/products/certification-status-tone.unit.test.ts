import { describe, expect, it } from "vitest";
import { ProductCertificationStatus } from "@/lib/codemeta/enums";
import { certStatusView } from "./certification-status";

/**
 * 為什麼要有這支測試：賣家在認證區塊掃一眼時，唯一真正要分辨的是
 * 「這張已經生效了」還是「還在等平台審」。這兩個狀態曾經**同為綠色 pill**
 * （SUBMITTED=primary、APPROVED=success），只剩文字可讀。
 *
 * 斷言守的是**可區分性**而不是特定色名——換色票不該讓這支測試誤紅，
 * 但把兩者改回同一個 tone 一定要轉紅。
 */
describe("認證狀態徽章的色調", () => {
	const toneOf = (status: string) => certStatusView(status, {})?.tone ?? null;

	/**
	 * ⚠️ 這裡**不能**只斷言「兩個 tone 字串不同」——那是恆真的：本專案把 HeroUI 的
	 * primary 在某些頁面範圍會被覆寫成品牌色（見 styles/features/home.css），
	 * 而 success 本身也是綠，所以 `primary !== success` 通過的同時，賣家看到的仍是
	 * 兩顆一樣的淺綠 pill。要守的是**視覺可區分**，不是字串不等。
	 */
	const GREEN_FAMILY = ["primary", "success"];

	it("「審核中」不得落在與「已通過」相同的綠色系", () => {
		// 前提：APPROVED 是綠色系。若這一條先紅，代表配色改過了，
		// 下面那條斷言的意義要重新檢視，不要直接把它改綠。
		expect(GREEN_FAMILY).toContain(toneOf(ProductCertificationStatus.APPROVED));
		expect(GREEN_FAMILY).not.toContain(
			toneOf(ProductCertificationStatus.SUBMITTED),
		);
	});

	it("「審核中」用等待語意的色調，與 ADM 佇列一致", () => {
		expect(toneOf(ProductCertificationStatus.SUBMITTED)).toBe("warning");
	});

	it("終態的色調各自可辨：通過=success、駁回與撤銷=danger", () => {
		expect(toneOf(ProductCertificationStatus.APPROVED)).toBe("success");
		expect(toneOf(ProductCertificationStatus.REJECTED)).toBe("danger");
		expect(toneOf(ProductCertificationStatus.REVOKED)).toBe("danger");
	});

	/**
	 * 「已撤回」與「已被取代」是賣家不需要動作的狀態，維持中性；
	 * 但它們不能和「還在等」同色，否則賣家會以為還有下文。
	 */
	it("中性狀態不得與「審核中」同色", () => {
		const submitted = toneOf(ProductCertificationStatus.SUBMITTED);
		expect(toneOf(ProductCertificationStatus.WITHDRAWN)).not.toBe(submitted);
		expect(toneOf(ProductCertificationStatus.SUPERSEDED)).not.toBe(submitted);
	});
});
