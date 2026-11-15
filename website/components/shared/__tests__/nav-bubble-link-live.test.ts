/** @vitest-environment jsdom */

import { cleanup, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/components/shared/Link", async () => {
	const ReactModule = await vi.importActual<typeof import("react")>("react");
	return {
		default: ({
			prefetch: _prefetch,
			hardNavigation: _hardNavigation,
			...props
		}: React.AnchorHTMLAttributes<HTMLAnchorElement> & {
			prefetch?: boolean | null;
			hardNavigation?: boolean;
		}) => ReactModule.createElement("a", props),
	};
});

import NavBubbleLink from "@/components/shared/NavBubbleLink";

const unreadLabel = (display: string) => `${display} 則未讀`;

function bubble(count: number) {
	return React.createElement(NavBubbleLink, {
		href: "/account/messages",
		label: "訊息",
		count,
		countLabel: unreadLabel,
	});
}

describe("NavBubbleLink", () => {
	afterEach(cleanup);

	it("announces a count change with the same text as the accessible name", () => {
		const { container, rerender } = render(bubble(7));
		expect(screen.getByRole("link", { name: "訊息 7 則未讀" })).toBeTruthy();

		rerender(bubble(8));
		const live = container.querySelector("[data-nav-bubble-live]");
		expect(live?.textContent).toBe("訊息 8 則未讀");
		expect(screen.getByRole("link", { name: "訊息 8 則未讀" })).toBeTruthy();
	});

	it("names an icon-only bubble with its label when there is no count", () => {
		render(bubble(0));
		expect(screen.getByRole("link", { name: "訊息" })).toBeTruthy();
	});
});
