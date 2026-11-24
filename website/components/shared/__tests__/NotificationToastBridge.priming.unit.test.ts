// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

const router = vi.hoisted(() => ({
	push: vi.fn(),
	replace: vi.fn(),
	refresh: vi.fn(),
}));
const repo = vi.hoisted(() => ({
	fetchUnreadCount: vi.fn(),
	listInboxItems: vi.fn(),
}));
const toast = vi.hoisted(() => ({ show: vi.fn() }));
const stream = vi.hoisted(() => ({ onSignal: vi.fn() }));

vi.mock("@/lib/navigation/use-observed-router", () => ({
	useObservedRouter: () => router,
}));
vi.mock("@/lib/intl/navigation", () => ({
	usePathname: () => "/account/notifications",
	useRouter: () => router,
}));
vi.mock("@sentry/nextjs", () => ({ captureMessage: vi.fn() }));
vi.mock("@/lib/navigation/document-location", () => ({
	commitDocumentNavigation: vi.fn(),
}));
vi.mock("@/lib/notifications/inbox", async (importOriginal) => ({
	...(await importOriginal<typeof import("@/lib/notifications/inbox")>()),
	fetchUnreadCount: repo.fetchUnreadCount,
	listInboxItems: repo.listInboxItems,
}));
vi.mock("@/lib/notifications/toast-context", () => ({
	useNotificationToast: () => ({ show: toast.show, dismiss: vi.fn() }),
}));
vi.mock("@/lib/chat/useChatStream", () => ({
	useChatStream: () => ({ state: "connected", stream: null }),
}));
vi.mock("@/lib/chat/repo", () => ({ listConversations: vi.fn() }));
vi.mock("@/lib/i18n/ui-text", () => ({ useUiText: () => () => "" }));
vi.mock("@/lib/notifications/watch", () => ({
	watchNotifications: (_audience: unknown, onSignal: unknown) => {
		stream.onSignal(onSignal);
		return { stop: vi.fn(), resume: vi.fn() };
	},
}));

import NotificationToastBridge from "@/components/shared/NotificationToastBridge";
import { AuthProvider } from "@/lib/auth/context";
import type { SessionState } from "@/lib/auth/session-state";
import { ZoneIdentityProvider } from "@/lib/auth/zone-context";
import { messagesFor } from "@/lib/intl/config";
import { NotificationUnreadProvider } from "@/lib/notifications/unread-context";

afterEach(() => {
	cleanup();
	vi.clearAllMocks();
	vi.unstubAllGlobals();
});

const h = React.createElement as unknown as (
	type: unknown,
	props: unknown,
	...children: unknown[]
) => React.ReactElement;

function memoryStorage() {
	const store = new Map<string, string>();
	return {
		getItem: (key: string) => store.get(key) ?? null,
		setItem: (key: string, value: string) => void store.set(key, value),
		removeItem: (key: string) => void store.delete(key),
	};
}

function deferred<T>() {
	let resolve!: (value: T) => void;
	const promise = new Promise<T>((done) => {
		resolve = done;
	});
	return { promise, resolve };
}

const flush = () =>
	act(async () => {
		await new Promise((done) => setTimeout(done, 20));
	});

const IDENTITY = { accountId: "1", isSeller: false };
const SIGNED_IN: SessionState = { status: "signedIn", identity: IDENTITY };

function mountBridge(session: Promise<SessionState>, zone: boolean): void {
	const bridge = h(
		NotificationUnreadProvider,
		null,
		h(NotificationToastBridge, null),
	);
	render(
		h(
			QueryClientProvider,
			{ client: new QueryClient() },
			h(
				NextIntlClientProvider,
				{ locale: "zh-TW", messages: messagesFor("zh-TW") },
				h(
					AuthProvider,
					{ session },
					zone
						? h(
								ZoneIdentityProvider,
								{ identity: IDENTITY, workspace: "buyer" },
								bridge,
							)
						: bridge,
				),
			),
		),
	);
}

describe("NotificationToastBridge priming when the unread count settles without an observable loading render", () => {
	for (const zone of [false, true]) {
		it(`reads the baseline page and toasts a later row (zone=${zone})`, async () => {
			vi.stubGlobal("localStorage", memoryStorage());
			vi.stubGlobal("sessionStorage", memoryStorage());
			repo.fetchUnreadCount.mockResolvedValue(1);
			const row = (id: string) => ({
				id,
				title: "訂單成立通知",
				body: "",
				deepLink: "",
				read: false,
				presentation: 1,
			});
			repo.listInboxItems.mockResolvedValue({
				items: [row("old-1")],
			});
			const session = deferred<SessionState>();
			mountBridge(session.promise, zone);
			await flush();

			(
				globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }
			).IS_REACT_ACT_ENVIRONMENT = false;
			session.resolve(SIGNED_IN);
			await new Promise((done) => setTimeout(done, 50));
			(
				globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }
			).IS_REACT_ACT_ENVIRONMENT = true;

			expect(repo.listInboxItems).toHaveBeenCalledWith({
				audience: 1,
				pageSize: 10,
			});

			const onSignal = stream.onSignal.mock.lastCall?.[0] as (
				signal: unknown,
			) => void;
			repo.listInboxItems.mockResolvedValue({
				items: [row("new-2"), row("old-1")],
			});
			await act(async () =>
				onSignal({ type: "inbox", unreadCount: 2, hints: [] }),
			);
			await flush();
			expect(toast.show).toHaveBeenCalledTimes(1);
		});
	}
});

describe("NotificationToastBridge signal arriving while the baseline read is pending", () => {
	it("does not toast baseline rows once the baseline resolves", async () => {
		vi.stubGlobal("localStorage", memoryStorage());
		vi.stubGlobal("sessionStorage", memoryStorage());
		repo.fetchUnreadCount.mockResolvedValue(1);
		const row = (id: string) => ({
			id,
			title: "訂單成立通知",
			body: "",
			deepLink: "",
			read: false,
			presentation: 1,
		});
		const baseline = deferred<{ items: ReturnType<typeof row>[] }>();
		repo.listInboxItems.mockReturnValueOnce(baseline.promise);
		repo.listInboxItems.mockResolvedValue({
			items: [row("new-2"), row("old-1")],
		});
		mountBridge(Promise.resolve(SIGNED_IN), false);
		await flush();
		expect(repo.listInboxItems).toHaveBeenCalledTimes(1);

		const onSignal = stream.onSignal.mock.lastCall?.[0] as (
			signal: unknown,
		) => void;
		await act(async () =>
			onSignal({ type: "inbox", unreadCount: 2, hints: [] }),
		);
		expect(toast.show).not.toHaveBeenCalled();

		await act(async () => baseline.resolve({ items: [row("old-1")] }));
		await flush();
		expect(toast.show).toHaveBeenCalledTimes(1);
		expect(toast.show.mock.calls[0]?.[0]).toMatchObject({
			dedupeKey: "notif:new-2",
		});
	});
});
