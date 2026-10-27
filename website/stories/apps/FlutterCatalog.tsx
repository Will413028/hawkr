import { useEffect, useState } from "react";

// Defined by .storybook/main.ts, also for production catalog subpath builds.
declare const __STORYBOOK_BASE_PATH__: string;

export const sharedCatalogComponents = [
	"foundations",
	"buttons",
	"inputs",
	"chips",
	"cards",
	"states",
	"commerce",
] as const;

type SharedCatalogComponent = (typeof sharedCatalogComponents)[number];

export type FlutterCatalogProps = {
	textScale?: "1" | "1.4";
	width?: number;
} & (
	| { app: "buyer"; component: SharedCatalogComponent | "cart" }
	| { app: "seller"; component: SharedCatalogComponent }
);

/**
 * Embeds the compiled production Flutter widgets, never React replicas.
 * @import import { FlutterCatalog } from '@/stories/apps/FlutterCatalog';
 */
export function FlutterCatalog({
	app,
	component,
	textScale = "1",
	width = 390,
}: FlutterCatalogProps) {
	const [status, setStatus] = useState<"checking" | "ready" | "missing">(
		"checking",
	);
	const base = `${__STORYBOOK_BASE_PATH__}flutter/${app}/`;
	useEffect(() => {
		const controller = new AbortController();
		setStatus("checking");
		fetch(`${base}flutter_bootstrap.js`, { signal: controller.signal })
			.then(async (response) => {
				const body = await response.text();
				setStatus(
					response.ok && body.includes("_flutter") ? "ready" : "missing",
				);
			})
			.catch(() => {
				if (!controller.signal.aborted) setStatus("missing");
			});
		return () => controller.abort();
	}, [base]);
	const label = `${app === "buyer" ? "買家" : "賣家"} App · ${component}`;
	return (
		<section style={{ width, maxWidth: "100%" }} aria-label={label}>
			<h2 style={{ fontSize: 18 }}>{label}</h2>
			{status === "ready" ? (
				<iframe
					className="catalog-app-frame"
					title={label}
					src={`${base}index.html?component=${component}&textScale=${textScale}&embedded=true`}
					height={820}
				/>
			) : (
				<div role="status" className="catalog-note">
					{status === "checking" ? (
						"正在檢查 App 預覽…"
					) : (
						<>
							尚未建置 Flutter 元件預覽。請在 Web 專案執行{" "}
							<code>pnpm storybook:apps</code>，然後重新啟動 Storybook。
						</>
					)}
				</div>
			)}
			<p className="catalog-note">
				真實 Flutter 元件；App 使用自身正式淺色主題。Web
				工具列主題不影響此預覽。
			</p>
		</section>
	);
}
