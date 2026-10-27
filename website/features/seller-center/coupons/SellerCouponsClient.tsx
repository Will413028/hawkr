"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import {
	SellerCouponsSkeleton,
	SellerEmptyState,
} from "@/features/seller-center";
import {
	SellerMobileDisclosureIcon,
	SellerMobileDisclosureList,
} from "@/features/seller-center/components/SellerMobileDisclosureList";
import { formatTaipeiDate } from "@/lib/date/taipei";
import { useQueryIdentity } from "@/lib/hooks/useQueryIdentity";
import { queryKeys } from "@/lib/query/keys";
import {
	deleteSellerCoupon,
	fetchSellerCoupons,
	type SellerCoupon,
} from "@/lib/seller/repo";
import { SellerMobileDisclosureSkeleton } from "../components/SellerPageSkeletons";

function formatDate(ts?: number) {
	return formatTaipeiDate(ts) || "—";
}

function couponState(c: SellerCoupon): {
	label: string;
	cls: string;
} {
	const now = Date.now();
	const start = c.starts_at
		? c.starts_at < 1e12
			? c.starts_at * 1000
			: c.starts_at
		: 0;
	const end = c.expires_at
		? c.expires_at < 1e12
			? c.expires_at * 1000
			: c.expires_at
		: 0;
	const limit = c.usage_limit ?? Number.POSITIVE_INFINITY;
	const used = c.usage_count ?? 0;
	if (end > 0 && now > end) return { label: "已結束", cls: "is-ended" };
	if (start > 0 && now < start) return { label: "未開始", cls: "is-pending" };
	if (limit !== Number.POSITIVE_INFINITY && used >= limit)
		return { label: "已領完", cls: "is-ended" };
	return { label: "進行中", cls: "is-active" };
}

function couponKey(coupon: SellerCoupon): string {
	return coupon.id;
}

function couponValueLabel(coupon: SellerCoupon): string {
	return coupon.type === "percent"
		? `${coupon.value}% off`
		: `折 NT$ ${Number(coupon.value).toLocaleString()}`;
}

export default function SellerCouponsClient() {
	const identity = useQueryIdentity();
	const queryClient = useQueryClient();
	const [deleteError, setDeleteError] = React.useState<string | null>(null);

	const couponsKey = queryKeys.seller.coupons(identity, { pageSize: 50 });
	const query = useQuery({
		queryKey: couponsKey,
		queryFn: () => fetchSellerCoupons({ pageSize: 50 }),
	});
	const items = query.data ?? [];
	const loading = query.isLoading;
	const error =
		deleteError ?? (query.error instanceof Error ? query.error.message : null);

	// Optimistically drop the row from the cache; roll back if the delete fails.
	const deleteMutation = useMutation({
		mutationFn: (id: string) => deleteSellerCoupon(id),
		onMutate: async (id: string) => {
			setDeleteError(null);
			await queryClient.cancelQueries({ queryKey: couponsKey });
			const before = queryClient.getQueryData<SellerCoupon[]>(couponsKey);
			queryClient.setQueryData<SellerCoupon[]>(couponsKey, (old) =>
				(old ?? []).filter((c) => c.id !== id),
			);
			return { before };
		},
		onError: (_e, _id, ctx) => {
			if (ctx?.before) queryClient.setQueryData(couponsKey, ctx.before);
			setDeleteError("刪除失敗");
		},
	});

	const handleDelete = (id: string) => {
		if (!window.confirm("確定要收回這張優惠券？已領取的用戶仍可使用。")) return;
		deleteMutation.mutate(id);
	};

	return (
		<SellerPageShell
			className="seller-coupons"
			title="優惠券"
			subtitle={`建立活動券吸引回購，或在退貨時發補償券給客戶。共 ${items.length} 張`}
			actions={<Button href="/seller-center/coupons/new">＋ 建立優惠券</Button>}
		>
			{error && (
				<p className="seller-coupons__error" role="alert">
					{error}
				</p>
			)}

			{loading && items.length === 0 ? (
				<>
					<div className="seller-mobile-disclosure-desktop">
						<SellerCouponsSkeleton count={4} />
					</div>
					<SellerMobileDisclosureSkeleton count={4} />
				</>
			) : items.length === 0 ? (
				<div className="seller-card seller-coupons__empty">
					<SellerEmptyState variant="coupons" seamless />
					<Button href="/seller-center/coupons/new">建立第一張優惠券</Button>
				</div>
			) : (
				<>
					<ul className="seller-coupons__list seller-mobile-disclosure-desktop">
						{items.map((c) => {
							const state = couponState(c);
							const valueLabel = couponValueLabel(c);
							return (
								<li key={c.id} className="seller-coupons__item">
									<div className="seller-coupons__chip">
										<strong>{valueLabel}</strong>
										<span>優惠券</span>
									</div>
									<div className="seller-coupons__main">
										<strong>{c.name}</strong>
										<small>
											{formatDate(c.starts_at)} – {formatDate(c.expires_at)}
										</small>
										<small>
											已領 {c.usage_count ?? 0} / {c.usage_limit ?? "∞"}
										</small>
									</div>
									<div className="seller-coupons__state">
										<span className={`seller-coupons__state-pill ${state.cls}`}>
											{state.label}
										</span>
									</div>
									<div className="seller-coupons__actions">
										<Button
											href={`/seller-center/coupons/${c.id}/edit`}
											variant="bordered"
											color="default"
											size="sm"
										>
											檢視
										</Button>
										<Button
											variant="bordered"
											color="danger"
											size="sm"
											onClick={() => void handleDelete(c.id)}
										>
											收回
										</Button>
									</div>
								</li>
							);
						})}
					</ul>
					<SellerMobileDisclosureList
						items={items}
						getKey={couponKey}
						ariaLabel="優惠券清單"
						getSummary={(coupon) => {
							const state = couponState(coupon);
							return {
								label: `優惠券：${coupon.name}`,
								leading: <SellerMobileDisclosureIcon name="discount" />,
								title: coupon.name,
								meta: couponValueLabel(coupon),
								status: (
									<span className={`seller-coupons__state-pill ${state.cls}`}>
										{state.label}
									</span>
								),
							};
						}}
						renderDetails={(coupon) => (
							<>
								<div className="seller-mobile-disclosure__body">
									<div className="seller-mobile-disclosure__detail-row">
										<span>活動日期</span>
										<strong>
											{formatDate(coupon.starts_at)} –{" "}
											{formatDate(coupon.expires_at)}
										</strong>
									</div>
									<div className="seller-mobile-disclosure__detail-row">
										<span>已領取</span>
										<strong>
											{coupon.usage_count ?? 0} / {coupon.usage_limit ?? "∞"}
										</strong>
									</div>
								</div>
								<div className="seller-mobile-disclosure__actions">
									<Button
										href={`/seller-center/coupons/${coupon.id}/edit`}
										variant="bordered"
										color="default"
										size="sm"
									>
										檢視
									</Button>
									<Button
										variant="bordered"
										color="danger"
										size="sm"
										onClick={() => void handleDelete(coupon.id)}
									>
										收回
									</Button>
								</div>
							</>
						)}
					/>
				</>
			)}
		</SellerPageShell>
	);
}
