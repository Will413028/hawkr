"use client";

import { SELLER_COVER_ASPECT } from "@/lib/media/seller-cover";
import type { SellerShareCardData } from "@/lib/seller/share-card";

const CARD_WIDTH = 1080;
const CARD_COVER_HEIGHT = CARD_WIDTH / SELLER_COVER_ASPECT;
const CARD_HEIGHT = CARD_COVER_HEIGHT + 1210;
const CARD_RADIUS = 64;
const PNG_SIGNATURE = [137, 80, 78, 71, 13, 10, 26, 10] as const;
const FONT_FAMILY =
	'"Noto Sans TC", "PingFang TC", "Microsoft JhengHei", system-ui, sans-serif';
// Bound each remote image fetch so one slow/hung asset can't stall the whole
// share-card export behind Promise.all — a timeout just drops that image to its
// canvas fallback (the surrounding try/catch already treats it as unavailable).
const SHARE_CARD_IMAGE_TIMEOUT_MS = 12_000;

function blobToDataUrl(blob: Blob): Promise<string> {
	return new Promise((resolve, reject) => {
		const reader = new FileReader();
		reader.onload = () => resolve(String(reader.result ?? ""));
		reader.onerror = () => reject(reader.error);
		reader.readAsDataURL(blob);
	});
}

async function imageToDataUrl(src: string): Promise<string | null> {
	if (!src) return null;
	if (src.startsWith("data:")) return src;
	try {
		const absoluteUrl = new URL(src, window.location.href).toString();
		const response = await fetch(absoluteUrl, {
			mode: "cors",
			credentials: absoluteUrl.startsWith(window.location.origin)
				? "same-origin"
				: "omit",
			signal: AbortSignal.timeout(SHARE_CARD_IMAGE_TIMEOUT_MS),
		});
		if (response.ok) {
			const blob = await response.blob();
			if (blob.type.startsWith("image/") && blob.size > 0) {
				return await blobToDataUrl(blob);
			}
		}
		return await imageToDataUrlViaProxy(absoluteUrl);
	} catch {
		return imageToDataUrlViaProxy(src);
	}
}

async function imageToDataUrlViaProxy(src: string): Promise<string | null> {
	try {
		const url = new URL(src, window.location.href);
		if (url.protocol !== "https:" || url.origin === window.location.origin) {
			return null;
		}
		const proxyUrl = `/api/seller/share-card/image?url=${encodeURIComponent(url.toString())}`;
		const response = await fetch(proxyUrl, {
			credentials: "same-origin",
			signal: AbortSignal.timeout(SHARE_CARD_IMAGE_TIMEOUT_MS),
		});
		if (!response.ok) return null;
		const blob = await response.blob();
		if (!blob.type.startsWith("image/") || blob.size === 0) return null;
		return await blobToDataUrl(blob);
	} catch {
		return null;
	}
}

function loadImage(src: string): Promise<HTMLImageElement> {
	return new Promise((resolve, reject) => {
		const image = new Image();
		image.decoding = "async";
		image.onload = () => resolve(image);
		image.onerror = () =>
			reject(new Error("Unable to decode seller card image"));
		image.src = src;
	});
}

async function loadRemoteImage(src: string): Promise<HTMLImageElement | null> {
	const dataUrl = await imageToDataUrl(src);
	if (!dataUrl) return null;
	try {
		return await loadImage(dataUrl);
	} catch {
		return null;
	}
}

async function svgElementToImage(
	element: SVGElement | null,
): Promise<HTMLImageElement | null> {
	if (!element) return null;
	try {
		const clone = element.cloneNode(true) as SVGElement;
		clone.setAttribute("xmlns", "http://www.w3.org/2000/svg");
		clone.setAttribute("color", "#b8441f");
		clone.setAttribute("width", "512");
		clone.setAttribute("height", "512");
		const source = new XMLSerializer().serializeToString(clone);
		return await loadImage(
			await blobToDataUrl(
				new Blob([source], { type: "image/svg+xml;charset=utf-8" }),
			),
		);
	} catch {
		return null;
	}
}

function roundedRectPath(
	ctx: CanvasRenderingContext2D,
	x: number,
	y: number,
	width: number,
	height: number,
	radius: number,
) {
	const r = Math.min(radius, width / 2, height / 2);
	ctx.beginPath();
	ctx.moveTo(x + r, y);
	ctx.lineTo(x + width - r, y);
	ctx.quadraticCurveTo(x + width, y, x + width, y + r);
	ctx.lineTo(x + width, y + height - r);
	ctx.quadraticCurveTo(x + width, y + height, x + width - r, y + height);
	ctx.lineTo(x + r, y + height);
	ctx.quadraticCurveTo(x, y + height, x, y + height - r);
	ctx.lineTo(x, y + r);
	ctx.quadraticCurveTo(x, y, x + r, y);
	ctx.closePath();
}

function fillRoundRect(
	ctx: CanvasRenderingContext2D,
	x: number,
	y: number,
	width: number,
	height: number,
	radius: number,
	fill: string | CanvasGradient,
) {
	roundedRectPath(ctx, x, y, width, height, radius);
	ctx.fillStyle = fill;
	ctx.fill();
}

function drawImageCover(
	ctx: CanvasRenderingContext2D,
	image: HTMLImageElement,
	x: number,
	y: number,
	width: number,
	height: number,
) {
	const scale = Math.max(
		width / image.naturalWidth,
		height / image.naturalHeight,
	);
	const sourceWidth = width / scale;
	const sourceHeight = height / scale;
	ctx.drawImage(
		image,
		(image.naturalWidth - sourceWidth) / 2,
		(image.naturalHeight - sourceHeight) / 2,
		sourceWidth,
		sourceHeight,
		x,
		y,
		width,
		height,
	);
}

function drawCardCover(
	ctx: CanvasRenderingContext2D,
	image: HTMLImageElement | null,
) {
	const height = CARD_COVER_HEIGHT;
	ctx.save();
	roundedRectPath(ctx, 0, 0, CARD_WIDTH, CARD_HEIGHT, CARD_RADIUS);
	ctx.clip();

	if (image) {
		drawImageCover(ctx, image, 0, 0, CARD_WIDTH, height);
	} else {
		const fallback = ctx.createLinearGradient(0, 0, CARD_WIDTH, height);
		fallback.addColorStop(0, "#f5f7fa");
		fallback.addColorStop(0.55, "#e4e7eb");
		fallback.addColorStop(1, "#f6edcf");
		ctx.fillStyle = fallback;
		ctx.fillRect(0, 0, CARD_WIDTH, height);
	}

	const overlayTop = height * 0.47;
	const overlay = ctx.createLinearGradient(0, height * 0.5, 0, height);
	overlay.addColorStop(0, "rgba(22, 28, 35, 0)");
	overlay.addColorStop(1, "rgba(22, 28, 35, 0.34)");
	ctx.fillStyle = overlay;
	ctx.fillRect(0, overlayTop, CARD_WIDTH, height - overlayTop);
	ctx.restore();
}

function drawAvatar(
	ctx: CanvasRenderingContext2D,
	image: HTMLImageElement | null,
) {
	const x = 184;
	const y = CARD_COVER_HEIGHT + 66;
	const radius = 112;
	ctx.save();
	ctx.shadowColor = "rgba(22, 28, 35, 0.22)";
	ctx.shadowBlur = 30;
	ctx.shadowOffsetY = 16;
	ctx.fillStyle = "#fffdf7";
	ctx.beginPath();
	ctx.arc(x, y, radius + 10, 0, Math.PI * 2);
	ctx.fill();
	ctx.restore();

	ctx.save();
	ctx.beginPath();
	ctx.arc(x, y, radius, 0, Math.PI * 2);
	ctx.clip();
	if (image) {
		drawImageCover(ctx, image, x - radius, y - radius, radius * 2, radius * 2);
	} else {
		const emptyFill = ctx.createLinearGradient(
			x - radius,
			y - radius,
			x + radius,
			y + radius,
		);
		emptyFill.addColorStop(0, "#f5f7fa");
		emptyFill.addColorStop(1, "#e4e7eb");
		ctx.fillStyle = emptyFill;
		ctx.fillRect(x - radius, y - radius, radius * 2, radius * 2);
	}
	ctx.restore();
}

function fitText(
	ctx: CanvasRenderingContext2D,
	text: string,
	maxWidth: number,
	startSize: number,
	minSize: number,
	weight = 800,
) {
	let size = startSize;
	while (size > minSize) {
		ctx.font = `${weight} ${size}px ${FONT_FAMILY}`;
		if (ctx.measureText(text).width <= maxWidth) break;
		size -= 2;
	}
	return size;
}

function ellipsizeText(
	ctx: CanvasRenderingContext2D,
	text: string,
	maxWidth: number,
) {
	if (ctx.measureText(text).width <= maxWidth) return text;
	const chars = Array.from(text);
	while (
		chars.length > 1 &&
		ctx.measureText(`${chars.join("")}…`).width > maxWidth
	) {
		chars.pop();
	}
	return `${chars.join("")}…`;
}

function wrapText(
	ctx: CanvasRenderingContext2D,
	text: string,
	maxWidth: number,
	maxLines: number,
): string[] {
	const lines: string[] = [];
	let line = "";
	for (const char of Array.from(text.trim())) {
		const next = `${line}${char}`;
		if (line && ctx.measureText(next).width > maxWidth) {
			lines.push(line.trimEnd());
			line = char.trimStart();
			if (lines.length === maxLines) break;
		} else {
			line = next;
		}
	}
	if (lines.length < maxLines && line) lines.push(line.trimEnd());
	if (lines.length === maxLines) {
		lines[maxLines - 1] = ellipsizeText(
			ctx,
			lines[maxLines - 1] ?? "",
			maxWidth,
		);
	}
	return lines;
}

function drawIdentity(
	ctx: CanvasRenderingContext2D,
	data: SellerShareCardData,
) {
	ctx.textAlign = "left";
	ctx.textBaseline = "alphabetic";
	ctx.fillStyle = "#161c23";
	const nameSize = fitText(ctx, data.name, 650, 78, 52);
	ctx.font = `800 ${nameSize}px ${FONT_FAMILY}`;
	ctx.fillText(ellipsizeText(ctx, data.name, 650), 342, CARD_COVER_HEIGHT + 98);

	// 沒有地點就不畫這顆 badge：空字串照樣會留下一個 56px 寬的色塊。
	if (!data.location) return;
	ctx.font = `700 34px ${FONT_FAMILY}`;
	const location = ellipsizeText(ctx, data.location, 560);
	const badgeWidth = Math.min(600, ctx.measureText(location).width + 56);
	fillRoundRect(
		ctx,
		342,
		CARD_COVER_HEIGHT + 132,
		badgeWidth,
		62,
		31,
		"rgba(184, 68, 31, 0.11)",
	);
	ctx.fillStyle = "#b8441f";
	ctx.textBaseline = "middle";
	ctx.fillText(location, 370, CARD_COVER_HEIGHT + 163);
}

function drawDescription(
	ctx: CanvasRenderingContext2D,
	data: SellerShareCardData,
) {
	let descriptionY = CARD_COVER_HEIGHT + 292;
	if (data.prize) {
		ctx.font = `700 31px ${FONT_FAMILY}`;
		const prize = ellipsizeText(ctx, data.prize, 690);
		const width = Math.min(760, ctx.measureText(prize).width + 92);
		fillRoundRect(ctx, 88, CARD_COVER_HEIGHT + 236, width, 58, 18, "#fbf4e1");
		fillRoundRect(ctx, 104, CARD_COVER_HEIGHT + 248, 34, 34, 17, "#ead79c");
		ctx.fillStyle = "#876f2d";
		ctx.textAlign = "center";
		ctx.textBaseline = "middle";
		ctx.font = `800 22px ${FONT_FAMILY}`;
		ctx.fillText("獎", 121, CARD_COVER_HEIGHT + 266);
		ctx.textAlign = "left";
		ctx.font = `700 31px ${FONT_FAMILY}`;
		ctx.fillText(prize, 154, CARD_COVER_HEIGHT + 265);
		descriptionY = CARD_COVER_HEIGHT + 350;
	}

	ctx.fillStyle = "#5f6658";
	ctx.font = `500 42px ${FONT_FAMILY}`;
	ctx.textAlign = "left";
	ctx.textBaseline = "alphabetic";
	const lines = wrapText(ctx, data.description, CARD_WIDTH - 176, 4);
	lines.forEach((line, index) => {
		ctx.fillText(line, 88, descriptionY + index * 66);
	});
}

function drawQrSection(
	ctx: CanvasRenderingContext2D,
	data: SellerShareCardData,
	qrImage: HTMLImageElement,
	logoImage: HTMLImageElement | null,
) {
	ctx.strokeStyle = "rgba(184, 68, 31, 0.14)";
	ctx.lineWidth = 2;
	ctx.beginPath();
	ctx.moveTo(88, CARD_COVER_HEIGHT + 630);
	ctx.lineTo(CARD_WIDTH - 88, CARD_COVER_HEIGHT + 630);
	ctx.stroke();

	const frameSize = 360;
	const frameX = (CARD_WIDTH - frameSize) / 2;
	const frameY = CARD_COVER_HEIGHT + 674;
	fillRoundRect(ctx, frameX, frameY, frameSize, frameSize, 42, "#fffdf8");
	ctx.strokeStyle = "rgba(184, 68, 31, 0.18)";
	ctx.lineWidth = 3;
	roundedRectPath(ctx, frameX, frameY, frameSize, frameSize, 42);
	ctx.stroke();

	const qrSize = 286;
	const qrX = (CARD_WIDTH - qrSize) / 2;
	const qrY = frameY + (frameSize - qrSize) / 2;
	ctx.drawImage(qrImage, qrX, qrY, qrSize, qrSize);

	const markSize = 72;
	const markX = (CARD_WIDTH - markSize) / 2;
	const markY = frameY + (frameSize - markSize) / 2;
	fillRoundRect(ctx, markX, markY, markSize, markSize, 18, "#fffdf8");
	if (logoImage) {
		ctx.drawImage(
			logoImage,
			markX + 13,
			markY + 13,
			markSize - 26,
			markSize - 26,
		);
	} else {
		ctx.fillStyle = "#b8441f";
		ctx.font = `800 34px ${FONT_FAMILY}`;
		ctx.textAlign = "center";
		ctx.textBaseline = "middle";
		ctx.fillText("日", CARD_WIDTH / 2, frameY + frameSize / 2 + 2);
	}

	ctx.fillStyle = "#7a806f";
	ctx.textAlign = "center";
	ctx.textBaseline = "alphabetic";
	ctx.font = `600 27px ${FONT_FAMILY}`;
	ctx.fillText(`商店 ID ${data.displayId}`, CARD_WIDTH / 2, CARD_HEIGHT - 98);
	ctx.fillText("掃描 QR code 開啟一頁式名片", CARD_WIDTH / 2, CARD_HEIGHT - 48);
}

function canvasToPngBlob(canvas: HTMLCanvasElement): Promise<Blob> {
	return new Promise((resolve, reject) => {
		canvas.toBlob((blob) => {
			if (blob?.type === "image/png" && blob.size > 0) resolve(blob);
			else reject(new Error("Unable to create seller card image"));
		}, "image/png");
	});
}

async function assertPngBlob(blob: Blob) {
	const bytes = new Uint8Array(
		await blob.slice(0, PNG_SIGNATURE.length).arrayBuffer(),
	);
	if (
		bytes.length !== PNG_SIGNATURE.length ||
		PNG_SIGNATURE.some((byte, index) => bytes[index] !== byte)
	) {
		throw new Error("Seller card export is not a valid PNG");
	}
}

export async function createSellerShareCardBlob(
	element: HTMLElement,
	data: SellerShareCardData,
): Promise<Blob> {
	try {
		await document.fonts?.ready;
	} catch {
		// System fonts remain available when a web font is blocked or slow.
	}

	const qrSvg = element.querySelector<SVGElement>(
		".seller-business-card__qr > svg",
	);
	const logoSvg = element.querySelector<SVGElement>(
		".seller-business-card__qr-mark svg",
	);
	const [coverImage, avatarImage, qrImage, logoImage] = await Promise.all([
		loadRemoteImage(data.coverUrl),
		loadRemoteImage(data.avatarUrl),
		svgElementToImage(qrSvg),
		svgElementToImage(logoSvg),
	]);
	if (!qrImage) throw new Error("Seller card QR code is not ready");

	const canvas = document.createElement("canvas");
	canvas.width = CARD_WIDTH;
	canvas.height = CARD_HEIGHT;
	const ctx = canvas.getContext("2d", { alpha: false });
	if (!ctx) throw new Error("Canvas is not available");
	ctx.imageSmoothingEnabled = true;
	ctx.imageSmoothingQuality = "high";

	ctx.fillStyle = "#e4e7eb";
	ctx.fillRect(0, 0, CARD_WIDTH, CARD_HEIGHT);
	ctx.save();
	roundedRectPath(ctx, 0, 0, CARD_WIDTH, CARD_HEIGHT, CARD_RADIUS);
	ctx.clip();
	const background = ctx.createLinearGradient(
		0,
		CARD_COVER_HEIGHT + 30,
		CARD_WIDTH,
		CARD_HEIGHT,
	);
	background.addColorStop(0, "#fffdf7");
	background.addColorStop(0.58, "#f5f7fa");
	background.addColorStop(1, "#f5f7fa");
	ctx.fillStyle = background;
	ctx.fillRect(0, 0, CARD_WIDTH, CARD_HEIGHT);
	drawCardCover(ctx, coverImage);
	ctx.restore();

	drawAvatar(ctx, avatarImage);
	drawIdentity(ctx, data);
	drawDescription(ctx, data);
	drawQrSection(ctx, data, qrImage, logoImage);

	ctx.strokeStyle = "rgba(184, 68, 31, 0.18)";
	ctx.lineWidth = 3;
	roundedRectPath(
		ctx,
		1.5,
		1.5,
		CARD_WIDTH - 3,
		CARD_HEIGHT - 3,
		CARD_RADIUS - 1.5,
	);
	ctx.stroke();

	const blob = await canvasToPngBlob(canvas);
	await assertPngBlob(blob);
	return blob;
}

export function downloadSellerShareCardBlob(blob: Blob, filename: string) {
	const url = URL.createObjectURL(blob);
	const link = document.createElement("a");
	link.href = url;
	link.download = filename;
	link.rel = "noopener";
	link.style.display = "none";
	document.body.appendChild(link);
	link.click();
	link.remove();
	// Safari may defer reading the Blob until the save sheet is confirmed.
	// Keeping it alive avoids zero-byte or broken files on slower devices.
	window.setTimeout(() => URL.revokeObjectURL(url), 60_000);
}
