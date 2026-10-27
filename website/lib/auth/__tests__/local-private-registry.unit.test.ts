import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { LOCAL_PRIVATE_KEY_SPECS } from "../local-private-keys";

/**
 * 完整性檢查點（hawkr-website#382 續）。
 *
 * #382 的修復建立了 `syncLocalPrivateState` 這個統一清除入口，卻只登記了當下
 * 知道的 2 把 key。2026-08-01 的稽核發現另外 8 把漏網，2026-08-07 補掃又發現
 * 3 把（seller-shipment-operations / hawkr-agent handoff 等）。同一個洞連開三次
 * 的原因不是有人不小心，而是**「所有本地 key 都要登記」這件事沒有任何機器強制**
 * ——靠人記得就一定會漏。
 *
 * 所以這支測試不驗行為，驗的是「清單有沒有涵蓋現況」：掃描原始碼裡所有
 * storage key 字面值，每一把都必須被歸類為
 *   (a) 帳號私有 → 登記在 local-private-keys registry，換帳號時清除，或
 *   (b) 裝置層級 → 明確列進下面的白名單，並寫下為什麼不必清。
 * 兩者皆非 → 紅燈。新增一個持久化 store 而忘了登記時，這裡會先擋下來。
 */

const REPO_ROOT = fileURLToPath(new URL("../../..", import.meta.url));
const SOURCE_ROOTS = ["app", "components", "features", "lib"];

/**
 * 裝置層級、刻意不隨身份轉換清除的 key。加東西進這裡等於宣告「這把 key 不含
 * 任何帳號私有資訊」——請連同理由一起寫，半年後的人要能判斷這個豁免還成不成立。
 */
const DEVICE_SCOPED_KEYS: Record<string, string> = {
	"hk:theme": "淺色/深色偏好，屬於這台裝置而非帳號。",
	"hk:text-scale":
		"字體大小（小/中/大）偏好，屬於這台裝置而非帳號；後端尚無欄位，先只存本機。",
	"hk:site-background-paused":
		"全站背景播放偏好，僅保存本分頁的暫停旗標，不含帳號私有資訊。",
	"hk:home-grainient-paused":
		"舊版首頁背景暫停旗標，僅供全站背景偏好向後相容讀取，不含帳號私有資訊。",
	"hkui:chat:sidebar-collapsed": "側欄收合的版面偏好，無帳號私有內容。",
	"hkui:auth:active-role":
		"auth 機制自身的 active role 標記，由 setStoredActiveRole/resolveActiveRole 在登入登出時自行維護；交給 registry 清會與 auth 的狀態機打架。",
	__hk_navigation_transaction:
		"導覽觀測用的一次性 transaction 標記，毫秒級生命週期、無帳號私有內容。",
	__hk_navigation_recovery_used:
		"本分頁導航鏈的自動恢復防循環旗標，只保存 1，不含帳號資料；須跨整頁跳轉與身份切換保留，直到新的使用者操作或分頁關閉才清除。",
	"post-viewer-scroll-hint-seen":
		"一次性捲動提示是否看過，UX 提示旗標非帳號資料。",
	"seller-scroll-world-hint-seen": "同上，賣家招募頁的一次性提示旗標。",
	"hawkr:adm:end-pending":
		"ADM 代行工作區「結束結果不明」的封鎖旗標，值是代行工作階段 id（不是任何 www 帳號的資料，credential 本身在 httpOnly cookie）。刻意不進 registry：它是 fail-closed 守衛，隨 www 身份轉換被清掉等於無聲解除封鎖。生命週期由 sessionStorage（分頁關閉即消失）與「值必須等於當前工作階段 id」雙重界定。",
};

function sourceFiles(dir: string, out: string[] = []): string[] {
	for (const entry of readdirSync(dir)) {
		if (entry === "node_modules" || entry === ".next") continue;
		const full = join(dir, entry);
		if (statSync(full).isDirectory()) sourceFiles(full, out);
		else if (/\.tsx?$/.test(full) && !/\.(unit\.)?test\.tsx?$/.test(full))
			out.push(full);
	}
	return out;
}

interface FoundKey {
	key: string;
	file: string;
}

/**
 * 兩種寫法都要抓：直接寫在 storage 呼叫裡的字面值，以及宣告成 `*STORAGE_KEY`
 * 常數的字面值（後者是本 repo 的慣例，registry 化之後這些常數應改為引用
 * registry，於是自然從掃描結果消失）。
 */
function findStorageKeys(files: string[]): FoundKey[] {
	const found: FoundKey[] = [];
	for (const file of files) {
		const source = readFileSync(file, "utf8");
		const rel = file.slice(REPO_ROOT.length);
		for (const m of source.matchAll(
			/(?:window\.)?(?:local|session)Storage\.(?:get|set|remove)Item\(\s*"([^"]+)"/g,
		))
			found.push({ key: m[1], file: rel });
		for (const m of source.matchAll(
			/[A-Z_]*STORAGE_KEY[A-Za-z]*\s*=\s*\n?\s*"([^"]+)"/g,
		))
			found.push({ key: m[1], file: rel });
	}
	return found;
}

describe("本地私有狀態 registry 完整性", () => {
	const files = SOURCE_ROOTS.flatMap((root) =>
		sourceFiles(join(REPO_ROOT, root)),
	);
	const registered = new Set(LOCAL_PRIVATE_KEY_SPECS.map((spec) => spec.key));

	it("掃得到原始碼（掃描器本身沒壞）", () => {
		// 沒有這條，掃描器若因路徑改動掃到 0 個檔案，下面每條都會「通過」。
		expect(files.length).toBeGreaterThan(200);
	});

	it("每一把 storage key 不是登記為帳號私有，就是明確豁免為裝置層級", () => {
		const unclassified = findStorageKeys(files).filter(
			({ key }) => !registered.has(key) && !(key in DEVICE_SCOPED_KEYS),
		);
		expect(
			unclassified,
			`發現未歸類的 storage key。若含帳號私有資訊 → 登記進 lib/auth/local-private-keys.ts（並讓模組反向引用該 key）；若屬裝置層級 → 連同理由加進本檔的 DEVICE_SCOPED_KEYS：\n${unclassified
				.map(({ key, file }) => `  - "${key}"  (${file})`)
				.join("\n")}`,
		).toEqual([]);
	});

	it("zustand persist 只能經由 createLocalPrivateStore 使用", () => {
		// 直接用 persist() 就能繞過 registry 綁定與記憶體 reset 註冊——那正是
		// cartUiStore 與賣家中心 5 個 store 當初漏掉的方式。
		const offenders = files.filter((file) => {
			if (file.endsWith(join("lib", "state", "createLocalPrivateStore.ts")))
				return false;
			const source = readFileSync(file, "utf8");
			return (
				/from\s+"zustand\/middleware"/.test(source) &&
				/\bpersist\b/.test(source)
			);
		});
		expect(
			offenders.map((f) => f.slice(REPO_ROOT.length)),
			"這些檔案自行使用 zustand persist，會繞過 registry 綁定與身份轉換 reset。請改用 createLocalPrivateStore。",
		).toEqual([]);
	});

	it("registry 的 key 沒有重複（複製貼上時最容易犯）", () => {
		const keys = LOCAL_PRIVATE_KEY_SPECS.map((spec) => spec.key);
		expect(new Set(keys).size).toBe(keys.length);
	});

	it("registry 每筆都至少宣告一個 storage area", () => {
		for (const spec of LOCAL_PRIVATE_KEY_SPECS) {
			expect(spec.in.length, `${spec.key} 未宣告 storage area`).toBeGreaterThan(
				0,
			);
		}
	});
});
