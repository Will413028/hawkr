import { useEffect, useRef, useState } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { page, userEvent } from "vitest/browser";
import { track } from "@/lib/analytics/gtag";
import { renderWithIntl } from "@/tests/browser/render";
import { commitDocumentNavigation } from "./document-location";
import {
	__resetNavigationObservabilityForTest,
	beginNavigationIntent,
	cancelNavigationIntent,
	markNavigationCommitted,
	markRouterTransitionStarted,
	navigateDocument,
} from "./observability";
import { navigationRecoveryTimeout } from "./recovery-state";

vi.mock("./document-location", { spy: true });
vi.mock("./recovery-state", { spy: true });
vi.mock("@/lib/analytics/gtag", { spy: true });

const DESTINATION = "/account/orders?status=paid#latest";
const LOGIN_DESTINATION = "/account?from=login#profile";

let documentNavigationListener:
	| ((target: string, replace?: boolean) => void)
	| undefined;

function NavigationRecoveryPreview({
	scenario,
}: {
	scenario: "unstarted" | "late-start" | "login";
}) {
	const [status, setStatus] = useState("尚未導覽");
	const [dispatches, setDispatches] = useState(0);
	const lateStart = useRef<ReturnType<typeof setTimeout> | undefined>(
		undefined,
	);
	useEffect(() => {
		documentNavigationListener = (target, replace) => {
			const url = new URL(target);
			setDispatches((count) => count + 1);
			setStatus(
				`整頁目的地：${url.pathname}${url.search}${url.hash}（${replace ? "replace" : "push"}）`,
			);
		};
		return () => {
			clearTimeout(lateStart.current);
			cancelNavigationIntent();
			documentNavigationListener = undefined;
		};
	}, []);
	function startNavigation() {
		setStatus("等待訂單頁面");
		beginNavigationIntent({
			target: DESTINATION,
			trigger: "link",
			replace: true,
		});
		if (scenario === "late-start") {
			lateStart.current = setTimeout(() => {
				markRouterTransitionStarted(DESTINATION, "replace");
				markNavigationCommitted(DESTINATION);
				setStatus("延遲導覽已完成，保留目前文件");
			}, 450);
		}
	}
	return (
		<section>
			<button type="button" onClick={startNavigation}>
				前往訂單頁面
			</button>
			{scenario === "login" ? (
				<button
					type="button"
					onClick={() =>
						navigateDocument(LOGIN_DESTINATION, {
							componentId: "login",
							actionId: "login_completed",
						})
					}
				>
					模擬登入完成
				</button>
			) : null}
			<output aria-live="polite">{status}</output>
			<p>整頁跳轉次數：{dispatches}</p>
		</section>
	);
}

describe("navigation recovery", () => {
	let previousMarker: string | null;
	let previousMotion: string | undefined;

	beforeEach(() => {
		__resetNavigationObservabilityForTest();
		previousMarker = sessionStorage.getItem("__hk_navigation_recovery_used");
		sessionStorage.removeItem("__hk_navigation_recovery_used");
		vi.mocked(navigationRecoveryTimeout).mockReturnValue(800);
		vi.mocked(track).mockImplementation(() => {});
		vi.mocked(commitDocumentNavigation).mockImplementation((target, replace) =>
			documentNavigationListener?.(target, replace),
		);
		previousMotion = document.documentElement.dataset.vt;
		document.documentElement.dataset.vt = "none";
	});

	afterEach(() => {
		__resetNavigationObservabilityForTest();
		if (previousMarker === null)
			sessionStorage.removeItem("__hk_navigation_recovery_used");
		else
			sessionStorage.setItem("__hk_navigation_recovery_used", previousMarker);
		vi.mocked(navigationRecoveryTimeout).mockRestore();
		vi.mocked(track).mockRestore();
		vi.mocked(commitDocumentNavigation).mockRestore();
		vi.useRealTimers();
		if (previousMotion === undefined)
			delete document.documentElement.dataset.vt;
		else document.documentElement.dataset.vt = previousMotion;
	});

	it("falls back to a document navigation when the router never starts", async () => {
		renderWithIntl(<NavigationRecoveryPreview scenario="unstarted" />);
		await userEvent.click(
			page.getByRole("button", { name: "前往訂單頁面", exact: true }),
		);
		await expect
			.element(page.getByRole("status"))
			.toHaveTextContent("等待訂單頁面");
		await expect
			.element(page.getByRole("status"), { timeout: 2_000 })
			.toHaveTextContent(`整頁目的地：${DESTINATION}（replace）`);
		await expect
			.element(page.getByText("整頁跳轉次數：1", { exact: true }))
			.toBeVisible();
	});

	it("keeps the document when the router starts late", async () => {
		renderWithIntl(<NavigationRecoveryPreview scenario="late-start" />);
		vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
		await userEvent.click(
			page.getByRole("button", { name: "前往訂單頁面", exact: true }),
		);
		await vi.advanceTimersByTimeAsync(450);
		await expect
			.element(page.getByRole("status"))
			.toHaveTextContent("延遲導覽已完成，保留目前文件");
		await vi.advanceTimersByTimeAsync(1_000);
		await expect
			.element(page.getByText("整頁跳轉次數：0", { exact: true }))
			.toBeVisible();
	});

	it("lets a completed login supersede a pending navigation", async () => {
		renderWithIntl(<NavigationRecoveryPreview scenario="login" />);
		vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
		await userEvent.click(
			page.getByRole("button", { name: "前往訂單頁面", exact: true }),
		);
		await userEvent.click(
			page.getByRole("button", { name: "模擬登入完成", exact: true }),
		);
		await expect
			.element(page.getByRole("status"))
			.toHaveTextContent(`整頁目的地：${LOGIN_DESTINATION}（push）`);
		await vi.advanceTimersByTimeAsync(1_000);
		await expect
			.element(page.getByText("整頁跳轉次數：1", { exact: true }))
			.toBeVisible();
		await expect
			.element(page.getByRole("status"))
			.toHaveTextContent(`整頁目的地：${LOGIN_DESTINATION}（push）`);
	});
});
