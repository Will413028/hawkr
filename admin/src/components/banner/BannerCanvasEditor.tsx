"use client";

import type React from "react";
import { useCallback, useEffect, useRef, useState } from "react";
import { CanvasEditor } from "@/utils/CanvasEditor";
import { COLOR_SWATCHES, GRADIENT_PRESETS } from "./bannerPalette";

interface BannerCanvasEditorProps {
	initialJson?: string;
	onComplete: (imageFile: File, canvasJson: string) => void;
	onCancel: () => void;
}

const CANVAS_W = 1600;
const CANVAS_H = 900;

type TabType = "shape" | "text" | "image" | "color";

const BannerCanvasEditor: React.FC<BannerCanvasEditorProps> = ({
	initialJson,
	onComplete,
	onCancel,
}) => {
	const editorRef = useRef<CanvasEditor | null>(null);
	const containerRef = useRef<HTMLDivElement>(null);
	const fileInputRef = useRef<HTMLInputElement>(null);
	// initialJson 僅作掛載時的初始狀態,用 ref 持有以避免成為重建畫布的反應依賴
	const initialJsonRef = useRef(initialJson);
	const [activeTab, setActiveTab] = useState<TabType>("shape");
	const [scale, setScale] = useState(1);
	const [hasSelection, setHasSelection] = useState(false);

	// Initialize canvas(只在掛載時建立/銷毀一次)
	useEffect(() => {
		const editor = new CanvasEditor("banner-canvas", CANVAS_W, CANVAS_H);
		editorRef.current = editor;

		editor.setOnSelectionChange((obj) => setHasSelection(!!obj));

		const initialJson = initialJsonRef.current;
		if (initialJson) {
			try {
				editor.loadFromJSON(initialJson);
			} catch (e) {
				console.error("Failed to load canvas JSON:", e);
			}
		}

		return () => {
			editor.dispose();
			editorRef.current = null;
		};
	}, []);

	// Responsive scaling
	useEffect(() => {
		const container = containerRef.current;
		if (!container) return;

		const updateScale = () => {
			const rect = container.getBoundingClientRect();
			const padding = 40;
			const availW = rect.width - padding;
			const availH = rect.height - padding;
			const s = Math.min(availW / CANVAS_W, availH / CANVAS_H, 1);
			setScale(s);
		};

		updateScale();
		const observer = new ResizeObserver(updateScale);
		observer.observe(container);
		return () => observer.disconnect();
	}, []);

	const handleComplete = useCallback(() => {
		const editor = editorRef.current;
		if (!editor) return;

		const canvasJson = JSON.stringify(editor.toJSON());
		const dataUrl = editor.toDataURL("png");

		const byteString = atob(dataUrl.split(",")[1]);
		const mimeString = dataUrl.split(",")[0].split(":")[1].split(";")[0];
		const ab = new ArrayBuffer(byteString.length);
		const ia = new Uint8Array(ab);
		for (let i = 0; i < byteString.length; i++) {
			ia[i] = byteString.charCodeAt(i);
		}
		const blob = new Blob([ab], { type: mimeString });
		const file = new File([blob], "banner.png", { type: "image/png" });

		onComplete(file, canvasJson);
	}, [onComplete]);

	const ed = () => editorRef.current;

	return (
		<div className="flex flex-col h-full font-['Roboto','Noto_Sans_JP',sans-serif]">
			{/* Top bar */}
			<div className="h-[48px] border-b border-[var(--border-color)] flex items-center justify-between px-[16px] shrink-0 bg-white">
				<button
					type="button"
					onClick={onCancel}
					className="flex items-center gap-[6px] text-[14px] text-[var(--brand-gray1-100)] hover:text-black"
				>
					<svg
						aria-hidden="true"
						className="w-[18px] h-[18px]"
						viewBox="0 0 24 24"
						fill="none"
						stroke="currentColor"
						strokeWidth="2"
						strokeLinecap="round"
						strokeLinejoin="round"
					>
						<path d="M19 12H5" />
						<path d="M12 19l-7-7 7-7" />
					</svg>
					跳轉 Banner
				</button>
				<div className="flex items-center gap-[8px]">
					<button
						type="button"
						onClick={() => ed()?.undo()}
						className="p-[6px] rounded hover:bg-gray-100 text-[var(--brand-gray1-100)]"
						title="復原 (Ctrl+Z)"
					>
						<svg
							aria-hidden="true"
							className="w-[18px] h-[18px]"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							strokeWidth="2"
							strokeLinecap="round"
							strokeLinejoin="round"
						>
							<path d="M3 7v6h6" />
							<path d="M3 13a9 9 0 0118 0" />
						</svg>
					</button>
					<button
						type="button"
						onClick={() => ed()?.redo()}
						className="p-[6px] rounded hover:bg-gray-100 text-[var(--brand-gray1-100)]"
						title="重做 (Ctrl+Shift+Z)"
					>
						<svg
							aria-hidden="true"
							className="w-[18px] h-[18px]"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							strokeWidth="2"
							strokeLinecap="round"
							strokeLinejoin="round"
						>
							<path d="M21 7v6h-6" />
							<path d="M21 13a9 9 0 01-18 0" />
						</svg>
					</button>
					<button
						type="button"
						onClick={() => ed()?.deleteActiveObject()}
						className="p-[6px] rounded hover:bg-gray-100 text-[var(--brand-gray1-100)]"
						title="刪除"
					>
						<svg
							aria-hidden="true"
							className="w-[18px] h-[18px]"
							viewBox="0 0 24 24"
							fill="none"
							stroke="currentColor"
							strokeWidth="2"
							strokeLinecap="round"
							strokeLinejoin="round"
						>
							<path d="M3 6h18" />
							<path d="M8 6V4h8v2" />
							<path d="M5 6v14a2 2 0 002 2h10a2 2 0 002-2V6" />
						</svg>
					</button>
					<div className="w-px h-[24px] bg-[var(--border-color)] mx-[4px]" />
					<button
						type="button"
						onClick={handleComplete}
						className="px-[14px] py-[6px] rounded-[8px] bg-[var(--color)] text-[13px] font-bold text-white hover:opacity-90"
					>
						完成設計
					</button>
				</div>
			</div>

			<div className="flex flex-1 min-h-0">
				{/* Dark left panel */}
				<div className="w-[220px] bg-[#2b2b2b] shrink-0 flex flex-col overflow-y-auto">
					{/* Tab icons */}
					<div className="flex border-b border-[#444]">
						<TabBtn
							icon={<ShapeTabIcon />}
							active={activeTab === "shape"}
							onClick={() => setActiveTab("shape")}
						/>
						<TabBtn
							icon={<TextTabIcon />}
							active={activeTab === "text"}
							onClick={() => setActiveTab("text")}
						/>
						<TabBtn
							icon={<ImgTabIcon />}
							active={activeTab === "image"}
							onClick={() => setActiveTab("image")}
						/>
						<TabBtn
							icon={<ColorTabIcon />}
							active={activeTab === "color"}
							onClick={() => setActiveTab("color")}
						/>
					</div>

					<div className="p-[12px] flex flex-col gap-[12px]">
						{activeTab === "shape" && (
							<>
								<SectionLabel>形狀</SectionLabel>
								<div className="grid grid-cols-3 gap-[8px]">
									<ShapeBtn onClick={() => ed()?.addRect()} title="矩形">
										<rect x="4" y="4" width="16" height="16" rx="1" />
									</ShapeBtn>
									<ShapeBtn
										onClick={() => ed()?.addRoundedRect()}
										title="圓角矩形"
									>
										<rect x="4" y="4" width="16" height="16" rx="5" />
									</ShapeBtn>
									<ShapeBtn onClick={() => ed()?.addCircle()} title="圓形">
										<circle cx="12" cy="12" r="8" />
									</ShapeBtn>
									<ShapeBtn onClick={() => ed()?.addTriangle()} title="三角形">
										<polygon points="12,4 4,20 20,20" />
									</ShapeBtn>
									<ShapeBtn onClick={() => ed()?.addStar()} title="星形">
										<polygon points="12,2 15,9 22,9 17,14 19,22 12,17 5,22 7,14 2,9 9,9" />
									</ShapeBtn>
									<ShapeBtn onClick={() => ed()?.addLine()} title="線條">
										<line x1="4" y1="20" x2="20" y2="4" />
									</ShapeBtn>
								</div>
								<SectionLabel>基本形狀</SectionLabel>
								<div className="grid grid-cols-3 gap-[8px]">
									<ShapeBtn onClick={() => ed()?.addArrow()} title="箭頭">
										<polygon points="4,10 14,10 14,4 22,12 14,20 14,14 4,14" />
									</ShapeBtn>
									<ShapeBtn onClick={() => ed()?.addHexagon()} title="六邊形">
										<polygon points="12,3 20,7 20,17 12,21 4,17 4,7" />
									</ShapeBtn>
								</div>
							</>
						)}

						{activeTab === "text" && (
							<>
								<SectionLabel>文字</SectionLabel>
								<DarkBtn onClick={() => ed()?.addHeading()}>標題文字</DarkBtn>
								<DarkBtn onClick={() => ed()?.addSubheading()}>副標題</DarkBtn>
								<DarkBtn onClick={() => ed()?.addBodyText()}>內文文字</DarkBtn>
							</>
						)}

						{activeTab === "image" && (
							<>
								<SectionLabel>圖片</SectionLabel>
								<DarkBtn onClick={() => fileInputRef.current?.click()}>
									上傳圖片
								</DarkBtn>
							</>
						)}

						{activeTab === "color" && (
							<>
								{/* 物件填色 — 有選中物件時顯示在最上方 */}
								{hasSelection && (
									<>
										<SectionLabel>物件填色（已選取）</SectionLabel>
										{COLOR_SWATCHES.map((row) => (
											<div key={`obj-${row[0]}`} className="flex gap-[3px]">
												{row.map((color) => (
													<button
														type="button"
														key={color}
														onClick={() => ed()?.setObjectFill(color)}
														className="w-[18px] h-[18px] rounded-[2px] border border-[#555] hover:scale-125 transition-transform"
														style={{ background: color }}
														title={color}
													/>
												))}
											</div>
										))}
										<div className="h-px bg-[#444] my-[4px]" />
									</>
								)}

								<SectionLabel>背景顏色</SectionLabel>
								{COLOR_SWATCHES.map((row) => (
									<div key={`bg-${row[0]}`} className="flex gap-[3px]">
										{row.map((color) => (
											<button
												type="button"
												key={color}
												onClick={() => ed()?.setBackgroundColor(color)}
												className="w-[18px] h-[18px] rounded-[2px] border border-[#555] hover:scale-125 transition-transform"
												style={{ background: color }}
												title={color}
											/>
										))}
									</div>
								))}

								<SectionLabel>背景漸層</SectionLabel>
								<div className="flex flex-col gap-[6px]">
									{GRADIENT_PRESETS.map((g) => (
										<button
											type="button"
											key={`${g.c1}-${g.c2}`}
											onClick={() =>
												ed()?.setBackgroundGradient(g.c1, g.c2, "horizontal")
											}
											className="h-[24px] rounded-[4px] hover:opacity-80 transition-opacity"
											style={{
												background: `linear-gradient(to right, ${g.c1}, ${g.c2})`,
											}}
										/>
									))}
								</div>
							</>
						)}
					</div>
				</div>

				{/* Canvas area */}
				<div
					ref={containerRef}
					className="flex-1 flex items-center justify-center bg-[#e0e0e0] overflow-hidden"
				>
					<div
						style={{
							transform: `scale(${scale})`,
							transformOrigin: "center center",
						}}
					>
						<div
							className="shadow-lg"
							style={{ width: CANVAS_W, height: CANVAS_H }}
						>
							<canvas id="banner-canvas" />
						</div>
					</div>
				</div>
			</div>

			{/* Hidden file input */}
			<input
				ref={fileInputRef}
				type="file"
				accept="image/*"
				className="hidden"
				onChange={(e) => {
					const file = e.target.files?.[0];
					if (file) ed()?.addImageFromFile(file);
					e.target.value = "";
				}}
			/>
		</div>
	);
};

// --- Sub-components ---

function TabBtn({
	icon,
	active,
	onClick,
}: {
	icon: React.ReactNode;
	active: boolean;
	onClick: () => void;
}) {
	return (
		<button
			type="button"
			onClick={onClick}
			className={`flex-1 flex items-center justify-center py-[10px] transition-colors ${
				active
					? "bg-[#3a3a3a] text-white border-b-2 border-[var(--color)]"
					: "text-[#888] hover:text-white"
			}`}
		>
			{icon}
		</button>
	);
}

function SectionLabel({ children }: { children: React.ReactNode }) {
	return (
		<p className="text-[11px] font-bold text-[#888] uppercase tracking-wider">
			{children}
		</p>
	);
}

function ShapeBtn({
	children,
	onClick,
	title,
}: {
	children: React.ReactNode;
	onClick: () => void;
	title: string;
}) {
	return (
		<button
			type="button"
			onClick={onClick}
			title={title}
			className="aspect-square flex items-center justify-center rounded-[6px] bg-[#3a3a3a] hover:bg-[#4a4a4a] transition-colors"
		>
			<svg
				aria-hidden="true"
				className="w-[24px] h-[24px]"
				viewBox="0 0 24 24"
				fill="none"
				stroke="white"
				strokeWidth="1.5"
				strokeLinejoin="round"
			>
				{children}
			</svg>
		</button>
	);
}

function DarkBtn({
	children,
	onClick,
}: {
	children: React.ReactNode;
	onClick: () => void;
}) {
	return (
		<button
			type="button"
			onClick={onClick}
			className="w-full text-left px-[12px] py-[8px] rounded-[6px] bg-[#3a3a3a] text-[13px] text-[#ccc] hover:bg-[#4a4a4a] transition-colors"
		>
			{children}
		</button>
	);
}

// --- Tab Icons ---

function ShapeTabIcon() {
	return (
		<svg
			aria-hidden="true"
			className="w-[20px] h-[20px]"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="1.5"
		>
			<rect x="3" y="3" width="18" height="18" rx="2" />
		</svg>
	);
}

function TextTabIcon() {
	return (
		<svg
			aria-hidden="true"
			className="w-[20px] h-[20px]"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="2"
			strokeLinecap="round"
		>
			<path d="M4 7V4h16v3" />
			<path d="M12 4v16" />
			<path d="M8 20h8" />
		</svg>
	);
}

function ImgTabIcon() {
	return (
		<svg
			aria-hidden="true"
			className="w-[20px] h-[20px]"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="1.5"
			strokeLinecap="round"
			strokeLinejoin="round"
		>
			<rect x="3" y="3" width="18" height="18" rx="2" />
			<circle cx="8.5" cy="8.5" r="1.5" />
			<path d="M21 15l-5-5L5 21" />
		</svg>
	);
}

function ColorTabIcon() {
	return (
		<svg
			aria-hidden="true"
			className="w-[20px] h-[20px]"
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			strokeWidth="1.5"
			strokeLinecap="round"
		>
			<circle cx="12" cy="12" r="9" />
			<circle cx="12" cy="7" r="2" fill="currentColor" />
			<circle cx="7" cy="14" r="2" fill="currentColor" />
			<circle cx="17" cy="14" r="2" fill="currentColor" />
		</svg>
	);
}

export default BannerCanvasEditor;
