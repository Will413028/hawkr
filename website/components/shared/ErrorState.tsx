"use client";

import * as Sentry from "@sentry/nextjs";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Link from "@/components/shared/Link";
import StatusPageLayout, {
	STATUS_PAGE_GHOST_ACTION_CLASS,
	STATUS_PAGE_PRIMARY_ACTION_CLASS,
} from "@/components/shared/StatusPageLayout";

interface ErrorStateProps {
	error: Error & { digest?: string };
	reset: () => void;
	title?: string;
	description?: React.ReactNode;
	/** Secondary escape hatch — defaults to the home page. */
	homeHref?: string;
	homeLabel?: string;
	/**
	 * 已知/預期的失敗態（如首頁 gateway-down 空態）傳 false 關閉 Sentry 上報
	 * ——transport 對該類 5xx 已經報過，boundary 再報一次只會分裂 grouping。
	 */
	reportToSentry?: boolean;
}

/**
 * Shared visual for segment error boundaries (app/[locale]/error.tsx and the
 * commerce-specific one). A warning icon in the porthole, retry + escape actions, and the digest for support reports.
 *
 * The chrome (disc / eyebrow / heading / pill row) lives in
 * `StatusPageLayout`, shared with `NotFoundView`.
 */
export default function ErrorState({
	error,
	reset,
	title = "糟糕，火箭偏離了航道",
	description = (
		<>
			頁面在路上出了點狀況，多半是暫時的。
			<br />
			再試一次，或先回首頁看看今天的新鮮事。
		</>
	),
	homeHref = "/",
	homeLabel = "回到首頁",
	reportToSentry = true,
}: ErrorStateProps) {
	React.useEffect(() => {
		// Surface the real error for debugging/monitoring; the buyer-facing
		// copy stays generic. React errors caught by a boundary never reach
		// window.onerror, so without the explicit capture the client side of
		// this page is invisible in Sentry（近 14 天 0 筆 client render error
		// 不是沒發生，是沒上報）.
		console.error(error);
		if (reportToSentry) Sentry.captureException(error);
	}, [error, reportToSentry]);

	return (
		<StatusPageLayout
			data-hk-route-terminal-ready="error"
			announce
			media={
				<AppIcon
					name="danger"
					size={216}
					className="size-full"
					style={{ color: "var(--hkui-primary)" }}
				/>
			}
			eyebrow="暫時迷航"
			title={title}
			description={description}
			actions={
				<>
					<button
						type="button"
						onClick={reset}
						className={STATUS_PAGE_PRIMARY_ACTION_CLASS}
					>
						再試一次
					</button>
					<Link href={homeHref} className={STATUS_PAGE_GHOST_ACTION_CLASS}>
						{homeLabel}
					</Link>
				</>
			}
			footnote={
				error.digest ? (
					<p className="m-0 mt-6 text-xs tracking-[0.04em] text-hk-faint">
						錯誤代碼 {error.digest}
					</p>
				) : null
			}
		/>
	);
}
