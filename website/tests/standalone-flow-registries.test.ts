import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

/**
 * 「這是一段脫離站台外框的獨立流程」這件事，在三個地方各登記了一次：
 *
 *   1. lib/navigation/standalone-flow.ts   STANDALONE_FLOW_ROOTS —— 抑制頁面 chrome
 *   2. components/shared/RoleAwareNavbar   早退 return null      —— 不渲染導覽列
 *   3. components/shared/SellerWorkspaceLock PUBLIC_ALLOWLIST    —— 不把賣家鎖回工作區
 *
 * 三份都是純字串清單，彼此沒有任何連結。`/hawkr-agent-rfq-preview/external-vendor`
 * 加進來時只更新了前兩份，於是「已登入且 activeRole === 'seller'」的訪客一進外部
 * 供應商 portal 就被第 3 份導回 /seller-center —— 而受邀供應商本來就可能同時是賣家。
 * 症狀只在特定身分出現，前兩份的測試全綠。
 *
 * 這支測試把三份釘在一起：任何一份新增了根路徑而其他兩份沒跟上，就轉紅。
 */

const read = (p: string) => readFileSync(p, "utf8");

function rootsOf(source: string, arrayName: string): string[] {
	const m = source.match(new RegExp(`${arrayName}\\s*=\\s*\\[([\\s\\S]*?)\\]`));
	assert.ok(m, `找不到 ${arrayName} —— 這支測試已失去鑑別力，請更新抓取方式`);
	return [...m[1].matchAll(/"([^"]+)"/g)].map((x) => x[1]);
}

test("standalone flow roots stay registered in all three chrome registries", () => {
	const standalone = rootsOf(
		read("lib/navigation/standalone-flow.ts"),
		"STANDALONE_FLOW_ROOTS",
	);
	const lockAllowlist = rootsOf(
		read("components/shared/SellerWorkspaceLock.tsx"),
		"PUBLIC_ALLOWLIST",
	);
	const navbar = read("components/shared/RoleAwareNavbar.tsx");

	assert.ok(
		standalone.length >= 2,
		"STANDALONE_FLOW_ROOTS 少於 2 條，抓取多半失敗了",
	);

	for (const root of standalone) {
		assert.ok(
			lockAllowlist.some((p) => root === p || root.startsWith(`${p}/`)),
			`${root} 在 STANDALONE_FLOW_ROOTS 內，但 SellerWorkspaceLock 的 PUBLIC_ALLOWLIST 沒有涵蓋它：` +
				`已登入的賣家一進這條流程就會被導回 /seller-center`,
		);
		assert.ok(
			navbar.includes(root),
			`${root} 在 STANDALONE_FLOW_ROOTS 內，但 RoleAwareNavbar 沒有提到它：站台導覽列會疊在這條獨立流程上`,
		);
	}
});
