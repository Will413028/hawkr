"use client";

import * as React from "react";
import EmptyStatePanel from "@/components/shared/EmptyStatePanel";
import Button from "@/components/ui/Button";
import { removeFavorite } from "@/lib/favorites/repo";

interface Props {
	productId: string;
}

/**
 * Rendered in place of the PDP when `/public/products/{id}` 404s — i.e. the
 * product was unpublished/deleted but a buyer still has a stale link to it.
 *
 * The actions are context-aware via a `?from=` query param so the page speaks
 * the right language depending on where the dead link was clicked:
 *   - `from=recent` (瀏覽紀錄) → just point back to history; nothing to remove.
 *   - default / `from=favorites` (收藏) → offer a one-click remove of the dead
 *     favourite row so it stops showing up in the collection.
 *
 * `removeFavorite` is idempotent — when the product wasn't in the buyer's
 * favourites the backend silently 404s the DELETE, which we treat as "already
 * gone" rather than an error.
 */
export default function ProductUnavailableView({ productId }: Props) {
	// Read the origin on the client so the server render stays cache-friendly
	// (this view only appears for the rare removed-product case). Defaults to
	// the favourites flavour, then switches if the link came from history.
	const [fromRecent, setFromRecent] = React.useState(false);
	React.useEffect(() => {
		if (typeof window === "undefined") return;
		const from = new URLSearchParams(window.location.search).get("from");
		if (from === "recent") setFromRecent(true);
	}, []);

	const [status, setStatus] = React.useState<
		"idle" | "removing" | "removed" | "error"
	>("idle");
	const [errorMsg, setErrorMsg] = React.useState<string | null>(null);

	const handleRemove = async () => {
		setStatus("removing");
		setErrorMsg(null);
		try {
			await removeFavorite(productId);
			setStatus("removed");
		} catch (e) {
			const message = e instanceof Error ? e.message : String(e);
			if (/404|not.?found/i.test(message)) {
				setStatus("removed");
				return;
			}
			setErrorMsg(message || "移除失敗，請稍後再試");
			setStatus("error");
		}
	};

	return (
		<main
			className="min-h-[60vh] grid place-items-center p-[var(--hkui-space-6)]"
			data-hk-page-content-ready="product-detail"
			data-hk-product-detail-id={productId}
		>
			<div className="max-w-[480px] w-full flex flex-col items-center text-center gap-[var(--hkui-space-3)]">
				<EmptyStatePanel
					icon="box"
					title="商品已下架"
					description={
						fromRecent
							? "這個商品目前已經下架或無法購買。你可以回到瀏覽紀錄看看其他商品。"
							: "這個商品目前已經下架或無法購買。如果它原本在你的收藏清單裡，你可以一鍵移除這筆死連結。"
					}
					iconSize={128}
					className="!bg-transparent !border-0"
				/>
				{!fromRecent && status === "removed" ? (
					<p className="text-[length:var(--hkui-text-small)] text-[var(--hkui-foreground-soft)]">
						已從收藏移除。
					</p>
				) : status === "error" && errorMsg ? (
					<p
						role="alert"
						className="text-[length:var(--hkui-text-small)] text-[var(--hkui-danger)]"
					>
						{errorMsg}
					</p>
				) : null}
				<div className="flex flex-wrap gap-[var(--hkui-space-2)] justify-center">
					{fromRecent ? (
						<Button href="/account/recent" variant="bordered" color="default">
							回瀏覽紀錄
						</Button>
					) : (
						<>
							{status !== "removed" && (
								<Button
									variant="bordered"
									color="default"
									onClick={() => void handleRemove()}
									disabled={status === "removing"}
									isLoading={status === "removing"}
									loadingLabel="移除收藏中…"
								>
									移除這筆收藏
								</Button>
							)}
							<Button
								href="/account/favorites"
								variant="bordered"
								color="default"
							>
								回我的收藏
							</Button>
						</>
					)}
					<Button href="/market/products">去逛逛商品</Button>
				</div>
			</div>
		</main>
	);
}
