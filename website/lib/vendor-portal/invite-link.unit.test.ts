import { describe, expect, it } from "vitest";
import {
	parseInviteSecret,
	parseRecipientId,
	recipientQuery,
} from "./invite-link";

/**
 * 邀請 secret 只走 URL fragment（`/vendor/rfq#invite=<secret>`，
 * notification-service `buildVendorInviteMessage` 組的連結）。fragment 不會送到
 * server，所以解析一定發生在瀏覽器；這支純函式是那個解析的全部。
 */
describe("parseInviteSecret", () => {
	const SECRET = "a".repeat(64);

	it("取出 fragment 的 invite 值", () => {
		expect(parseInviteSecret(`#invite=${SECRET}`)).toBe(SECRET);
	});

	it("沒有前導 # 也照樣解析（history.replaceState 之後的殘值）", () => {
		expect(parseInviteSecret(`invite=${SECRET}`)).toBe(SECRET);
	});

	it("fragment 帶其他參數時仍取 invite", () => {
		expect(parseInviteSecret(`#foo=1&invite=${SECRET}`)).toBe(SECRET);
	});

	it("沒有 invite key 回 null", () => {
		expect(parseInviteSecret("#foo=1")).toBeNull();
	});

	it("空 fragment 回 null", () => {
		expect(parseInviteSecret("")).toBeNull();
		expect(parseInviteSecret("#")).toBeNull();
	});

	it("invite 是空值或只有空白時回 null——不拿它去打端點浪費節流額度", () => {
		expect(parseInviteSecret("#invite=")).toBeNull();
		expect(parseInviteSecret("#invite=%20%20")).toBeNull();
	});
});

describe("recipientQuery／parseRecipientId", () => {
	it("兌換後的網址帶收件人 ID，解析得回同一個不透明字串（不轉數字）", () => {
		const id = "843856384063471755";
		expect(recipientQuery(id)).toBe(`?r=${id}`);
		expect(parseRecipientId(recipientQuery(id))).toBe(id);
	});

	it("沒有或空的 r ＝ null", () => {
		expect(parseRecipientId("")).toBeNull();
		expect(parseRecipientId("?r=")).toBeNull();
		expect(parseRecipientId("?x=1")).toBeNull();
	});

	it("不是正的標準十進位 int64（非數字、負數、前導 0、0、超過 int64）＝ null", () => {
		for (const bad of [
			"abc",
			"12a",
			"-5",
			"0",
			"007",
			"1.5",
			"9223372036854775808",
		]) {
			expect(parseRecipientId(`?r=${bad}`), bad).toBeNull();
		}
		expect(parseRecipientId("?r=9223372036854775807")).toBe(
			"9223372036854775807",
		);
	});
});
