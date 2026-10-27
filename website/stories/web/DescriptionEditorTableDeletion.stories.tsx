import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { expect, fn, waitFor, within } from "storybook/test";
import { DescriptionEditorCanvas } from "@/features/seller/components/DescriptionEditor";
import type { MediaUploadRepo } from "@/lib/media/upload";
import type { InlineRun, ProductDescription } from "@/lib/products/seller-repo";
import "@/styles/features/seller-center.css";

type NativeUserEvent = typeof import("vitest/browser").userEvent;
interface TableCell {
	runs: InlineRun[];
	textAlign?: "center" | "right";
}
interface TableModel {
	withHeadings: boolean;
	rows: TableCell[][];
}

const tableModel: TableModel = {
	withHeadings: true,
	rows: [
		[
			{ runs: [{ text: "第一欄", bold: true }] },
			{ runs: [{ text: "第二欄", bold: true }] },
			{ runs: [{ text: "第三欄", bold: true }] },
		],
		[
			{
				runs: [
					{ text: "前端保留", bold: true, color: "#682e0e" },
					{ text: "刪除起點" },
				],
			},
			{
				runs: [
					{ text: "保留", bold: true },
					{ text: "刪除" },
					{ text: "尾端", italic: true },
				],
				textAlign: "center",
			},
			{
				runs: [{ text: "刪除終點" }, { text: "尾端保留", underline: true }],
				textAlign: "right",
			},
		],
		[
			{ runs: [{ text: "其他列甲", color: "#b8441f" }] },
			{ runs: [{ text: "其他列乙", italic: true }] },
			{ runs: [{ text: "其他列丙", underline: true }] },
		],
	],
};

const tableDescription: ProductDescription = {
	version: "1.0",
	blocks: [
		{ id: "before-table", type: "text", runs: [{ text: "表格前方文字保留" }] },
		{
			id: "table-deletion-fixture",
			type: "table",
			tableJson: JSON.stringify(tableModel),
		},
		{ id: "after-table", type: "text", runs: [{ text: "表格後方文字保留" }] },
	],
};

const meta = {
	title: "Web/Seller Center/DescriptionEditor Table Deletion",
	component: DescriptionEditorCanvas,
	args: {
		value: tableDescription,
		onChange: fn<(value: ProductDescription) => void>(),
		uploadImageFile: fn<MediaUploadRepo["uploadImageFile"]>(async () => {
			throw new Error("表格刪除回歸不建立媒體上傳交易");
		}),
	},
	parameters: {
		docs: {
			description: {
				component:
					"表格文字刪除與復原的原生瀏覽器回歸。單格刪字與跨格 Range 刪除都保留表格列欄、其他儲存格、對齊及前後文件；同時比對可見表格和 onChange 的 tableJson。MCP test-run 使用 Vitest Browser 原生鍵盤；普通預覽保留固定 3×3 表格供手動操作。所有資料只存於本地 state，上傳為 stub。\n\n實際 import：`@/features/seller/components/DescriptionEditor`。",
			},
		},
	},
	render: function Controlled(args) {
		const [value, setValue] = useState(args.value);
		return (
			<div style={{ maxWidth: 1040, marginInline: "auto" }}>
				<DescriptionEditorCanvas
					{...args}
					value={value}
					onChange={(next) => {
						setValue(next);
						args.onChange(next);
					}}
				/>
			</div>
		);
	},
} satisfies Meta<typeof DescriptionEditorCanvas>;
export default meta;
type Story = StoryObj<typeof meta>;

async function nativeTableTest(
	canvasElement: HTMLElement,
	globals: Record<string, unknown>,
	run: (dialog: HTMLElement, user: NativeUserEvent) => Promise<void>,
) {
	const dialog = within(canvasElement).getByRole("dialog", {
		name: "商品描述編輯器",
	});
	await waitFor(() => expect(dialog).toHaveAttribute("data-ready", "true"), {
		timeout: 15_000,
	});
	const hasNativeBrowser = "__vitest_browser__" in globalThis;
	if (globals["storybook/test-provided"])
		await expect(hasNativeBrowser).toBe(true);
	if (!hasNativeBrowser) return;
	const { userEvent } = await import("vitest/browser");
	const view = dialog.ownerDocument.defaultView;
	if (!view) throw new Error("缺少表格編輯器的瀏覽器視窗");
	const errors: string[] = [];
	const onError = (event: ErrorEvent) => errors.push(event.message);
	const onRejection = (event: PromiseRejectionEvent) =>
		errors.push(String(event.reason));
	view.addEventListener("error", onError);
	view.addEventListener("unhandledrejection", onRejection);
	try {
		await userEvent.unhover(dialog);
		await run(dialog, userEvent);
		await expect(errors).toEqual([]);
	} finally {
		view.removeEventListener("error", onError);
		view.removeEventListener("unhandledrejection", onRejection);
	}
}

function domRows(dialog: HTMLElement): HTMLElement[][] {
	return Array.from(
		dialog.querySelectorAll<HTMLElement>(".tc-table > .tc-row"),
	).map((row) =>
		Array.from(row.querySelectorAll<HTMLElement>(":scope > .tc-cell")),
	);
}

function selectCellText(
	start: HTMLElement,
	startOffset: number,
	end: HTMLElement,
	endOffset: number,
) {
	start.focus();
	const textPoint = (element: HTMLElement, offset: number) => {
		const walker = element.ownerDocument.createTreeWalker(
			element,
			NodeFilter.SHOW_TEXT,
		);
		let remaining = offset;
		for (let node = walker.nextNode(); node; node = walker.nextNode()) {
			const length = node.textContent?.length ?? 0;
			if (remaining <= length) return { node, offset: remaining };
			remaining -= length;
		}
		throw new Error("選取範圍超過儲存格文字");
	};
	const from = textPoint(start, startOffset);
	const to = textPoint(end, endOffset);
	const range = start.ownerDocument.createRange();
	range.setStart(from.node, from.offset);
	range.setEnd(to.node, to.offset);
	const selection = start.ownerDocument.getSelection();
	selection?.removeAllRanges();
	selection?.addRange(range);
}

function savedTable(value: ProductDescription): TableModel {
	const block = value.blocks.find(
		(candidate) => candidate.id === "table-deletion-fixture",
	);
	if (!block?.tableJson) throw new Error("儲存文件失去原表格區塊");
	return JSON.parse(block.tableJson) as TableModel;
}

function assertTableState(
	dialog: HTMLElement,
	value: ProductDescription,
	expected: TableModel,
	phase: string,
	initialCellHtml: string[][],
) {
	expect(
		value.blocks.map((block) => ({ id: block.id, type: block.type })),
	).toEqual(
		tableDescription.blocks.map((block) => ({
			id: block.id,
			type: block.type,
		})),
	);
	expect(value.blocks[0]).toEqual(tableDescription.blocks[0]);
	expect(value.blocks[2]).toEqual(tableDescription.blocks[2]);
	const actual = savedTable(value);
	try {
		expect(actual).toEqual(expected);
	} catch {
		// Keep the failing assertion strict, but report only changed cells: MCP's
		// error excerpt otherwise truncates Vitest's full 3×3 object diff.
		const differences: Record<string, unknown>[] = [];
		if (actual.withHeadings !== expected.withHeadings) {
			differences.push({
				field: "withHeadings",
				actual: actual.withHeadings,
				expected: expected.withHeadings,
			});
		}
		const currentDom = domRows(dialog);
		for (
			let row = 0;
			row < Math.max(actual.rows.length, expected.rows.length);
			row++
		) {
			for (
				let column = 0;
				column <
				Math.max(
					actual.rows[row]?.length ?? 0,
					expected.rows[row]?.length ?? 0,
				);
				column++
			) {
				const cell = actual.rows[row]?.[column];
				const expectedCell = expected.rows[row]?.[column];
				try {
					expect(cell).toEqual(expectedCell);
				} catch {
					differences.push({
						cell: `${row},${column}`,
						actual: cell,
						expected: expectedCell,
						initialHtml: initialCellHtml[row]?.[column],
						currentHtml: currentDom[row]?.[column]?.innerHTML,
					});
				}
			}
		}
		throw new Error(
			`TABLE_STATE_DIFFERENCES ${phase}: ${JSON.stringify(differences)}`,
		);
	}
	const rows = domRows(dialog);
	expect(rows).toHaveLength(3);
	expect(rows.map((row) => row.length)).toEqual([3, 3, 3]);
	expect(rows.map((row) => row.map((cell) => cell.textContent ?? ""))).toEqual(
		expected.rows.map((row) =>
			row.map((cell) => cell.runs.map((run) => run.text).join("")),
		),
	);
	expect(within(dialog).getByText("表格前方文字保留")).toBeVisible();
	expect(within(dialog).getByText("表格後方文字保留")).toBeVisible();
}

export const SingleCellTextAndUndo: Story = {
	name: "單格刪字 · 保留列欄與復原",
	play: async ({ canvasElement, args, globals }) =>
		nativeTableTest(canvasElement, globals, async (dialog, user) => {
			const initialCellHtml = domRows(dialog).map((row) =>
				row.map((cell) => cell.innerHTML),
			);
			const cell = domRows(dialog)[1][1];
			await user.click(cell);
			selectCellText(cell, 2, cell, 4);
			await user.keyboard("{Backspace}");
			const expected = structuredClone(tableModel);
			expected.rows[1][1].runs = [
				{ text: "保留", bold: true },
				{ text: "尾端", italic: true },
			];
			await waitFor(() =>
				assertTableState(
					dialog,
					args.onChange.mock.lastCall?.[0] ?? args.value,
					expected,
					"single-delete",
					initialCellHtml,
				),
			);
			await user.click(within(dialog).getByRole("button", { name: /^復原（/ }));
			await waitFor(() =>
				assertTableState(
					dialog,
					args.onChange.mock.lastCall?.[0] ?? args.value,
					tableModel,
					"single-undo",
					initialCellHtml,
				),
			);
		}),
};

export const CrossCellRangeAndUndo: Story = {
	name: "跨格選取 · 清除文字並保留表格",
	play: async ({ canvasElement, args, globals }) =>
		nativeTableTest(canvasElement, globals, async (dialog, user) => {
			const initialCellHtml = domRows(dialog).map((row) =>
				row.map((cell) => cell.innerHTML),
			);
			const row = domRows(dialog)[1];
			await user.click(row[0]);
			selectCellText(row[0], 4, row[2], 4);
			await user.keyboard("{Delete}");
			const expected = structuredClone(tableModel);
			expected.rows[1][0].runs = [
				{ text: "前端保留", bold: true, color: "#682e0e" },
			];
			expected.rows[1][1].runs = [];
			expected.rows[1][2].runs = [{ text: "尾端保留", underline: true }];
			await waitFor(() =>
				assertTableState(
					dialog,
					args.onChange.mock.lastCall?.[0] ?? args.value,
					expected,
					"cross-delete",
					initialCellHtml,
				),
			);
			await user.click(within(dialog).getByRole("button", { name: /^復原（/ }));
			await waitFor(() =>
				assertTableState(
					dialog,
					args.onChange.mock.lastCall?.[0] ?? args.value,
					tableModel,
					"cross-undo",
					initialCellHtml,
				),
			);
		}),
};
