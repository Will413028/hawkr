/** @vitest-environment jsdom */

import {
	cleanup,
	fireEvent,
	render as renderWithoutIntl,
	screen,
	waitFor,
} from "@testing-library/react";
import * as React from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { ProcurementCandidateSearch } from "@/lib/api/openapi-types";
import { IntlTestWrapper } from "@/tests/mocks/intl-test-wrapper";
import AssistantWorkspace from "../AssistantWorkspace";
import { hasProcurementResults } from "../session-logic";
import { initialState, restoredStateFrom } from "../useProcurementSession";

// session 的資料來源被換掉，Workspace 與 SidePanel 本身是真的。網路還原路徑由
// useProcurementSession 自己的測試守；這裡要問的是「拿到那份還原狀態之後，
// 使用者看不看得到成果」。⛔ restoredStateFrom 仍是真的（importOriginal），
// 不自己拼 state，否則就測不到「candidateSet 恆為 null、stage 不會是 candidates」。
const mocked = vi.hoisted(() => ({ session: null as unknown }));

vi.mock("../useProcurementSession", async (importOriginal) => {
	const actual =
		await importOriginal<typeof import("../useProcurementSession")>();
	return { ...actual, useProcurementSession: () => mocked.session };
});

vi.mock("next/navigation", async (importOriginal) => ({
	...(await importOriginal<typeof import("next/navigation")>()),
	usePathname: () => "/account/messages/assistant/procurement",
}));

// 對話氣泡只拿暱稱來標大頭貼，與成果面板的可見性無關；沿用本 feature 既有的
// auth mock 慣例（見 useProcurementSession.*.unit.test.ts），不為此拉起真 provider。
vi.mock("@/lib/auth/context", () => ({
	useAuth: () => ({
		isLoggedIn: true,
		activeRole: "buyer",
		identity: { accountId: "1001", nickname: "測試買家" },
	}),
}));

// jsdom 沒有 ResizeObserver。composer 的自動高度用得到它，但與成果面板無關，
// 所以給一個 no-op，而不是把 composer 從這棵樹裡拿掉。
class NoopResizeObserver {
	observe() {}
	unobserve() {}
	disconnect() {}
}
vi.stubGlobal("ResizeObserver", NoopResizeObserver);

type Session = ReturnType<
	typeof import("../useProcurementSession").useProcurementSession
>;

function sessionWith(state: Partial<ReturnType<typeof initialState>>): Session {
	return {
		...initialState(),
		...state,
		restoring: false,
		pendingSelectionCandidateIds: new Set<string>(),
		reset: vi.fn(),
		submitTurn: vi.fn(),
		submitAnswer: vi.fn(),
		rewindSlot: vi.fn(),
		startSearch: vi.fn(),
		retryPolling: vi.fn(),
		toggleSelection: vi.fn(),
		chooseCropDisambiguation: vi.fn(),
		previewRfqPrice: vi.fn(),
		confirmRfqSnapshot: vi.fn(),
		previewRfqPublication: vi.fn(),
		executeRfqPublication: vi.fn(),
		refreshCurrentSnapshot: vi.fn(),
		resume: vi.fn(),
	} as unknown as Session;
}

/**
 * desktop／mobile 的唯一分岔就是 `(min-width: 1200px)` 這個 media query：
 * 命中時 Workspace 會自己把面板展開，沒命中就等使用者按「查看成果」。
 * jsdom 沒有 matchMedia，所以兩條路都必須在這裡明確餵值，不能靠預設。
 */
function setViewport(desktop: boolean) {
	window.matchMedia = ((query: string) => ({
		matches: desktop,
		media: query,
		onchange: null,
		addListener: () => {},
		removeListener: () => {},
		addEventListener: () => {},
		removeEventListener: () => {},
		dispatchEvent: () => false,
	})) as unknown as typeof window.matchMedia;
}

const render = (
	ui: React.ReactElement,
	options?: Parameters<typeof renderWithoutIntl>[1],
) => renderWithoutIntl(ui, { ...options, wrapper: IntlTestWrapper });

function renderWorkspace() {
	return render(
		React.createElement(AssistantWorkspace, {
			initialRequestId: "1234567890123456788",
		}),
	);
}

/**
 * ⛔ 用 role 查，不用 `getElementById`。`hidden` 的 <aside> 仍然掛在 DOM 上，
 * 只是不在 accessibility tree 裡——A8 的壞法正是「掛得起來但永遠 hidden」，
 * 驗 DOM mount 會把那個 bug 判成綠的。
 */
const resultsPanel = () =>
	screen.queryByRole("complementary", { name: "採購成果" });
const resultsOpener = () => screen.queryByRole("button", { name: "查看成果" });
const mountedPanel = () => document.getElementById("hawkr-agent-side-panel");

afterEach(cleanup);

/**
 * A8 的缺陷不是「面板沒掛載」——掛載條件早就吃了 publishedPublication，而且有一條
 * source 斷言釘著它。壞的是可見性那一半：`hasResults` 只看 candidateSet，於是
 * resultsVisible、強制關閉 effect 與「查看成果」按鈕三處一起把面板鎖死。
 *
 * 重整必然踩中：restoredStateFrom 固定把 candidateSet 設成 null，而 stage 來自
 * deriveStageFromStatus，它刻意只回 "clarifying"／"ready"（見該函式註解），永遠不會
 * 是 "candidates"。所以「有 publication、沒有 candidateSet」就是重整後的常態。
 */
function publicationOnlySnapshot() {
	return {
		requestId: "1234567890123456788",
		revisionId: "1234567890123456791",
		status: "CANDIDATES_READY" as const,
		slots: {},
		missingRequired: [],
		openConfirmation: null,
		cropDisambiguation: null,
		// ⛔ 形狀照 Gateway 的 ProcurementSnapshotResponseDTO（procurement_response_mapping.go
		// 的 ProcurementPublishedPublicationDTO）：wire 上就只有這三個欄位。
		// 不要在 fixture 裡多塞 state／audienceMode 之類不存在的鍵——正規化器會忽略它們，
		// 測試照樣綠，但後來讀這份 fixture 的人會把它當成契約。
		publishedPublication: {
			publicationId: "1234567890123456804",
			version: "1",
			publishedAt: "2026-08-22T12:40:00Z",
		},
	};
}

describe("hasProcurementResults — 成果存在的唯一判準", () => {
	it("只有 publication 也算有成果", () => {
		expect(
			hasProcurementResults({
				stage: "ready",
				candidateSet: null,
				publishedPublication: { publicationId: "42" },
			}),
		).toBe(true);
	});

	it("候選輪詢期間的候選集算有成果", () => {
		expect(
			hasProcurementResults({
				stage: "candidates",
				candidateSet: { candidateSetId: "7" },
				publishedPublication: null,
			}),
		).toBe(true);
	});

	it("兩者皆無就是沒有成果", () => {
		expect(
			hasProcurementResults({
				stage: "ready",
				candidateSet: null,
				publishedPublication: null,
			}),
		).toBe(false);
	});

	it("stage 還沒到 candidates 時，光有候選集不算（維持既有行為）", () => {
		expect(
			hasProcurementResults({
				stage: "ready",
				candidateSet: { candidateSetId: "7" },
				publishedPublication: null,
			}),
		).toBe(false);
	});
});

describe("重整後的真實還原狀態", () => {
	it("restoredStateFrom 產出的 publication-only 狀態必須判定為有成果", () => {
		// ⛔ 不自己拼一個 state：用 production 的還原函式，才測得到
		// 「candidateSet 固定為 null、stage 永遠不是 candidates」這兩個真實約束。
		const restored = restoredStateFrom(
			null,
			publicationOnlySnapshot() as never,
		);

		expect(restored.candidateSet).toBeNull();
		expect(restored.stage).not.toBe("candidates");
		expect(restored.publishedPublication).not.toBeNull();
		expect(
			hasProcurementResults({
				stage: restored.stage as never,
				candidateSet: restored.candidateSet ?? null,
				publishedPublication: restored.publishedPublication ?? null,
			}),
		).toBe(true);
	});
});

function restoredPublicationOnly() {
	// ⛔ 經過 production 的還原函式，不手捏 state：candidateSet 恆為 null、
	// stage 永遠不是 "candidates"，這兩個約束正是 A8 踩中的東西。
	return restoredStateFrom(null, publicationOnlySnapshot() as never);
}

function candidateOnlyState() {
	const candidateSet: ProcurementCandidateSearch = {
		requestId: "1234567890123456788",
		runId: "1234567890123456792",
		candidateSetId: "1234567890123456790",
		status: "SUCCEEDED",
		platformLaneStatus: "SUCCEEDED",
		externalLaneStatus: "SUCCEEDED",
		candidates: { platform: [], external: [] },
	};
	// session 內剛跑完搜尋的樣子：stage 是 "candidates"，還沒有任何 publication。
	return {
		requestId: "1234567890123456788",
		revisionId: "1234567890123456791",
		stage: "candidates" as const,
		candidateSet,
	};
}

describe("真 render：重整後的成果面板看得見也打得開", () => {
	it("desktop／publication-only：面板自己就展開，關掉後還能再開", async () => {
		setViewport(true);
		mocked.session = sessionWith(restoredPublicationOnly());
		renderWorkspace();

		await waitFor(() => {
			expect(resultsPanel()).not.toBeNull();
		});

		fireEvent.click(screen.getByRole("button", { name: "回到對話" }));
		await waitFor(() => {
			expect(resultsPanel()).toBeNull();
		});

		// 關掉面板不等於成果消失：入口必須還在，而且按得動。
		const opener = resultsOpener();
		expect(opener).not.toBeNull();
		expect((opener as HTMLButtonElement).disabled).toBe(false);
		fireEvent.click(opener as HTMLButtonElement);
		await waitFor(() => {
			expect(resultsPanel()).not.toBeNull();
		});
	});

	it("mobile／publication-only：不自動展開，但入口打得開、關得掉、再開得起來", async () => {
		setViewport(false);
		mocked.session = sessionWith(restoredPublicationOnly());
		renderWorkspace();

		expect(resultsPanel()).toBeNull();
		expect(resultsOpener()).not.toBeNull();

		fireEvent.click(screen.getByRole("button", { name: "查看成果" }));
		await waitFor(() => {
			expect(resultsPanel()).not.toBeNull();
		});

		fireEvent.click(screen.getByRole("button", { name: "回到對話" }));
		await waitFor(() => {
			expect(resultsPanel()).toBeNull();
		});

		fireEvent.click(screen.getByRole("button", { name: "查看成果" }));
		await waitFor(() => {
			expect(resultsPanel()).not.toBeNull();
		});
	});

	it("正控制：candidate-only 在 desktop 一樣看得見（兩個來源走同一條路）", async () => {
		setViewport(true);
		mocked.session = sessionWith(candidateOnlyState());
		renderWorkspace();

		await waitFor(() => {
			expect(resultsPanel()).not.toBeNull();
		});
	});

	it("負控制：兩者皆無時，入口與面板都不存在", async () => {
		setViewport(true);
		mocked.session = sessionWith({
			requestId: "1234567890123456788",
			revisionId: "1234567890123456791",
			stage: "ready",
		});
		renderWorkspace();

		await waitFor(() => {
			expect(
				screen.getByRole("log", { name: "Hawkr Agent 採購經理紀錄" }),
			).toBeTruthy();
		});
		expect(resultsOpener()).toBeNull();
		expect(mountedPanel()).toBeNull();
	});

	it("負控制：處理中時面板仍掛著但不可見，入口也按不動（鎖定由 isProcessing 控制）", async () => {
		setViewport(true);
		mocked.session = sessionWith({
			...restoredPublicationOnly(),
			pendingTurn: true,
		});
		renderWorkspace();

		await waitFor(() => {
			expect(mountedPanel()).not.toBeNull();
		});
		// 這一組斷言同時證明上面的查詢真的分得出「掛載」與「看得見」。
		expect(resultsPanel()).toBeNull();
		expect((resultsOpener() as HTMLButtonElement).disabled).toBe(true);
	});
});
