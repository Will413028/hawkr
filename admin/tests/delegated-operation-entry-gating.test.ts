import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import test from "node:test";

/**
 * 賣家列的代行入口有兩條互不相干的關閉路徑，設計書 6.1 對兩者的要求剛好相反：
 *
 *   ① 操作者自己沒有 session-create capability → 入口**不顯示**
 *      （驗收原文：「不能只用 disabled button 掩飾權限」）
 *   ② 目標店家不合格（停權／未啟用／缺 storeId）→ **停用按鈕，label 就是原因**
 *      （圖 3 的「綠野有機農場（停權中）」列：灰色藥丸寫著原因；字由伺服器給：「帳號停權中，無法代行。」）
 *
 * 修前兩者共用同一個布林，兩種情形都走隱藏 ⇒ 管理員看到的是「有些列有按鈕、有些
 * 沒有」，而原因整個消失。所以本檔的每一條都成對：①、② 各要能單獨被打紅。
 *
 * 這幾支真的把元件渲染出來（`renderToStaticMarkup`），不是拿 regex 掃源碼 ——
 * 「按鈕在不在」「有沒有 disabled」「上面寫什麼」正是 regex 最容易假綠的地方。
 * 為此要處理三件事，都與契約無關、純粹是把元件搬進 node 跑起來：
 *   - `.css` 匯入（`@/components/UI` 那條鏈）在 node 下是語法錯 → 換成空模組
 *   - React／react-query 必須是**單一實例**，走 `import()` 會拿到與元件不同的那份
 *     （package exports 的 ESM／CJS 兩個入口），context 就對不上 → 統一用 require
 *   - `useSearchParams()` 沒有 provider 時回 null，元件第一行就炸 → 自己給一個
 */

const requireFromTest = createRequire(__filename);
requireFromTest.extensions[".css"] = () => {};

interface ReactLike {
	createElement: (
		type: unknown,
		props?: unknown,
		...children: unknown[]
	) => unknown;
}

const { createElement } = requireFromTest("react") as ReactLike;
const { renderToStaticMarkup } = requireFromTest("react-dom/server") as {
	renderToStaticMarkup: (element: unknown) => string;
};
const { QueryClient, QueryClientProvider } = requireFromTest(
	"@tanstack/react-query",
) as {
	QueryClient: new (options?: unknown) => unknown;
	QueryClientProvider: unknown;
};
const { SearchParamsContext } = requireFromTest(
	"next/dist/shared/lib/hooks-client-context.shared-runtime",
) as { SearchParamsContext: { Provider: unknown } };

const MerchantOperationEntry = requireFromTest(
	"../src/features/delegated-operations/components/MerchantOperationEntry",
).default as unknown;
const OperationPreflightDialog = requireFromTest(
	"../src/features/delegated-operations/components/OperationPreflightDialog",
).default as unknown;

/**
 * 元件用到的兩個 provider。不傳 `client` 時 query 一律是 disabled，不會發請求；
 * ③ 那一組會先用 `prefetchQuery` 把同一把 queryKey 的結果（成功或錯誤）放進 cache，
 * 元件 render 時直接讀到那個狀態 —— 不碰 react-query 內部，也不真的打網路。
 */
function renderEntry(
	props: Record<string, unknown>,
	search = "",
	client: unknown = new QueryClient({
		defaultOptions: { queries: { retry: false } },
	}),
): string {
	return renderToStaticMarkup(
		createElement(
			SearchParamsContext.Provider,
			{ value: new URLSearchParams(search) },
			createElement(
				QueryClientProvider,
				{ client },
				createElement(MerchantOperationEntry, props),
			),
		),
	);
}

const STORE_ID = "2685914682";

const { DelegatedOperationClientError } = requireFromTest(
	"../src/services/delegated-operations",
) as {
	DelegatedOperationClientError: new (
		message: string,
		status: number,
		code?: number | string,
		traceId?: string,
		deniedCode?: number,
	) => Error;
};

/**
 * 把 preflight 的結果（成功或錯誤）先放進 cache，元件 render 時直接讀到它。
 *
 * ⚠️ `retryOnMount: false` 不是為了省一次請求，是**沒有它就量不到錯誤態**：
 * react-query 的 `shouldLoadOnMount` 看的是「有沒有 data」而不是「有沒有 error」，
 * 掛載時它會樂觀地把無 data 的錯誤態重設成 pending／fetching（`fetchState` 那一段），
 * 於是單次 `renderToStaticMarkup` 永遠只看得到 loading，錯誤分支測不到。瀏覽器上不會
 * 有這個問題——那裡 fetch 會真的跑完再 render 第二次。
 */
async function seedPreflight(result: {
	resolve?: unknown;
	reject?: unknown;
}): Promise<unknown> {
	const client = new QueryClient({
		defaultOptions: { queries: { retry: false, retryOnMount: false } },
	}) as {
		prefetchQuery: (options: unknown) => Promise<void>;
	};
	await client.prefetchQuery({
		queryKey: ["delegated-operation", "preflight", STORE_ID],
		queryFn: () =>
			"reject" in result
				? Promise.reject(result.reject)
				: Promise.resolve(result.resolve),
		retry: false,
	});
	return client;
}

const ALLOWED_PREFLIGHT = {
	allowed: true,
	target: {
		storeId: STORE_ID,
		sellerAccountId: STORE_ID,
		storeName: "陳記",
	},
	actor: { adminId: "2661027599", displayName: "alexischen" },
	reasonOptions: [
		{ code: "seller_request", label: "賣家請求", caseReferenceRequired: false },
	],
	capabilities: ["storefront.read"],
	ttlSeconds: 600,
	idleSeconds: 900,
	caseReferenceRequired: false,
};

test("① 操作者無 session-create capability：入口整個不渲染，且原因不得把它救回來", () => {
	assert.equal(renderEntry({ storeId: "2685914682", enabled: false }), "");

	// 這一條才是 6.1 驗收真正要擋的東西：目標店家同時也不合格時，仍然**不准**退化成
	// 一顆停用按鈕 —— 那就是拿 disabled 掩飾權限。②「有原因就渲染」若寫在權限閘門
	// 之前，這裡會吐出整顆按鈕。
	assert.equal(
		renderEntry({
			storeId: "2685914682",
			enabled: false,
			blockedReason: "帳號停權中，無法代行。",
		}),
		"",
	);
});

test("② 目標店家不合格：按鈕仍在原位，disabled，且 label 就是原因", () => {
	const html = renderEntry({
		storeId: "2685914682",
		enabled: true,
		blockedReason: "帳號停權中，無法代行。",
		label: "代行營運",
		compact: true,
	});

	const button = html.match(/<button[^>]*>([\s\S]*?)<\/button>/u);
	assert.ok(button, `不合格的列沒有渲染任何按鈕（操作欄空白）：${html}`);
	assert.match(
		button[0],
		/\sdisabled\b/u,
		`按鈕沒有 disabled ⇒ 不可代行的店家看起來可以按：${button[0]}`,
	);
	// 原因必須是按鈕自己的文字（可讀名稱），不是藏在 title／tooltip 裡。
	assert.equal(button[1].trim(), "帳號停權中，無法代行。");
	// 通用 label 不得蓋掉原因 —— 顯示「代行營運」等於又把原因藏起來了。
	assert.doesNotMatch(button[0], /代行營運/u);

	// 停用態不得帶對話框語意：它不會打開任何東西。
	assert.doesNotMatch(button[0], /aria-haspopup/u);
	assert.doesNotMatch(button[0], /aria-expanded/u);
});

test("② 合格的列不受影響：仍是可按的入口，會開對話框", () => {
	const html = renderEntry({
		storeId: "2685914682",
		enabled: true,
		label: "代行營運",
		compact: true,
	});
	assert.match(html, /aria-haspopup="dialog"/u);
	assert.doesNotMatch(html, /\sdisabled\b/u);
	assert.match(html, /代行營運/u);
});

/**
 * ③ 單店畫面（商家詳情頁）走 `resolveEligibility`：資格由伺服器的 preflight 決定。
 *
 * 修前詳情頁自己算 `isApproved && suspensionConfirmed && !activeSuspension`，那份判準
 * 問不到 preflight 的任何一道前置 —— 治理停權只寫 `sellers.status`、不寫
 * `seller_suspensions` 列 ⇒ 按鈕照樣可按，點下去才被 9272 擋；feature flag 與 allowlist
 * 那一側（9265）更是完全看不到。
 */
test("③ preflight 拒絕：按鈕停用，label 就是伺服器下發的原因", async () => {
	const client = await seedPreflight({
		reject: new DelegatedOperationClientError(
			"店家等待刪除，無法代行。",
			403,
			"DELEGATED_OPERATION_DENIED",
			undefined,
			9272,
		),
	});
	const html = renderEntry(
		{ storeId: STORE_ID, enabled: true, resolveEligibility: true },
		"",
		client,
	);

	const button = html.match(/<button[^>]*>([\s\S]*?)<\/button>/u);
	assert.ok(button, `沒有渲染任何按鈕：${html}`);
	assert.match(
		button[0],
		/\sdisabled\b/u,
		`preflight 已經說不行，按鈕卻還可以按：${button[0]}`,
	);
	assert.equal(button[1].trim(), "店家等待刪除，無法代行。");
	assert.doesNotMatch(button[0], /aria-haspopup/u);
});

test("③ preflight 還在問：停用並說正在確認，不先給可按的入口再抽掉", () => {
	const html = renderEntry({
		storeId: STORE_ID,
		enabled: true,
		resolveEligibility: true,
	});
	const button = html.match(/<button[^>]*>([\s\S]*?)<\/button>/u);
	assert.ok(button, `沒有渲染任何按鈕：${html}`);
	assert.match(button[0], /\sdisabled\b/u);
	assert.equal(button[1].trim(), "確認代行資格中…");
});

test("③ preflight 通過：仍是可按的入口", async () => {
	const client = await seedPreflight({ resolve: ALLOWED_PREFLIGHT });
	const html = renderEntry(
		{ storeId: STORE_ID, enabled: true, resolveEligibility: true },
		"",
		client,
	);
	assert.match(html, /aria-haspopup="dialog"/u);
	assert.doesNotMatch(html, /\sdisabled\b/u);
});

/**
 * 「問不到」不是「不行」。preflight 回 404／502／504 時把入口畫成停用，管理員連
 * 重試的入口都沒有（重試鈕在對話框裡）——而伺服器端本來就 fail-closed，前端不必
 * 替它再擋一次。
 */
test("③ preflight 問不到（非 403）：入口保持可按，讓對話框顯示錯誤與重試", async () => {
	const client = await seedPreflight({
		reject: new DelegatedOperationClientError(
			"無法連線至代行營運服務",
			502,
			"UPSTREAM_UNREACHABLE",
		),
	});
	const html = renderEntry(
		{ storeId: STORE_ID, enabled: true, resolveEligibility: true },
		"",
		client,
	);
	assert.match(html, /aria-haspopup="dialog"/u);
	assert.doesNotMatch(html, /\sdisabled\b/u);
});

test("③ 權限閘門仍在最前面：enabled=false 時 resolveEligibility 救不回入口", async () => {
	const client = await seedPreflight({ resolve: ALLOWED_PREFLIGHT });
	assert.equal(
		renderEntry(
			{ storeId: STORE_ID, enabled: false, resolveEligibility: true },
			"",
			client,
		),
		"",
	);
});

test("賣家列把入口永遠掛上去，原因只來自伺服器給的 delegationUnavailabilities，client 沒有本地判準", async () => {
	const page = await readFile("src/app/admin/merchants/page.tsx", "utf8");

	// 修前是 `{canStartDelegatedOperation && <MerchantOperationEntry …>}`：不合格的
	// 列連 DOM 都沒有，於是原因無處可放。
	assert.doesNotMatch(
		page,
		/canStartDelegatedOperation && \(?\s*<MerchantOperationEntry/u,
		"入口又被包回條件渲染 ⇒ 不合格的列回到空白操作欄",
	);
	assert.match(page, /<MerchantOperationEntry/u);
	assert.match(page, /blockedReason=\{delegationBlockedReason\}/u);

	// 原因只讀伺服器的原因清單（店家狀態：已關閉、等待刪除、停權；句子由伺服器給）。
	assert.match(
		page,
		/const delegationBlockedReason =\s*m\.delegationUnavailabilities\.length > 0\s*\?\s*unavailableLabels\(m\.delegationUnavailabilities\)\[0\]\s*:\s*undefined;/u,
		"原因不再來自 delegationUnavailabilities ⇒ client 又在自己推導",
	);
	// 不得有本地判準：清單頁不讀 StoreStatus、isSuspended、canStartDelegatedOperation 與 rowState(...).isActive && 這類資格。
	assert.doesNotMatch(
		page,
		/StoreStatus\.|isSuspended|canStartDelegatedOperation|rowState\(m\)\.isActive\s*&&|lifecycleBlockedLabel|delegationBlockedLabel/u,
		"清單頁出現本地的代行資格判準",
	);
});

/**
 * JSX 的 `disabled` 只擋得住點擊，擋不住它**看起來**可以點：`.adm-operation-trigger`
 * 的金色底與 hover 浮起原本對停用態照樣生效（`:hover` 在 disabled 按鈕上仍然命中）。
 * 兩半各守一邊 —— 上面那支拿掉 `disabled` 會紅，這一支拿掉樣式才會紅。
 */
test("停用態有自己的中性樣式，且 hover／active 不再套到它身上", async () => {
	const css = await readFile(
		"src/styles/pages/DelegatedOperations.css",
		"utf8",
	);
	const ruleBody = (selector: string): string | undefined =>
		css
			.split("}")
			.find((chunk) => chunk.includes(`${selector} {`))
			?.split("{")[1];

	const disabled = ruleBody(".adm-operation-trigger:disabled");
	assert.ok(
		disabled,
		"沒有 .adm-operation-trigger:disabled ⇒ 停用按鈕沿用金色底，看起來仍可按",
	);
	assert.match(disabled, /cursor:\s*not-allowed/u);
	assert.doesNotMatch(
		disabled,
		/--hk-secondary/u,
		"停用態又用回金色 token ⇒ 與可按的入口分不出來",
	);

	for (const state of ["hover", "active"]) {
		assert.ok(
			ruleBody(`.adm-operation-trigger:${state}:not(:disabled)`),
			`.adm-operation-trigger:${state} 沒有排除 :disabled ⇒ 游標移過停用按鈕它會浮起來、換成可按的顏色`,
		);
	}
});

/**
 * A11：代行原因是必選，但同一張表單只有下方案件欄標了必填。修前這一組的必填性
 * 要等到送出被擋（handleSubmit 的「請選擇代行原因。」）才第一次現身，而設計稿是
 * 一進場就寫在標題上（`* 代行操作理由（必選）`）。
 */
test("代行原因群組一進場就標示必選，與案件欄同一種寫法", () => {
	const html = renderToStaticMarkup(
		createElement(OperationPreflightDialog, {
			preflight: {
				allowed: true,
				target: {
					storeId: "2685914682",
					sellerAccountId: "2685914682",
					storeName: "陳記",
				},
				actor: { adminId: "2661027599", displayName: "alexischen" },
				reasonOptions: [
					{
						code: "seller_request",
						label: "賣家請求",
						caseReferenceRequired: false,
					},
				],
				capabilities: ["storefront.read"],
				ttlSeconds: 600,
				idleSeconds: 900,
				caseReferenceRequired: false,
			},
			requestedStoreId: "2685914682",
			onClose: () => {},
		}),
	);

	const legend = html.match(/<legend[^>]*>([\s\S]*?)<\/legend>/u);
	assert.ok(legend, "找不到代行原因群組的 legend");
	assert.match(
		legend[1],
		/（必選）/u,
		`代行原因沒有必填標示（legend 為「${legend[1]}」）⇒ 必填性只在送出失敗後才揭露，而同一張表單的案件欄一進場就講明了`,
	);
	// 標示是加在既有問句上，不是把問句換掉。
	assert.match(legend[1], /這次為什麼需要代行？/u);
});
