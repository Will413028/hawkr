"use client";

import AppIcon from "@/components/shared/AppIcon";
import SellerBusinessCard from "@/features/seller/components/SellerBusinessCard";
import SellerShareButton from "@/features/seller/components/SellerShareButton";
import { buildSellerShareCardData } from "@/lib/seller/share-card";

interface SellerShareCardPreviewProps {
	storeId?: string;
	storeCode?: string;
	storeName: string;
	description: string;
	prize: string;
	/** 縣市名與鄉鎮市區名（顯示用）。 */
	city: string;
	district: string;
	logoUrl?: string;
	coverUrl?: string;
}

function BusinessCardIcon() {
	return <AppIcon name="work" />;
}

export default function SellerShareCardPreview({
	storeId,
	storeCode,
	storeName,
	description,
	prize,
	city,
	district,
	logoUrl,
	coverUrl,
}: SellerShareCardPreviewProps) {
	const data = buildSellerShareCardData(
		storeId || "me",
		{
			storeId,
			city,
			coverUrl,
			description,
			district,
			logoUrl,
			prize,
			storeName,
		},
		storeCode,
	);

	return (
		<aside className="seller-profile__share-card" aria-label="商家名片預覽">
			<div className="seller-profile__share-card-head">
				<div className="seller-profile__share-card-title">
					<span className="seller-profile__share-card-icon">
						<BusinessCardIcon />
					</span>
					<span>
						<strong>商家名片</strong>
						<small>買家掃 QR code 開啟一頁式名片</small>
					</span>
				</div>
			</div>
			<div className="seller-profile__share-card-stage">
				<div className="seller-profile__share-card-body">
					<SellerBusinessCard data={data} compact />
				</div>
				<div className="seller-profile__share-card-actions">
					<SellerShareButton data={data} label="打開名片" />
				</div>
			</div>
		</aside>
	);
}
