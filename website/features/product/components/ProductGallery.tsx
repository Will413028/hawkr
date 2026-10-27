"use client";

import { type MotionValue, motion, useTransform } from "framer-motion";
import * as React from "react";
import AppImage from "@/components/shared/AppImage";
import { buildImageUrl, postCardPresetForMedia } from "@/lib/media/cdn";
import { PRODUCT_GALLERY_SLIDE_GAP_PX } from "@/lib/product/gallery-swipe";
import { useGalleryCardMotion } from "./useGalleryCardMotion";

import {
	type GalleryImagePreset,
	useGalleryImagePreset,
} from "./useGalleryImagePreset";

interface ProductGalleryProps {
	thumbnails: string[];
	productName: string;
	/**
	 * Product tags rendered as `#tag` pills in the top-right of the main
	 * image — mirrors the cream chip on the small product card preview.
	 * Pass already-formatted tag strings (with or without leading `#`); the
	 * component prepends `#` if missing. Pass an empty array (or omit) to
	 * hide the overlay entirely.
	 */
	tags?: string[];
}

export default function ProductGallery({
	thumbnails,
	productName,
	tags,
}: ProductGalleryProps) {
	const images = React.useMemo(
		() =>
			Array.from(
				new Set(thumbnails.filter((image) => image.trim().length > 0)),
			).slice(0, 9),
		[thumbnails],
	);
	const canSwipe = images.length > 1;
	const gallerySize = useGalleryImagePreset();
	const { activeIndex, dragging, position, reducedMotion, select, handlers } =
		useGalleryCardMotion(images.length, gallerySize.ref);

	const visibleTags = (tags ?? [])
		.slice(0, 3)
		.map((t) => {
			const trimmed = t.trim();
			if (!trimmed) return "";
			return trimmed.startsWith("#") ? trimmed : `#${trimmed}`;
		})
		.filter(Boolean);

	return (
		<div className="flex flex-col" data-product-gallery>
			{/* 4:3 跟隨縮圖與上傳裁切框的比例——原圖(700×525)在主圖不再被
			    object-cover 二次裁切，主圖與縮圖呈現的構圖一致。 */}
			<section
				ref={gallerySize.ref}
				className={`relative mb-4 aspect-[4/3] w-full overflow-hidden touch-pan-y outline-none focus-visible:ring-2 focus-visible:ring-hk-primary ${
					canSwipe ? (dragging ? "cursor-grabbing" : "cursor-grab") : ""
				}`}
				aria-roledescription="輪播"
				aria-label="商品圖片"
				tabIndex={canSwipe ? 0 : undefined}
				{...handlers}
				onDragStart={(event) => event.preventDefault()}
			>
				{images.length > 0 ? (
					<div
						data-product-gallery-track
						style={{ perspective: "1200px" }}
						className="relative h-full w-full select-none [&_img]:pointer-events-none"
					>
						{images.map((image, index) => (
							<GalleryCard
								key={image}
								index={index}
								position={position}
								reducedMotion={reducedMotion}
								active={index === activeIndex}
							>
								<GalleryImage
									image={image}
									preset={gallerySize.preset}
									alt={index === activeIndex ? productName : ""}
									first={index === 0}
									nearby={Math.abs(index - activeIndex) <= 1}
									active={index === activeIndex}
								/>
								{visibleTags.length > 0 && (
									<ul
										className="absolute top-4 right-4 m-0 p-0 list-none inline-flex flex-col items-end gap-1.5 pointer-events-none"
										aria-label="商品標籤"
									>
										{visibleTags.map((tag) => (
											<li
												key={tag}
												className="py-1.5 px-3.5 rounded-full text-[0.8125rem] font-semibold text-foreground bg-[var(--theme-surface-card)] border border-[var(--theme-border-soft)] whitespace-nowrap max-w-[180px] overflow-hidden text-ellipsis"
											>
												{tag}
											</li>
										))}
									</ul>
								)}
							</GalleryCard>
						))}
					</div>
				) : (
					<AppImage
						className="object-cover rounded-[30px] max-md:rounded-2xl max-sm:rounded-hk-lg"
						src=""
						alt={productName}
						fill
						sizes="(max-width: 1024px) 100vw, (max-width: 1440px) 50vw, 700px"
						priority
					/>
				)}
			</section>
			<p className="sr-only" aria-live="polite" aria-atomic="true">
				{images.length > 0
					? `第 ${activeIndex + 1} 張商品圖片，共 ${images.length} 張`
					: "尚無商品圖片"}
			</p>
			{/* 縮圖最多 9 張（對齊正式商品上架圖庫上限）：維持 5 欄佈局，
			    第 6~9 張自然換行到第二列，不破壞既有 RWD 排版。
			    無圖商品不顯示縮圖列（主圖已顯示空圖片 fallback）。 */}
			{images.length > 0 ? (
				<div className="grid grid-cols-5 gap-2.5 max-md:gap-1.5 max-sm:gap-1">
					{images.map((thumb, index) => {
						return (
							<button
								key={thumb}
								type="button"
								className={`relative w-full aspect-[4/3] rounded-[20px] overflow-hidden border-[3px] cursor-pointer bg-hk-cream p-0 transition-colors duration-200 ease-in-out max-md:rounded-hk-md max-md:border-2 max-sm:rounded-[10px] ${
									index === activeIndex
										? "border-hk-primary"
										: "border-transparent"
								}`}
								onClick={() => select(index)}
								aria-pressed={index === activeIndex}
								aria-label={`查看商品圖片 ${index + 1}`}
							>
								<AppImage
									className="object-cover"
									src={thumb}
									// The named thumb preset is square; card preserves the
									// uploaded 4:3 composition. GIFs keep their motion preset.
									preset={postCardPresetForMedia(thumb, "card")}
									alt={`縮圖 ${index + 1}`}
									fill
									sizes="(max-width: 768px) 18vw, 104px"
									loading="lazy"
									fetchPriority="low"
								/>
							</button>
						);
					})}
				</div>
			) : null}
		</div>
	);
}

function GalleryCard({
	index,
	position,
	reducedMotion,
	active,
	children,
}: {
	index: number;
	position: MotionValue<number>;
	reducedMotion: boolean;
	active: boolean;
	children: React.ReactNode;
}) {
	const transform = useTransform(position, (current) => {
		// Keep distant cards parked offscreen; only nearby cards change transforms.
		const offset = Math.max(-1.2, Math.min(1.2, index - current));
		const distance = Math.min(Math.abs(offset), 1);
		const angle = reducedMotion ? 0 : Math.max(-1, Math.min(1, offset)) * -5;
		const scale = reducedMotion ? 1 : 1 - distance * 0.04;
		return `translate3d(calc(${offset * 100}% + ${offset * PRODUCT_GALLERY_SLIDE_GAP_PX}px),0,0) rotateY(${angle}deg) scale(${scale})`;
	});
	return (
		<motion.div
			data-product-gallery-card
			className="absolute inset-0 overflow-hidden rounded-[30px] bg-hk-cream max-md:rounded-2xl max-sm:rounded-hk-lg [backface-visibility:hidden]"
			aria-hidden={!active}
			style={{ transform }}
		>
			{children}
		</motion.div>
	);
}

// Reuse the thumbnail URL at small sizes; only request a second image when
// the measured box and pixel density call for a larger gateway preset.
const GalleryImage = React.memo(function GalleryImage({
	image,
	alt,
	first,
	nearby,
	active,
	preset,
}: {
	image: string;
	alt: string;
	first: boolean;
	nearby: boolean;
	active: boolean;
	preset: GalleryImagePreset;
}) {
	const previewSrc = buildImageUrl(
		image,
		postCardPresetForMedia(image, "card"),
	);
	const mainPreset = postCardPresetForMedia(image, preset);
	const mainSrc = buildImageUrl(image, mainPreset);
	const needsLargerImage = mainSrc !== previewSrc;
	return (
		<>
			<AppImage
				src={image}
				preset={postCardPresetForMedia(image, "card")}
				alt={alt}
				fill
				className="object-cover"
				sizes="(max-width: 1024px) 100vw, 700px"
				priority={first}
				loading={first ? undefined : nearby ? "eager" : "lazy"}
				fetchPriority={active ? "high" : "low"}
				data-gallery-preview
			/>
			{needsLargerImage && (
				<GalleryDetailImage
					key={mainSrc}
					image={image}
					preset={mainPreset}
					nearby={nearby}
					active={active}
				/>
			)}
		</>
	);
});

function GalleryDetailImage({
	image,
	preset,
	nearby,
	active,
}: {
	image: string;
	preset: React.ComponentProps<typeof AppImage>["preset"];
	nearby: boolean;
	active: boolean;
}) {
	const [ready, setReady] = React.useState(false);
	const [failed, setFailed] = React.useState(false);
	if (failed) return null;
	return (
		<AppImage
			src={image}
			preset={preset}
			data-gallery-detail
			alt=""
			aria-hidden="true"
			fill
			className={`object-cover ${ready ? "opacity-100" : "opacity-0"}`}
			sizes="(max-width: 1024px) 100vw, 700px"
			loading={nearby ? "eager" : "lazy"}
			fetchPriority={active ? "high" : "low"}
			onLoad={() => setReady(true)}
			onError={() => setFailed(true)}
		/>
	);
}
