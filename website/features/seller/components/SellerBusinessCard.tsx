"use client";

import "@/styles/features/seller-share-card.css";
import { QRCodeSVG } from "qrcode.react";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Avatar from "@/components/shared/Avatar";
import { SELLER_COVER_ASPECT } from "@/lib/media/seller-cover";
import type { SellerShareCardData } from "@/lib/seller/share-card";
import SellerCoverMedia from "./SellerCoverMedia";

const TRANSPARENT_QR_LOGO_PLACEHOLDER =
	"data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw==";

function HawkrLogoMark() {
	return (
		<svg
			aria-hidden="true"
			viewBox="0 0 32 32"
			fill="none"
			xmlns="http://www.w3.org/2000/svg"
		>
			<path
				d="M8.5 5.5 H23.5 L27 12 a3.667 3.667 0 0 1 -7.333 0 a3.667 3.667 0 0 1 -7.333 0 a3.667 3.667 0 0 1 -7.333 0 Z"
				fill="currentColor"
			/>
			<g stroke="currentColor" strokeWidth="3.6" strokeLinecap="round">
				<path d="M10.5 17.5 V26" />
				<path d="M21.5 17.5 V26" />
				<path d="M10.5 21.5 H21.5" />
			</g>
		</svg>
	);
}

interface SellerBusinessCardProps {
	data: SellerShareCardData;
	compact?: boolean;
}

const SellerBusinessCard = React.forwardRef<
	HTMLElement,
	SellerBusinessCardProps
>(function SellerBusinessCard({ data, compact = false }, ref) {
	const qrSize = compact ? 78 : 96;
	const qrLogoSize = compact ? 15 : 18;

	return (
		<article
			ref={ref}
			className={`seller-business-card${compact ? " seller-business-card--compact" : ""}`}
			aria-label={`${data.name} 商家名片`}
			data-seller-card-name={data.name}
		>
			<div
				className="seller-business-card__cover"
				aria-hidden="true"
				style={{ aspectRatio: SELLER_COVER_ASPECT }}
			>
				{data.coverUrl ? (
					<SellerCoverMedia src={data.coverUrl} sizes="340px" quality={70} />
				) : (
					<span className="seller-business-card__cover-fallback" />
				)}
				<span className="seller-business-card__cover-overlay" />
			</div>
			<div className="seller-business-card__body">
				<div className="seller-business-card__profile">
					<div className="seller-business-card__avatar">
						<Avatar
							src={data.avatarUrl}
							name={data.name}
							alt={data.name}
							entity="seller"
							fallback="empty"
						/>
					</div>
					<div className="seller-business-card__identity">
						<strong>{data.name}</strong>
						{/* 沒有地點就不畫這一行：補一個佔位字串會被讀成「這家店在那裡」。 */}
						{data.location ? <span>{data.location}</span> : null}
					</div>
				</div>
				{data.prize ? (
					<p className="seller-business-card__prize" title={data.prize}>
						<AppIcon name="trophy" size={12} />
						<span>{data.prize}</span>
					</p>
				) : null}
				<p className="seller-business-card__description">{data.description}</p>
				<div className="seller-business-card__footer">
					<div
						className="seller-business-card__qr"
						role="img"
						aria-label="商店 QR code"
					>
						<span
							className="seller-business-card__qr-ring"
							aria-hidden="true"
						/>
						<QRCodeSVG
							value={data.shareUrl}
							size={qrSize}
							level="H"
							includeMargin={false}
							marginSize={2}
							bgColor="transparent"
							fgColor="#1F2933"
							imageSettings={{
								src: TRANSPARENT_QR_LOGO_PLACEHOLDER,
								width: qrLogoSize,
								height: qrLogoSize,
								excavate: true,
							}}
						/>
						<span className="seller-business-card__qr-mark" aria-hidden="true">
							<HawkrLogoMark />
						</span>
					</div>
					<small className="seller-business-card__store-id">
						商店 ID {data.displayId}
					</small>
				</div>
			</div>
		</article>
	);
});

export default SellerBusinessCard;
