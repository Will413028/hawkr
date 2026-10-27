import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

/**
 * 代行建立貼文的兩條路徑必須都是**畫面上看得見的按鈕**。
 *
 * 「一律建為未發布」這件事本身沒變（`POST .../posts` 不收 `publishMode`），變的是
 * 它不能是隱藏預設：操作者按下去之前就要知道自己選的是哪一種，事後也要能從按鈕上
 * 的字回想起來。
 *
 * ⚠️ 這一支掃的是**原始碼**而不是渲染結果：這支元件綁 `useSellerDataScope`、
 * `useQuery` 與代行工作階段 context，掛起來需要一整組 provider；而本檔要守的三件事
 * （按鈕在不在、誰是 primary、缺 capability 時有沒有說明）都是原始碼層面的形狀。
 */

const ROOT = fileURLToPath(new URL("../../..", import.meta.url));
const COMPOSE = "features/seller-center/posts/PostComposeClient.tsx";

/** 逐行剝掉自成一行的註解（理由同 `create-copy.unit.test.ts` 的同名函式）。 */
function stripComments(source: string): string {
	const kept: string[] = [];
	let inBlock = false;
	for (const line of source.split("\n")) {
		const trimmed = line.trim();
		if (inBlock) {
			if (trimmed.includes("*/")) inBlock = false;
			continue;
		}
		if (trimmed.startsWith("//") || trimmed.startsWith("*")) continue;
		if (trimmed.startsWith("/*") || trimmed.startsWith("{/*")) {
			if (!trimmed.includes("*/")) inBlock = true;
			continue;
		}
		kept.push(line);
	}
	return kept.join("\n");
}

const raw = readFileSync(`${ROOT}/${COMPOSE}`, "utf8");
const rendered = stripComments(raw);

function buttonBefore(label: string): string {
	const labelAt = rendered.lastIndexOf(label);
	expect(labelAt).toBeGreaterThan(-1);
	const buttonAt = rendered.lastIndexOf("<Button", labelAt);
	expect(buttonAt).toBeGreaterThan(-1);
	return rendered.slice(buttonAt, labelAt);
}

describe("代行建立貼文：兩條路徑都要顯性", () => {
	it("剝註解後仍留著 JSX 文字（否則底下每一條都會空過）", () => {
		expect(rendered).toContain("adm-post-compose__bar-actions");
		expect(rendered).toContain("代行原因");
	});

	it("兩顆按鈕的文字都在 JSX 裡", () => {
		expect(rendered).toContain("儲存，暫不發布");
		expect(rendered).toContain("儲存並發布");
	});

	it("有 posts.publish 時「儲存並發布」是主要按鈕", () => {
		const publish = rendered.lastIndexOf("儲存並發布");
		expect(publish).toBeGreaterThan(-1);
		const button = rendered.slice(
			rendered.lastIndexOf("<Button", publish),
			publish,
		);
		// 主要按鈕不帶 variant（預設 solid）。
		expect(button).not.toContain("variant=");
	});

	it("缺 posts.publish 時「暫不發布」升為主要，不留一顆孤零零的次要按鈕", () => {
		// 發布鍵在缺 capability 時不渲染 ⇒ 若草稿鍵恆為 bordered，畫面上只剩一顆
		// 次要樣式的按鈕，看起來像被停用的殘骸。條件式 variant 就是這條退化的實作。
		expect(rendered).toMatch(
			/variant=\{canPublishPosts && !isEdit \? "bordered" : "solid"\}/,
		);
	});

	it("缺 posts.publish 時補一句說明，而不是靜默少一顆按鈕", () => {
		expect(rendered).toMatch(/\{canPublishPosts \|\| isEdit \? null : \(/);
		expect(rendered).toContain("本次工作階段沒有「貼文發布」授權");
	});

	it("不得改回 disabled（推翻 L3857 的裁決）", () => {
		expect(raw).not.toContain("disabled={!canPublishPosts}");
		expect(raw).not.toContain("disabled={delegated && !canPublishPosts}");
	});

	it("兩顆按鈕各自帶 loading，不共用一個布林", () => {
		// 共用布林時，「儲存並發布」的等待轉圈會畫在「暫不發布」那顆上。
		expect(rendered).toContain('isLoading={submittingIntent === "draft"}');
		expect(rendered).toContain('isLoading={submittingIntent === "publish"}');
	});

	it("只有已確認的 imagePreview 能解除發布照片閘門", () => {
		// pendingImage 只是尚未確認裁切的來源檔；把它算成有圖會讓使用者取消裁切後
		// 仍能發布。create/edit 共用的權威狀態是畫面真正正在預覽的 imagePreview。
		expect(rendered).toMatch(
			/const\s+\w*[Hh]as(?:Publish)?Image\w*\s*=\s*Boolean\(imagePreview\)/,
		);
		expect(rendered).not.toMatch(
			/const\s+\w*[Hh]as(?:Publish)?Image\w*\s*=\s*Boolean\(pendingImage\)/,
		);
	});

	it("送出層同時守賣家自助發布與顯式 publish intent，且在任何 mutation 前擋下無圖", () => {
		const submitStart = rendered.indexOf(
			'const submitPost = async (intent: "draft" | "publish") =>',
		);
		const firstMutation = rendered.indexOf(
			"scope.confirmMutation",
			submitStart,
		);
		expect(submitStart).toBeGreaterThan(-1);
		expect(firstMutation).toBeGreaterThan(submitStart);
		const guard = rendered.slice(submitStart, firstMutation);

		expect(guard).toContain("imagePreview");
		expect(guard).toMatch(
			/const publishing = !delegated \|\| intent === "publish";/,
		);
		expect(guard).toMatch(
			/if \(publishing && !imagePreview\) \{\s*promptForRequiredImage\(\);\s*return;/,
		);
	});

	it("賣家自助與代行的發布按鈕都有缺圖狀態，且停用條件使用發布專屬 gate", () => {
		const selfSaveAt = rendered.indexOf('form="post-compose-form"');
		expect(selfSaveAt).toBeGreaterThan(-1);
		const selfButtonStart = rendered.lastIndexOf("<Button", selfSaveAt);
		const selfButtonEnd = rendered.indexOf(">", selfSaveAt);
		const selfButton = rendered.slice(selfButtonStart, selfButtonEnd);
		const delegatedPublishButton = buttonBefore("儲存並發布");

		for (const [surface, button] of [
			["賣家自助", selfButton],
			["代行發布", delegatedPublishButton],
		] as const) {
			expect(button, surface).toContain("data-image-required=");
			expect(button, surface).toContain("aria-disabled=");
		}
		expect(rendered).toContain(
			"const sellerSubmitDisabled = submitting || (hasImage && !hasContent);",
		);
		expect(selfButton).toContain("disabled={sellerSubmitDisabled}");
		expect(delegatedPublishButton).toContain(
			"disabled={submitting || (hasImage && !hasContent)}",
		);
	});

	it("儲存未發布草稿仍只受內容與 submitting 控制，不被照片 gate 汙染", () => {
		expect(rendered).toContain("const hasContent = content.trim().length > 0;");
		expect(rendered).toContain(
			"const canSaveDraft = hasContent && !submitting;",
		);
		const draftButton = buttonBefore("儲存，暫不發布");
		expect(draftButton).toContain("disabled={!canSaveDraft}");
		expect(draftButton).not.toContain("data-image-required");
		expect(draftButton).not.toContain("imagePreview");
	});
});
