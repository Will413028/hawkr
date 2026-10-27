"use client";

import "@/styles/features/seller-profile.css";
import "@/styles/features/seller-share-card.css";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import type { SellerShareCardData } from "@/lib/seller/share-card";
import {
	createSellerShareCardBlob,
	downloadSellerShareCardBlob,
} from "@/lib/seller/share-card-client";
import SellerBusinessCard from "./SellerBusinessCard";

interface SellerShareButtonProps {
	data: SellerShareCardData;
	className?: string;
	label?: string;
}

type SellerSharePendingAction = "share" | "download";

async function copyText(value: string) {
	if (!navigator.clipboard) throw new Error("clipboard unavailable");
	await navigator.clipboard.writeText(value);
}

export default function SellerShareButton({
	data,
	className,
	label = "名片",
}: SellerShareButtonProps) {
	const [open, setOpen] = React.useState(false);
	const [notice, setNotice] = React.useState("");
	const [pendingAction, setPendingAction] =
		React.useState<SellerSharePendingAction | null>(null);
	const exporting = pendingAction !== null;
	const cardRef = React.useRef<HTMLElement | null>(null);
	const noticeTimerRef = React.useRef<ReturnType<typeof setTimeout> | null>(
		null,
	);

	React.useEffect(() => {
		return () => {
			if (noticeTimerRef.current) clearTimeout(noticeTimerRef.current);
		};
	}, []);

	const flash = React.useCallback((message: string) => {
		setNotice(message);
		if (noticeTimerRef.current) clearTimeout(noticeTimerRef.current);
		noticeTimerRef.current = setTimeout(() => setNotice(""), 2200);
	}, []);

	const shareCard = async () => {
		if (exporting) return;
		const shareTitle = `${data.name} 的 Hawkr 商家名片`;
		setPendingAction("share");
		try {
			if (typeof navigator.share === "function") {
				if (cardRef.current && typeof File === "function") {
					const blob = await createSellerShareCardBlob(cardRef.current, data);
					const file = new File([blob], `hawkr-seller-${data.sellerId}.png`, {
						type: "image/png",
					});
					if (
						typeof navigator.canShare === "function" &&
						navigator.canShare({ files: [file] })
					) {
						await navigator.share({
							files: [file],
							text: data.shareText,
							title: shareTitle,
							url: data.shareUrl,
						});
						flash("已開啟名片分享");
						return;
					}
				}
				await navigator.share({
					text: data.shareText,
					title: shareTitle,
					url: data.shareUrl,
				});
				flash("已開啟名片分享");
				return;
			}
			await copyText(data.shareUrl);
			flash("此瀏覽器不支援分享，已複製網址");
		} catch {
			flash("無法分享名片，請改用複製網址");
		} finally {
			setPendingAction(null);
		}
	};

	const copyUrl = async () => {
		try {
			await copyText(data.shareUrl);
			flash("網址已複製");
		} catch {
			flash("無法複製，請手動複製網址");
		}
	};

	const downloadCard = async () => {
		if (!cardRef.current || exporting) {
			flash("名片尚未載入，請稍後再試");
			return;
		}
		setPendingAction("download");
		try {
			const blob = await createSellerShareCardBlob(cardRef.current, data);
			downloadSellerShareCardBlob(blob, `hawkr-seller-${data.sellerId}.png`);
			flash("名片圖片已產生");
		} catch {
			flash("名片圖片產生失敗，請稍後再試");
		} finally {
			setPendingAction(null);
		}
	};

	return (
		<>
			<button
				type="button"
				className={
					className ??
					"hkui-seller-profile__cta hkui-seller-profile__cta--ghost"
				}
				onClick={() => setOpen(true)}
			>
				<AppIcon name="user" size={18} className="seller-share-button__glyph" />
				{label}
			</button>

			<Dialog
				open={open}
				onClose={() => setOpen(false)}
				title={`${data.name}的商家名片`}
				description="分享商店連結與名片圖片。"
				size="sm"
				className="seller-share-dialog"
			>
				<div className="seller-share-sheet" aria-busy={exporting}>
					<SellerBusinessCard ref={cardRef} data={data} />
					<div className="seller-share-actions">
						<Button
							type="button"
							variant="light"
							color="default"
							size="sm"
							className="seller-share-actions__item h-auto! min-h-[76px] p-0!"
							aria-label="分享名片"
							onClick={shareCard}
							disabled={exporting}
							isLoading={pendingAction === "share"}
							loadingLabel="分享名片中…"
						>
							<span className="inline-flex flex-col items-center gap-2">
								<span className="seller-share-actions__icon">
									<AppIcon
										name="send"
										size={20}
										className="seller-share-action__glyph"
									/>
								</span>
								<span>分享</span>
							</span>
						</Button>
						<Button
							type="button"
							variant="light"
							color="default"
							size="sm"
							className="seller-share-actions__item h-auto! min-h-[76px] p-0!"
							aria-label="下載名片圖"
							onClick={downloadCard}
							disabled={exporting}
							isLoading={pendingAction === "download"}
							loadingLabel="下載名片中…"
						>
							<span className="inline-flex flex-col items-center gap-2">
								<span className="seller-share-actions__icon">
									<AppIcon
										name="download"
										size={20}
										className="seller-share-action__glyph"
									/>
								</span>
								<span>下載</span>
							</span>
						</Button>
						<button
							type="button"
							className="seller-share-actions__item"
							aria-label="複製商店網址"
							onClick={copyUrl}
						>
							<span className="seller-share-actions__icon">
								<AppIcon
									name="paper"
									size={20}
									className="seller-share-action__glyph"
								/>
							</span>
							複製
						</button>
					</div>
					{notice && (
						<p className="seller-share-sheet__notice" role="status">
							{notice}
						</p>
					)}
				</div>
			</Dialog>
		</>
	);
}
