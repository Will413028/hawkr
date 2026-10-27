"use client";

import "@/styles/features/cart-typography.css";

import * as React from "react";
import { Button, Dialog } from "@/components/ui";
import { getCartCouponAvailability } from "@/features/cart/couponAvailability";
import type { CartAvailableCoupon } from "@/lib/coupons/repo";
import { formatTaipeiDate } from "@/lib/date/taipei";
import { CouponSelectionSkeleton } from "./CouponSelectionSkeleton";

interface Props {
	open: boolean;
	coupons: CartAvailableCoupon[];
	selectedTemplateId: string | null;
	orderAmount: number;
	shippingFee: number;
	loading: boolean;
	error?: string | null;
	onSelect: (coupon: CartAvailableCoupon) => void;
	onClear: () => void;
	onRetry: () => void;
	onClose: () => void;
}

function ntFormat(n: number) {
	return `NT$ ${Math.max(0, Math.round(n)).toLocaleString()}`;
}

function couponBadge(c: CartAvailableCoupon): string {
	if (c.kind === "percent") return "折扣券";
	return "現金折抵";
}

function formatExpiry(c: CartAvailableCoupon): string {
	const date = formatTaipeiDate(c.expires_at);
	return date ? `有效至 ${date}` : "";
}

interface CouponRowModel {
	coupon: CartAvailableCoupon;
	templateId: string;
	usable: boolean;
	reason: string | null;
}

/** Cart coupon dialog using backend `estimatedDiscount` as its source of truth. */
export function CouponSelectionSheet({
	open,
	coupons,
	selectedTemplateId,
	orderAmount,
	shippingFee,
	loading,
	error,
	onSelect,
	onClear,
	onRetry,
	onClose,
}: Props) {
	const rows = React.useMemo<CouponRowModel[]>(
		() =>
			coupons.map((coupon) => {
				const availability = getCartCouponAvailability(coupon, {
					orderAmount,
					shippingFee,
				});
				return {
					coupon,
					templateId: coupon.templateId ?? String(coupon.id),
					...availability,
				};
			}),
		[coupons, orderAmount, shippingFee],
	);
	const availableRows = rows.filter((row) => row.usable);
	const unavailableRows = rows.filter((row) => !row.usable);

	return (
		<Dialog
			open={open}
			onClose={onClose}
			title="選擇優惠券"
			description={
				loading && rows.length === 0
					? "正在確認本次訂單可用優惠券"
					: `本次訂單有 ${availableRows.length} 張可用優惠券`
			}
			size="md"
			className="coupon-dialog"
			footer={
				<Button
					variant="light"
					color="default"
					fullWidth
					className="coupon-dialog__action"
					onClick={() => {
						onClear();
						onClose();
					}}
				>
					不使用優惠券
				</Button>
			}
		>
			{error ? (
				<div className="coupon-dialog__error" role="alert">
					<div>
						<strong>優惠券載入失敗</strong>
						<p>{error}</p>
					</div>
					<Button
						variant="bordered"
						color="danger"
						size="md"
						className="coupon-dialog__action"
						onClick={onRetry}
					>
						重新載入
					</Button>
				</div>
			) : null}

			{loading && rows.length === 0 ? (
				<CouponSelectionSkeleton />
			) : rows.length === 0 ? (
				<div className="coupon-dialog__empty">
					<strong>目前沒有優惠券</strong>
					<p>可到領券中心查看最新優惠，領取後再回購物車套用。</p>
				</div>
			) : (
				<div className="coupon-dialog__sections">
					{availableRows.length > 0 ? (
						<CouponSection
							title="可使用"
							rows={availableRows}
							selectedTemplateId={selectedTemplateId}
							onSelect={onSelect}
						/>
					) : (
						<div className="coupon-dialog__notice">
							目前沒有符合本次訂單條件的優惠券
						</div>
					)}

					{unavailableRows.length > 0 ? (
						<CouponSection
							title="目前不可使用"
							rows={unavailableRows}
							selectedTemplateId={selectedTemplateId}
							onSelect={onSelect}
						/>
					) : null}
				</div>
			)}
		</Dialog>
	);
}

function CouponSection({
	title,
	rows,
	selectedTemplateId,
	onSelect,
}: {
	title: string;
	rows: CouponRowModel[];
	selectedTemplateId: string | null;
	onSelect: (coupon: CartAvailableCoupon) => void;
}) {
	return (
		<section className="coupon-dialog__section" aria-label={title}>
			<header className="coupon-dialog__section-header">
				<h3>{title}</h3>
				<span>{rows.length} 張</span>
			</header>
			<ul className="coupon-sheet__list">
				{rows.map((row) => {
					const coupon = row.coupon;
					const active = row.usable && selectedTemplateId === row.templateId;
					return (
						<li key={row.templateId}>
							<button
								type="button"
								className={`coupon-sheet__row${active ? " coupon-sheet__row--active" : ""}${row.usable ? "" : " coupon-sheet__row--disabled"}`}
								onClick={() => onSelect(coupon)}
								disabled={!row.usable}
								aria-pressed={row.usable ? active : undefined}
							>
								<div className="coupon-sheet__lead">
									<span className="coupon-sheet__tag">
										{couponBadge(coupon)}
									</span>
									<strong className="coupon-sheet__discount">
										{coupon.estimatedDiscount > 0
											? `省 ${ntFormat(coupon.estimatedDiscount)}`
											: coupon.kind === "percent"
												? `${coupon.discountPercent ?? coupon.value ?? 0}%`
												: ntFormat(coupon.discountAmount ?? coupon.value ?? 0)}
									</strong>
								</div>
								<div className="coupon-sheet__body">
									<strong>{coupon.title ?? coupon.name}</strong>
									<small>{coupon.sellerName || "全平台券"}</small>
									{coupon.thresholdAmount || coupon.min_spend ? (
										<small>
											訂單滿{" "}
											{ntFormat(
												Number(coupon.thresholdAmount ?? coupon.min_spend),
											)}
										</small>
									) : (
										<small>不限最低消費</small>
									)}
									{formatExpiry(coupon) ? (
										<small>{formatExpiry(coupon)}</small>
									) : null}
									{row.reason ? (
										<small className="coupon-sheet__reason">{row.reason}</small>
									) : null}
								</div>
								<span className="coupon-sheet__selection" aria-hidden="true">
									{active ? "✓" : ""}
								</span>
							</button>
						</li>
					);
				})}
			</ul>
		</section>
	);
}
