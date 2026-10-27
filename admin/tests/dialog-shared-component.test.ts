import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test, { afterEach } from "node:test";
import { dom, React, requireFromTest } from "./fixtures/dom-harness";

/**
 * 共用 Dialog（src/components/UI/Dialog.tsx）的行為：後台所有對話框的 a11y 都由它一處保證，
 * 所以這裡直接把它掛進 DOM 驗 WAI-ARIA APG modal dialog 的每一項，而不是掃各對話框的原始碼。
 * 「沒有人繞過它」由 dialog-skeleton-guard.test.ts 守。
 */

// JSDOM 不排版，offsetParent 恆為 null，useFocusTrap 會把每個元素都當成看不見。
Object.defineProperty(dom.window.HTMLElement.prototype, "offsetParent", {
	configurable: true,
	get(this: HTMLElement) {
		return this.parentNode;
	},
});

const { createRoot } = requireFromTest("react-dom/client") as {
	createRoot: (el: Element) => {
		render: (node: unknown) => void;
		unmount: () => void;
	};
};
const Dialog = requireFromTest("../src/components/UI/Dialog")
	.default as unknown;

const doc = dom.window.document;
const h = React.createElement;
const { Fragment } = requireFromTest("react") as { Fragment: unknown };

type Mounted = { unmount: () => Promise<void> };
const open: Mounted[] = [];
afterEach(async () => {
	for (const m of open.splice(0)) await m.unmount();
	doc.body.replaceChildren();
	doc.body.style.overflow = "";
});

/** 在 body 底下放一個「背景」：觸發鈕與一段頁面內容，再掛載對話框。 */
async function mountWith(dialog: (onClose: () => void) => unknown) {
	const page = doc.createElement("main");
	const trigger = doc.createElement("button");
	trigger.textContent = "開啟";
	page.appendChild(trigger);
	doc.body.appendChild(page);
	trigger.focus();

	const host = doc.createElement("div");
	doc.body.appendChild(host);
	const root = createRoot(host);
	let mounted = true;
	const closed = { count: 0 };
	const unmount = () =>
		React.act(async () => {
			if (!mounted) return;
			mounted = false;
			root.unmount();
		});
	open.push({ unmount });
	await React.act(async () => {
		root.render(
			dialog(() => {
				closed.count += 1;
			}),
		);
	});
	return { page, trigger, host, closed, unmount };
}

const standard = (onClose: () => void, extra: Record<string, unknown> = {}) =>
	h(
		Dialog,
		{
			title: "標題",
			onClose,
			footer: h("button", { type: "button" }, "確認"),
			...extra,
		},
		h("input", { "aria-label": "欄位" }),
	);

const operation = (onClose: () => void, extra: Record<string, unknown> = {}) =>
	h(
		Dialog,
		{
			variant: "operation",
			title: "操作",
			titleIcon: "lock",
			description: "說明文字",
			onClose,
			footer: h("button", { type: "submit" }, "送出"),
			...extra,
		},
		h("input", { "aria-label": "欄位" }),
	);

async function key(key: string, shiftKey = false) {
	await React.act(async () => {
		(doc.activeElement ?? doc.body).dispatchEvent(
			new dom.window.KeyboardEvent("keydown", { key, shiftKey, bubbles: true }),
		);
	});
}

const panel = () => doc.querySelector('[role="dialog"]') as HTMLElement;
/** 面板的版面子項（不含最後那組 toast live region）。 */
const chrome = (el: Element) =>
	[...el.children].filter((c) => !c.classList.contains("adm-toast-host"));

test("aria：role、aria-modal、標題與說明的關聯、aria-busy、tabindex", async () => {
	await mountWith((c) => standard(c));
	const p = panel();
	assert.equal(p.getAttribute("aria-modal"), "true");
	assert.equal(p.getAttribute("tabindex"), "-1");
	assert.equal(p.getAttribute("aria-busy"), "false");
	const title = doc.getElementById(p.getAttribute("aria-labelledby") ?? "");
	assert.equal(title?.textContent, "標題");
	assert.equal(p.getAttribute("aria-describedby"), null);
	assert.equal(
		p.querySelector(".modal-close")?.getAttribute("aria-label"),
		"關閉",
	);
});

test("aria：operation 的說明成為 aria-describedby，圖示是裝飾", async () => {
	await mountWith((c) => operation(c, { closeLabel: "關閉操作視窗" }));
	const p = panel();
	const desc = doc.getElementById(p.getAttribute("aria-describedby") ?? "");
	assert.equal(desc?.textContent, "說明文字");
	const title = doc.getElementById(p.getAttribute("aria-labelledby") ?? "");
	assert.equal(title?.tagName, "H2");
	assert.equal(
		title?.querySelector("svg")?.getAttribute("aria-hidden"),
		"true",
	);
	assert.equal(
		p.querySelector(".adm-operation-dialog-close")?.getAttribute("aria-label"),
		"關閉操作視窗",
	);
});

test("aria：busy 時 aria-busy 為 true；ariaBusy 可單獨指定", async () => {
	await mountWith((c) => standard(c, { busy: true }));
	assert.equal(panel().getAttribute("aria-busy"), "true");
	await open[0].unmount();
	await mountWith((c) => standard(c, { ariaBusy: true }));
	assert.equal(panel().getAttribute("aria-busy"), "true");
	assert.equal(
		(doc.querySelector(".modal-close") as HTMLButtonElement).disabled,
		false,
	);
});

test("開啟時焦點進到對話框內第一個可聚焦元素", async () => {
	await mountWith((c) => standard(c));
	assert.equal(doc.activeElement, doc.querySelector(".modal-close"));
});

test("focus trap：最後一個元素按 Tab 回到第一個，第一個按 Shift+Tab 到最後一個", async () => {
	await mountWith((c) => standard(c));
	const first = doc.querySelector(".modal-close") as HTMLElement;
	const last = doc.querySelector(".modal-footer button") as HTMLElement;
	last.focus();
	await key("Tab");
	assert.equal(doc.activeElement, first);
	await key("Tab", true);
	assert.equal(doc.activeElement, last);
});

test("focus trap：Tab 的預設行為在邊界被攔下（不離開對話框）", async () => {
	await mountWith((c) => standard(c));
	(doc.querySelector(".modal-footer button") as HTMLElement).focus();
	let prevented = false;
	await React.act(async () => {
		const event = new dom.window.KeyboardEvent("keydown", {
			key: "Tab",
			bubbles: true,
			cancelable: true,
		});
		doc.activeElement?.dispatchEvent(event);
		prevented = event.defaultPrevented;
	});
	assert.equal(prevented, true);
});

test("Escape 呼叫 onClose；busy 時不呼叫，關閉鍵 disabled", async () => {
	const free = await mountWith((c) => standard(c));
	await key("Escape");
	assert.equal(free.closed.count, 1);
	await free.unmount();

	const busy = await mountWith((c) => standard(c, { busy: true }));
	await key("Escape");
	assert.equal(busy.closed.count, 0);
	const close = doc.querySelector(".modal-close") as HTMLButtonElement;
	assert.equal(close.disabled, true);
});

test("標題列的關閉鍵呼叫 onClose（standard 與 operation）", async () => {
	const a = await mountWith((c) => standard(c));
	await React.act(async () => {
		(doc.querySelector(".modal-close") as HTMLElement).click();
	});
	assert.equal(a.closed.count, 1);
	await a.unmount();

	const b = await mountWith((c) => operation(c));
	await React.act(async () => {
		(doc.querySelector(".adm-operation-dialog-close") as HTMLElement).click();
	});
	assert.equal(b.closed.count, 1);
});

test("點遮罩不關閉（所有對話框都是確認框，誤觸遮罩不得丟掉輸入）", async () => {
	const m = await mountWith((c) => standard(c));
	await React.act(async () => {
		(doc.querySelector(".modal-overlay") as HTMLElement).click();
	});
	assert.equal(m.closed.count, 0);
});

test("關閉（卸載）後焦點還給原本的觸發鈕", async () => {
	const m = await mountWith((c) => standard(c));
	assert.notEqual(doc.activeElement, m.trigger);
	await m.unmount();
	await React.act(async () => {
		await Promise.resolve();
	});
	assert.equal(doc.activeElement, m.trigger);
});

test("背景 inert：開啟時對話框以外的頁面 inert 且 aria-hidden，body 不捲動；關閉後精確還原", async () => {
	const m = await mountWith((c) => standard(c));
	assert.equal((m.page as unknown as { inert: boolean }).inert, true);
	assert.equal(m.page.getAttribute("aria-hidden"), "true");
	assert.equal(doc.body.style.overflow, "hidden");
	const overlay = doc.querySelector(".modal-overlay") as HTMLElement;
	assert.equal(
		Boolean((overlay as unknown as { inert: boolean }).inert),
		false,
	);
	assert.equal(overlay.getAttribute("aria-hidden"), null);

	await m.unmount();
	assert.equal(Boolean((m.page as unknown as { inert: boolean }).inert), false);
	assert.equal(m.page.getAttribute("aria-hidden"), null);
	assert.equal(doc.body.style.overflow, "");
});

test("結構：標題列、內容、動作列是面板的直接子項，只有內容區在中間（捲動的結構前提）", async () => {
	await mountWith((c) => standard(c));
	const kids = chrome(panel()).map((el) => el.className);
	assert.deepEqual(kids, ["modal-header", "modal-body", "modal-footer"]);

	await open[0].unmount();
	await mountWith((c) => operation(c));
	const opKids = chrome(panel()).map((el) => el.className);
	assert.deepEqual(opKids, [
		"adm-operation-dialog-header",
		"adm-operation-dialog-body",
		"adm-operation-dialog-actions",
	]);
});

test("form：內容與動作列一起包進 noValidate 的 form，標題列在外面", async () => {
	const submits: string[] = [];
	await mountWith((c) =>
		operation(c, {
			form: {
				method: "post",
				action: "/x",
				onSubmit: (e: Event) => {
					e.preventDefault();
					submits.push("sent");
				},
			},
		}),
	);
	const kids = chrome(panel());
	assert.deepEqual(
		kids.map((el) => el.tagName),
		["DIV", "FORM"],
	);
	const form = kids[1] as HTMLFormElement;
	assert.equal(form.noValidate, true);
	assert.equal(form.getAttribute("method"), "post");
	assert.deepEqual(
		[...form.children].map((el) => el.className),
		["adm-operation-dialog-body", "adm-operation-dialog-actions"],
	);
	await React.act(async () => {
		(form.querySelector('button[type="submit"]') as HTMLElement).click();
	});
	assert.deepEqual(submits, ["sent"]);
});

test("沒給 footer 就沒有動作列；size 與 maxWidth 只改面板", async () => {
	await mountWith((c) =>
		h(
			Dialog,
			{ variant: "operation", title: "稽核", size: "audit", onClose: c },
			"內容",
		),
	);
	assert.equal(doc.querySelector(".adm-operation-dialog-actions"), null);
	assert.ok(panel().classList.contains("adm-operation-dialog--audit"));
	await open[0].unmount();

	await mountWith((c) => standard(c, { maxWidth: 640 }));
	assert.equal(panel().style.maxWidth, "640px");
});

test("portal：overlay 一律掛在 body 底下（不在呼叫端的 DOM 子樹），仍隔離頁面並接得到 Escape", async () => {
	const m = await mountWith((c) => standard(c));
	assert.equal(m.host.querySelectorAll(".modal-overlay").length, 0);
	assert.equal(
		(doc.querySelector(".modal-overlay") as HTMLElement).parentElement,
		doc.body,
	);
	assert.equal((m.page as unknown as { inert: boolean }).inert, true);
	await key("Escape");
	assert.equal(m.closed.count, 1);
	await m.unmount();
	for (const variant of [operation]) {
		const other = await mountWith((c) => variant(c));
		assert.equal(other.host.querySelectorAll('[role="dialog"]').length, 0);
		assert.equal(panel().parentElement?.parentElement, doc.body);
		await other.unmount();
	}
});

test("side：與 standard 同一組結構與 a11y，只多 modifier class", async () => {
	const m = await mountWith((c) => standard(c, { variant: "side" }));
	const p = panel();
	assert.ok(p.classList.contains("modal-content"));
	assert.ok(p.classList.contains("modal-content--side"));
	assert.ok(
		(p.parentElement as HTMLElement).classList.contains("modal-overlay--side"),
	);
	assert.equal(p.getAttribute("aria-modal"), "true");
	assert.ok(doc.getElementById(p.getAttribute("aria-labelledby") ?? ""));
	assert.ok(
		p.querySelector(".modal-header") && p.querySelector(".modal-footer"),
	);
	assert.equal((m.page as unknown as { inert: boolean }).inert, true);
	await key("Escape");
	assert.equal(m.closed.count, 1);
});

test("堆疊：兩個對話框疊著，Escape 只關最上層；上層關掉後下層接手", async () => {
	const closes: string[] = [];
	const host = doc.createElement("div");
	doc.body.appendChild(host);
	const root = createRoot(host);
	const render = (showTop: boolean) =>
		root.render(
			h(
				Fragment,
				null,
				h(
					Dialog,
					{ title: "下層", onClose: () => closes.push("下層") },
					h("input", { "aria-label": "下層欄位" }),
				),
				showTop &&
					h(
						Dialog,
						{ title: "上層", onClose: () => closes.push("上層") },
						h("input", { "aria-label": "上層欄位" }),
					),
			),
		);
	open.push({ unmount: () => React.act(async () => root.unmount()) });
	await React.act(async () => render(true));
	assert.equal(doc.querySelectorAll('[role="dialog"]').length, 2);
	await key("Escape");
	assert.deepEqual(closes, ["上層"], "Escape 只該關最上層");
	await React.act(async () => render(false));
	await key("Escape");
	assert.deepEqual(closes, ["上層", "下層"], "上層卸載後下層才處理 Escape");
});

test("堆疊：Tab 循環只由最上層處理，下層的 focus trap 不介入", async () => {
	const host = doc.createElement("div");
	doc.body.appendChild(host);
	const root = createRoot(host);
	open.push({ unmount: () => React.act(async () => root.unmount()) });
	await React.act(async () => {
		root.render(
			h(
				Fragment,
				null,
				h(
					Dialog,
					{
						title: "下層",
						onClose() {},
						footer: h("button", { type: "button", id: "low-last" }, "下層鈕"),
					},
					h("input", { "aria-label": "下層欄位" }),
				),
				h(
					Dialog,
					{
						title: "上層",
						onClose() {},
						footer: h("button", { type: "button", id: "top-last" }, "上層鈕"),
					},
					h("input", { "aria-label": "上層欄位", id: "top-first" }),
				),
			),
		);
	});
	const [low, top] = [
		...doc.querySelectorAll('[role="dialog"]'),
	] as HTMLElement[];
	// 上層開啟後焦點在上層第一個可聚焦元素（標題列的關閉鍵）。
	assert.ok(top.contains(doc.activeElement), "焦點在上層");
	const lowLast = doc.getElementById("low-last") as HTMLElement;
	// 對下層的最後一個元素送 Tab：下層不是最上層，不得把焦點繞回下層第一個元素。
	lowLast.focus();
	const before = doc.activeElement;
	await key("Tab");
	assert.equal(doc.activeElement, before, "下層的 trap 不處理 Tab");
	// 上層自己的最後一個元素 Tab 會繞回上層第一個。
	(doc.getElementById("top-last") as HTMLElement).focus();
	await key("Tab");
	assert.ok(
		top.contains(doc.activeElement) &&
			doc.activeElement !== doc.getElementById("top-last"),
		"上層循環",
	);
	assert.ok(!low.contains(doc.activeElement));
});

/**
 * 捲動結構的另一半在 CSS：面板限高並排成直欄，標題列與動作列不縮，只有內容區捲動。
 * 沒有這幾條，DOM 結構對了也不會捲（高內容會把動作列推出視窗）。
 */
test("CSS：兩套面板都限高、直欄，只有內容區 overflow-y: auto", async () => {
	/** 規則的選擇器清單含這個選擇器（逐項比對）者，宣告串在一起。 */
	const declarations = (css: string, selector: string) => {
		const noComments = css.replace(/\/\*[\s\S]*?\*\//gu, "");
		let out = "";
		for (const m of noComments.matchAll(/([^{}]+)\{([^{}]*)\}/gu)) {
			const list = m[1].split(",").map((part) => part.trim());
			if (list.includes(selector)) out += m[2];
		}
		return out;
	};

	const visual = await readFile("src/styles/AdminVisualRefresh.css", "utf8");
	assert.match(
		declarations(visual, ".modal-content"),
		/max-height:\s*calc\(100dvh/u,
	);
	assert.match(
		declarations(visual, ".modal-content"),
		/flex-direction:\s*column/u,
	);
	assert.match(
		declarations(visual, ".modal-content .modal-body"),
		/overflow-y:\s*auto/u,
	);
	assert.match(
		declarations(visual, ".modal-content .modal-header"),
		/flex:\s*none/u,
	);
	assert.match(
		declarations(visual, ".modal-content .modal-footer"),
		/flex:\s*none/u,
	);

	const operationCss = await readFile("src/components/UI/Dialog.css", "utf8");
	assert.match(
		declarations(operationCss, ".adm-operation-dialog"),
		/max-height:/u,
	);
	assert.match(
		declarations(operationCss, ".adm-operation-dialog"),
		/flex-direction:\s*column/u,
	);
	assert.match(
		declarations(operationCss, ".adm-operation-dialog-body"),
		/overflow-y:\s*auto/u,
	);
	assert.match(
		declarations(operationCss, ".adm-operation-dialog-header"),
		/flex:\s*0 0 auto/u,
	);
	assert.match(
		declarations(operationCss, ".adm-operation-dialog-actions"),
		/flex:\s*0 0 auto/u,
	);
});

test("樣式不依賴祖先：沒有任何 CSS 規則把對話框骨架限定在某個頁面祖先底下", async () => {
	// 對話框 portal 到 body，不在頁面的 DOM 子樹裡；`.某頁 .modal-content` 這種規則一律不會命中。
	// 對話框的樣式只能用骨架自己的 class（.modal-*、.adm-operation-dialog-*）。
	const { readdir } = await import("node:fs/promises");
	const files = (await readdir("src", { recursive: true, withFileTypes: true }))
		.filter((e) => e.isFile() && e.name.endsWith(".css"))
		.map((e) => `${e.parentPath}/${e.name}`);
	assert.ok(files.length > 10);
	const SKELETON =
		/^(?:modal-(?:overlay|content|header|body|footer|close)|adm-operation-dialog[\w-]*)$/u;
	const offenders: string[] = [];
	for (const file of files) {
		const css = (await readFile(file, "utf8")).replace(
			/\/\*[\s\S]*?\*\//gu,
			"",
		);
		for (const m of css.matchAll(/([^{}]+)\{/gu)) {
			for (const selector of m[1].split(",")) {
				const compounds = selector.trim().split(/\s*[>+~]\s*|\s+/u);
				// 第一個複合選擇器是對話框骨架 class，才是「自己的規則」；骨架出現在後面的位置，
				// 而且前面有不屬於骨架的祖先 ⇒ 依賴祖先。
				const classesOf = (c: string) =>
					[...c.matchAll(/\.([\w-]+)/gu)].map((x) => x[1]);
				const first = classesOf(compounds[0]);
				const startsInsideDialog = first.some((c) => SKELETON.test(c));
				if (startsInsideDialog) continue;
				const hit = compounds
					.slice(1)
					.some((c) => classesOf(c).some((x) => SKELETON.test(x)));
				if (hit) offenders.push(`${file}: ${selector.trim()}`);
			}
		}
	}
	assert.deepEqual(offenders, []);
});

/** 兩個並列的對話框（a、b），可個別開關；b 在 a 之後掛載＝最上層。 */
async function mountPair() {
	const page = doc.createElement("main");
	page.textContent = "頁面";
	doc.body.appendChild(page);
	const host = doc.createElement("div");
	doc.body.appendChild(host);
	const root = createRoot(host);
	const render = (a: boolean, b: boolean) =>
		root.render(
			h(
				Fragment,
				null,
				a &&
					h(
						Dialog,
						{ key: "a", title: "下", onClose: () => {} },
						h("input", { "aria-label": "下欄位" }),
					),
				b &&
					h(
						Dialog,
						{ key: "b", title: "上", onClose: () => {} },
						h("input", { "aria-label": "上欄位" }),
					),
			),
		);
	open.push({
		unmount: () =>
			React.act(async () => {
				root.unmount();
			}),
	});
	const set = (a: boolean, b: boolean) => React.act(async () => render(a, b));
	return { page, set, root };
}

const isInert = (el: Element) =>
	Boolean((el as unknown as { inert: boolean }).inert);

test("堆疊隔離：先關下層，上層仍開著時頁面維持 inert 與鎖捲動；全關後完整還原", async () => {
	const m = await mountPair();
	await m.set(true, true);
	await m.set(false, true);
	assert.equal(isInert(m.page), true, "上層還開著，頁面不該被還原");
	assert.equal(m.page.getAttribute("aria-hidden"), "true");
	assert.equal(doc.body.style.overflow, "hidden");
	await m.set(false, false);
	assert.equal(isInert(m.page), false);
	assert.equal(m.page.getAttribute("aria-hidden"), null);
	assert.equal(doc.body.style.overflow, "");
});

test("堆疊隔離：先關上層，下層接手隔離；全關後完整還原", async () => {
	const m = await mountPair();
	await m.set(true, true);
	await m.set(true, false);
	assert.equal(isInert(m.page), true);
	assert.equal(doc.body.style.overflow, "hidden");
	assert.equal(
		isInert(doc.querySelector(".modal-overlay") as Element),
		false,
		"下層成為最上層，自己不能 inert",
	);
	await m.set(false, false);
	assert.equal(isInert(m.page), false);
	assert.equal(doc.body.style.overflow, "");
});

test("堆疊隔離：兩個對話框在同一個 commit 一起卸載，inert、aria-hidden 與 body 捲動都還原", async () => {
	const m = await mountPair();
	await m.set(true, true);
	await React.act(async () => m.root.unmount());
	assert.equal(isInert(m.page), false);
	assert.equal(m.page.getAttribute("aria-hidden"), null);
	assert.equal(doc.body.style.overflow, "");
});

test("堆疊隔離：本來就 inert／aria-hidden／有 overflow 的元素，全關後回到本來的值", async () => {
	const m = await mountPair();
	(m.page as unknown as { inert: boolean }).inert = true;
	m.page.setAttribute("aria-hidden", "false");
	doc.body.style.overflow = "scroll";
	await m.set(true, true);
	await m.set(false, true);
	await m.set(false, false);
	assert.equal(isInert(m.page), true);
	assert.equal(m.page.getAttribute("aria-hidden"), "false");
	assert.equal(doc.body.style.overflow, "scroll");
});

test("toast 不被隔離：帶 data-modal-exempt 的 sibling 在對話框開著時不 inert，也不 aria-hidden", async () => {
	const toast = doc.createElement("div");
	toast.setAttribute("data-modal-exempt", "");
	toast.setAttribute("role", "alert");
	doc.body.appendChild(toast);
	const m = await mountPair();
	await m.set(true, false);
	assert.equal(isInert(toast), false);
	assert.equal(toast.getAttribute("aria-hidden"), null);
	assert.equal(isInert(m.page), true);
});

test("tone=warning：標題列帶 warning modifier（standard 與 operation）", async () => {
	await mountWith((c) => standard(c, { tone: "warning" }));
	assert.ok(doc.querySelector(".modal-header--warning"));
	await open[0].unmount();
	await mountWith((c) => operation(c, { tone: "warning" }));
	assert.ok(doc.querySelector(".adm-operation-dialog-header--warning"));
	await open[1].unmount();
	await mountWith((c) => standard(c));
	assert.equal(doc.querySelector(".modal-header--warning"), null);
});

test("樣式：overlay 根明確設與 body 相同的字型與文字色（token），對話框與開啟它的頁面無關", async () => {
	const css = await readFile("src/styles/AdminVisualRefresh.css", "utf8");
	const rule = css.match(
		/\.modal-overlay,\s*\.adm-operation-dialog-overlay\s*\{([^}]*)\}/u,
	);
	assert.ok(rule, "找不到 overlay 根的字型規則");
	assert.match(rule[1], /color:\s*var\(--admin-ink\)/u);
	assert.match(rule[1], /font-family:\s*var\(--admin-font-ui\)/u);
	const body = css.match(/\nbody \{([^}]*)\}/u);
	assert.ok(body);
	assert.match(body[1], /color:\s*var\(--admin-ink\)/u);
	assert.match(body[1], /font-family:\s*var\(--admin-font-ui\)/u);
});

test("堆疊：外層與它的內層在同一個 commit 掛載（內層 effect 先跑），最上層仍是內層", async () => {
	const closes: string[] = [];
	const host = doc.createElement("div");
	doc.body.appendChild(host);
	const root = createRoot(host);
	open.push({ unmount: () => React.act(async () => root.unmount()) });
	await React.act(async () =>
		root.render(
			h(
				Dialog,
				{ title: "外層", onClose: () => closes.push("外層") },
				h(
					Dialog,
					{ title: "內層", onClose: () => closes.push("內層") },
					h("input", { "aria-label": "內層欄位" }),
				),
			),
		),
	);
	const overlays = [...doc.querySelectorAll(".modal-overlay")];
	assert.equal(overlays.length, 2);
	const byTitle = (title: string) =>
		overlays.find(
			(o) => o.querySelector("h3")?.textContent === title,
		) as Element;
	const outer = byTitle("外層");
	const inner = byTitle("內層");
	assert.equal(isInert(outer), true, "外層在內層底下，必須 inert");
	assert.equal(isInert(inner), false, "最上層的內層不能 inert");
	await key("Escape");
	assert.deepEqual(closes, ["內層"], "Escape 只關內層");
	// 內層與外層同在一個 commit 一起卸載後，隔離完整還原。
	await React.act(async () => root.unmount());
	assert.equal(doc.body.style.overflow, "");
});

const { ToastProvider, useToast } = requireFromTest(
	"../src/components/UI/Toast",
) as {
	ToastProvider: unknown;
	useToast: () => {
		showToast: (
			message: string,
			tone?: "success" | "warning" | "error",
		) => void;
	};
};

/** 掛一個 ToastProvider，底下可選地放對話框；回傳 showToast。 */
async function mountToast(dialogs: unknown[] = []) {
	let api: ReturnType<typeof useToast> | undefined;
	const Probe = () => {
		api = useToast();
		return null;
	};
	const host = doc.createElement("div");
	doc.body.appendChild(host);
	const root = createRoot(host);
	open.push({ unmount: () => React.act(async () => root.unmount()) });
	const render = (items: unknown[]) =>
		React.act(async () => {
			root.render(h(ToastProvider, null, h(Probe), ...items));
		});
	await render(dialogs);
	return {
		render,
		show: (message: string, tone?: "success" | "warning" | "error") =>
			React.act(async () => api?.showToast(message, tone)),
	};
}

const regionOf = (name: "status" | "alert", within: ParentNode = doc) =>
	within.querySelector(`[data-toast-region="${name}"]`) as HTMLElement;

/** 這個元素自己與祖先都沒有 inert／aria-hidden。 */
const reachable = (el: Element) => {
	for (let n: Element | null = el; n; n = n.parentElement) {
		if (isInert(n) || n.getAttribute("aria-hidden") === "true") return false;
	}
	return true;
};

test("toast：沒有對話框時進 body 的 live region；success 是 status，warning 與 error 是 alert", async () => {
	const t = await mountToast();
	await t.show("存好了");
	assert.equal(regionOf("status").textContent, "存好了");
	assert.equal(regionOf("alert").textContent, "");
	await t.show("結果不對勁", "warning");
	assert.equal(regionOf("alert").textContent, "結果不對勁");
	await t.show("失敗", "error");
	assert.equal(regionOf("alert").textContent, "失敗");
	assert.equal(regionOf("status").textContent, "");
	assert.ok(
		doc.querySelector(".adm-toast--error"),
		"色調走 class（token），不是內聯顏色",
	);
	assert.equal(
		(doc.querySelector(".adm-toast") as HTMLElement).getAttribute("style"),
		null,
	);
});

test("toast：新的取代舊的，舊的計時器不能把新的關掉；3 秒後消失", async (ctx) => {
	ctx.mock.timers.enable({ apis: ["setTimeout"] });
	const t = await mountToast();
	await t.show("第一則");
	ctx.mock.timers.tick(2000);
	await t.show("第二則");
	await React.act(async () => ctx.mock.timers.tick(2000));
	assert.equal(
		regionOf("status").textContent,
		"第二則",
		"第一則的計時器不該關掉第二則",
	);
	await React.act(async () => ctx.mock.timers.tick(1000));
	assert.equal(regionOf("status").textContent, "");
});

test("toast：body 的 live region 帶 data-modal-exempt，是 body 的直接子項", async () => {
	await mountToast();
	const host = regionOf("status").parentElement?.parentElement as HTMLElement;
	assert.equal(host.parentElement, doc.body);
	assert.ok(host.hasAttribute("data-modal-exempt"));
});

test("toast：對話框開著時進最上層對話框面板內的 live region，不是 inert 也不是 aria-hidden", async () => {
	const t = await mountToast([
		h(
			Dialog,
			{ key: "d", title: "對話框", onClose: () => {} },
			h("input", { "aria-label": "欄位" }),
		),
	]);
	await t.show("儲存失敗", "error");
	const region = regionOf("alert", panel());
	assert.equal(
		region.textContent,
		"儲存失敗",
		"訊息必須在 aria-modal 面板內，面板外的內容可能不進無障礙樹",
	);
	assert.equal(reachable(region), true);
	// body 那份不該同時顯示。
	const bodyHost = [...doc.body.children].find((c) =>
		c.hasAttribute("data-modal-exempt"),
	) as HTMLElement;
	assert.equal(regionOf("alert", bodyHost).textContent, "");
});

test("toast：疊著兩個對話框時進最上層；對話框關掉後換到 body 的 live region", async () => {
	const lower = h(
		Dialog,
		{ key: "a", title: "下層", onClose: () => {} },
		h("input", { "aria-label": "下" }),
	);
	const upper = h(
		Dialog,
		{ key: "b", title: "上層", onClose: () => {} },
		h("input", { "aria-label": "上" }),
	);
	const t = await mountToast([lower, upper]);
	await t.show("好了");
	const panels = [...doc.querySelectorAll('[role="dialog"]')];
	assert.equal(regionOf("status", panels[0]).textContent, "");
	assert.equal(regionOf("status", panels[1]).textContent, "好了");
	assert.equal(reachable(regionOf("status", panels[1])), true);
	await t.render([]);
	assert.equal(
		regionOf("status").textContent,
		"好了",
		"對話框都關了，訊息換到 body 的 region",
	);
});

test("toast：手機導覽抽屜是最上層的 modal 時，進抽屜內的 live region", async () => {
	const { ToastRegions } = requireFromTest(
		"../src/components/UI/ToastRegions",
	) as {
		ToastRegions: unknown;
	};
	const { useModalStack } = requireFromTest("../src/lib/a11y") as {
		useModalStack: (ref: unknown, enabled?: boolean) => unknown;
	};
	// 與 admin layout 相同的接法：導覽層登記進 modal 堆疊，抽屜（aria-modal 的 aside）裡放 ToastRegions。
	const { useRef } = requireFromTest("react") as {
		useRef: <T>(v: T) => { current: T };
	};
	const Drawer = () => {
		const ref = useRef(null);
		useModalStack(ref, true);
		return h(
			"div",
			{ ref },
			h(
				"aside",
				{ role: "dialog", "aria-modal": true, "aria-label": "主選單" },
				h(ToastRegions),
			),
		);
	};
	const t = await mountToast([h(Drawer, { key: "drawer" })]);
	await t.show("儲存失敗", "error");
	const drawer = doc.querySelector('aside[role="dialog"]') as HTMLElement;
	assert.equal(regionOf("alert", drawer).textContent, "儲存失敗");
	assert.equal(reachable(regionOf("alert", drawer)), true);
});
