import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { type ComponentProps, type ReactNode, useState } from "react";
import { expect, fn, userEvent, waitFor } from "storybook/test";
import Button from "@/components/ui/Button";
import ClarifyQuestionCard from "@/features/procurement-assistant/ClarifyQuestionCard";
import type { ClarifyQuestion } from "@/features/procurement-assistant/types";
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

const meta = {
	title: "Web/Procurement/ClarifyQuestion",
	component: ClarifyQuestionCard,
	args: {
		question: customQuestion,
		pending: false,
		onPick: fn<CardProps["onPick"]>(),
		onSubmitFreeText: fn<NonNullable<CardProps["onSubmitFreeText"]>>(),
		onSubmitCropOther: fn<NonNullable<CardProps["onSubmitCropOther"]>>(),
	},
	parameters: {
		docs: {
			description: {
				component:
					"正式採購問答卡的大類、小類與自行描述品項情境。allowFreeText 啟用時，文字用來重新查找品項；找不到符合品項時，買家只填品項描述，走 onSubmitCropOther 回呼。選項由後端提供，範例全部為固定資料，不建立採購需求、不連接交易，也不代表真實行情。",
			},
		},
	},
	decorators: [
		(Story) => (
			<div style={{ width: "100%", maxWidth: 860 }}>
				<Story />
			</div>
		),
	],
	render: (args) => (
		<QuestionList>
			<ClarifyQuestionCard {...args} />
		</QuestionList>
	),
} satisfies Meta<typeof ClarifyQuestionCard>;

export default meta;
type Story = StoryObj<typeof ClarifyQuestionCard>;

export const CustomOnly: Story = {
	name: "自行描述品項 · 空白",
	play: async ({ canvas, args }) => {
		await expect(
			canvas.getByRole("heading", { name: "自訂採購品項" }),
		).toBeVisible();
		await expect(canvas.getByText(customQuestion.prompt)).toBeVisible();
		await expect(
			canvas.queryByRole("button", { name: "自行描述品項" }),
		).not.toBeInTheDocument();
		await expect(
			canvas.getByRole("textbox", { name: "品項描述" }),
		).toBeEnabled();
		await expect(
			canvas.queryByRole("textbox", { name: "預期單價（新台幣／公斤）" }),
		).not.toBeInTheDocument();
		await expect(
			canvas.getByRole("button", { name: "確認品項" }),
		).toBeDisabled();
		await expect(args.onPick).not.toHaveBeenCalled();
	},
};

export const KeyboardSubmit: Story = {
	name: "自行描述品項 · 填妥與鍵盤送出",
	play: async ({ canvas, args }) => {
		const description = canvas.getByRole("textbox", { name: "品項描述" });
		const submit = canvas.getByRole("button", { name: "確認品項" });
		await expect(submit).toBeDisabled();
		await userEvent.type(description, "  有機甘藍  ");
		await expect(submit).toBeDisabled();
		await expect(submit).toBeEnabled();
		await userEvent.tab();
		await expect(submit).toHaveFocus();
		await userEvent.keyboard("{Enter}");
		await expect(args.onSubmitCropOther).toHaveBeenCalledOnce();
		await expect(args.onSubmitCropOther).toHaveBeenCalledWith({
			description: "有機甘藍",
		});
		await expect(args.onPick).not.toHaveBeenCalled();
	},
};

export const ChoicesAndOther: Story = {
	name: "候選品項 · 其他展開與直接選取",
	args: { question: choiceQuestion },
	play: async ({ canvas, args }) => {
		const other = canvas.getByRole("button", { name: "其他（自己輸入）" });
		await expect(
			canvas.queryByRole("textbox", { name: "品項描述" }),
		).not.toBeInTheDocument();
		await userEvent.click(other);
		await expect(other).toHaveAttribute("aria-expanded", "true");
		await waitFor(() =>
			expect(canvas.getByRole("textbox", { name: "品項描述" })).toHaveFocus(),
		);
		await expect(args.onPick).not.toHaveBeenCalled();
		await expect(args.onSubmitCropOther).not.toHaveBeenCalled();
		await userEvent.click(canvas.getByRole("button", { name: "甘藍－初秋" }));
		await expect(args.onPick).toHaveBeenCalledOnce();
		await expect(args.onPick).toHaveBeenCalledWith({
			slotKey: "crop_disambiguation",
			optionId: "crop-LA1",
			optionLabel: "甘藍－初秋",
		});
	},
};

export const BroadCategoryRetry: Story = {
	name: "大類選擇 · 自由描述重新查找",
	args: { question: broadQuestion },
	play: async ({ canvas, args }) => {
		const retry = canvas.getByRole("textbox", { name: "重新描述品項" });
		await expect(retry).toBeVisible();
		await expect(
			canvas.queryByRole("textbox", { name: "品項描述" }),
		).not.toBeInTheDocument();
		await userEvent.type(retry, "  臺灣豬五花  ");
		await userEvent.keyboard("{Enter}");
		await expect(args.onSubmitFreeText).toHaveBeenCalledWith("臺灣豬五花");
		await expect(args.onPick).not.toHaveBeenCalled();
		await expect(args.onSubmitCropOther).not.toHaveBeenCalled();
	},
};

export const NarrowItemRetry: Story = {
	name: "小類選擇 · 重新查找與自行描述分開",
	args: { question: narrowQuestion },
	play: async ({ canvas, args }) => {
		await expect(
			canvas.getByRole("textbox", { name: "重新描述品項" }),
		).toBeVisible();
		await userEvent.click(canvas.getByRole("button", { name: "豬五花" }));
		await expect(args.onPick).toHaveBeenCalledWith({
			slotKey: "crop_disambiguation",
			optionId: "fixture-item-pork-belly",
			optionLabel: "豬五花",
		});
		await userEvent.click(canvas.getByRole("button", { name: "自行描述品項" }));
		await userEvent.type(
			canvas.getByRole("textbox", { name: "品項描述" }),
			"帶皮五花肉",
		);
		await userEvent.click(canvas.getByRole("button", { name: "確認品項" }));
		await expect(args.onSubmitCropOther).toHaveBeenCalledWith({
			description: "帶皮五花肉",
		});
		await expect(args.onSubmitFreeText).not.toHaveBeenCalled();
	},
};

export const SameSlotStageChange: Story = {
	name: "同一問題鍵 · 大類到小類到自行描述",
	render: function StagedQuestions(args) {
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
	},
	play: async ({ canvas, args }) => {
		await userEvent.type(
			canvas.getByRole("textbox", { name: "重新描述品項" }),
			"第一題草稿",
		);
		await userEvent.click(canvas.getByRole("button", { name: "豬肉" }));
		await expect(
			canvas.getByRole("heading", { name: narrowQuestion.prompt }),
		).toBeVisible();
		await expect(
			canvas.getByRole("textbox", { name: "重新描述品項" }),
		).toHaveValue("");
		await userEvent.click(canvas.getByRole("button", { name: "自行描述品項" }));
		await userEvent.type(
			canvas.getByRole("textbox", { name: "品項描述" }),
			"第二題草稿",
		);
		await userEvent.click(canvas.getByRole("button", { name: "豬五花" }));
		await expect(
			canvas.getByRole("heading", { name: "自訂採購品項" }),
		).toBeVisible();
		await expect(
			canvas.queryByRole("textbox", { name: "重新描述品項" }),
		).not.toBeInTheDocument();
		await expect(canvas.getByRole("textbox", { name: "品項描述" })).toHaveValue(
			"",
		);
		await expect(args.onPick).toHaveBeenCalledTimes(2);
		await expect(args.onSubmitCropOther).not.toHaveBeenCalled();
	},
};

export const RetryWithoutCandidates: Story = {
	name: "仍可重新描述 · 不提前改為自行描述",
	args: { question: { ...broadQuestion, options: [broadQuestion.options[2]] } },
	play: async ({ canvas, args }) => {
		await expect(
			canvas.getByRole("heading", { name: broadQuestion.prompt }),
		).toBeVisible();
		await expect(
			canvas.queryByRole("textbox", { name: "品項描述" }),
		).not.toBeInTheDocument();
		await userEvent.type(
			canvas.getByRole("textbox", { name: "重新描述品項" }),
			"五花肉",
		);
		await userEvent.click(canvas.getByRole("button", { name: "重新搜尋品項" }));
		await expect(args.onSubmitFreeText).toHaveBeenCalledWith("五花肉");
		await expect(args.onSubmitCropOther).not.toHaveBeenCalled();
	},
};

export const BroadCategoryDarkNarrow: Story = {
	...BroadCategoryRetry,
	name: "大類選擇 · 深色窄版大字",
	globals: { theme: "dark", textScale: "1.4" },
	decorators: [
		(Story) => (
			<div style={{ width: 358, maxWidth: "100%" }}>
				<Story />
			</div>
		),
	],
};

export const Pending: Story = {
	name: "自行描述品項 · 等待中",
	args: { question: choiceQuestion },
	render: function PendingPreview(args) {
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
	},
	play: async ({ canvas, args }) => {
		await userEvent.click(
			canvas.getByRole("button", { name: "其他（自己輸入）" }),
		);
		await userEvent.type(
			canvas.getByRole("textbox", { name: "品項描述" }),
			"有機甘藍",
		);
		await userEvent.click(canvas.getByRole("button", { name: "顯示等待狀態" }));
		for (const input of canvas.getAllByRole("textbox")) {
			await expect(input).toBeDisabled();
		}
		for (const button of canvas.getAllByRole("button")) {
			await expect(button).toBeDisabled();
		}
		await expect(canvas.getByRole("textbox", { name: "品項描述" })).toHaveValue(
			"有機甘藍",
		);
		await expect(args.onSubmitCropOther).not.toHaveBeenCalled();
	},
};

export const MissingSubmitHandler: Story = {
	name: "自行描述品項 · 提交尚未就緒",
	args: { onSubmitCropOther: undefined },
	play: async ({ canvas }) => {
		for (const input of canvas.getAllByRole("textbox")) {
			await expect(input).toBeDisabled();
		}
		await expect(
			canvas.getByRole("button", { name: "確認品項" }),
		).toBeDisabled();
	},
};

export const Dark: Story = {
	name: "自行描述品項 · 深色",
	globals: { theme: "dark" },
};

export const NarrowLargeText: Story = {
	name: "自行描述品項 · 窄版大字",
	globals: { theme: "light", textScale: "1.4" },
	decorators: [
		(Story) => (
			<div style={{ width: 358, maxWidth: "100%" }}>
				<Story />
			</div>
		),
	],
};
