import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import PostPublishControl from "../features/seller-center/posts/PostPublishControl";
import {
	POST_PUBLISH_ACTION_LABEL,
	postPublishToggleTarget,
} from "../features/seller-center/posts/postPublishActions";
import { PostPublishStatus } from "../lib/codemeta/enums";

/**
 * 賣家貼文的發布／下架入口。
 *
 * 這一顆補的是「後端做了、使用者摸不到」：`POST /app/sellers/posts/{id}/publish`
 * 與 `.../unpublish` 兩支端點在 gateway 註冊了（`routes_post.go:76-77`），而整個網頁
 * 一個呼叫端都沒有 —— 賣家看得到「未發布」pill，卻沒有任何一顆按鈕能發布。
 *
 * 所以斷言的重點不是「元件會不會渲染」，是**每一個賣家真的會遇到的狀態下，那顆
 * 按鈕在不在、方向對不對**。
 */

const PUBLISH_LABEL = POST_PUBLISH_ACTION_LABEL[PostPublishStatus.PUBLISHED];
const UNPUBLISH_LABEL =
	POST_PUBLISH_ACTION_LABEL[PostPublishStatus.UNPUBLISHED];

function render(
	props: Partial<React.ComponentProps<typeof PostPublishControl>> = {},
): string {
	return renderToStaticMarkup(
		React.createElement(PostPublishControl, {
			hasImage: true,
			onToggle: () => {},
			...props,
		}),
	);
}

type ClickableProps = {
	children?: React.ReactNode;
	onClick?: (event: unknown) => void;
};

/**
 * `renderToStaticMarkup` 能守住可見／無障礙狀態，卻不會替我們執行事件。這顆控制項
 * 沒有 provider 或資料請求，因此直接展開它回傳的 React tree，找實際 click handler
 * 驗證 gate：反灰仍可被使用者觸發以取得原因，但絕不能把發布命令漏給 `onToggle`。
 */
function firstClickHandler(
	node: React.ReactNode,
): ClickableProps["onClick"] | undefined {
	if (Array.isArray(node)) {
		for (const child of node) {
			const handler = firstClickHandler(child);
			if (handler) return handler;
		}
		return undefined;
	}
	if (!React.isValidElement<ClickableProps>(node)) return undefined;
	if (typeof node.props.onClick === "function") return node.props.onClick;
	return firstClickHandler(node.props.children);
}

function activate(
	props: React.ComponentProps<typeof PostPublishControl>,
): void {
	const control = PostPublishControl(props);
	const onClick = firstClickHandler(control);
	assert.ok(onClick, "發布控制項必須保留可觸發提示的 click handler");
	onClick({
		preventDefault() {},
		stopPropagation() {},
	});
}

/**
 * 共用 `Button` 會多畫一個 `aria-hidden`、`opacity-0` 的寬度預留 span，裡面是
 * pending 文案（「發布中…」）。所以 `html.includes("發布")` 這種寫法會把**看不見的
 * 那一份**也算進來 —— 斷言必須指名可見的那個 label span。
 */
function hasVisibleLabel(html: string, label: string): boolean {
	return html.includes(`>${label}</span>`);
}

/**
 * `disabled:opacity-50` 這類 Tailwind class 恆在 class 字串裡，`/disabled/` 會恆真。
 * 停用與否只看 `<button>` 上真正的 `disabled` 屬性。
 */
function actionButton(
	html: string,
): { ariaLabel: string; title: string; disabled: boolean } | null {
	const tag = html.match(/<button\b[^>]*>/)?.[0];
	if (!tag) return null;
	return {
		ariaLabel: /aria-label="([^"]*)"/.exec(tag)?.[1] ?? "",
		title: /title="([^"]*)"/.exec(tag)?.[1] ?? "",
		disabled: /\sdisabled=""/.test(tag),
	};
}

test("兩個動詞不同（否則下面每一條互斥斷言都恆真，等於沒驗）", () => {
	assert.ok(PUBLISH_LABEL, "發布動詞取不到");
	assert.ok(UNPUBLISH_LABEL, "下架動詞取不到");
	assert.notEqual(PUBLISH_LABEL, UNPUBLISH_LABEL);
});

// ① 未發布（4401）——這正是 bug 的現場：pill 說「未發布」，而畫面上沒有出口。
test("① 未發布的貼文有「發布」入口", () => {
	const html = render({ publishStatus: PostPublishStatus.DRAFT });
	assert.ok(
		hasVisibleLabel(html, PUBLISH_LABEL),
		`4401 應給「${PUBLISH_LABEL}」按鈕，實得：${html}`,
	);
	assert.ok(!hasVisibleLabel(html, UNPUBLISH_LABEL), "4401 不該出現下架");
	assert.equal(actionButton(html)?.ariaLabel, PUBLISH_LABEL);
	assert.equal(
		actionButton(html)?.disabled,
		false,
		"未停權、未被平台下架時發布鍵應可按",
	);
});

// ② 已發布（4402）
test("② 已發布的貼文有「下架」入口", () => {
	const html = render({ publishStatus: PostPublishStatus.PUBLISHED });
	assert.ok(
		hasVisibleLabel(html, UNPUBLISH_LABEL),
		`4402 應給「${UNPUBLISH_LABEL}」按鈕，實得：${html}`,
	);
	assert.ok(!hasVisibleLabel(html, PUBLISH_LABEL), "4402 不該出現發布");
	assert.equal(actionButton(html)?.ariaLabel, UNPUBLISH_LABEL);
	assert.equal(
		actionButton(html)?.disabled,
		false,
		"下架是收縮型，任何情況都不該被停用",
	);
});

// 已下架（4403）可以再發布 —— 值域不對稱的另一半：回得到 4402，回不到 4401。
test("已下架的貼文可以再次發布", () => {
	const html = render({ publishStatus: PostPublishStatus.UNPUBLISHED });
	assert.ok(
		hasVisibleLabel(html, PUBLISH_LABEL),
		`4403 應給「${PUBLISH_LABEL}」`,
	);
	assert.equal(actionButton(html)?.disabled, false);
});

test("無圖的未發布／已下架貼文：發布呈現照片閘門，觸發只提示而不送出", () => {
	for (const publishStatus of [
		PostPublishStatus.DRAFT,
		PostPublishStatus.UNPUBLISHED,
	]) {
		let toggled: string | null = null;
		const props: React.ComponentProps<typeof PostPublishControl> = {
			publishStatus,
			hasImage: false,
			onToggle: (target) => {
				toggled = target;
			},
		};
		const html = render(props);

		assert.ok(hasVisibleLabel(html, PUBLISH_LABEL));
		assert.match(
			html,
			/data-image-required="true"/,
			`${publishStatus} 無圖時必須帶可辨識的照片閘門狀態`,
		);
		assert.match(html, /請先補上照片/, "畫面必須說明如何解除閘門");
		assert.match(
			actionButton(html)?.ariaLabel ?? "",
			/請先補上照片/,
			"不能只靠滑鼠 tooltip；輔助科技也要讀得到缺圖原因",
		);
		assert.equal(
			actionButton(html)?.disabled,
			false,
			"照片閘門不可用 native disabled，否則使用者無法觸發補圖提示",
		);

		activate(props);
		assert.equal(toggled, null, "無圖提示不得漏呼叫發布 onToggle");
	}
});

test("有圖時發布照常送出，且不殘留照片閘門", () => {
	let toggled: string | null = null;
	const props: React.ComponentProps<typeof PostPublishControl> = {
		publishStatus: PostPublishStatus.DRAFT,
		hasImage: true,
		onToggle: (target) => {
			toggled = target;
		},
	};
	const html = render(props);

	assert.doesNotMatch(html, /data-image-required="true"/);
	assert.doesNotMatch(actionButton(html)?.ariaLabel ?? "", /請先補上照片/);
	assert.equal(actionButton(html)?.disabled, false);
	activate(props);
	assert.equal(toggled, PostPublishStatus.PUBLISHED);
});

test("無圖只擋擴張方向：已發布貼文仍可下架", () => {
	let toggled: string | null = null;
	const props: React.ComponentProps<typeof PostPublishControl> = {
		publishStatus: PostPublishStatus.PUBLISHED,
		hasImage: false,
		onToggle: (target) => {
			toggled = target;
		},
	};
	const html = render(props);

	assert.ok(hasVisibleLabel(html, UNPUBLISH_LABEL));
	assert.doesNotMatch(html, /data-image-required="true"/);
	assert.doesNotMatch(actionButton(html)?.ariaLabel ?? "", /請先補上照片/);
	assert.equal(actionButton(html)?.disabled, false);
	activate(props);
	assert.equal(toggled, PostPublishStatus.UNPUBLISHED);
});

// ③ 停權：發布＝擴張型（CAP_MARKETING，停權不可）／下架＝收縮型（CAP_CONTENT_CONTRACT，
//    停權仍可自清）。兩個方向必須分開驗，只驗一邊會讓「整顆都不渲染」也通過。
test("③ 停權賣家看不到發布入口", () => {
	for (const status of [
		PostPublishStatus.DRAFT,
		PostPublishStatus.UNPUBLISHED,
	]) {
		const html = render({ publishStatus: status, suspended: true });
		// 整顆不渲染：沒有核發的能力就不出現 CTA（不是停用一顆按不下去的按鈕）。
		assert.equal(html, "", `停權賣家在 ${status} 仍看得到發布入口：${html}`);
	}
});

test("③ 停權賣家仍看得到下架入口（收縮型自清不可被拿走）", () => {
	const html = render({
		publishStatus: PostPublishStatus.PUBLISHED,
		suspended: true,
	});
	assert.ok(
		hasVisibleLabel(html, UNPUBLISH_LABEL),
		`停權賣家應仍能下架自己的貼文，實得：${html}`,
	);
	assert.equal(actionButton(html)?.disabled, false, "停權時下架鍵不得被停用");
});

// ④ 值域不對稱：賣家回不到「未發布」。窮舉每一個輸入狀態 × 停權/未停權 ×
//    平台下架/未下架，斷言 4401 這個目標從來沒有出口。
test("④ 任何狀態組合都不提供「改回未發布」的選項", () => {
	const draftLabel = "未發布";
	for (const publishStatus of [
		PostPublishStatus.DRAFT,
		PostPublishStatus.PUBLISHED,
		PostPublishStatus.UNPUBLISHED,
		undefined,
		"",
		"9999",
	]) {
		for (const suspended of [false, true]) {
			for (const hidden of [false, true]) {
				const html = render({ publishStatus, suspended, hidden });
				assert.ok(
					!html.includes(draftLabel),
					`(${publishStatus}/${suspended}/${hidden}) 畫出了「${draftLabel}」入口：${html}`,
				);
				// 述詞層同樣不得吐出 4401。
				assert.notEqual(
					postPublishToggleTarget(publishStatus),
					PostPublishStatus.DRAFT,
				);
			}
		}
	}
});

test("缺值不畫任何東西（上游沒給 ≠ 已發布）", () => {
	assert.equal(render({ publishStatus: undefined }), "");
	assert.equal(render({ publishStatus: "" }), "");
});

// 平台強制下架與發布狀態是兩件事：發布得出去，但買家仍看不到（social 的
// UpdatePublishStatus 對 hidden 的貼文 delta = 0）。所以停用並說明，而不是藏起來 ——
// 藏起來就等於再造一次「摸不到的入口」。
test("平台下架的貼文：發布鍵停用並說明，下架鍵照常", () => {
	const blocked = render({
		publishStatus: PostPublishStatus.DRAFT,
		hidden: true,
	});
	assert.ok(hasVisibleLabel(blocked, PUBLISH_LABEL), "應仍畫出發布鍵");
	assert.equal(actionButton(blocked)?.disabled, true, "平台下架時發布鍵應停用");
	assert.ok(
		actionButton(blocked)?.title.includes("買家仍看不到"),
		`停用要說得出理由，實得：${actionButton(blocked)?.title}`,
	);

	const shrink = render({
		publishStatus: PostPublishStatus.PUBLISHED,
		hidden: true,
	});
	assert.ok(hasVisibleLabel(shrink, UNPUBLISH_LABEL));
	assert.equal(
		actionButton(shrink)?.disabled,
		false,
		"只擋擴張方向，下架一律允許",
	);
});

/**
 * 元件對了不代表使用者摸得到 —— 本次修的就是這個 class。列表沒有 DOM 測試環境可以
 * 整頁掛起來，所以接線由原始碼契約守：入口必須真的在賣家的列表裡渲染，而且必須把
 * 停權旗標傳進去（漏傳的話停權賣家會看到一顆發布鍵，撞 9240 才知道）。
 */
test("賣家貼文列表的桌機／手機入口都傳入停權、平台下架與權威圖片狀態", () => {
	const source = readFileSync(
		new URL(
			"../features/seller-center/posts/SellerPostsClient.tsx",
			import.meta.url,
		),
		"utf8",
	);
	assert.ok(
		source.includes("<PostPublishControl"),
		"SellerPostsClient 沒有渲染 PostPublishControl ⇒ 端點依舊零 caller",
	);
	assert.ok(
		/suspended=\{isSuspended\}/.test(source),
		"沒把停權旗標傳進去 ⇒ 停權賣家會看到一顆按下去必定 9240 的發布鍵",
	);
	assert.ok(
		/hidden=\{isPostHidden\(p\)\}/.test(source),
		"沒把平台下架旗標傳進去 ⇒ 會宣稱一則被平台下架的貼文發布後買家看得到",
	);
	assert.equal(
		source.match(/hasImage=\{Boolean\((?:p|post)\.imageKeys\[0\]\)\}/g)?.length,
		2,
		"桌機 p 與手機 post 都必須用各自的 imageKeys[0] 判斷；漏一邊就能從該版型無圖發布",
	);
});

/** 資料層真的打得到那兩支端點（元件接了 handler、handler 卻沒有出口也是同一個病）。 */
test("資料層有 publish / unpublish 兩條路徑", () => {
	const source = readFileSync(
		new URL("../lib/seller/repo.ts", import.meta.url),
		"utf8",
	);
	assert.ok(
		/\$\{root\}\/\$\{id\}\/\$\{command\}/.test(source),
		"repo 沒有組出 /publish|/unpublish 的路徑",
	);
	assert.ok(
		/"publish"\s*:\s*"unpublish"/.test(source),
		"repo 沒有依目標狀態選命令端點",
	);
});
