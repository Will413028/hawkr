import { useState } from "react";
import { describe, expect, it } from "vitest";
import { page, userEvent } from "vitest/browser";
import { renderWithIntl } from "@/tests/browser/render";
import Button from "./Button";
import { Dialog } from "./Dialog";

function Example() {
	const [open, setOpen] = useState(false);
	return (
		<>
			<Button onClick={() => setOpen(true)}>查看配送說明</Button>
			<Dialog
				open={open}
				onClose={() => setOpen(false)}
				title="配送說明"
				description="預覽用的靜態內容，不會修改訂單。"
			>
				<p>請依商品標示的保存方式收貨。</p>
			</Dialog>
		</>
	);
}

describe("Dialog", () => {
	it("opens from its trigger and closes on Escape", async () => {
		renderWithIntl(<Example />);
		await userEvent.click(page.getByRole("button", { name: "查看配送說明" }));
		await expect
			.element(page.getByRole("dialog", { name: "配送說明" }))
			.toBeVisible();
		await userEvent.keyboard("{Escape}");
		await expect.element(page.getByRole("dialog")).not.toBeInTheDocument();
	});
});
