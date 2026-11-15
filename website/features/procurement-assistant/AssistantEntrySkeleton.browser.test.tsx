import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ViewTransitions } from "@/components/shared/view-transitions";
import { AuthProvider } from "@/lib/auth/context";
import type { SessionState } from "@/lib/auth/session-state";
import { renderWithIntl } from "@/tests/browser/render";
import { AssistantEntrySkeleton } from "./AssistantEntryLayout";
import AssistantEntryLink from "./AssistantEntryLink";

// 預覽不掛 App Router：next/navigation 的 useRouter 在它之外會丟錯，換成身分穩定的靜態 router。
vi.mock("next/navigation", async (importOriginal) => {
	const actual = await importOriginal<typeof import("next/navigation")>();
	const router = {
		push: () => {},
		replace: () => {},
		prefetch: () => {},
		back: () => {},
		forward: () => {},
		refresh: () => {},
	};
	return { ...actual, useRouter: () => router };
});

const GUEST_SESSION = Promise.resolve<SessionState>({ status: "guest" });

beforeEach(() => {
	vi.stubGlobal(
		"fetch",
		(() => new Promise<Response>(() => {})) as typeof fetch,
	);
});

afterEach(() => {
	vi.unstubAllGlobals();
});

describe("AssistantEntrySkeleton", () => {
	it("keeps the same height as the loaded entry link", () => {
		const client = new QueryClient({
			defaultOptions: { queries: { retry: false } },
		});
		const query = "預覽商品與規格比較，這是一段會在手機換行的已知搜尋內容";
		const { container } = renderWithIntl(
			<QueryClientProvider client={client}>
				<ViewTransitions>
					<AuthProvider session={GUEST_SESSION}>
						<div className="mx-auto grid max-w-[720px] gap-6">
							<div data-agent-entry-loading>
								<AssistantEntrySkeleton query={query} />
							</div>
							<div data-agent-entry-loaded>
								<AssistantEntryLink query={query} />
							</div>
						</div>
					</AuthProvider>
				</ViewTransitions>
			</QueryClientProvider>,
		);
		const loading = container.querySelector(
			"[data-agent-entry-loading] > div",
		) as HTMLElement;
		const loaded = container.querySelector(
			"[data-agent-entry-loaded] > a",
		) as HTMLElement;
		expect(
			Math.abs(
				loading.getBoundingClientRect().height -
					loaded.getBoundingClientRect().height,
			),
		).toBeLessThan(1);
	});
});
