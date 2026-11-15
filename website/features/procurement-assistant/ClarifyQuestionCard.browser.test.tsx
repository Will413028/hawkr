import { type ComponentProps, type ReactNode, useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import Button from "@/components/ui/Button";
import { renderWithIntl } from "@/tests/browser/render";
import ClarifyQuestionCard from "./ClarifyQuestionCard";
import type { ClarifyQuestion } from "./types";
import "@/styles/features/messages.css";
import "@/styles/features/procurement-assistant.css";

type CardProps = ComponentProps<typeof ClarifyQuestionCard>;

const customQuestion: ClarifyQuestion = {
	slotKey: "crop_disambiguation",
	prompt: "找不到符合的品項，請描述你要的品項：",
	options: [{ optionId: "crop-other", label: "自行描述品項", value: "OTHER" }],
	allowFreeText: false,
	source: "FIXED",
	canSkip: false,
};

const choiceQuestion: ClarifyQuestion = {
	...customQuestion,
	prompt: "找到幾個相符的品項，麻煩選一下你要的是哪一個：",
	options: [
		{ optionId: "crop-LA1", label: "甘藍－初秋", value: "甘藍－初秋" },
		{ optionId: "crop-LA2", label: "甘藍－改良種", value: "甘藍－改良種" },
		{ optionId: "crop-other", label: "其他（自己輸入）", value: "OTHER" },
	],
};

// 模擬後端逐輪回傳的白名單，不代表現行行情目錄或真實價格。
const broadQuestion: ClarifyQuestion = {
	...customQuestion,
	prompt: "先確認你要採購的大品項，請選擇一項，或輸入品項名稱：",
	allowFreeText: true,
	options: [
		{ optionId: "fixture-group-pork", label: "豬肉", value: "豬肉" },
		{ optionId: "fixture-group-beef", label: "牛肉", value: "牛肉" },
		{ optionId: "crop-other", label: "自行描述品項", value: "OTHER" },
	],
};

const narrowQuestion: ClarifyQuestion = {
	...broadQuestion,
	prompt: "接著確認「豬肉」的細項，請選擇一項，或輸入更具體的名稱：",
	options: [
		{ optionId: "fixture-item-pork-belly", label: "豬五花", value: "豬五花" },
		{ optionId: "fixture-item-pork-loin", label: "豬里肌", value: "豬里肌" },
		broadQuestion.options[2],
	],
};

function QuestionList({ children }: { children: ReactNode }) {
	return (
		<div className="hawkr-agent-phase1-thread">
			<ul className="thread-page__list" aria-label="採購需求問答">
				{children}
			</ul>
		</div>
	);
}

function makeArgs(overrides: Partial<CardProps> = {}): CardProps {
	return {
		question: customQuestion,
		pending: false,
		onPick: vi.fn<CardProps["onPick"]>(),
		onSubmitFreeText: vi.fn<NonNullable<CardProps["onSubmitFreeText"]>>(),
		onSubmitCropOther: vi.fn<NonNullable<CardProps["onSubmitCropOther"]>>(),
		...overrides,
	};
}

type RenderOptions = Parameters<typeof renderWithIntl>[1];

function setup(overrides: Partial<CardProps> = {}, options?: RenderOptions) {
	const args = makeArgs(overrides);
	renderWithIntl(
		<QuestionList>
			<ClarifyQuestionCard {...args} />
		</QuestionList>,
		{ frameMaxWidth: 860, ...options },
	);
	return args;
}

const button = (name: string | RegExp) =>
	page.getByRole("button", { name, exact: typeof name === "string" });
const textbox = (name: string) =>
	page.getByRole("textbox", { name, exact: true });
const heading = (name: string) =>
	page.getByRole("heading", { name, exact: true });

async function expectBroadCategoryRetry(args: CardProps) {
	const retry = textbox("重新描述品項");
	await expect.element(retry).toBeVisible();
	await expect.element(textbox("品項描述")).not.toBeInTheDocument();
	await userEvent.type(retry, "  臺灣豬五花  ");
	await userEvent.keyboard("{Enter}");
	expect(args.onSubmitFreeText).toHaveBeenCalledWith("臺灣豬五花");
	expect(args.onPick).not.toHaveBeenCalled();
	expect(args.onSubmitCropOther).not.toHaveBeenCalled();
}

describe("ClarifyQuestionCard", () => {
	it("shows only the custom description form when blank", async () => {
		const args = setup();
		await expect.element(heading("自訂採購品項")).toBeVisible();
		await expect
			.element(page.getByText(customQuestion.prompt, { exact: true }))
			.toBeVisible();
		await expect.element(button("自行描述品項")).not.toBeInTheDocument();
		await expect.element(textbox("品項描述")).toBeEnabled();
		await expect
			.element(textbox("預期單價（新台幣／公斤）"))
			.not.toBeInTheDocument();
		await expect.element(button("確認品項")).toBeDisabled();
		expect(args.onPick).not.toHaveBeenCalled();
	});

	it("submits the trimmed description from the keyboard", async () => {
		const args = setup();
		const description = textbox("品項描述");
		const submit = button("確認品項");
		await expect.element(submit).toBeDisabled();
		await userEvent.type(description, "  切絲高麗菜  ");
		await expect.element(submit).toBeEnabled();
		await userEvent.tab();
		await expect.element(submit).toHaveFocus();
		await userEvent.keyboard("{Enter}");
		expect(args.onSubmitCropOther).toHaveBeenCalledOnce();
		expect(args.onSubmitCropOther).toHaveBeenCalledWith({
			description: "切絲高麗菜",
		});
		expect(args.onPick).not.toHaveBeenCalled();
	});

	it("expands 其他 without picking, and picks a candidate directly", async () => {
		const args = setup({ question: choiceQuestion });
		const other = button("其他（自己輸入）");
		await expect.element(textbox("品項描述")).not.toBeInTheDocument();
		await userEvent.click(other);
		await expect.element(other).toHaveAttribute("aria-expanded", "true");
		await expect.element(textbox("品項描述")).toHaveFocus();
		expect(args.onPick).not.toHaveBeenCalled();
		expect(args.onSubmitCropOther).not.toHaveBeenCalled();
		await userEvent.click(button("甘藍－初秋"));
		expect(args.onPick).toHaveBeenCalledOnce();
		expect(args.onPick).toHaveBeenCalledWith({
			slotKey: "crop_disambiguation",
			optionId: "crop-LA1",
			optionLabel: "甘藍－初秋",
		});
	});

	it("re-searches with free text at the broad category stage", async () => {
		const args = setup({ question: broadQuestion });
		await expectBroadCategoryRetry(args);
	});

	it("keeps re-search and custom description separate at the narrow stage", async () => {
		const args = setup({ question: narrowQuestion });
		await expect.element(textbox("重新描述品項")).toBeVisible();
		await userEvent.click(button("豬五花"));
		expect(args.onPick).toHaveBeenCalledWith({
			slotKey: "crop_disambiguation",
			optionId: "fixture-item-pork-belly",
			optionLabel: "豬五花",
		});
		await userEvent.click(button("自行描述品項"));
		await userEvent.type(textbox("品項描述"), "帶皮五花肉");
		await userEvent.click(button("確認品項"));
		expect(args.onSubmitCropOther).toHaveBeenCalledWith({
			description: "帶皮五花肉",
		});
		expect(args.onSubmitFreeText).not.toHaveBeenCalled();
	});

	it("resets drafts when the same slot key moves from broad to narrow to custom", async () => {
		const args = makeArgs();
		function StagedQuestions() {
			const [stage, setStage] = useState(0);
			return (
				<QuestionList>
					<ClarifyQuestionCard
						{...args}
						question={[broadQuestion, narrowQuestion, customQuestion][stage]}
						onPick={(option) => {
							args.onPick(option);
							setStage((current) => Math.min(current + 1, 2));
						}}
					/>
				</QuestionList>
			);
		}
		renderWithIntl(<StagedQuestions />, { frameMaxWidth: 860 });
		await userEvent.type(textbox("重新描述品項"), "第一題草稿");
		await userEvent.click(button("豬肉"));
		await expect.element(heading(narrowQuestion.prompt)).toBeVisible();
		await expect.element(textbox("重新描述品項")).toHaveValue("");
		await userEvent.click(button("自行描述品項"));
		await userEvent.type(textbox("品項描述"), "第二題草稿");
		await userEvent.click(button("豬五花"));
		await expect.element(heading("自訂採購品項")).toBeVisible();
		await expect.element(textbox("重新描述品項")).not.toBeInTheDocument();
		await expect.element(textbox("品項描述")).toHaveValue("");
		expect(args.onPick).toHaveBeenCalledTimes(2);
		expect(args.onSubmitCropOther).not.toHaveBeenCalled();
	});

	it("still allows re-describing when no candidates remain", async () => {
		const args = setup({
			question: { ...broadQuestion, options: [broadQuestion.options[2]] },
		});
		await expect.element(heading(broadQuestion.prompt)).toBeVisible();
		await expect.element(textbox("品項描述")).not.toBeInTheDocument();
		await userEvent.type(textbox("重新描述品項"), "五花肉");
		await userEvent.click(button("重新搜尋品項"));
		expect(args.onSubmitFreeText).toHaveBeenCalledWith("五花肉");
		expect(args.onSubmitCropOther).not.toHaveBeenCalled();
	});

	it("re-searches with free text in dark theme, narrow width and 1.4x text", async () => {
		const args = makeArgs({ question: broadQuestion });
		renderWithIntl(
			<QuestionList>
				<ClarifyQuestionCard {...args} />
			</QuestionList>,
			{ frameMaxWidth: 358, theme: "dark", textScale: 1.4 },
		);
		await expectBroadCategoryRetry(args);
	});

	it("disables every control while pending and keeps the draft", async () => {
		const args = makeArgs({ question: choiceQuestion });
		function PendingPreview() {
			const [pending, setPending] = useState(false);
			return (
				<>
					<QuestionList>
						<ClarifyQuestionCard {...args} pending={pending} />
					</QuestionList>
					{!pending ? (
						<Button variant="light" onClick={() => setPending(true)}>
							顯示等待狀態
						</Button>
					) : null}
				</>
			);
		}
		renderWithIntl(<PendingPreview />, { frameMaxWidth: 860 });
		await userEvent.click(button("其他（自己輸入）"));
		await userEvent.type(textbox("品項描述"), "有機甘藍");
		await userEvent.click(button("顯示等待狀態"));
		await expect.element(button("顯示等待狀態")).not.toBeInTheDocument();
		const inputs = page.getByRole("textbox");
		const buttons = page.getByRole("button");
		await expect.poll(() => inputs.elements().length).toBeGreaterThan(0);
		await expect.poll(() => buttons.elements().length).toBeGreaterThan(0);
		for (const input of inputs.elements())
			await expect.element(input).toBeDisabled();
		for (const control of buttons.elements())
			await expect.element(control).toBeDisabled();
		await expect.element(textbox("品項描述")).toHaveValue("有機甘藍");
		expect(args.onSubmitCropOther).not.toHaveBeenCalled();
	});

	it("disables the form when the submit handler is not ready", async () => {
		setup({ onSubmitCropOther: undefined });
		const inputs = page.getByRole("textbox");
		await expect.poll(() => inputs.elements().length).toBeGreaterThan(0);
		for (const input of inputs.elements())
			await expect.element(input).toBeDisabled();
		await expect.element(button("確認品項")).toBeDisabled();
	});
});
