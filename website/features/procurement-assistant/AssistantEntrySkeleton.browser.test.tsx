import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ViewTransitions } from "@/components/shared/view-transitions";
import { AuthProvider } from "@/lib/auth/context";
import type { SessionState } from "@/lib/auth/session-state";
import { renderWithIntl } from "@/tests/browser/render";
import { AssistantEntrySkeleton } from "./AssistantEntryLayout";
import AssistantEntryLink from "./AssistantEntryLink";

// 測試不掛 App Router：next/navigation 的 useRouter 在它之外會丟錯。mock 專案自己的
// `@/lib/intl/use-router`（所有 router 出口都經過它），不 mock 預先打包的 next/navigation。
vi.mock("@/lib/intl/use-router", async (importOriginal) => {
	const actual = await importOriginal<typeof import("@/lib/intl/use-router")>();
	const { staticRouter } = await import("@/tests/browser/static-router");
	return { ...actual, useRouter: () => staticRouter };
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
	it("keeps the same height as the loaded entry link", async () => {
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
		const loading = container.querySelector<HTMLElement>(
			"[data-agent-entry-loading] > div",
		);
		expect(loading).not.toBeNull();
		// 入口連結依 HAWKR_AGENT_ENABLED 旗標渲染，等它出現再量。
		await vi.waitFor(() => {
			expect(
				container.querySelector("[data-agent-entry-loaded] > a"),
			).not.toBeNull();
		});
		const loaded = container.querySelector<HTMLElement>(
			"[data-agent-entry-loaded] > a",
		);
		if (!loading || !loaded) throw new Error("entry elements missing");
		expect(
			Math.abs(
				loading.getBoundingClientRect().height -
					loaded.getBoundingClientRect().height,
			),
		).toBeLessThan(1);
	});
});
