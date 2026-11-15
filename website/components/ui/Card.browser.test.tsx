import { describe, expect, it, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import { renderWithIntl } from "@/tests/browser/render";
import { Card, CardBody, CardFooter, CardHeader } from "./Card";
import { Chip } from "./Chip";

describe("Card", () => {
	it("calls onClick once when a pressable card is activated with Enter", async () => {
		const onClick = vi.fn();
		renderWithIntl(
			<Card isPressable aria-label="查看花蓮小農香米" onClick={onClick}>
				<CardHeader>
					<strong>花蓮小農香米</strong>
				</CardHeader>
				<CardBody>每日餐桌上的安心選擇。</CardBody>
				<CardFooter>
					<Chip variant="flat" color="primary">
						宅配
					</Chip>
				</CardFooter>
			</Card>,
			{ frameMaxWidth: 420 },
		);
		(page.getByRole("button").element() as HTMLElement).focus();
		await userEvent.keyboard("{Enter}");
		expect(onClick).toHaveBeenCalledTimes(1);
	});
});
