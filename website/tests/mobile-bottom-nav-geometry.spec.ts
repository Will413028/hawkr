import { expect, test } from "@playwright/test";

/**
 * 防漂移閘門：`--mobile-bottom-nav-height` / `--mobile-bottom-nav-bottom`
 * （`styles/components/navigation-final.css`）必須等於 `MobileBottomNav` 的**實際渲染幾何**。
 *
 * 為什麼需要這支：該元件已遷 Tailwind，高度由內容決定、`bottom` 是 `bottom-2.5`，
 * CSS 那兩個變數只是「鏡像」而非成因，沒有任何機制保證同步。2026-07-30 實測發現
 * 宣告 74px、實際 67.5px —— 靜靜漂了 6.5px（且 `bottom` 宣告 `max(10px, env(safe-area-inset-bottom))`
 * 而元件其實固定 10px）。兩個消費者（`.page-content` 的 padding-bottom、sonner 手機版
 * 底部 offset）都吃這兩個變數，方向剛好偏大所以沒炸 —— 但那是運氣，不是正確。
 *
 * **這支是整條鏈唯一能發現「宣告值 ≠ 實際渲染」的環節。** 姊妹檔
 * `tests/mobile-bottom-nav-tokens.test.ts`（純文字、無瀏覽器）只比對 CSS 與 AppToaster
 * 兩份字面值，兩邊一起改就會過，抓不到與真實元件的落差。
 *
 * 下面 ①③④⑤ 量的其實是**同一個漂移量**，只是分別從四個角度看：① 直接比對，
 * ③ 透過 `.page-content` 的淨空、④ 透過 sonner 的底部 offset、⑤ 透過聊天附件選單
 * （`.attachment-picker` ≤540px，web#386 的第四份常數）。四條都用同一個
 * `TOLERANCE_PX`，所以任何一次「常數 ≠ 實際渲染」都會同時觸發，不會只差
 * 一點點就從上界溜過去（原本 ③④ 寫死上界 24px，6.5px 的漂移只差 1.5px 就漏掉）。
 *
 * 這支不是截圖比對，而是斷言不變式，所以不會因為配色/文案改動而假紅。
 *
 * 底部導覽列的顯示條件是元件上的 `max-[840px]:grid`＝`not (min-width: 840px)`，
 * **不含** 840，故本檔只在 <840 的 viewport 有意義（`--project=mobile`；chromium
 * 桌機 project 會 skip）。
 */

/** 元件 `max-[840px]:grid` 的斷點；viewport ≥ 此值時導覽列 display:none。 */
const BOTTOM_NAV_BREAKPOINT = 840;

/** sonner 手機版底部 offset 的表達式；與 AppToaster 的 MOBILE_DOCK_BOTTOM_OFFSET 同一份
 * （那份的 `+ 12px` 由 tokens.test.ts 的逐字比對釘住，這裡不會單獨漂）。 */
const TOAST_DOCK_OFFSET =
	"calc(var(--mobile-bottom-nav-height) + var(--mobile-bottom-nav-bottom) + 12px)";

/** `.page-content` 淨空的設計意圖：導覽列佔用高度之外再留這麼多呼吸空間
 * （來源＝navigation-final.css 那段 calc 的 `+ 16px`）。 */
const PAGE_CONTENT_BREATHING_PX = 16;

/** sonner 底緣與導覽列頂緣的設計意圖間距（來源＝上方 TOAST_DOCK_OFFSET 的 `+ 12px`）。 */
const TOAST_BREATHING_PX = 12;

/** 聊天附件選單（`.attachment-picker` 的 ≤540px 規則）讓開導覽列的位移表達式；
 * 與 messages.css 同一份，由 tokens.test.ts 的逐字比對釘住不會單獨漂。
 * 設計意圖是「切齊導覽列頂緣」（原本的 84px = 74px 高 + 10px 離底，零額外間距）。 */
const ATTACHMENT_PICKER_OFFSET =
	"calc(var(--mobile-bottom-nav-height) + var(--mobile-bottom-nav-bottom))";

/** `.attachment-picker` 底緣與導覽列頂緣的設計意圖間距。 */
const ATTACHMENT_PICKER_BREATHING_PX = 0;

/**
 * 全檔唯一的容差來源。
 *
 * 依據：導覽列高度的組成中，只有 11px 標籤那一行的 `line-height: normal` 由字型度量
 * 決定（1px 上下框、8px padding、30px icon、3px gap、`min-h-12` 都是固定 px），
 * 而本機 Chrome 與 CI 的 headless shell 可能落在不同 fallback 字型，該行高會有
 * 不到 1px 的差。2px＝給那一行兩倍餘裕。
 *
 * 為什麼不更小：0.1px 會讓字型差異變成假紅（flake）。
 * 為什麼不更大：這支存在的理由就是抓 6.5px 那種級別的漂移；上界一旦鬆到 24px，
 * 6.5px 只差 1.5px 就能溜過去 —— 那不是防線，是裝飾。
 */
const TOLERANCE_PX = 2;

function px(value: string): number {
	const n = Number.parseFloat(value);
	expect(Number.isFinite(n), `無法解析為 px 數值：${value}`).toBe(true);
	return n;
}

test("宣告的底部導覽列幾何 == 實際渲染幾何，且淨空不與其重疊", {
	tag: ["@ui", "@live"],
}, async ({ page, viewport }) => {
	// 邊界與元件一致：`max-[840px]` 不含 840，正好 840px 時導覽列已 display:none，
	// 本檔的斷言在那裡沒有意義（寫成 `> 840` 會讓 840px 不 skip 而直接失敗）。
	test.skip(
		!viewport || viewport.width >= BOTTOM_NAV_BREAKPOINT,
		`mobile-only：底部導覽列在 ≥${BOTTOM_NAV_BREAKPOINT}px 是 display:none`,
	);

	await page.goto("/", { waitUntil: "domcontentloaded" });

	const nav = page.getByRole("navigation", { name: "手機底部導覽" });
	await expect(nav).toBeVisible();

	// 標籤行高受字型度量影響，字型載入前後高度會差 —— 先等字型就緒再量。
	await page.evaluate(() => document.fonts.ready);

	const geo = await page.evaluate(
		([toastOffsetExpr, pickerOffsetExpr]) => {
			const root = getComputedStyle(document.documentElement);
			const el = document.querySelector<HTMLElement>(
				'nav[aria-label="手機底部導覽"]',
			);
			if (!el) throw new Error("底部導覽列不存在");
			const s = getComputedStyle(el);
			const rect = el.getBoundingClientRect();

			// 用一個零高度探針把 offset 表達式交給瀏覽器求值，避免在測試裡
			// 重算 calc（重算就等於再抄一份數字，又是一個會漂的副本）。
			const probeFromBottom = (expr: string): number => {
				const probe = document.createElement("div");
				probe.style.position = "fixed";
				probe.style.left = "0";
				probe.style.height = "0";
				probe.style.width = "1px";
				probe.style.bottom = expr;
				document.body.appendChild(probe);
				const bottom = probe.getBoundingClientRect().bottom;
				probe.remove();
				return window.innerHeight - bottom;
			};

			const pageContent = document.querySelector(".page-content");

			return {
				declaredHeight: root
					.getPropertyValue("--mobile-bottom-nav-height")
					.trim(),
				declaredBottom: root
					.getPropertyValue("--mobile-bottom-nav-bottom")
					.trim(),
				display: s.display,
				computedBottom: s.bottom,
				renderedHeight: rect.height,
				// 導覽列頂緣距 viewport 底緣的距離 = 它實際佔掉的空間
				occupiedFromBottom: window.innerHeight - rect.top,
				toastBaselineFromBottom: probeFromBottom(toastOffsetExpr),
				attachmentPickerBaselineFromBottom: probeFromBottom(pickerOffsetExpr),
				// 元件從未套用 safe-area；這裡量出來恆為 0，是「不准寫 env()」的依據。
				safeAreaInsetBottom: probeFromBottom("env(safe-area-inset-bottom)"),
				pageContentPadBottom: pageContent
					? getComputedStyle(pageContent).paddingBottom
					: null,
				innerHeight: window.innerHeight,
			};
		},
		[TOAST_DOCK_OFFSET, ATTACHMENT_PICKER_OFFSET] as const,
	);

	expect(geo.display).toBe("grid");

	// --- 1) 宣告高度 vs 實際渲染高度（唯一直接比對「常數 vs 真實元件」的斷言）---
	const declaredHeight = px(geo.declaredHeight);
	expect(
		Math.abs(declaredHeight - geo.renderedHeight),
		`--mobile-bottom-nav-height 宣告 ${geo.declaredHeight}，實際渲染 ${geo.renderedHeight}px。` +
			`改過 MobileBottomNav 的 padding / icon 尺寸 / 標籤字級就要同步這個變數。`,
	).toBeLessThanOrEqual(TOLERANCE_PX);

	// --- 2) 宣告的離底距離 vs 元件實際的 bottom ---
	// 這一項沒有字型度量參與（`bottom-2.5` 是固定值），所以維持嚴格比對。
	expect(
		px(geo.declaredBottom),
		`--mobile-bottom-nav-bottom 宣告 ${geo.declaredBottom}，元件實際 bottom 為 ${geo.computedBottom}`,
	).toBeCloseTo(px(geo.computedBottom), 1);

	// --- 3) 內容淨空：page-content 的 padding-bottom 必須讓開整條導覽列 ---
	expect(geo.pageContentPadBottom).not.toBeNull();
	const pad = px(geo.pageContentPadBottom as string);
	// 3a) 使用者可見的不變式：內容不能被導覽列蓋住。
	expect(
		pad,
		`page-content padding-bottom (${pad}px) 必須 ≥ 導覽列佔用高度 (${geo.occupiedFromBottom}px)`,
	).toBeGreaterThanOrEqual(geo.occupiedFromBottom);
	// 3b) 漂移閘門：留白必須「正好是」設計意圖，偏多偏少都代表常數與實際渲染脫鉤。
	const padBreathing = pad - geo.occupiedFromBottom;
	expect(
		Math.abs(padBreathing - PAGE_CONTENT_BREATHING_PX),
		`page-content 底部留白為 ${padBreathing}px，設計意圖是 ${PAGE_CONTENT_BREATHING_PX}px。` +
			`差距即為「宣告值 − 實際渲染高度」的漂移量。`,
	).toBeLessThanOrEqual(TOLERANCE_PX);

	// --- 4) toast 淨空：sonner 底部停靠處必須在導覽列頂緣之上，且不脫太遠 ---
	// 這同時是「不恢復 z-index: --z-toast + 1」那個決定的依據：兩者不相交，
	// 所以誰疊誰沒有可觀察的差別（見 navigation-final.css 的 z-index 決策註解）。
	const toastGap = geo.toastBaselineFromBottom - geo.occupiedFromBottom;
	expect(
		toastGap,
		`toast 底緣 (${geo.toastBaselineFromBottom}px) 必須高於導覽列頂緣 (${geo.occupiedFromBottom}px)`,
	).toBeGreaterThan(0);
	expect(
		Math.abs(toastGap - TOAST_BREATHING_PX),
		`toast 與導覽列的間距為 ${toastGap}px，設計意圖是 ${TOAST_BREATHING_PX}px。` +
			`差距即為「宣告值 − 實際渲染高度」的漂移量。`,
	).toBeLessThanOrEqual(TOLERANCE_PX);

	// --- 5) 聊天附件選單的底部位移必須切齊導覽列頂緣（web#386 的第四份常數）---
	// 這條與 ③④ 是同一個漂移量的第四個視角。原本寫死 `calc(84px + env(...))`：
	// 84px 是遷 Tailwind 前的 74px 高 + 10px 離底，實測比導覽列頂緣高 6.5px，
	// 而 env() 在真機上還會再多讓 24px 以上。
	const pickerGap =
		geo.attachmentPickerBaselineFromBottom - geo.occupiedFromBottom;
	expect(
		Math.abs(pickerGap - ATTACHMENT_PICKER_BREATHING_PX),
		`.attachment-picker 底緣 (${geo.attachmentPickerBaselineFromBottom}px) 與導覽列頂緣 ` +
			`(${geo.occupiedFromBottom}px) 的間距為 ${pickerGap}px，設計意圖是 ` +
			`${ATTACHMENT_PICKER_BREATHING_PX}px（切齊）。`,
	).toBeLessThanOrEqual(TOLERANCE_PX);

	// --- 6) safe-area 探針：模擬環境恆為 0，坐實「元件沒有套 safe-area」---
	// 這一項不是為了防漂移，而是把「為什麼上面那些表達式不准寫 env()」的依據
	// 留在測試輸出裡：一旦哪天瀏覽器模擬環境開始回傳非 0，這個假設就要重新檢視。
	expect(
		geo.safeAreaInsetBottom,
		"env(safe-area-inset-bottom) 在瀏覽器模擬環境應為 0；" +
			"若不再為 0，navigation-final.css 與 messages.css 的「不准寫 env()」註解需重新評估",
	).toBe(0);
});
