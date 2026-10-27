import { expect, test } from "@playwright/test";

const PORTAL = "/hawkr-agent-rfq-preview/external-vendor";

test.describe("external vendor RFQ portal", { tag: ["@ui"] }, () => {
	test("moves from a one-time invitation to a reusable vendor portal", async ({
		page,
	}) => {
		await page.goto(PORTAL);

		await expect(
			page.getByRole("heading", {
				name: "豐田產地直送，您收到一份採購詢價",
			}),
		).toBeVisible();
		await expect(page.getByText("前端流程預覽")).toBeVisible();
		await expect(page.getByText("財務視角")).toHaveCount(0);
		await expect(page.getByText("Hawkr 動態", { exact: true })).toHaveCount(0);
		await expect(page.getByText("快速連結", { exact: true })).toHaveCount(0);
		const progress = page.getByRole("navigation", {
			name: "外部供應商交易流程",
		});
		await expect(progress).toHaveCount(0);

		await page.getByRole("link", { name: "確認邀請並繼續" }).click();
		await expect(page).toHaveURL(`${PORTAL}/exchange`);
		await expect(
			page.getByRole("heading", { name: "正在確認這份邀請" }),
		).toBeVisible();

		await page.getByRole("link", { name: "預覽兌換成功並繼續" }).click();
		await expect(page).toHaveURL(`${PORTAL}/sign-in`);
		await expect(
			page.getByRole("button", {
				name: "使用受邀 Google 帳號繼續（API 待接）",
			}),
		).toBeDisabled();

		await page.getByRole("link", { name: "預覽寄出 6 位數驗證碼" }).click();
		await expect(page).toHaveURL(`${PORTAL}/verify-code`);
		const codeInput = page.getByLabel("驗證碼");
		await codeInput.fill("111111");
		await page.getByRole("button", { name: "確認驗證碼" }).click();
		await expect(page.locator("#vendor-code-error")).toContainText(
			"驗證碼不正確",
		);
		await page.getByRole("button", { name: "重新取得前端示意碼" }).click();
		await expect(codeInput).toHaveValue("");
		await codeInput.fill("135790");
		await page.getByRole("button", { name: "確認驗證碼" }).click();
		await expect(
			page.getByRole("heading", { name: "信箱驗證完成" }),
		).toBeVisible();

		await page.getByRole("link", { name: "進入供應商工作台" }).click();
		await expect(page).toHaveURL(`${PORTAL}/portal`);
		await expect(
			page.getByRole("heading", { name: "早安，豐田產地直送" }),
		).toBeVisible();
		await expect(page.getByText("待確認詢價單")).toBeVisible();
		await expect(page.getByText("進行中交易")).toBeVisible();

		await page.getByRole("link", { name: "確認條件" }).click();
		await expect(page).toHaveURL(`${PORTAL}/quote`);
		await expect(progress.getByRole("link")).toHaveCount(5);
		for (const label of ["驗證", "詢價", "條件", "回覆", "追蹤"]) {
			await expect(progress.getByText(label, { exact: true })).toBeVisible();
		}
		await expect(
			page.getByRole("heading", { name: "確認報價條件" }),
		).toBeVisible();
		await expect(page.getByText("買家可接受單價範圍")).toBeVisible();
		await expect(page.getByText("NT$250–NT$280／箱")).toBeVisible();
		await expect(page.getByText("NT$530／箱")).toBeVisible();
		await expect(page.getByLabel("每箱單價（含稅）")).toHaveCount(0);

		await page.getByRole("link", { name: "接受這份條件" }).click();
		await expect(page).toHaveURL(`${PORTAL}/review`);
		const previewLink = page.getByRole("link", { name: "預覽接受後狀態" });
		await expect(previewLink).toHaveAttribute("aria-disabled", "true");
		await page.getByRole("checkbox").check();
		await expect(previewLink).not.toHaveAttribute("aria-disabled", "true");
		await previewLink.click();

		await expect(page).toHaveURL(`${PORTAL}/transaction`);
		await page.getByRole("tab", { name: "履約中" }).click();
		await expect(page.getByText("已付款｜待出貨")).toBeVisible();
		await expect(
			page.getByRole("button", { name: "開始備貨（API 待接）" }),
		).toBeDisabled();
	});

	test("lets the buyer tune Pricebook bounds and ideal quality before sending", async ({
		page,
	}) => {
		await page.goto("/hawkr-agent-rfq-preview/rfq-compose");

		const minimum = page.getByRole("slider", { name: "最低可接受單價" });
		const ideal = page.getByRole("slider", { name: "理想成交單價" });
		const maximum = page.getByRole("slider", { name: "最高可接受單價" });
		await expect(minimum).toHaveValue("250");
		await expect(ideal).toHaveValue("265");
		await expect(maximum).toHaveValue("280");

		await ideal.fill("275");
		await expect(page.getByText("品質優先", { exact: true })).toBeVisible();
		await expect(page.getByText("NT$550／箱", { exact: true })).toBeVisible();

		await page.getByRole("button", { name: "確認並送出給賣家" }).click();
		await expect(
			page.getByRole("heading", { name: "確認價格與品質後送出？" }),
		).toBeVisible();
		await expect(page.getByText(/理想 NT\$275／箱/)).toBeVisible();
	});

	test("offers explicit recovery for expired sessions and unavailable invites", async ({
		page,
	}) => {
		await page.goto(`${PORTAL}/session-expired`);
		await expect(
			page.getByRole("heading", { name: "這次工作階段已結束" }),
		).toBeVisible();
		await expect(page.getByRole("link", { name: "重新登入" })).toHaveAttribute(
			"href",
			`${PORTAL}/sign-in`,
		);

		await page.goto(`${PORTAL}/invite-unavailable`);
		await expect(
			page.getByRole("heading", { name: "這個邀請連結已經使用過" }),
		).toBeVisible();
		await page.getByRole("button", { name: "已過期" }).click();
		await expect(
			page.getByRole("heading", { name: "這份邀請已超過可回覆期限" }),
		).toBeVisible();
		await expect(
			page.getByRole("button", { name: "重新邀請（API 待接）" }),
		).toBeDisabled();
		await page.getByRole("button", { name: "已撤回" }).click();
		await expect(
			page.getByRole("heading", { name: "邀請方已撤回這份邀請" }),
		).toBeVisible();
	});
});
