import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useEffect, useRef, useState } from "react";
import { expect, mocked, userEvent, waitFor } from "storybook/test";
import Button from "@/components/ui/Button";
import { track } from "@/lib/analytics/gtag";
import { commitDocumentNavigation } from "@/lib/navigation/document-location";
import {
	__resetNavigationObservabilityForTest,
	beginNavigationIntent,
	cancelNavigationIntent,
	markNavigationCommitted,
	markRouterTransitionStarted,
	navigateDocument,
} from "@/lib/navigation/observability";
import { navigationRecoveryTimeout } from "@/lib/navigation/recovery-state";

const DESTINATION = "/account/orders?status=paid#latest";
const LOGIN_DESTINATION = "/account?from=login#profile";

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
		const documentNavigation = mocked(commitDocumentNavigation);
		documentNavigation.mockImplementation((target, replace) => {
			const url = new URL(target);
			setDispatches((count) => count + 1);
			setStatus(
				`整頁目的地：${url.pathname}${url.search}${url.hash}（${replace ? "replace" : "push"}）`,
			);
		});
		return () => {
			clearTimeout(lateStart.current);
			cancelNavigationIntent();
			documentNavigation.mockRestore();
		};
	}, []);
	function startNavigation() {
		setStatus("等待訂單頁面");
		beginNavigationIntent({
			target: DESTINATION,
			trigger: "link",
			replace: true,
		});
		// Drive the real router lifecycle boundaries without RSC/network. The
		// production Link wiring is verified separately in Link.navigation tests.
		if (scenario === "late-start") {
			lateStart.current = setTimeout(() => {
				markRouterTransitionStarted(DESTINATION, "replace");
				markNavigationCommitted(DESTINATION);
				setStatus("延遲導覽已完成，保留目前文件");
			}, 450);
		}
	}
	return (
		<section className="mx-auto grid max-w-xl gap-4 text-[var(--text)]">
			<h1 className="text-xl font-semibold">導覽與登入跳轉</h1>
			<p className="text-[var(--muted)]">
				隔離預覽：等待時間縮短為 800ms，正式環境為 10／20 秒。
				不會登入、發送交易或離開預覽。
			</p>
			<Button variant="bordered" onClick={startNavigation}>
				前往訂單頁面
			</Button>
			{scenario === "login" ? (
				<Button
					color="primary"
					onClick={() =>
						navigateDocument(LOGIN_DESTINATION, {
							componentId: "login",
							actionId: "login_completed",
						})
					}
				>
					模擬登入完成
				</Button>
			) : null}
			<output aria-live="polite" className="break-all">
				{status}
			</output>
			<p>整頁跳轉次數：{dispatches}</p>
		</section>
	);
}

const meta = {
	title: "Web/Navigation/Recovery",
	component: NavigationRecoveryPreview,
	parameters: {
		docs: {
			description: {
				component:
					"正式導覽狀態機的隔離驅動預覽。僅替換整頁 location I/O、縮短等待預算並模擬 router 的開始／提交訊號；300ms 診斷與取消／復原規則保持正式實作。正式 Link 和登入 hook 接線另由整合單元測試驗證。詳見 docs/navigation-recovery.md。",
			},
		},
	},
	beforeEach: () => {
		__resetNavigationObservabilityForTest();
		const previousMarker = sessionStorage.getItem(
			"__hk_navigation_recovery_used",
		);
		sessionStorage.removeItem("__hk_navigation_recovery_used");
		mocked(navigationRecoveryTimeout).mockReturnValue(800);
		mocked(track).mockImplementation(() => {});
		const previousMotion = document.documentElement.dataset.vt;
		document.documentElement.dataset.vt = "none";
		return () => {
			__resetNavigationObservabilityForTest();
			if (previousMarker === null)
				sessionStorage.removeItem("__hk_navigation_recovery_used");
			else
				sessionStorage.setItem("__hk_navigation_recovery_used", previousMarker);
			mocked(navigationRecoveryTimeout).mockRestore();
			mocked(track).mockRestore();
			if (previousMotion === undefined)
				delete document.documentElement.dataset.vt;
			else document.documentElement.dataset.vt = previousMotion;
		};
	},
} satisfies Meta<typeof NavigationRecoveryPreview>;

export default meta;
type Story = StoryObj<typeof meta>;

export const UnstartedRecovery: Story = {
	args: { scenario: "unstarted" },
	play: async ({ canvas }) => {
		await userEvent.click(canvas.getByRole("button", { name: "前往訂單頁面" }));
		await expect(canvas.getByRole("status")).toHaveTextContent("等待訂單頁面");
		await waitFor(
			() =>
				expect(canvas.getByRole("status")).toHaveTextContent(
					`${DESTINATION}（replace）`,
				),
			{ timeout: 2_000 },
		);
		await expect(canvas.getByText("整頁跳轉次數：1")).toBeVisible();
	},
};

export const LateRouterStart: Story = {
	args: { scenario: "late-start" },
	play: async ({ canvas }) => {
		await userEvent.click(canvas.getByRole("button", { name: "前往訂單頁面" }));
		await waitFor(() =>
			expect(canvas.getByRole("status")).toHaveTextContent("延遲導覽已完成"),
		);
		await new Promise((resolve) => setTimeout(resolve, 500));
		await expect(canvas.getByText("整頁跳轉次數：0")).toBeVisible();
	},
};

export const LoginSupersedesPending: Story = {
	args: { scenario: "login" },
	play: async ({ canvas }) => {
		await userEvent.click(canvas.getByRole("button", { name: "前往訂單頁面" }));
		await userEvent.click(canvas.getByRole("button", { name: "模擬登入完成" }));
		await expect(canvas.getByRole("status")).toHaveTextContent(
			`${LOGIN_DESTINATION}（push）`,
		);
		await new Promise((resolve) => setTimeout(resolve, 950));
		await expect(canvas.getByText("整頁跳轉次數：1")).toBeVisible();
		await expect(canvas.getByRole("status")).toHaveTextContent(
			LOGIN_DESTINATION,
		);
	},
};
