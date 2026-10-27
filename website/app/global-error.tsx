"use client";

import * as Sentry from "@sentry/nextjs";
import * as React from "react";

/**
 * Last-resort boundary: only renders when the ROOT layout itself throws,
 * which means globals.css / providers may not exist — everything here is
 * inline-styled and dependency-free on purpose (no animation assets, no tokens; the
 * Sentry SDK is already in the client bundle via instrumentation-client).
 */
export default function GlobalError({
	error,
	reset,
}: {
	error: Error & { digest?: string };
	reset: () => void;
}) {
	React.useEffect(() => {
		// Root-layout crashes bypass ErrorState — capture here or they are
		// invisible in Sentry (boundary-caught errors never hit window.onerror).
		Sentry.captureException(error);
	}, [error]);
	return (
		<html lang="zh-TW">
			<body
				style={{
					margin: 0,
					minHeight: "100vh",
					display: "grid",
					placeItems: "center",
					// The root provider may be broken: retain the market's solid
					// fallback without importing the failed theme/animation tree.
					background: "#f8f6ef",
					color: "#1f2933",
					fontFamily:
						"'PingFang TC', 'Noto Sans TC', system-ui, -apple-system, sans-serif",
					textAlign: "center",
					padding: 24,
				}}
			>
				<div>
					<h1 style={{ margin: 0, fontSize: "1.5rem", fontWeight: 700 }}>
						糟糕，Hawkr 暫時迷航了
					</h1>
					<p
						style={{
							margin: "12px 0 0",
							fontSize: "0.875rem",
							lineHeight: 1.7,
							color: "#616e7c",
						}}
					>
						頁面發生未預期的錯誤，多半是暫時的。
					</p>
					<button
						type="button"
						onClick={reset}
						style={{
							marginTop: 24,
							minHeight: 44,
							padding: "0 28px",
							borderRadius: 999,
							border: "1px solid #b8441f",
							background: "#b8441f",
							color: "#fffdf7",
							fontSize: "0.875rem",
							fontWeight: 600,
							cursor: "pointer",
						}}
					>
						重新載入
					</button>
					{error.digest ? (
						<p
							style={{ marginTop: 20, fontSize: "0.6875rem", color: "#616e7c" }}
						>
							錯誤代碼 {error.digest}
						</p>
					) : null}
				</div>
			</body>
		</html>
	);
}
