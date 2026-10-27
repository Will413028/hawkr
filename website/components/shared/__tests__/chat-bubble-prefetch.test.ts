/** @vitest-environment jsdom */

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

interface CapturedNextLinkProps
	extends React.AnchorHTMLAttributes<HTMLAnchorElement> {
	prefetch?: boolean | null;
	hardNavigation?: boolean;
}

const nextLinkCapture = vi.hoisted(() => ({
	props: null as CapturedNextLinkProps | null,
}));

// ChatBubble renders through NavBubbleLink, which uses the SHARED transition
// link (it used to reach for next/link directly and so opted this one bubble
// out of the app's role-aware navigation). The capture therefore mocks the
// shared link; the contract under test — `prefetch` stays false until the user
// shows intent — is unchanged.
vi.mock("@/components/shared/Link", async () => {
	const ReactModule = await vi.importActual<typeof import("react")>("react");
	return {
		default: (props: CapturedNextLinkProps) => {
			nextLinkCapture.props = props;
			const {
				prefetch: _prefetch,
				hardNavigation: _hardNavigation,
				...anchorProps
			} = props;
			return ReactModule.createElement("a", anchorProps);
		},
	};
});

vi.mock("@/lib/chat/unread-context", () => ({
	useChatUnread: () => ({ count: 7 }),
}));

import ChatBubble from "@/components/shared/ChatBubble";

function renderBubble(
	callbacks: Partial<React.ComponentProps<typeof ChatBubble>> = {},
) {
	return render(
		React.createElement(
			ChatBubble,
			{
				href: "/account/messages",
				...callbacks,
			},
			"訊息",
		),
	);
}

function currentPrefetch(): boolean | null | undefined {
	return nextLinkCapture.props?.prefetch;
}

describe("ChatBubble intent-prefetch boundary", () => {
	afterEach(() => {
		cleanup();
		nextLinkCapture.props = null;
	});

	it("stays cold until hover and preserves mouse/click callbacks", () => {
		const onMouseEnter = vi.fn();
		const onClick = vi.fn((event: React.MouseEvent<HTMLAnchorElement>) => {
			event.preventDefault();
		});
		renderBubble({ onClick, onMouseEnter });
		const link = screen.getByRole("link", { name: "訊息（7 則未讀）" });

		expect(link.getAttribute("href")).toBe("/account/messages");
		expect(currentPrefetch()).toBe(false);
		fireEvent.mouseEnter(link);
		expect(currentPrefetch()).toBeNull();
		expect(onMouseEnter).toHaveBeenCalledTimes(1);

		fireEvent.click(link);
		expect(onClick).toHaveBeenCalledTimes(1);
	});

	it("enables framework prefetch on keyboard focus and preserves the callback", () => {
		const onFocus = vi.fn();
		renderBubble({ onFocus });
		const link = screen.getByRole("link", { name: "訊息（7 則未讀）" });

		expect(currentPrefetch()).toBe(false);
		fireEvent.focus(link);
		expect(currentPrefetch()).toBeNull();
		expect(onFocus).toHaveBeenCalledTimes(1);
	});

	it("enables framework prefetch on touch pointer intent", () => {
		const onPointerDown = vi.fn();
		renderBubble({ onPointerDown });
		const link = screen.getByRole("link", { name: "訊息（7 則未讀）" });

		expect(currentPrefetch()).toBe(false);
		fireEvent.pointerDown(link, { pointerType: "touch" });
		expect(currentPrefetch()).toBeNull();
		expect(onPointerDown).toHaveBeenCalledTimes(1);
	});
});
