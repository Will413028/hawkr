"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import { BackLink } from "@/components/shared/BackLink";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { ApiClientError } from "@/lib/api/client";
import type { MyPostDetail } from "@/lib/api/openapi-types";
import { sellerPostRepo } from "@/lib/seller/repo";
import PostComposeClient from "./PostComposeClient";

/**
 * Edit-post route shell. Hydrates the editable fields from the owner's
 * single-post read (`/app/sellers/posts/{id}`, any publish state) and hands them
 * to `PostComposeClient` in edit mode. Refresh-safe: state comes from the URL
 * id, not from in-memory navigation state.
 */
export default function PostEditClient({ id }: { id: string }) {
	const [detail, setDetail] = React.useState<MyPostDetail | null>(null);
	const [loading, setLoading] = React.useState(true);
	const [error, setError] = React.useState<string | null>(null);

	React.useEffect(() => {
		let cancelled = false;
		(async () => {
			try {
				const d = await sellerPostRepo.fetchSellerPostDetail(id);
				if (cancelled) return;
				setDetail(d);
			} catch (e) {
				if (cancelled) return;
				const notFound =
					e instanceof ApiClientError && (e.status === 404 || e.code === 404);
				setError(notFound ? "找不到這則動態" : "載入動態失敗");
			} finally {
				if (!cancelled) setLoading(false);
			}
		})();
		return () => {
			cancelled = true;
		};
	}, [id]);

	if (loading) {
		return (
			<div className="post-compose">
				<SellerPageShell title="編輯貼文">
					<p className="post-compose__hint">載入中…</p>
				</SellerPageShell>
			</div>
		);
	}

	if (error || !detail) {
		return (
			<div className="post-compose">
				<BackLink
					fallbackHref="/seller-center/posts"
					className="post-compose__back"
				>
					<AppIcon name="arrow-left" size={16} />
					返回
				</BackLink>
				<SellerPageShell title="編輯貼文">
					<p className="post-compose__error" role="alert">
						{error ?? "找不到這則動態"}
					</p>
				</SellerPageShell>
			</div>
		);
	}

	const linkedProductId = detail.products[0]?.id;

	return (
		<PostComposeClient
			edit={{
				id: detail.id,
				content: detail.content,
				imageKey: detail.imageKeys[0],
				productId:
					linkedProductId != null ? String(linkedProductId) : undefined,
			}}
		/>
	);
}
