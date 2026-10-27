import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

/**
 * 這組是 source-level UI safety contract，不是假裝成瀏覽器 E2E。
 * Focus 實際循環、inert、1024px reflow 與視覺對比仍需 browser smoke 驗證。
 *
 * ⚠️ 代行**工作區**的 UI 已移到 `hawkr-website`（56 號 A28），對應的情境列／
 * 阻擋畫面／lifecycle guard 斷言也隨之搬走。本檔只守 ADM 這一側剩下的東西：
 * 賣家列表與商家頁的**入口**、preflight 對話框、以及那支 same-origin service。
 */

const UI_FILES = [
	"src/features/delegated-operations/components/MerchantOperationEntry.tsx",
	"src/features/delegated-operations/components/OperationPreflightDialog.tsx",
	// 治理台（代行紀錄／稽核內容／撤銷）與入口走同一條保證：它們讀的是稽核與工作
	// 階段摘要，任何一支改打 /api/bff 或賣家 API 都會讓治理查詢帶著錯誤的身分。
	"src/features/delegated-operations/components/SessionAuditConsole.tsx",
	"src/features/delegated-operations/components/AuditDetailDialog.tsx",
	"src/features/delegated-operations/components/SessionRevokeDialog.tsx",
	"src/services/delegated-operations.ts",
];

async function readUiBundle() {
	return (
		await Promise.all(UI_FILES.map((file) => readFile(file, "utf8")))
	).join("\n");
}

test("代行 UI 只呼叫專用 same-origin routes，不碰 seller API 或 token storage", async () => {
	const source = await readUiBundle();
	assert.doesNotMatch(source, /\/api\/bff/u);
	assert.doesNotMatch(source, /\/api\/v1\/app\//u);
	assert.doesNotMatch(source, /sellerToken|accessToken|refreshToken/u);
	assert.doesNotMatch(source, /localStorage|sessionStorage/u);
	assert.match(source, /\/api\/admin\/delegated-operations/u);
});

test("高權限 preflight dialog 走共用 Dialog（modal 語意、focus trap、背景隔離都在 Dialog 內）", async () => {
	const [preflight, dialog, a11y] = await Promise.all([
		readFile(
			"src/features/delegated-operations/components/OperationPreflightDialog.tsx",
			"utf8",
		),
		readFile("src/components/UI/Dialog.tsx", "utf8"),
		readFile("src/lib/a11y.ts", "utf8"),
	]);
	assert.match(preflight, /<Dialog\b/u);
	assert.doesNotMatch(preflight, /role="dialog"/u);
	assert.match(dialog, /role="dialog"/u);
	assert.match(dialog, /aria-modal="true"/u);
	assert.match(dialog, /useFocusTrap/u);
	assert.match(dialog, /useModalStack/u);
	assert.match(dialog, /useEscapeKey/u);
	assert.match(a11y, /\.inert = true/u);
});

/**
 * 詳情頁的入口資格**只有伺服器一個來源**。
 *
 * 修前寫的是 `enabled={isApproved && suspensionConfirmed && !activeSuspension}`，
 * 那是一份自建的本地判準，而它問不到 preflight 的任何一道前置：
 *   - 治理停權（違規處分）只寫 `sellers.status='suspended_governance'`、**不寫**
 *     `seller_suspensions` 列 ⇒ `activeSuspension` 恆為 null ⇒ 按鈕照樣可按，
 *     點下去才被 9272 擋；
 *   - feature flag 關閉、管理員不在 allowlist（9265）這一側前端完全看不到。
 * 而這一頁的 profile 連 `lifecycleStatus` 都沒有 —— 要在本地補齊等於再造一份會漂移
 * 的判準。設計書 6.1 S-1 已經定版：preflight 就是「入口按鈕的可用性」，觸發點含詳情頁。
 */
test("詳情頁的代行入口資格交給 preflight，不自建第二份店家判準", async () => {
	const [merchantPage, entry] = await Promise.all([
		readFile("src/app/admin/merchants/[id]/page.tsx", "utf8"),
		readFile(
			"src/features/delegated-operations/components/MerchantOperationEntry.tsx",
			"utf8",
		),
	]);
	assert.match(
		merchantPage,
		/<MerchantOperationEntry[^>]*\n\s*storeId=\{store\.id\}\n\s*enabled\n\s*resolveEligibility\n/u,
		"詳情頁必須把資格判定交給 preflight（enabled + resolveEligibility）",
	);
	assert.doesNotMatch(
		merchantPage,
		/enabled=\{[^}]*activeSuspension[^}]*\}/u,
		"詳情頁又自建了一份店家資格判準 ⇒ 治理停權與 allowlist 兩側再度失明",
	);
	assert.match(entry, /getDelegatedOperationPreflight/u);
	assert.match(entry, /preflightQuery\.data\.target\.storeId !== storeId/u);
	// 不合格時按鈕停用並把伺服器下發的原因寫在上面，不是「可以按，按了才知道」。
	assert.match(entry, /eligibilityGateLabel/u);
	assert.match(entry, /preflightDenial\.message/u);
});

test("賣家列表提供分離的詳情與代行入口，且 preflight 只在開啟後查詢", async () => {
	const [merchantList, entry, preflight] = await Promise.all([
		readFile("src/app/admin/merchants/page.tsx", "utf8"),
		readFile(
			"src/features/delegated-operations/components/MerchantOperationEntry.tsx",
			"utf8",
		),
		readFile(
			"src/features/delegated-operations/components/OperationPreflightDialog.tsx",
			"utf8",
		),
	]);
	// 代行資格只讀伺服器的 delegationUnavailabilities（tests/delegated-operation-entry-gating.test.ts 逐項釘住），列表頁沒有本地判準；rowState 只用於徽章與按鈕顏色。
	assert.match(merchantList, /m\.delegationUnavailabilities/u);
	assert.doesNotMatch(merchantList, /isActive && !isSuspended/u);
	assert.match(merchantList, /rowState\(m\)/u);
	assert.doesNotMatch(
		merchantList,
		/contactEmail|overallCertificationStatus|lifecycleStatus/u,
		"列表頁又讀了舊 REST 的欄位 ⇒ 資格判定回到 gateway 的綜合狀態",
	);
	assert.match(merchantList, /label="代行營運"/u);
	assert.match(merchantList, /<MaterialSymbol name="search"/u);
	// preflight 不得在列表 render 時無條件核發（一頁十列＝十次）。除了使用者觸發的
	// 兩個一次性事件（開啟對話框、剛被拒導回本頁要問「現在為什麼不行」）之外，只有
	// 單店畫面可以用 `resolveEligibility` 進場就問 —— 列表頁不得傳它。
	assert.match(
		entry,
		/enabled: enabled && \(resolveEligibility \|\| dialogOpen \|\| startDenied\)/u,
	);
	assert.doesNotMatch(
		merchantList,
		/resolveEligibility/u,
		"列表頁傳了 resolveEligibility ⇒ 一頁十列就是十次 preflight",
	);
	assert.match(entry, /getDelegatedOperationPreflight/u);
	assert.match(preflight, /type="radio"/u);
	assert.doesNotMatch(preflight, /type="checkbox"/u);
});

/**
 * 送出必須是原生表單導覽：`fetch` 讀不到跨 host 303 的 `Location`
 * （`redirect:"manual"` → opaqueredirect；`redirect:"follow"` → fetch 自己去打
 * www，一次性兌換碼當場燒掉、cookie 落不到瀏覽器）。這條若被改回 fetch，
 * 症狀是「按下建立之後什麼都沒發生」而工作階段已經建好、佔住 partial UNIQUE。
 */
test("建立工作階段是頂層表單導覽，不是 fetch，也不在瀏覽器端組工作區網址", async () => {
	const preflight = await readFile(
		"src/features/delegated-operations/components/OperationPreflightDialog.tsx",
		"utf8",
	);
	assert.match(preflight, /method: "post"/u);
	assert.match(preflight, /action: START_ACTION/u);
	assert.match(
		preflight,
		/START_ACTION = "\/api\/admin\/delegated-operations"/u,
	);
	assert.match(preflight, /name="storeId"/u);
	assert.match(preflight, /name="reasonCode"/u);
	assert.match(preflight, /name="caseReference"/u);
	assert.match(preflight, /name="ttlSeconds"/u);
	assert.match(preflight, /name="idempotencyKey"/u);
	assert.doesNotMatch(preflight, /fetch\(/u);
	// 工作區已不在 ADM：任何指向舊工作區路徑或自組的兌換網址都是回歸。
	assert.doesNotMatch(preflight, /\/operate\//u);
	assert.doesNotMatch(preflight, /adm\/enter/u);
});

/**
 * 動作篩選下拉在資料到達之前就要列滿 19 個選項，那時後端逐列下發的 `actionLabel`
 * 還不存在 —— 修前因此直接印 `PRODUCT_BATCH_UNPUBLISH` 這種機器名，而同一頁的
 * 「資源」「結果」兩個下拉都是中文。
 *
 * 引導用的中文放在 CodeMeta 生成物 `src/lib/codemeta/generated.ts`。這裡守的是**結構性
 * 漂移**：值域新增一個動作卻沒補標籤時，下拉會退回印機器名而且沒有任何人會發現。
 * 措辭漂移不歸這支管 —— 主控台在資料到手後就以 `actionLabel` 就地覆蓋，稽核列本身
 * 更是從頭到尾只讀 `actionLabel`。
 */
test("CodeMeta 鏡像的動作鍵集合與封閉值域完全相等", async () => {
	const [{ DELEGATION_AUDIT_ACTION_LABEL }, { DELEGATED_AUDIT_ACTIONS }] =
		await Promise.all([
			import("../src/lib/codemeta"),
			import("../src/features/delegated-operations/governance"),
		]);
	assert.deepEqual(
		Object.keys(DELEGATION_AUDIT_ACTION_LABEL).sort(),
		[...DELEGATED_AUDIT_ACTIONS].sort(),
	);
	for (const [action, label] of Object.entries(DELEGATION_AUDIT_ACTION_LABEL)) {
		assert.notEqual(label, action, `${action} 沒有中文標籤`);
		assert.doesNotMatch(label, /[A-Z]{2,}_/u, `${action} 的標籤看起來仍是常數`);
	}
});

test("動作下拉渲染標籤而不是常數，且權威值優先於鏡像", async () => {
	const console_ = await readFile(
		"src/features/delegated-operations/components/SessionAuditConsole.tsx",
		"utf8",
	);
	// 修前是 `<option key={value} value={value}>{value}</option>`。
	assert.match(
		console_,
		/<option key=\{value\} value=\{value\}>\s*\{codeLabel\(actionLabels, value\)\}/u,
	);
	// 資料一到就用該列自己帶的 actionLabel 覆蓋鏡像，鏡像只補沒出現過的動作。
	assert.match(console_, /labels\[entry\.action\] = entry\.actionLabel/u);
	// 稽核列本身永遠只讀後端下發的值，不查鏡像。
	assert.match(console_, /<strong>\{entry\.actionLabel\}<\/strong>/u);
});

/**
 * 1440 這一級筆電上，兩張表的最右欄（賣家表的「操作」、稽核表的「結果」）曾被
 * 容器的 overflow 裁到可視區外 —— 前者連 elementFromPoint 都回 null。幾何要靠
 * 瀏覽器量，但「守住那個幾何的規則還在不在」可以在這裡機械檢查。
 */
test("兩張寬表的關鍵欄有結構性保護，不是靠剛好塞得下", async () => {
	const [merchantCss, delegationCss] = await Promise.all([
		readFile("src/styles/pages/MerchantManagement.css", "utf8"),
		readFile("src/styles/pages/DelegatedOperations.css", "utf8"),
	]);
	// 賣家表：「操作」欄釘在右緣，欄位再長也還在畫面上。
	assert.match(
		merchantCss,
		/\.merchant-management \.merchant-table td:last-child \{[^}]*position: sticky;[^}]*right: 0;/u,
	);
	// 列狀態以 token 表達，底色與色調才拆得開（不透明性由下一支測試單獨守）。
	assert.match(
		merchantCss,
		/\.merchant-table tbody tr \{[^}]*--merchant-row-tint: transparent;/u,
	);
	// 稽核表獨佔整個寬度：稽核內容是 modal，不得再有東西跟主表搶同一條水平空間。
	// 側欄版有過兩代（`grid-template-columns: minmax(0,1fr) 340px` 會把「結果」欄裁掉，
	// 之後的 `flex-wrap: wrap` 則讓面板換行到表格下方、開在畫面外），兩代都不要回來。
	//
	// 剝註解再比：解釋「為什麼不要那個佈局」的註解本身就寫著那些選擇器名。
	const delegationRules = stripCssComments(delegationCss);
	assert.doesNotMatch(delegationRules, /\.adm-audit-layout\b/u);
	assert.doesNotMatch(
		delegationRules,
		/\.adm-audit-main \{[^}]*flex(?:-basis)?:/u,
		".adm-audit-main 又有 flex-basis ⇒ 它旁邊被塞了第二欄",
	);
	// 「變更摘要」是唯一可換行的欄，必須同時有下限與吃下剩餘寬度的能力。
	assert.match(
		delegationCss,
		/\.adm-audit-col-summary \{[^}]*width: 100%;[^}]*min-width: var\(--adm-audit-summary-min\);/u,
	);
});

/**
 * 以下三支守的是「算出來的結果」而不是 CSS 檔的文字形狀。
 *
 * 起因：`釘住的欄與表頭同色、且底層不透明` 那支只用 regex 比對宣告是否存在，
 * 於是漏掉一個真迴歸 —— `AdminVisualRefresh.css` 的
 * `.page-content table tbody tr:hover`（特異性 (0,2,3)）比
 * `.merchant-table tbody tr`（(0,1,2)）強，hover 時用 `background` 簡寫把列的
 * background-color 與 background-image 一起換掉，而釘住的欄因為選擇器更強而活著
 * ⇒ 同一列在 sticky 邊界分岔成兩種顏色，「我選了哪一列」的提示在滑鼠移上去時消失。
 * 文字形狀完全正確，測試照樣綠。
 *
 * CI 沒有瀏覽器（`node --test` + tsx，無 jsdom 也無版面計算），所以這裡自己對
 * **真實 CSS 檔**解一次 cascade：只支援後代組合子（這些檔案的相關規則全是），
 * 算特異性、比來源順序，得出每個 longhand 的勝出宣告。
 */

type Decl = { prop: string; value: string };
type ParsedRule = { selector: string; decls: Decl[]; order: number };

/** 只收「模擬 1440px 視窗時會生效」的 @media；其餘（含 prefers-*）整段略過。 */
function acceptsMedia(condition: string, viewport: number): boolean {
	const max = /\(\s*max-width:\s*(\d+)px\s*\)/u.exec(condition);
	const min = /\(\s*min-width:\s*(\d+)px\s*\)/u.exec(condition);
	if (!max && !min) return false;
	if (max && viewport > Number(max[1])) return false;
	if (min && viewport < Number(min[1])) return false;
	return true;
}

function parseCss(
	source: string,
	viewport: number,
	base: number,
): ParsedRule[] {
	const clean = source.replace(/\/\*[\s\S]*?\*\//gu, "");
	const rules: ParsedRule[] = [];
	let order = base;
	const walk = (text: string, inAccepted: boolean) => {
		let i = 0;
		while (i < text.length) {
			const brace = text.indexOf("{", i);
			if (brace === -1) break;
			const prelude = text.slice(i, brace).trim();
			// 找對應的右括號
			let depth = 1;
			let j = brace + 1;
			while (j < text.length && depth > 0) {
				if (text[j] === "{") depth++;
				else if (text[j] === "}") depth--;
				j++;
			}
			const body = text.slice(brace + 1, j - 1);
			if (prelude.startsWith("@")) {
				if (prelude.startsWith("@media")) {
					walk(body, inAccepted && acceptsMedia(prelude, viewport));
				}
			} else if (inAccepted) {
				const decls: Decl[] = [];
				for (const part of body.split(";")) {
					const c = part.indexOf(":");
					if (c === -1) continue;
					const prop = part.slice(0, c).trim();
					const value = part.slice(c + 1).trim();
					// 自訂屬性也收：列的狀態色是以 `--merchant-row-tint` 表達的，不解它就
					// 只能比到「兩格都寫著 var(--merchant-row-tint)」這種恆真的字串。
					// `resolveBackground` 只看 background 三個屬性，多收這些不影響它。
					if (prop && value) decls.push({ prop, value });
				}
				if (decls.length)
					rules.push({ selector: prelude, decls, order: order++ });
			}
			i = j;
		}
	};
	walk(clean, true);
	return rules;
}

type Compound = Set<string>;
type ElementPath = { ancestors: Compound[]; subject: Compound };

const TOKEN =
	/(::?[a-z-]+(?:\([^)]*\))?|[.#][\w-]+|\[[^\]]*\]|[a-z][\w-]*|\*)/giu;
function tokenize(compound: string): string[] {
	return compound.match(TOKEN) ?? [];
}

function specificity(selector: string): [number, number, number] {
	let a = 0;
	let b = 0;
	let c = 0;
	for (const t of tokenize(selector)) {
		if (t.startsWith("#")) a++;
		else if (t.startsWith("::")) c++;
		else if (t.startsWith(".") || t.startsWith("[") || t.startsWith(":")) b++;
		else if (t !== "*") c++;
	}
	return [a, b, c];
}

/**
 * 支援後代與**子**組合子（`>`）；`+` `~` `:not()` 一律當作不匹配（這些檔案的相關
 * 規則沒有用到）。子組合子是必要的：對話框那條「只有內容捲」的不變式裡，中間那一層
 * 是 `.adm-operation-dialog > form`（送出必須是原生表單導覽，見元件檔頭），把它當成
 * 後代會讓「表單這一層有沒有傳遞 min-height: 0」變成測不到的一環。
 *
 * 本 repo 的寫法一律在 `>` 兩側留空白（biome format 的結果），因此按空白切詞即可。
 */
function matches(selector: string, el: ElementPath): boolean {
	if (/[+~]/u.test(selector) || selector.includes(":not(")) return false;
	const parts = selector.trim().split(/\s+/u);
	if (parts.at(-1) === ">") return false;
	const subject = tokenize(parts.at(-1) as string);
	if (!subject.every((t) => el.subject.has(t))) return false;
	let idx = el.ancestors.length - 1;
	// 下一個（往左的）compound 是否必須恰好是直接父層
	let directParent = false;
	for (let p = parts.length - 2; p >= 0; p--) {
		if (parts[p] === ">") {
			directParent = true;
			continue;
		}
		const want = tokenize(parts[p]);
		if (directParent) {
			if (idx < 0) return false;
			const parent = el.ancestors[idx];
			idx--;
			if (!want.every((t) => parent.has(t))) return false;
			directParent = false;
			continue;
		}
		let found = false;
		while (idx >= 0) {
			const anc = el.ancestors[idx];
			idx--;
			if (want.every((t) => anc.has(t))) {
				found = true;
				break;
			}
		}
		if (!found) return false;
	}
	return true;
}

type Won = { value: string; spec: [number, number, number]; order: number };

/** 同一個屬性上，新的宣告是否贏過目前勝出者：先比特異性，平手比來源順序。 */
function wins(
	spec: [number, number, number],
	order: number,
	cur: Won | undefined,
): boolean {
	if (!cur) return true;
	for (let i = 0; i < 3; i++) {
		if (spec[i] !== cur.spec[i]) return spec[i] > cur.spec[i];
	}
	return order >= cur.order;
}

const BG_LONGHANDS = ["background-color", "background-image"] as const;

/** 回傳該元素在這批樣式下勝出的 background-color / background-image。 */
function resolveBackground(rules: ParsedRule[], el: ElementPath) {
	const won: Record<string, Won> = {};
	const consider = (
		prop: string,
		value: string,
		spec: [number, number, number],
		order: number,
	) => {
		if (wins(spec, order, won[prop])) {
			won[prop] = { value, spec, order };
		}
	};
	for (const rule of rules) {
		for (const sel of rule.selector.split(",")) {
			const s = sel.trim();
			if (!s || !matches(s, el)) continue;
			const spec = specificity(s);
			for (const d of rule.decls) {
				if (d.prop === "background") {
					// 簡寫：兩個 longhand 一起被設定（未指定的部分回到初始值）
					consider("background-color", d.value, spec, rule.order);
					consider(
						"background-image",
						/gradient|url\(/u.test(d.value) ? d.value : "none",
						spec,
						rule.order,
					);
				} else if ((BG_LONGHANDS as readonly string[]).includes(d.prop)) {
					consider(d.prop, d.value, spec, rule.order);
				}
			}
		}
	}
	return {
		"background-color": won["background-color"]?.value ?? "transparent",
		"background-image": won["background-image"]?.value ?? "none",
	};
}

/**
 * 與 `resolveBackground` 同一套 cascade，只是屬性換成版面相關的 longhand。
 *
 * 展開兩個簡寫，因為兩者都是這批不變式的關鍵而且都會被簡寫悄悄清掉：
 *   `overflow: hidden`  → overflow-x / overflow-y
 *   `flex: 0 0 auto`    → flex-grow / flex-shrink / flex-basis
 * 沒有勝出宣告時回 `undefined`（＝該屬性用初始值），呼叫端自己決定初始值算不算合格。
 */
function resolveProps(
	rules: ParsedRule[],
	el: ElementPath,
	wanted: readonly string[],
): Record<string, string | undefined> {
	const won: Record<string, Won> = {};
	const consider = (
		prop: string,
		value: string,
		spec: [number, number, number],
		order: number,
	) => {
		if (!wanted.includes(prop)) return;
		if (wins(spec, order, won[prop])) won[prop] = { value, spec, order };
	};
	for (const rule of rules) {
		for (const sel of rule.selector.split(",")) {
			const s = sel.trim();
			if (!s || !matches(s, el)) continue;
			const spec = specificity(s);
			for (const d of rule.decls) {
				if (d.prop === "overflow") {
					consider("overflow-x", d.value, spec, rule.order);
					consider("overflow-y", d.value, spec, rule.order);
				} else if (d.prop === "flex") {
					const parts = d.value.split(/\s+/u);
					// 形態：`none`／`auto`／`initial`（關鍵字）、`<g> <s> <basis>`、`<g> <s>`、`<g>`。
					// 關鍵字必須單獨列出 —— 把 `flex: none` 當成 `<g>` 會解出 grow="none"、
					// shrink="1"，而它實際是 `0 0 auto`。那個 "1" 會讓「shrink 必須是 0」這類
					// 斷言**假通過**（本 repo 的 MerchantManagement.css 就有一處 `flex: none`）。
					const KEYWORDS: Record<string, [string, string, string]> = {
						none: ["0", "0", "auto"],
						auto: ["1", "1", "auto"],
						initial: ["0", "1", "auto"],
					};
					const kw = KEYWORDS[d.value.trim()];
					const [grow, shrink, basis] = kw
						? kw
						: parts.length >= 3
							? (parts as [string, string, string])
							: parts.length === 2
								? [parts[0], parts[1], "0%"]
								: [parts[0], "1", "0%"];
					consider("flex-grow", grow, spec, rule.order);
					consider("flex-shrink", shrink, spec, rule.order);
					consider("flex-basis", basis, spec, rule.order);
				} else {
					consider(d.prop, d.value, spec, rule.order);
				}
			}
		}
	}
	const out: Record<string, string | undefined> = {};
	for (const prop of wanted) out[prop] = won[prop]?.value;
	return out;
}

/* ─────────────────────────── JSX 結構 harness ───────────────────────────
 *
 * 上面那套 cascade harness 只回答「這個 element path 上，誰贏了哪個屬性」——
 * element path 是**呼叫端自己組出來的**，它從不讀 `.tsx`。於是凡是用了子組合子
 * （`>`）的規則，都有一半的前提落在 harness 的視野外：選擇器命不命中，取決於
 * JSX 真的長成那個形狀。這一節補的就是那一半。
 *
 * 本檔目前有一條這樣的規則，失效方式是「CSS 還在、build 照過、測試照綠」：
 *   `.adm-operation-dialog > form` 中間插一層 `<div>` ⇒ min-height 不傳遞，送出鍵出畫面
 *
 * 為什麼是自己刻的掃描器而不是鬆的字串比對：`assert.match(source, /<span/)` 這種
 * 會被檔案任何角落的無關 span、甚至註解裡的字滿足（本檔的 `noValidate` 斷言第一版
 * 就是這樣零鑑別力的，見該支測試的註解）。要守的是**父子關係**，就必須真的把層級
 * 建出來——而層級只有兩件事會影響它：元素開合，以及 fragment 不產生節點。
 *
 * 掃描器的鑑別力由「harness：JSX 結構掃描器…」那支測試直接驗證，不靠推測。
 */

type JsxNode = {
	/** 標籤名；fragment（`<>`）為空字串。元件（`Button`）保留原樣。 */
	tag: string;
	attrs: string;
	/** `className="..."` 的字面值；寫成 `{expr}` 時放表達式原文。 */
	className: string;
	children: JsxNode[];
	/** 直接掛在這個節點下的文字與 `{expr}`，不含子元素的。 */
	text: string;
};

/**
 * 一個 JSX 標籤：開、閉、或自閉。
 *
 * 屬性區允許引號字串、樣板字串與最多三層巢狀的 `{...}`，因此含 `>` 的屬性值
 * （`onClick={() => f()}`、`title="a > b"`、`onSubmit={() => { if (a>b) {} }}`）
 * 不會被誤判成標籤結束——那是這個 repo 最常見的形狀，一旦誤判，層級就會歪掉而且
 * 是往「看起來還過得去」的方向歪。
 */
const JSX_TAG =
	/<(\/?)([A-Za-z][\w.]*)?((?:"[^"]*"|'[^']*'|`[^`]*`|\{(?:[^{}]|\{(?:[^{}]|\{[^{}]*\})*\})*\}|[^>"'`{}])*?)(\/?)>/gu;

// 先剝註解：JSX 註解裡寫到的標籤名（本檔守的兩處，原始碼註解正好都在解釋那個標籤
// 為什麼存在）會讓掃描器看到不存在的節點。
/** CSS 註解常在解釋「為什麼不要某個舊佈局」，裡面就寫著那些選擇器名。 */
function stripCssComments(css: string): string {
	return css.replace(/\/\*[\s\S]*?\*\//gu, "");
}

function stripJsxComments(source: string): string {
	return (
		source
			.replace(/\{\s*\/\*[\s\S]*?\*\/\s*\}/gu, "")
			.replace(/\/\*[\s\S]*?\*\//gu, "")
			// 只剝整行註解：`//` 也會出現在字串裡的網址中。
			.replace(/^[ \t]*\/\/.*$/gmu, "")
	);
}

function classNameOf(attrs: string): string {
	const literal = /className="([^"]*)"/u.exec(attrs);
	if (literal) return literal[1];
	const expression = /className=\{([\s\S]*?)\}/u.exec(attrs);
	return expression ? expression[1] : "";
}

function parseJsx(rawSource: string): JsxNode {
	const source = stripJsxComments(rawSource);
	const root: JsxNode = {
		tag: "#root",
		attrs: "",
		className: "",
		children: [],
		text: "",
	};
	const stack: JsxNode[] = [root];
	let consumed = 0;
	for (const match of source.matchAll(JSX_TAG)) {
		const top = stack[stack.length - 1];
		const at = match.index as number;
		top.text += source.slice(consumed, at);
		consumed = at + match[0].length;
		const tag = match[2] ?? "";
		if (match[1] === "/") {
			// 只在標籤名對得上時才收：不成對的 `<` 若被誤判成開標籤（例如泛型），
			// 這裡不會連帶把真正的層級一起彈掉。
			for (let i = stack.length - 1; i > 0; i--) {
				if (stack[i].tag === tag) {
					stack.length = i;
					break;
				}
			}
			continue;
		}
		const node: JsxNode = {
			tag,
			attrs: match[3] ?? "",
			className: classNameOf(match[3] ?? ""),
			children: [],
			text: "",
		};
		top.children.push(node);
		if (match[4] !== "/") stack.push(node);
	}
	stack[stack.length - 1].text += source.slice(consumed);
	return root;
}

/**
 * 直接子**元素**：fragment 不產生 DOM 節點，所以要穿透它。
 * 這正是 CSS 子組合子的語意——`.a > form` 命中包在 `<>…</>` 裡的 form。
 */
function domChildren(node: JsxNode): JsxNode[] {
	return node.children.flatMap((child) =>
		child.tag === "" ? domChildren(child) : [child],
	);
}

function findJsx(
	node: JsxNode,
	predicate: (n: JsxNode) => boolean,
	found: JsxNode[] = [],
): JsxNode[] {
	for (const child of node.children) {
		if (predicate(child)) found.push(child);
		findJsx(child, predicate, found);
	}
	return found;
}

function hasJsxClass(node: JsxNode, className: string): boolean {
	return node.className.split(/[\s"'`]+/u).includes(className);
}

const ADMIN_CHROME: Compound[] = [
	new Set(["main", ".page-content"]),
	new Set(["div", ".merchant-management"]),
	new Set(["div", ".merchant-table-container"]),
	new Set(["table", ".merchant-table"]),
	new Set(["tbody"]),
];

function rowStates(): Array<{ name: string; row: string[] }> {
	return [
		{ name: "一般", row: ["tr"] },
		{ name: "hover", row: ["tr", ":hover"] },
		{ name: "selected", row: ["tr", ".selected"] },
		{ name: "selected + hover", row: ["tr", ".selected", ":hover"] },
		{ name: "active", row: ["tr", ":active"] },
	];
}

async function merchantTableRules(viewport: number) {
	// 匯入順序＝ layout.tsx 先（AdminVisualRefresh），page.tsx 後（MerchantManagement）。
	const [refresh, merchant] = await Promise.all([
		readFile("src/styles/AdminVisualRefresh.css", "utf8"),
		readFile("src/styles/pages/MerchantManagement.css", "utf8"),
	]);
	return [
		...parseCss(refresh, viewport, 0),
		...parseCss(merchant, viewport, 100_000),
	];
}

/**
 * 斷言的是「所有儲存格同色 ＋ 不透明」。這兩件事同時成立時 `<tr>` 畫什麼都被蓋住、
 * 看不到，所以不必再比 `<tr>` —— 反過來說，任何讓某一格與其他格分岔的改動
 * （不論是它自己多了一條規則，還是別人搶走了列的上色）都會在這裡轉紅。
 */
test("解 cascade：釘住的欄與同列其他儲存格在每一種列狀態下同色", async () => {
	for (const viewport of [1440, 1920]) {
		const rules = await merchantTableRules(viewport);
		for (const { name, row } of rowStates()) {
			const ancestors = [...ADMIN_CHROME, new Set(row)];
			const first = resolveBackground(rules, {
				ancestors,
				subject: new Set(["td", ":first-child"]),
			});
			const last = resolveBackground(rules, {
				ancestors,
				subject: new Set(["td", ":last-child"]),
			});
			assert.deepEqual(
				last,
				first,
				`${viewport}px / ${name}：釘住的欄(${JSON.stringify(last)}) 與其他儲存格(${JSON.stringify(first)}) 不同色`,
			);
			// 釘住的欄下面會有捲過去的內容，底層必須不透明。
			assert.notEqual(
				last["background-color"],
				"transparent",
				`${viewport}px / ${name}：釘住的欄沒有不透明底色`,
			);
		}
	}
});

test("解 cascade：釘住的表頭與其他表頭同色", async () => {
	const rules = await merchantTableRules(1440);
	const ancestors = [
		...ADMIN_CHROME.slice(0, 4),
		new Set(["thead"]),
		new Set(["tr"]),
	];
	const first = resolveBackground(rules, {
		ancestors,
		subject: new Set(["th", ":first-child"]),
	});
	const last = resolveBackground(rules, {
		ancestors,
		subject: new Set(["th", ":last-child"]),
	});
	assert.deepEqual(
		last,
		first,
		`釘住的表頭 ${JSON.stringify(last)} 與其他表頭 ${JSON.stringify(first)} 不同色`,
	);
	assert.notEqual(first["background-color"], "transparent");
});

/**
 * 「變更摘要」欄的下限必須讀變數，不得寫死第二個數字。
 *
 * ⚠️ 這支原本還守著「換行門檻 ＝ 固定欄合計 ＋ 摘要下限」那組計算（側欄要不要換行）。
 * 稽核內容改成 modal 之後主表獨佔整個寬度，`--adm-audit-fixed-cols` 與那條
 * `flex-basis` 一併移除 —— 那不是本支測試被放寬，是它守的東西不存在了。
 */
test("摘要欄下限讀的是變數，且不加無效的 max-width", async () => {
	const css = await readFile(
		"src/styles/pages/DelegatedOperations.css",
		"utf8",
	);
	assert.doesNotMatch(
		stripCssComments(css),
		/--adm-audit-fixed-cols/u,
		"側欄換行門檻的調校值又回來了 ⇒ 稽核內容被改回側欄，或留了一個沒人讀的死值",
	);
	assert.match(
		css,
		/\.adm-audit-col-summary \{[^}]*min-width: var\(--adm-audit-summary-min\)/u,
	);
	// max-width 在 table cell 上會被瀏覽器忽略（CSS 2.1 §17.5.2），加回來就是死碼。
	const summary = /\.adm-audit-col-summary \{[^}]*\}/u.exec(css)?.[0] ?? "";
	assert.doesNotMatch(summary, /max-width/u);
});

/**
 * 上面兩支比的是「哪個宣告勝出」，值本身還是原樣字串 —— 兩格都寫著
 * `linear-gradient(0deg, var(--merchant-row-tint), var(--merchant-row-tint))`
 * 時它們必然相等，所以「列到底是什麼顏色」那一題它答不出來。
 *
 * 這一段把同一套 cascade 再往下走一層：解出 `<tr>` 上勝出的 `--merchant-row-tint`，
 * 沿祖先鏈把 `var(--x)` 展開成字面色（自訂屬性會繼承，因此每一層祖先各算一次、
 * 由遠到近覆蓋），最後比色相。
 *
 * 守的是：**同一個「已選取」狀態不得因為滑鼠移入而換色系**。修前 `.selected` 走
 * `--primary-orange-light`（2026-08 換膚後其實是品牌色，hue 85°），而
 * `.selected:hover` 是換膚漏掉的橘色字面值 `rgba(238, 150, 75, 0.15)`（hue 28°）
 * ⇒ 整列在綠與橘之間跳。名字叫 orange 值卻是綠，光讀 CSS 很難發現，只有把值真的
 * 解出來才擋得住。
 */

type Rgba = { r: number; g: number; b: number; a: number };

/** 自訂屬性會繼承：每一層祖先各算一次勝出者，由遠到近覆蓋。 */
function customProperties(
	rules: ParsedRule[],
	el: ElementPath,
): Map<string, string> {
	const chain: ElementPath[] = el.ancestors.map((subject, i) => ({
		ancestors: el.ancestors.slice(0, i),
		subject,
	}));
	chain.push(el);
	const env = new Map<string, string>();
	for (const node of chain) {
		const won = new Map<string, Won>();
		for (const rule of rules) {
			for (const sel of rule.selector.split(",")) {
				const s = sel.trim();
				if (!s || !matches(s, node)) continue;
				const spec = specificity(s);
				for (const d of rule.decls) {
					if (!d.prop.startsWith("--")) continue;
					if (wins(spec, rule.order, won.get(d.prop)))
						won.set(d.prop, { value: d.value, spec, order: rule.order });
				}
			}
		}
		for (const [name, decl] of won) env.set(name, decl.value);
	}
	return env;
}

/** 把 `var(--x)` / `var(--x, fallback)` 展開成字面值。 */
function deref(value: string, env: Map<string, string>): string {
	let out = value;
	for (let i = 0; i < 10 && out.includes("var("); i++) {
		out = out.replace(
			/var\(\s*(--[\w-]+)\s*(?:,([^()]*))?\)/gu,
			(_m, name: string, fallback?: string) =>
				env.get(name) ?? (fallback ?? "").trim(),
		);
	}
	return out.trim();
}

function parseColor(value: string): Rgba | null {
	const v = value.trim().toLowerCase();
	if (v === "transparent") return { r: 0, g: 0, b: 0, a: 0 };
	const fn = /^rgba?\(([^)]*)\)$/u.exec(v);
	if (fn) {
		const parts = fn[1]
			.split(/[\s,/]+/u)
			.filter(Boolean)
			.map(Number);
		if (parts.length < 3 || parts.slice(0, 3).some(Number.isNaN)) return null;
		return {
			r: parts[0],
			g: parts[1],
			b: parts[2],
			a: parts.length > 3 ? parts[3] : 1,
		};
	}
	const hexMatch = /^#([0-9a-f]{3}|[0-9a-f]{6})$/u.exec(v);
	if (hexMatch) {
		const h =
			hexMatch[1].length === 3
				? [...hexMatch[1]].map((c) => c + c).join("")
				: hexMatch[1];
		return {
			r: Number.parseInt(h.slice(0, 2), 16),
			g: Number.parseInt(h.slice(2, 4), 16),
			b: Number.parseInt(h.slice(4, 6), 16),
			a: 1,
		};
	}
	return null;
}

/** HSL 色相；灰階回 null（沒有色系可言）。 */
function hue({ r, g, b }: Rgba): number | null {
	const R = r / 255;
	const G = g / 255;
	const B = b / 255;
	const max = Math.max(R, G, B);
	const min = Math.min(R, G, B);
	const d = max - min;
	if (d === 0) return null;
	const h =
		max === R
			? ((G - B) / d) % 6
			: max === G
				? (B - R) / d + 2
				: (R - G) / d + 4;
	return (h * 60 + 360) % 360;
}

test("解 cascade：selected 與 selected:hover 是同一個色系，且 hover 更深", async () => {
	const rules = await merchantTableRules(1440);
	const tintOf = (row: string[]): Rgba => {
		const el: ElementPath = { ancestors: ADMIN_CHROME, subject: new Set(row) };
		const env = customProperties(rules, el);
		const raw = env.get("--merchant-row-tint");
		assert.ok(raw, `${row.join("")} 解不出 --merchant-row-tint`);
		const color = parseColor(deref(raw, env));
		assert.ok(color, `${row.join("")} 的列色解不成顏色：${raw}`);
		return color;
	};

	const selected = tintOf(["tr", ".selected"]);
	const hovered = tintOf(["tr", ".selected", ":hover"]);
	const hs = hue(selected);
	const hh = hue(hovered);
	assert.ok(
		hs !== null && hh !== null,
		`已選取的列色必須有色相：selected=${JSON.stringify(selected)} hover=${JSON.stringify(hovered)}`,
	);
	const delta = Math.abs(((hs - hh + 540) % 360) - 180);
	assert.ok(
		delta <= 15,
		`selected(hue ${hs.toFixed(1)}°) 與 selected:hover(hue ${hh.toFixed(1)}°) 差 ${delta.toFixed(1)}° —— 同一個狀態在滑鼠移入時換了色系`,
	);
	// 兩者若被「統一」成完全同色，色系斷言照樣過，但 hover 就沒有回饋了。
	assert.ok(
		hovered.a > selected.a,
		`selected:hover(alpha ${hovered.a}) 必須比 selected(alpha ${selected.a}) 深，否則 hover 沒有視覺回饋`,
	);
});

/**
 * 高權限對話框：**只有內容區捲動**，標題與動作列釘住。
 *
 * 修前是整個對話框 `overflow-y: auto` ⇒ 內容一超過 `max-height`（1440×1200 這種常見
 * 筆電尺寸就會超過），唯一能完成任務的「建立工作階段」跟著捲出可視區，而 macOS／
 * Chrome 的 overlay 捲軸只在捲動當下現身 ⇒ 畫面停在「可查看／可變更」卡片中段，沒有
 * 捲軸、沒有遮罩、沒有任何線索說下面還有東西。
 *
 * 幾何要靠瀏覽器量，但「這條不變式的每一環還在不在」是 cascade 問題：
 *   ① 對話框自己不捲（否則動作列在捲動區內）
 *   ② 中間的 `<form>` 傳遞 min-height: 0（漏了它 body 會撐高整個對話框，
 *      `overflow-y` 永遠不生效——這是最容易在重構中掉的一環，因為刪掉它畫面在
 *      內容短的時候完全正常）
 *   ③ body 是唯一捲動區，且 min-height: 0
 *   ④ 動作列不收縮
 *
 * ⚠️ 以下這支**只守 CSS 那一半**。②那條規則寫的是子組合子 `.adm-operation-dialog > form`，
 * 而這裡比對的 element path 是自己組出來的、從不讀 `.tsx`：在對話框與 `<form>` 之間插
 * 一層 `<div>`，選擇器整條不再命中、送出鍵回到畫面外，這支照樣全綠。那一半由下一支
 * （「解 DOM：<form> 是對話框的直接子層」）守。
 */
async function dialogRules(viewport: number) {
	// 匯入順序：layout.tsx 先（globals → AdminVisualRefresh），Dialog.css（Dialog.tsx 自己 import）次之，page.tsx 後。
	const [globals, refresh, dialogCss, delegation] = await Promise.all([
		readFile("src/app/globals.css", "utf8"),
		readFile("src/styles/AdminVisualRefresh.css", "utf8"),
		readFile("src/components/UI/Dialog.css", "utf8"),
		readFile("src/styles/pages/DelegatedOperations.css", "utf8"),
	]);
	return [
		...parseCss(globals, viewport, 0),
		...parseCss(refresh, viewport, 100_000),
		...parseCss(dialogCss, viewport, 200_000),
		...parseCss(delegation, viewport, 300_000),
	];
}

const DIALOG_CHROME: Compound[] = [
	new Set(["div", ".adm-operation-dialog-overlay"]),
];
const DIALOG: Compound = new Set([
	"div",
	".adm-operation-dialog",
	".adm-operation-dialog--preflight",
]);

function scrolls(value: string | undefined): boolean {
	return value === "auto" || value === "scroll";
}

test("解 cascade：對話框三段式，只有內容區捲動、動作列永遠在畫面上", async () => {
	// 760 是 `@media (max-width: 760px)` 的另一側：那一段會重寫對話框的
	// max-height 與各層的 min/max-width，不能只驗桌機那一支。
	for (const viewport of [1440, 760]) {
		const rules = await dialogRules(viewport);
		const where = `${viewport}px`;

		// ① 對話框：column flex 容器，且自己不是捲動區。
		const dialog = resolveProps(
			rules,
			{ ancestors: DIALOG_CHROME, subject: DIALOG },
			["display", "flex-direction", "overflow-x", "overflow-y", "max-height"],
		);
		assert.equal(dialog.display, "flex", `${where}：對話框不是 flex 容器`);
		assert.equal(
			dialog["flex-direction"],
			"column",
			`${where}：對話框不是 column`,
		);
		assert.ok(
			!scrolls(dialog["overflow-y"]),
			`${where}：對話框自己在捲（overflow-y: ${dialog["overflow-y"]}）⇒ 動作列會跟著捲出可視區`,
		);
		assert.ok(
			dialog["max-height"],
			`${where}：對話框沒有 max-height，內容長時會超出視窗`,
		);

		// ② 中間的 <form>：column flex ＋ min-height: 0，否則 min-height 不會傳遞。
		const form = resolveProps(
			rules,
			{ ancestors: [...DIALOG_CHROME, DIALOG], subject: new Set(["form"]) },
			["display", "flex-direction", "min-height", "flex-grow", "flex-shrink"],
		);
		assert.equal(form.display, "flex", `${where}：<form> 這一層不是 flex 容器`);
		assert.equal(
			form["flex-direction"],
			"column",
			`${where}：<form> 不是 column`,
		);
		assert.equal(
			form["min-height"],
			"0",
			`${where}：<form> 少了 min-height: 0 ⇒ body 會撐高對話框，overflow-y 永遠不生效`,
		);
		assert.equal(form["flex-grow"], "1", `${where}：<form> 不吃剩餘高度`);

		// ③ body：唯一捲動區，且解除 flex 的自動最小尺寸。
		const body = resolveProps(
			rules,
			{
				ancestors: [...DIALOG_CHROME, DIALOG, new Set(["form"])],
				subject: new Set(["div", ".adm-operation-dialog-body"]),
			},
			["overflow-y", "min-height", "flex-grow"],
		);
		assert.ok(
			scrolls(body["overflow-y"]),
			`${where}：內容區不捲（overflow-y: ${body["overflow-y"]}）⇒ 長內容會被裁掉而不是捲動`,
		);
		assert.equal(
			body["min-height"],
			"0",
			`${where}：內容區少了 min-height: 0 ⇒ 它會撐到內容全高，捲動與釘住雙雙失效`,
		);
		assert.equal(body["flex-grow"], "1", `${where}：內容區不吃剩餘高度`);

		// ④ 動作列：不收縮。
		const actions = resolveProps(
			rules,
			{
				ancestors: [...DIALOG_CHROME, DIALOG, new Set(["form"])],
				subject: new Set([
					"div",
					".adm-operation-dialog-actions",
					".adm-operation-dialog-actions--preflight",
				]),
			},
			["flex-shrink", "overflow-y"],
		);
		assert.equal(
			actions["flex-shrink"],
			"0",
			`${where}：動作列可被壓縮（flex-shrink: ${actions["flex-shrink"]}）`,
		);
		assert.ok(
			!scrolls(actions["overflow-y"]),
			`${where}：動作列自己成了捲動區`,
		);
	}
});

/**
 * 上面那支的另一半：`<form>` 必須是 `.adm-operation-dialog` 的**直接**子層。
 *
 * 中間那一環寫的是子組合子 `.adm-operation-dialog > form`，所以它成立的前提有兩個，
 * 而上面那支只驗了第一個：
 *   ① 那條規則帶著 `min-height: 0` 且沒被別的規則蓋掉（cascade，上面那支）
 *   ② JSX 真的把 `<form>` 掛在對話框底下、中間不隔任何元素（DOM 形狀，這一支）
 *
 * ② 沒有保護的後果比 ① 嚴重：在 dialog 與 form 之間插一層 `<div>`（包一層 wrapper
 * 是重構裡最不起眼的動作）會讓選擇器整條不再命中 ⇒ form 那一層回到預設的
 * `min-height: auto` ⇒ body 撐高整個對話框 ⇒ **送出鍵回到畫面外**，也就是修前那個
 * bug 原樣復活。而上面那支照樣全綠——它比對的是自己組出來的 element path，
 * 那條路徑裡永遠有一層 `form`，跟真實的 `.tsx` 一點關係都沒有。
 *
 * 兩支對話框都適用（CSS 那條規則的註解也是這樣寫的）：非表單的分支（preflight 還沒
 * 回來時的載入／封鎖畫面）走 fragment，body 與動作列直接掛在對話框這一層，本來就不
 * 需要中間那一環，因此不在這一支的要求內。
 */
const FORM_DIALOGS = [
	"src/features/delegated-operations/components/OperationPreflightDialog.tsx",
	"src/features/delegated-operations/components/SessionRevokeDialog.tsx",
] as const;

test("解 DOM：<form> 是對話框的直接子層，`> form` 那條規則才命中", async () => {
	// 結構由共用 Dialog 產生；這裡把它真的渲染出來，用 DOM 的子層關係驗證，再確認兩支對話框都走 `form` 屬性。
	const { createRequire } = await import("node:module");
	const req = createRequire(`${process.cwd()}/tests/anchor.js`);
	req.extensions[".css"] = () => {};
	const { createElement } = req("react") as {
		createElement: (type: unknown, props?: unknown, ...c: unknown[]) => unknown;
	};
	const { renderToStaticMarkup } = req("react-dom/server") as {
		renderToStaticMarkup: (element: unknown) => string;
	};
	const { JSDOM } = req("jsdom") as {
		JSDOM: new (html: string) => { window: { document: Document } };
	};
	const Dialog = req("../src/components/UI/Dialog").default as unknown;
	const html = renderToStaticMarkup(
		createElement(
			Dialog,
			{
				variant: "operation",
				title: "t",
				onClose: () => {},
				form: { onSubmit: () => {} },
				footer: "f",
			},
			"b",
		),
	);
	const doc = new JSDOM(html).window.document;
	assert.ok(
		doc.querySelector(".adm-operation-dialog > form"),
		"<form> 不是 .adm-operation-dialog 的直接子層 ⇒ min-height: 0 不再傳遞，送出鍵回到畫面外",
	);
	for (const cls of [
		"adm-operation-dialog-body",
		"adm-operation-dialog-actions",
	]) {
		assert.ok(
			doc.querySelector(`.adm-operation-dialog > form > .${cls}`),
			`.${cls} 不是 <form> 的直接子層`,
		);
	}
	for (const file of FORM_DIALOGS) {
		const source = await readFile(file, "utf8");
		assert.match(source, /\bform=\{/u, `${file}：沒有用 Dialog 的 form 屬性`);
		assert.doesNotMatch(source, /<form\b/u, `${file}：自己放了 <form>`);
	}
});

/**
 * 商家詳情頁 header：代行失敗的錯誤帶不得定義工具列的寬度。
 *
 * 修前 `.adm-operation-entry` 是橫排 inline-flex，錯誤帶（不定長的上游文字，未知碼
 * 會退回「…（代碼 X）。」而碼本身可到 80 字）的 max-content 就是那一列的寬度；
 * 而 `.header-actions` 是 `flex-shrink: 0` ⇒ 壓力全轉給 `.header-left`，後者的 flex
 * 自動最小尺寸是 min-content，撐不下就整排溢出卡片右緣。實測店名折成「十八養／場」、
 * 六個 meta 欄各自折行、分隔的「·」孤立成一行，`SOME_UNKNOWN_CODE_XYZ` 那張連
 * 「刪除帳號」整顆都不見。
 *
 * 兩道防線各守一半，都要在：
 *   ① 錯誤帶自身有寬度上限（它不再無限長）
 *   ② `.header-left` 恰好只拿剩餘寬度（真的塞不下時，溢出在結構上不可能發生）
 */
async function merchantHeaderRules(viewport: number) {
	const [globals, refresh, delegation, cert] = await Promise.all([
		readFile("src/app/globals.css", "utf8"),
		readFile("src/styles/AdminVisualRefresh.css", "utf8"),
		readFile("src/styles/pages/DelegatedOperations.css", "utf8"),
		readFile("src/styles/pages/MerchantCertificationDetail.css", "utf8"),
	]);
	return [
		...parseCss(globals, viewport, 0),
		...parseCss(refresh, viewport, 100_000),
		// page.tsx 的匯入順序：DelegatedOperations 先、MerchantCertificationDetail 後。
		...parseCss(delegation, viewport, 200_000),
		...parseCss(cert, viewport, 300_000),
	];
}

test("解 cascade：代行錯誤帶不定義 header 工具列的寬度", async () => {
	const rules = await merchantHeaderRules(1440);
	const headerChrome: Compound[] = [
		new Set(["main", ".page-content"]),
		new Set(["div", ".certification-detail-page"]),
		new Set(["div", ".detail-header"]),
	];

	// ① 入口本體上下排列 ＋ 錯誤帶有寬度上限。
	const entryPath: ElementPath = {
		ancestors: [...headerChrome, new Set(["div", ".header-actions"])],
		subject: new Set(["div", ".adm-operation-entry"]),
	};
	const entry = resolveProps(rules, entryPath, ["flex-direction", "max-width"]);
	assert.equal(
		entry["flex-direction"],
		"column",
		"入口仍是橫排 ⇒ 錯誤帶會與按鈕爭同一行的寬度",
	);
	assert.equal(entry["max-width"], "100%", "入口沒有以容器為上限");

	const band = resolveProps(
		rules,
		{
			ancestors: [...entryPath.ancestors, entryPath.subject],
			subject: new Set(["div", ".adm-operation-error"]),
		},
		["max-width"],
	);
	assert.ok(
		band["max-width"] && band["max-width"] !== "none",
		`入口內的錯誤帶沒有寬度上限（max-width: ${band["max-width"]}）⇒ 它會以 max-content 撐寬整個 header`,
	);

	// ①b 上限與「追蹤編號能斷行」是一組，缺一即等於沒修。
	// 追蹤編號是不含空白的字串（上游允許 128 字純英數），正常換行點一個都沒有；
	// 只加上限不讓它斷，實測 64 字純十六進位在 1440 下穿出錯誤帶 174.7px、右緣
	// 1453.7 已超出視窗 ⇒ 症狀從「錯誤帶撐爆 header」變成「文字自己跑出去」，
	// 兩個改動各自都對、組合起來互相抵銷。
	const trace = resolveProps(
		rules,
		{
			ancestors: [
				...entryPath.ancestors,
				entryPath.subject,
				new Set(["div", ".adm-operation-error"]),
				new Set(["div"]),
			],
			subject: new Set(["span", ".adm-operation-trace"]),
		},
		["overflow-wrap", "word-break"],
	);
	assert.ok(
		trace["overflow-wrap"] === "anywhere" ||
			trace["overflow-wrap"] === "break-word" ||
			trace["word-break"] === "break-all",
		`追蹤編號不會斷行（overflow-wrap: ${trace["overflow-wrap"]} / word-break: ${trace["word-break"]}）⇒ 它會穿出被 max-width 夾住的錯誤帶`,
	);

	// 反面：對話框／治理頁裡的同一個元件不該被這個上限波及（那裡它是整列寬的告示）。
	const dialogBand = resolveProps(
		rules,
		{
			ancestors: [
				new Set(["div", ".adm-operation-dialog-overlay"]),
				DIALOG,
				new Set(["form"]),
				new Set(["div", ".adm-operation-dialog-body"]),
			],
			subject: new Set(["div", ".adm-operation-error"]),
		},
		["max-width"],
	);
	assert.equal(
		dialogBand["max-width"],
		undefined,
		`對話框內的錯誤帶被加了寬度上限（${dialogBand["max-width"]}），那裡它應該撐滿`,
	);

	// ② 左側識別區只拿剩餘寬度：hypothetical main size 降為 0 ＋ 解除自動最小尺寸。
	const left = resolveProps(
		rules,
		{
			ancestors: headerChrome,
			subject: new Set(["div", ".header-left"]),
		},
		["flex-grow", "flex-shrink", "flex-basis", "min-width"],
	);
	assert.equal(left["flex-grow"], "1", "`.header-left` 不吃剩餘寬度");
	assert.ok(
		left["flex-shrink"] !== "0",
		"`.header-left` 不收縮 ⇒ 與同樣不收縮的操作列一起把 header 推出卡片外",
	);
	assert.equal(
		left["flex-basis"],
		"0",
		"`.header-left` 的 flex-basis 不是 0 ⇒ 它的 hypothetical main size 回到 max-content（那排 meta 攤成一行 >1300px）",
	);
	assert.equal(
		left["min-width"],
		"0",
		"`.header-left` 少了 min-width: 0 ⇒ flex 的自動最小尺寸（min-content）會讓它溢出而不是換行",
	);
});

/**
 * 自己驗證的表單不得讓原生氣泡搶先。
 *
 * `required` 的 constraint validation 在 submit 事件**之前**攔下送出，於是 preflight 的
 * 「請選擇代行原因。」「請填寫可追溯的案件編號或工單編號。」與撤銷視窗的「請填寫撤銷
 * 理由，它會寫入治理紀錄。」三則訊息全部是死碼 —— 操作者看到的是瀏覽器的「請填寫這個
 * 欄位。」，而那個氣泡還蓋住下方寫著格式要求的說明文字（`A3-validation-empty-case`）。
 *
 * `required` 本身要留（a11y 語意），關掉的是瀏覽器那一層提示；驗證仍在 `handleSubmit`，
 * 而且伺服器端各自還有一道（`CASE_REFERENCE_REQUIRED` / gateway 的撤銷理由值域）。
 */
test("帶自訂 onSubmit 驗證的表單一律 noValidate，錯誤走產品自有的錯誤帶", async () => {
	const forms = [
		[
			"src/features/delegated-operations/components/OperationPreflightDialog.tsx",
			["請選擇代行原因。", "請填寫可追溯的案件編號或工單編號。"],
		],
		[
			"src/features/delegated-operations/components/SessionRevokeDialog.tsx",
			["請填寫撤銷理由，它會寫入治理紀錄。"],
		],
	] as const;
	for (const [file, messages] of forms) {
		const source = await readFile(file, "utf8");
		// ⚠️ 必須先剝掉註解再比對。第一版直接 `assert.match(source, /noValidate/)`
		// 完全沒有鑑別力：把屬性從 `<form>` 上拿掉，測試照樣綠 —— 因為這兩個檔的
		// 檔頭註解正好在解釋「為什麼帶 noValidate」，那個字還在。變異測試才抓到。
		const code = source
			.replace(/\/\*[\s\S]*?\*\//gu, "")
			.replace(/^\s*\/\/.*$/gmu, "");
		// noValidate 由 Dialog 固定開啟（見 Dialog.tsx）；這裡確認檔案不自己放 <form>。
		assert.doesNotMatch(code, /<form\b/u, `${file}：自己放了 <form>`);
		assert.match(code, /\bform=\{/u, `${file}：沒有走 Dialog 的 form`);
		// 有 required 才需要 noValidate；沒有 required 的表單（查詢頁）不在本清單。
		assert.match(
			code,
			/\brequired\b/u,
			`${file}：沒有 required 了？請重看本清單`,
		);
		// 那些訊息必須真的存在，否則這支測試在守一個不存在的東西。
		for (const message of messages) {
			assert.ok(code.includes(message), `${file}：找不到訊息「${message}」`);
		}
		// 錯誤一律渲染在產品自有的錯誤帶裡。
		assert.match(code, /className="adm-operation-error"/u, file);
	}
});

/**
 * 工作階段代號要複製得走，而且全治理台只有一顆複製鍵的實作。
 *
 * 設計稿（image11）那張卡寫的是 `OPS-240803-0187 ⧉`；dev 上那顆 `⧉` 從缺，卡片右上
 * 只有一枚裝飾用的指紋圖示。而這個代號正是要被貼進工單與 log 查詢的東西 —— 同一頁的
 * 關聯 ID／冪等鍵／操作能力三處都有複製鍵，唯獨最常被複製的那個沒有。
 *
 * 後半段守的是「沿用既有那顆，不要再寫一顆」：複製的成敗回饋（reject 時不得留在
 * 「已複製」）只寫對一次就夠，多一份手寫實作就多一處會漏掉它，而漏掉的那顆看起來
 * 與正常的完全一樣。用剪貼簿 API 的出現位置當錨 —— 它是任何一顆複製鍵都繞不開的。
 */
test("解 DOM：工作階段代號卡帶複製鍵，且剪貼簿實作在治理台只有一處", async () => {
	const consoleFile =
		"src/features/delegated-operations/components/SessionAuditConsole.tsx";
	const root = parseJsx(await readFile(consoleFile, "utf8"));

	const cards = findJsx(
		root,
		(n) => n.tag === "StatCard" && n.attrs.includes('label="工作階段代號"'),
	);
	assert.equal(
		cards.length,
		1,
		`${consoleFile}：找到 ${cards.length} 張「工作階段代號」卡 —— 這一支假設只有一張`,
	);
	assert.match(
		cards[0].attrs,
		/valueAction=\{[\s\S]*CopyIconButton[\s\S]*report\.displayCode[\s\S]*\}/u,
		`「工作階段代號」卡沒有掛複製鍵（屬性為 ${JSON.stringify(cards[0].attrs)}）⇒ 這個代號只能用滑鼠圈選`,
	);

	// 標籤與它顯示的值必須對得上：這張卡的值是 `displayCode`（`OPS-…`），叫成
	// 「工作階段 ID」會讓人把代號貼進只收數字 ID 的地方（查詢框、維運 SQL）而被擋。
	assert.equal(
		findJsx(
			root,
			(n) => n.attrs.includes('label="工作階段 ID"') && n.tag === "StatCard",
		).length,
		0,
		"又出現一張標著「工作階段 ID」的 StatCard —— 那張卡顯示的是代號，不是數字 ID",
	);

	const owners: string[] = [];
	for (const file of [
		"src/features/delegated-operations/components/CopyIconButton.tsx",
		"src/features/delegated-operations/components/AuditDetailDialog.tsx",
		consoleFile,
	]) {
		const code = stripJsxComments(await readFile(file, "utf8"));
		if (/navigator\.clipboard/u.test(code)) owners.push(file);
	}
	assert.deepEqual(
		owners,
		["src/features/delegated-operations/components/CopyIconButton.tsx"],
		"剪貼簿呼叫散到不只一個檔 ⇒ 複製鍵被重寫了一顆，成敗回饋要各修一次",
	);
});

// harness 自己的正確性：`flex` 簡寫的關鍵字形態必須展開對。
//
// 這一支守的不是產品 CSS，是**上面每一支 flex 斷言的可信度**。`flex: none` 實際是
// `0 0 auto`，但若把它當成單值形態（`<grow>`）處理，會解出 grow="none"、
// **shrink="1"** —— 而「shrink 必須是 0」正是「動作列不得被壓縮」那類斷言的核心，
// 錯值 "1" 不會讓測試變紅，它會讓測試**假通過**。
//
// 這種錯誤沒有任何產品端症狀可以暴露它：CSS 是對的、畫面是對的，只有守門的那支
// 測試悄悄失去鑑別力。所以必須直接測 harness。
test("harness：flex 簡寫的關鍵字形態展開正確", async () => {
	const cases: [string, string, string, string][] = [
		// 宣告值            grow  shrink basis
		["none", "0", "0", "auto"],
		["auto", "1", "1", "auto"],
		["initial", "0", "1", "auto"],
		["0 0 auto", "0", "0", "auto"],
		["1 1 0", "1", "1", "0"],
		["2 3", "2", "3", "0%"],
		["3", "3", "1", "0%"],
	];

	for (const [value, grow, shrink, basis] of cases) {
		const rules = parseCss(`.probe { flex: ${value}; }`, 1440, 0);
		const got = resolveProps(
			rules,
			{ ancestors: [], subject: new Set(["div", ".probe"]) },
			["flex-grow", "flex-shrink", "flex-basis"],
		);
		assert.equal(got["flex-grow"], grow, `flex: ${value} 的 grow`);
		assert.equal(
			got["flex-shrink"],
			shrink,
			`flex: ${value} 的 shrink —— 這個值錯了不會轉紅，會讓別的斷言假通過`,
		);
		assert.equal(got["flex-basis"], basis, `flex: ${value} 的 basis`);
	}
});

// harness 自己的正確性（第二支）：JSX 掃描器的層級要建對。
//
// 理由與上面那支相同——兩支「解 DOM：…」全都建立在 `domChildren()` 真的等於 CSS 子
// 組合子的語意上。掃描器若在某種寫法下把層級建歪，那兩支不會轉紅，它們會**假通過**：
// 產品端已經插了一層 wrapper，測試卻還說是直接子層。
//
// 每一列都是「若掃描器少處理這件事，就會答錯」的最小反例：
//   fragment          不穿透 ⇒ 真正的直接子層被算成孫層（會誤紅）
//   自閉標籤          誤入棧 ⇒ 之後的兄弟全被算進它底下（會誤綠）
//   屬性含 `>`        誤判標籤結束 ⇒ 同上，而且是本 repo 最常見的寫法
//   註解含標籤名      看到不存在的節點 ⇒ 同上
test("harness：JSX 結構掃描器的直接子層等於 CSS 子組合子", () => {
	const kidsOf = (source: string): string[] | null => {
		const hit = findJsx(parseJsx(source), (n) => hasJsxClass(n, "probe"))[0];
		return hit ? domChildren(hit).map((n) => n.tag) : null;
	};

	const cases: [string, string, string[] | null][] = [
		["直接子層", `<div className="probe"><form></form></div>`, ["form"]],
		[
			"多包一層 ⇒ form 落到孫層（這正是 P1-2 要抓的變異）",
			`<div className="probe"><div><form></form></div></div>`,
			["div"],
		],
		[
			"fragment 不產生節點，要穿透",
			`<div className="probe"><><form></form></></div>`,
			["form"],
		],
		[
			"三元的兩個分支都在同一層",
			`<div className="probe">{a ? (<><span /><b /></>) : (<form></form>)}</div>`,
			["span", "b", "form"],
		],
		[
			"自閉標籤不入棧",
			`<div className="probe"><input type="hidden" /><form></form></div>`,
			["input", "form"],
		],
		[
			"屬性裡的箭頭函式含 >",
			`<div className="probe"><button onClick={() => f(1)}></button><form></form></div>`,
			["button", "form"],
		],
		[
			"屬性裡的巢狀大括號含 >",
			`<div className="probe"><button onClick={() => { if (a>b) { f(); } }}></button><form></form></div>`,
			["button", "form"],
		],
		[
			"屬性字串含 >",
			`<div className="probe"><i title="a > b"></i><form></form></div>`,
			["i", "form"],
		],
		[
			"JSX 註解裡的標籤名不算節點",
			`<div className="probe">{/* 這個 <div> 是說明用的 */}<form></form></div>`,
			["form"],
		],
		[
			"元件標籤也算元素（它會渲染出節點，插在中間一樣會斷開選擇器）",
			`<div className="probe"><Button>x</Button><form></form></div>`,
			["Button", "form"],
		],
		[
			// 刻意寬鬆：`className={cx("probe", x)}` 仍要找得到那個節點。反過來若找不到，
			// 「解 DOM：…」開頭那句 `cells.length === 1` 會轉紅而不是靜默跳過——
			// 兩個方向都不會變成假綠，所以這裡選對重構更耐受的那一邊。
			"className 寫成表達式時仍認得類名",
			`<div className={cx("probe")}><form></form></div>`,
			["form"],
		],
		["多類名", `<div className="probe extra"><form></form></div>`, ["form"]],
	];

	for (const [name, source, want] of cases) {
		assert.deepEqual(
			kidsOf(source),
			want,
			`${name} —— 掃描器答錯這一列，「解 DOM：…」那兩支就會假通過`,
		);
	}

	// 文字歸屬：`{expr}` 必須落在它真正的所在節點上，否則 P1-1 的「容器不得有裸文字」
	// 那道反面夾擊會恆真（永遠看不到文字）或恆假（永遠看到）。
	const wrapped = findJsx(
		parseJsx(`<div className="probe"><i /><span>{m.contactEmail}</span></div>`),
		(n) => hasJsxClass(n, "probe"),
	)[0];
	assert.doesNotMatch(
		wrapped.text,
		/contactEmail/u,
		"包起來時容器不該有裸文字",
	);
	assert.match(
		domChildren(wrapped).find((n) => n.tag === "span")?.text ?? "",
		/contactEmail/u,
		"包起來時 span 應該持有文字",
	);

	const bare = findJsx(
		parseJsx(`<div className="probe"><i />{m.contactEmail}</div>`),
		(n) => hasJsxClass(n, "probe"),
	)[0];
	assert.match(bare.text, /contactEmail/u, "裸文字時容器應該持有它");
});

test("可收縮欄的 class 每張表只掛一欄——掛兩欄是靜默的版面破壞", async () => {
	// `.adm-audit-col-summary` 宣告 `width: 100%`（DelegatedOperations.css 該處註解：
	// 「全表唯一可收縮的那一欄」）。把它掛在兩個欄位上不會報錯、不會轉紅、CSS 也照樣生效，
	// 只是 table auto layout 會把剩餘壓力全丟給沒宣告的欄——實測後果是「動作」欄被壓成
	// 一字一行（調/整/授/權/範/圍 直排六行），而右側還留著大片空白。
	//
	// 2026-08-19 dev 實測抓到：代行授權歷程表把它掛在「範圍變更」與「理由」兩個 <td> 上，
	// 且一個 <th> 都沒掛。修法是掛回一欄、th/td 成對。
	//
	// 每張表的合法數量恰為 2：一個 <th> ＋ 一個 <td>。
	const tables = [
		"src/features/delegated-operations/components/MerchantDelegationAuthorizationPanel.tsx",
		"src/features/delegated-operations/components/SessionAuditConsole.tsx",
	];
	for (const file of tables) {
		const source = await readFile(file, "utf8");
		const hits = source.match(/adm-audit-col-summary/gu) ?? [];
		assert.equal(
			hits.length,
			2,
			`${file} 的 .adm-audit-col-summary 出現 ${hits.length} 次，應為 2（一個 <th> ＋ 一個 <td>）。` +
				"多了代表有第二欄也宣告 width:100%，少了代表沒有任何欄可收縮、整張表會被最長的內容撐開。",
		);
		assert.match(
			source,
			/<th[^>]*className="adm-audit-col-summary"/u,
			`${file} 的 <th> 沒有掛 .adm-audit-col-summary——只掛 <td> 時表頭與內容欄寬會各自為政`,
		);
	}
});

test("稽核表的操作者欄：識別碼不得與顯示名黏在同一行", async () => {
	// `.adm-audit-table td strong { display: block }` 是換行的唯一來源；顯示名若是裸
	// text node，後面那個 <span>{adminId}</span> 會直接接在它屁股後面，實測畫面上是
	// 「alexischen@example.com2661027599」連成一串，中間沒有任何分隔。
	const source = await readFile(
		"src/features/delegated-operations/components/MerchantDelegationAuthorizationPanel.tsx",
		"utf8",
	);
	assert.match(
		source,
		/<strong>\{entry\.actorDisplayName\}<\/strong>\s*<span>\{entry\.actorAdminId\}<\/span>/u,
		"操作者顯示名必須包在 <strong> 裡（CSS 給它 display:block），否則 adminId 會黏在同一行",
	);
});

test("摘要欄下限只宣告一次——第二份就是會漂移的第二個數字", async () => {
	// `--adm-audit-summary-min` 只宣告在 `.adm-audit-table-wrap`，讀它的
	// `.adm-audit-col-summary` 在三張表裡都包在那層之下，一份就夠。
	//
	// 曾經有第二份宣告在 `.adm-audit-layout`（側欄版拿它算換行門檻），而獨立使用本
	// wrap 的代行授權歷程面板不在那層之下 —— 2026-08-19 實測它的 min-width computed
	// 成 auto，240px 下限從未生效。一個解析不到的 var() 不會報錯、不會轉紅，
	// 只是那條規則整條失效。側欄拆掉之後那份已移除，別再加回來。
	const css = await readFile(
		"src/styles/pages/DelegatedOperations.css",
		"utf8",
	);
	const values = [...css.matchAll(/--adm-audit-summary-min:\s*(\d+)px;/gu)].map(
		(m) => m[1],
	);
	assert.equal(
		values.length,
		1,
		`--adm-audit-summary-min 宣告了 ${values.length} 次，應為 1（只在 .adm-audit-table-wrap）`,
	);
});

test("動作欄有 nowrap，否則它是全表唯一沒有寬度保護的欄", async () => {
	// `.adm-audit-col-summary` 的 width:100% 吃掉剩餘寬度後，時間與理由靠
	// `td:first-child`／`td:last-child` 的 nowrap 守住 max-content，操作者靠內容長度，
	// 只有動作欄什麼都沒有 ⇒ 壓力全落在它身上。2026-08-19 在 1440/1600/1920 三個寬度
	// 實測欄寬恆為 58.3px，「調整授權範圍」被壓成直排六行。
	const [css, panel] = await Promise.all([
		readFile("src/styles/pages/DelegatedOperations.css", "utf8"),
		readFile(
			"src/features/delegated-operations/components/MerchantDelegationAuthorizationPanel.tsx",
			"utf8",
		),
	]);
	assert.match(
		css,
		/\.adm-audit-col-action \{[^}]*white-space: nowrap;/u,
		".adm-audit-col-action 必須有 white-space: nowrap",
	);
	const hits = panel.match(/adm-audit-col-action/gu) ?? [];
	assert.equal(
		hits.length,
		2,
		`動作欄的 class 出現 ${hits.length} 次，應為 2（一個 <th> ＋ 一個 <td>）`,
	);
});
