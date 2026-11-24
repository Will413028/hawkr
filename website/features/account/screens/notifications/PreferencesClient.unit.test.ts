// @vitest-environment jsdom

import {
	NotificationAudience,
	NotificationCategory,
	NotificationChannel,
} from "@hawkr/api/gen/hawkr/notification/v1alpha1/notification_pb";
import { act, cleanup, fireEvent, render } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	update: vi.fn(),
	invalidate: vi.fn(async () => {}),
	entries: [] as unknown[],
}));

vi.mock("@/lib/intl/navigation", () => ({
	usePathname: () => "/seller-center/notifications/preferences",
}));
vi.mock("@/components/shared/SellerPageShell", () => ({
	SellerPageShell: ({ children }: { children: React.ReactNode }) =>
		React.createElement("div", null, children),
}));
vi.mock("@/components/ui/Button", () => ({
	default: ({ children }: { children: React.ReactNode }) =>
		React.createElement("a", null, children),
}));
vi.mock("@/lib/hooks/useNotificationPreferences", () => ({
	useNotificationPreferences: () => ({
		entries: mocks.entries,
		loading: false,
		error: null,
	}),
	useInvalidateNotificationPreferences: () => mocks.invalidate,
}));
vi.mock("@/lib/notifications/inbox", () => ({
	audienceForRole: () => 2,
	updateNotificationPreference: mocks.update,
}));

import { messagesFor } from "@/lib/intl/config";
import PreferencesClient from "./PreferencesClient";

afterEach(() => {
	cleanup();
	vi.clearAllMocks();
});

const entry = (
	category: NotificationCategory,
	channel: NotificationChannel,
	canDisable = true,
) => ({ category, channel, enabled: true, canDisable });

const h = React.createElement as unknown as (
	type: unknown,
	props: unknown,
	...children: unknown[]
) => React.ReactElement;

function mount() {
	return render(
		h(
			NextIntlClientProvider,
			{ locale: "zh-TW", messages: messagesFor("zh-TW") },
			h(PreferencesClient, null),
		),
	);
}

describe("PreferencesClient", () => {
	it("一個分類一列，每個伺服器回傳的渠道各一個開關，名稱來自文案目錄", () => {
		mocks.entries = [
			entry(NotificationCategory.ORDER, NotificationChannel.INBOX),
			entry(NotificationCategory.ORDER, NotificationChannel.PUSH),
			entry(NotificationCategory.COMMENT, NotificationChannel.INBOX),
		];
		const view = mount();
		expect(view.container.querySelectorAll("li")).toHaveLength(2);
		expect(
			view.container.querySelectorAll("input[type=checkbox]"),
		).toHaveLength(3);
		expect(view.container.textContent).toContain("訂單");
		expect(view.container.textContent).toContain("推播");
	});

	it("切換開關時以分類與渠道呼叫更新 RPC", async () => {
		mocks.entries = [
			entry(NotificationCategory.ORDER, NotificationChannel.INBOX),
			entry(NotificationCategory.ORDER, NotificationChannel.PUSH),
		];
		mocks.update.mockResolvedValue(undefined);
		const view = mount();
		const boxes = view.container.querySelectorAll("input[type=checkbox]");
		await act(async () => {
			fireEvent.click(boxes[1] as HTMLInputElement);
		});
		expect(mocks.update).toHaveBeenCalledTimes(1);
		expect(mocks.update).toHaveBeenCalledWith({
			audience: NotificationAudience.STORE,
			category: NotificationCategory.ORDER,
			channel: NotificationChannel.PUSH,
			enabled: false,
		});
	});

	it("不可關閉的項目鎖住且不呼叫更新", async () => {
		mocks.entries = [
			entry(
				NotificationCategory.SHIPMENT_ANOMALY,
				NotificationChannel.PUSH,
				false,
			),
		];
		const view = mount();
		const box = view.container.querySelector(
			"input[type=checkbox]",
		) as HTMLInputElement;
		expect(box.disabled).toBe(true);
		expect(box.checked).toBe(true);
		await act(async () => {
			fireEvent.click(box);
		});
		expect(mocks.update).not.toHaveBeenCalled();
	});

	it("更新失敗時開關回到原值", async () => {
		mocks.entries = [
			entry(NotificationCategory.ORDER, NotificationChannel.INBOX),
		];
		mocks.update.mockRejectedValue(new Error("boom"));
		const view = mount();
		const box = view.container.querySelector(
			"input[type=checkbox]",
		) as HTMLInputElement;
		await act(async () => {
			fireEvent.click(box);
		});
		expect(box.checked).toBe(true);
	});
});
