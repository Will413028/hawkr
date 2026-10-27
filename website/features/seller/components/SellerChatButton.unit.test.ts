/** @vitest-environment jsdom */

import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const harness = vi.hoisted(() => ({
	activeRole: "buyer" as "buyer" | "seller" | null,
	authGateCall: vi.fn(),
	hasSeller: true,
	hydrated: true,
	isLoggedIn: true,
	openChat: vi.fn(),
	openLoginRequired: vi.fn(),
	prewarmChatDrawerThread: vi.fn(),
	sellerSelf: {
		data: {
			store: { id: "9223372036854775807", storeCode: "2601234567" },
		} as { store?: { id?: string; storeCode?: string } } | null,
		error: null as Error | null,
		loading: false,
	},
}));

vi.mock("@/components/shared/AuthGateWaitModal", () => ({
	default: () => null,
}));
vi.mock("@/components/shared/AppIcon", () => ({ default: () => null }));
vi.mock("@/features/messages/components/ChatDrawerThreadLoader", () => ({
	prewarmChatDrawerThread: harness.prewarmChatDrawerThread,
}));
vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({
		activeRole: harness.activeRole,
		hasSeller: harness.hasSeller,
		hydrated: harness.hydrated,
		isLoggedIn: harness.isLoggedIn,
	}),
}));
vi.mock("@/lib/auth/useAuthGate", () => ({
	useAuthGate: (onGuest: () => void) => ({
		authGate: (
			action: (snapshot: { activeRole: "buyer" | "seller" | null }) => void,
		) => {
			harness.authGateCall();
			if (!harness.isLoggedIn) {
				onGuest();
				return;
			}
			action({ activeRole: harness.activeRole });
		},
		authWaiting: false,
	}),
}));
vi.mock("@/lib/hooks/useSellerSelf", () => ({
	useSellerSelf: () => harness.sellerSelf,
}));
vi.mock("@/lib/state/app-ui-store", () => ({
	useAppUiStore: (selector: (state: never) => unknown) =>
		selector({ openLoginRequired: harness.openLoginRequired } as never),
}));
vi.mock("@/lib/state/chat-drawer-store", () => ({
	useChatDrawerStore: (selector: (state: never) => unknown) =>
		selector({ openChat: harness.openChat } as never),
}));

import SellerChatButton from "./SellerChatButton";

describe("SellerChatButton self-store guard", () => {
	beforeEach(() => {
		harness.activeRole = "buyer";
		harness.hasSeller = true;
		harness.hydrated = true;
		harness.isLoggedIn = true;
		harness.sellerSelf.data = {
			store: { id: "9223372036854775807", storeCode: "2601234567" },
		};
		harness.sellerSelf.error = null;
		harness.sellerSelf.loading = false;
		harness.authGateCall.mockReset();
		harness.openChat.mockReset();
		harness.openLoginRequired.mockReset();
		harness.prewarmChatDrawerThread.mockReset();
	});

	afterEach(() => {
		cleanup();
		vi.useRealTimers();
	});

	it("removes the impossible CTA for the viewer's own store", () => {
		render(
			React.createElement(SellerChatButton, {
				sellerId: "9223372036854775807",
			}),
		);

		expect(screen.queryByRole("button", { name: "聊天" })).toBeNull();
	});

	it("does not apply stale seller data to a buyer-only account", () => {
		harness.hasSeller = false;
		render(
			React.createElement(SellerChatButton, {
				sellerId: "9223372036854775807",
			}),
		);

		expect(screen.getByRole("button", { name: "聊天" })).not.toBeNull();
	});

	it("keeps the exact preview CTA inert even when previewing the own store", () => {
		render(
			React.createElement(SellerChatButton, {
				previewMode: true,
				sellerId: "2601234567",
			}),
		);

		const button = screen.getByRole("button", { name: "聊天" });
		expect(button.getAttribute("aria-disabled")).toBe("true");
		fireEvent.focus(button);
		fireEvent.pointerDown(button, { pointerType: "touch" });
		fireEvent.mouseEnter(button);
		fireEvent.click(button);
		expect(harness.prewarmChatDrawerThread).not.toHaveBeenCalled();
		expect(harness.authGateCall).not.toHaveBeenCalled();
		expect(harness.openChat).not.toHaveBeenCalled();
	});

	it("fails closed and explains when seller identity is still loading", () => {
		harness.sellerSelf.data = null;
		harness.sellerSelf.loading = true;
		render(
			React.createElement(SellerChatButton, {
				sellerId: "9223372036854775807",
			}),
		);

		const button = screen.getByRole("button", { name: "聊天" });
		expect(button.getAttribute("aria-busy")).toBe("true");
		expect(button.getAttribute("aria-disabled")).toBe("true");
		fireEvent.focus(button);
		fireEvent.pointerDown(button, { pointerType: "touch" });
		fireEvent.mouseEnter(button);
		fireEvent.click(button);
		expect(harness.prewarmChatDrawerThread).not.toHaveBeenCalled();
		expect(harness.openChat).not.toHaveBeenCalled();
		expect(screen.getByRole("alert").textContent).toContain("正在確認商家身分");
		expect(button.getAttribute("aria-describedby")).toBe(
			screen.getByRole("alert").id,
		);
	});

	it("opens another store using the exact string target", () => {
		const target = "9223372036854775806";
		render(React.createElement(SellerChatButton, { sellerId: target }));

		fireEvent.click(screen.getByRole("button", { name: "聊天" }));
		expect(harness.prewarmChatDrawerThread).toHaveBeenCalledTimes(1);
		expect(harness.openChat).toHaveBeenCalledWith(target);
	});

	it("prewarms immediately for buyer focus and touch intent", () => {
		const target = "9223372036854775806";
		render(React.createElement(SellerChatButton, { sellerId: target }));
		const button = screen.getByRole("button", { name: "聊天" });

		fireEvent.focus(button);
		fireEvent.pointerDown(button, { pointerType: "touch" });

		expect(harness.prewarmChatDrawerThread).toHaveBeenCalledTimes(2);
		expect(harness.authGateCall).not.toHaveBeenCalled();
		expect(harness.openChat).not.toHaveBeenCalled();
	});

	it("bounds hover prewarming to 140ms and cancels abandoned intent", () => {
		vi.useFakeTimers();
		const target = "9223372036854775806";
		render(React.createElement(SellerChatButton, { sellerId: target }));
		const button = screen.getByRole("button", { name: "聊天" });

		fireEvent.mouseEnter(button);
		vi.advanceTimersByTime(139);
		expect(harness.prewarmChatDrawerThread).not.toHaveBeenCalled();
		fireEvent.mouseLeave(button);
		vi.advanceTimersByTime(1);
		expect(harness.prewarmChatDrawerThread).not.toHaveBeenCalled();

		fireEvent.mouseEnter(button);
		vi.advanceTimersByTime(140);
		expect(harness.prewarmChatDrawerThread).toHaveBeenCalledTimes(1);
	});

	it("does not prewarm before click for guests or a seller-role workspace", () => {
		vi.useFakeTimers();
		harness.isLoggedIn = false;
		harness.activeRole = null;
		harness.hasSeller = false;
		const view = render(
			React.createElement(SellerChatButton, {
				sellerId: "9223372036854775806",
			}),
		);
		let button = screen.getByRole("button", { name: "聊天" });

		fireEvent.focus(button);
		fireEvent.pointerDown(button, { pointerType: "touch" });
		fireEvent.mouseEnter(button);
		vi.advanceTimersByTime(140);
		expect(harness.prewarmChatDrawerThread).not.toHaveBeenCalled();
		fireEvent.click(button);
		expect(harness.openLoginRequired).toHaveBeenCalledTimes(1);
		expect(harness.prewarmChatDrawerThread).not.toHaveBeenCalled();

		harness.isLoggedIn = true;
		harness.activeRole = "seller";
		harness.hasSeller = true;
		view.rerender(
			React.createElement(SellerChatButton, {
				sellerId: "9223372036854775806",
			}),
		);
		button = screen.getByRole("button", { name: "聊天" });
		fireEvent.focus(button);
		fireEvent.pointerDown(button, { pointerType: "touch" });
		fireEvent.mouseEnter(button);
		vi.advanceTimersByTime(140);

		expect(harness.prewarmChatDrawerThread).not.toHaveBeenCalled();
	});
});
