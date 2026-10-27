"use client";

import { createClient } from "@connectrpc/connect";
import { createConnectQueryKey, useTransport } from "@connectrpc/connect-query";
import { fieldMessage, submitError } from "@hawkr/api/connect/submit";
import {
	type Banner as AdminBanner,
	BannerSchema,
	ContentAdminService,
} from "@hawkr/api/gen/hawkr/admin/content/v1alpha1/content_pb";
import { fieldMask } from "@hawkr/api/proto/field-mask";
import { defaultErrorTextZhTW } from "@hawkr/ui-text/zh-TW";
import { useQueryClient } from "@tanstack/react-query";
import Image from "next/image";
import { useRouter } from "next/navigation";
import type React from "react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
	actionFromProto,
	actionToProto,
	type BannerActionType,
} from "@/components/banner/action";
import BannerCanvasEditor from "@/components/banner/BannerCanvasEditor";
import { cropImageFileTo16x9 } from "@/components/banner/crop";
import {
	endDateOf,
	scheduleFromBanner,
	scheduleToBanner,
	startDateOf,
} from "@/components/banner/schedule";
import {
	ConflictRow,
	FieldErrorText,
	VersionConflictNotice,
} from "@/components/connect/FormFeedback";
import { errorMessage } from "@/lib/error-text";
import { uploadImageToMedia } from "@/services/media-upload";
import { imageUri } from "@/utils/image";

interface BannerEditorClientProps {
	id?: string;
}

type ImageSlot = "preview" | "popup";

interface ImageSlotState {
	/** 使用者選的新圖；null 表示沿用現有的圖（修改）或還沒選（新增）。 */
	file: File | null;
	/** 顯示用：新圖的 blob URL，或現有圖的網址。 */
	previewUrl: string;
	/** 新圖上傳後的 staged token。版本衝突後重新送出時沿用，不必再上傳。 */
	stagedToken: string;
}

const emptySlot: ImageSlotState = {
	file: null,
	previewUrl: "",
	stagedToken: "",
};

/** 修改時列進 update_mask 的欄位：表單上有的欄位（順序與上架狀態不在這裡改）。 */
const FORM_FIELDS = [
	"title",
	"button_text",
	"canvas_json",
	"action",
	"start_time",
	"end_time",
];

const actionLabel: Record<BannerActionType, string> = {
	popup: "顯示彈窗大圖",
	url: "開啟外部連結",
	product: "跳轉至商品",
	post: "跳轉至貼文",
};

const BannerEditorClient: React.FC<BannerEditorClientProps> = ({ id }) => {
	const router = useRouter();
	const queryClient = useQueryClient();
	const transport = useTransport();
	const client = useMemo(
		() => createClient(ContentAdminService, transport),
		[transport],
	);
	const isEdit = Boolean(id && id !== "new");

	const previewInputRef = useRef<HTMLInputElement>(null);
	const popupInputRef = useRef<HTMLInputElement>(null);

	const [title, setTitle] = useState("");
	const [actionType, setActionType] = useState<BannerActionType>("popup");
	const [actionUrl, setActionUrl] = useState("");
	const [actionId, setActionId] = useState("");
	const [buttonText, setButtonText] = useState("");
	const [startDate, setStartDate] = useState("");
	const [endDate, setEndDate] = useState("");
	const [noExpiry, setNoExpiry] = useState(true);
	const [saving, setSaving] = useState(false);
	const [uploading, setUploading] = useState(false);
	const [loading, setLoading] = useState(false);
	const [showCanvasEditor, setShowCanvasEditor] = useState<ImageSlot | null>(
		null,
	);
	const [canvasJson, setCanvasJson] = useState("");
	/** 讀取時拿到的版本；修改時帶回去，別人先改過就回版本衝突。 */
	const [etag, setEtag] = useState("");
	const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
	const [formError, setFormError] = useState("");
	/** 版本衝突時的最新版本，給使用者對照。 */
	const [latest, setLatest] = useState<AdminBanner | null>(null);

	const [preview, setPreview] = useState<ImageSlotState>(emptySlot);
	const [popup, setPopup] = useState<ImageSlotState>(emptySlot);

	/** 把伺服器的 banner 放進表單（載入時，或使用者選擇改用最新版本時）。 */
	const fillForm = useCallback((b: AdminBanner) => {
		setTitle(b.title);
		setButtonText(b.buttonText);
		const action = actionFromProto(b.action);
		setActionType(action.actionType);
		setActionUrl(action.actionUrl);
		setActionId(action.actionId);
		setPreview({
			file: null,
			stagedToken: "",
			previewUrl: imageUri(b.previewImage, "banner") ?? "",
		});
		setPopup({
			file: null,
			stagedToken: "",
			previewUrl: imageUri(b.popupImage, "banner") ?? "",
		});
		const schedule = scheduleFromBanner(b);
		setStartDate(schedule.startDate);
		setEndDate(schedule.endDate);
		setNoExpiry(schedule.noExpiry);
		setCanvasJson(b.canvasJson);
		setEtag(b.etag);
	}, []);

	const loadBanner = useCallback(async () => {
		if (!id) return;
		setLoading(true);
		try {
			const { banner } = await client.getBanner({ id });
			if (banner) fillForm(banner);
		} catch (e) {
			console.error("Failed to load banner:", e);
			setFormError(errorMessage(e));
		} finally {
			setLoading(false);
		}
	}, [client, fillForm, id]);

	useEffect(() => {
		if (isEdit) loadBanner();
	}, [isEdit, loadBanner]);

	const handleFileSelect = useCallback(async (slot: ImageSlot, file: File) => {
		if (!file.type.startsWith("image/")) return;
		if (file.size > 5 * 1024 * 1024) {
			alert("檔案大小不能超過 5MB");
			return;
		}
		const processedFile =
			slot === "preview" ? await cropImageFileTo16x9(file) : file;
		const blobUrl = URL.createObjectURL(processedFile);
		const setter = slot === "preview" ? setPreview : setPopup;
		setter({ file: processedFile, previewUrl: blobUrl, stagedToken: "" });
	}, []);

	const actionForm = { actionType, actionUrl, actionId };

	/** 上傳新選的圖（已經上傳過的沿用 staged token），回傳 token；沿用現有的圖時回空字串。 */
	const stagedTokenOf = async (
		slot: ImageSlotState,
		setter: React.Dispatch<React.SetStateAction<ImageSlotState>>,
	): Promise<string> => {
		if (!slot.file) return "";
		if (slot.stagedToken) return slot.stagedToken;
		const token = await uploadImageToMedia(slot.file);
		setter((prev) => ({ ...prev, stagedToken: token }));
		return token;
	};

	/** 送出。overwriteEtag 是使用者在版本衝突後選擇以自己的內容覆蓋時，最新版本的 etag。 */
	const handleSave = async (overwriteEtag?: string) => {
		if (!title.trim()) {
			alert("請輸入標題");
			return;
		}
		if (!preview.previewUrl || !popup.previewUrl) {
			alert("請上傳 Banner 預覽圖與彈窗大圖");
			return;
		}
		// 日期區間與點擊行為的值由伺服器檢查，錯誤標在對應的欄位上（規則只在伺服器一處）。

		setSaving(true);
		setFieldErrors({});
		setFormError("");
		const method = isEdit
			? ContentAdminService.method.updateBanner
			: ContentAdminService.method.createBanner;
		try {
			setUploading(true);
			const previewToken = await stagedTokenOf(preview, setPreview);
			const popupToken = await stagedTokenOf(popup, setPopup);
			setUploading(false);
			const banner = {
				title,
				action: actionToProto(actionForm),
				buttonText: actionType === "popup" ? "" : buttonText.trim(),
				canvasJson,
				...scheduleToBanner({ startDate, endDate, noExpiry }),
				previewImageStagedToken: previewToken,
				popupImageStagedToken: popupToken,
			};
			if (isEdit && id) {
				// 只換有新圖的那一張：沿用的圖不列進 update_mask（ADR 2026-09-25-api-image-uri-template D5）。
				const paths = [...FORM_FIELDS];
				if (previewToken) paths.push("preview_image_staged_token");
				if (popupToken) paths.push("popup_image_staged_token");
				await client.updateBanner({
					banner: { ...banner, id, etag: overwriteEtag ?? etag },
					updateMask: fieldMask(BannerSchema, paths),
				});
			} else {
				await client.createBanner({ banner: { ...banner, active: true } });
			}
			// 清單的 staleTime 是 30s，不失效就會在存檔後退回清單時繼續顯示改動前的資料。
			queryClient.invalidateQueries({
				queryKey: createConnectQueryKey({
					schema: ContentAdminService.method.listBanners,
					cardinality: "infinite",
				}),
			});
			router.push("/admin/banners");
		} catch (e) {
			setUploading(false);
			console.error("[BannerEditor] Save failed:", e);
			const failure = submitError(e, defaultErrorTextZhTW, method);
			if (failure.kind === "conflict" && id) {
				// 別人先改過：保留輸入，讀最新版本給使用者對照。
				try {
					const { banner } = await client.getBanner({ id });
					setLatest(banner ?? null);
				} catch (reloadErr) {
					setFormError(errorMessage(reloadErr));
				}
			} else if (failure.kind === "fields") {
				setFieldErrors(failure.fields);
				setFormError(failure.message ?? "");
				// 被拒的 staged token 不能再用，要重新上傳。
				if (fieldMessage(failure.fields, "banner.preview_image_staged_token")) {
					setPreview((prev) => ({ ...prev, stagedToken: "" }));
				}
				if (fieldMessage(failure.fields, "banner.popup_image_staged_token")) {
					setPopup((prev) => ({ ...prev, stagedToken: "" }));
				}
			} else if (failure.kind === "message") {
				setFormError(failure.message);
			}
		} finally {
			setSaving(false);
		}
	};

	// 必填（標題、兩張圖）沒填就不讓按；值的格式與完整性交給伺服器。
	const canSubmit = title.trim() && preview.previewUrl && popup.previewUrl;

	if (loading) {
		return (
			<div className="flex items-center justify-center h-64">
				<div className="animate-spin w-8 h-8 border-4 border-[var(--color)] border-t-transparent rounded-full" />
			</div>
		);
	}

	if (showCanvasEditor) {
		const activeSlot = showCanvasEditor;
		return (
			<div className="h-[calc(100vh-64px)]">
				<BannerCanvasEditor
					initialJson={canvasJson || undefined}
					onComplete={(file, json) => {
						const blobUrl = URL.createObjectURL(file);
						const setter = activeSlot === "preview" ? setPreview : setPopup;
						setter({ file, previewUrl: blobUrl, stagedToken: "" });
						setCanvasJson(json);
						setShowCanvasEditor(null);
					}}
					onCancel={() => setShowCanvasEditor(null)}
				/>
			</div>
		);
	}

	const ImageDropZone = ({
		slot,
		state,
		inputRef,
		label,
		hint,
		error,
	}: {
		slot: ImageSlot;
		state: ImageSlotState;
		inputRef: React.RefObject<HTMLInputElement | null>;
		label: string;
		hint: string;
		error?: string;
	}) => (
		<div className="flex flex-col gap-[12px]">
			<div className="flex items-center justify-between">
				<span className="text-[16px] font-bold text-[var(--brand-gray1-100)]">
					{label}
				</span>
				<button
					type="button"
					onClick={() => setShowCanvasEditor(slot)}
					className="rounded-[8px] text-white text-[14px] font-bold leading-[1.23] px-[12px] py-[6px] flex items-center gap-[6px]"
					style={{ background: "linear-gradient(to bottom, #00c4cc, #7d2ae8)" }}
				>
					<svg
						aria-hidden="true"
						className="w-[18px] h-[18px]"
						viewBox="0 0 24 24"
						fill="none"
					>
						<circle cx="12" cy="12" r="8" stroke="white" strokeWidth="1.5" />
						<path
							d="M12 8v8M8 12h8"
							stroke="white"
							strokeWidth="1.5"
							strokeLinecap="round"
						/>
					</svg>
					使用Canva設計
				</button>
			</div>

			{state.previewUrl ? (
				<button
					type="button"
					className="btn-reset relative border-2 border-[var(--border-color)] rounded-[10px] overflow-hidden h-[200px] cursor-pointer group w-full"
					onClick={() => inputRef.current?.click()}
				>
					<Image
						src={state.previewUrl}
						alt={label}
						width={400}
						height={200}
						unoptimized
						className="w-full h-full object-cover"
					/>
					<div className="absolute inset-0 bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center">
						<span className="text-white text-[16px] font-bold">
							點擊更換圖片
						</span>
					</div>
				</button>
			) : (
				<button
					type="button"
					className="btn-reset border-2 border-dashed rounded-[10px] h-[200px] w-full flex flex-col items-center justify-center gap-[8px] cursor-pointer transition-colors border-[var(--border-color)]"
					onClick={() => inputRef.current?.click()}
					onDrop={(e) => {
						e.preventDefault();
						const file = e.dataTransfer.files[0];
						if (file) handleFileSelect(slot, file);
					}}
					onDragOver={(e) => e.preventDefault()}
				>
					<div className="w-[40px] h-[40px] rounded-full bg-[var(--admin-primary)] flex items-center justify-center">
						<svg
							aria-hidden="true"
							className="w-[22px] h-[22px]"
							viewBox="0 0 24 24"
							fill="none"
						>
							<path
								d="M12 16V4M12 4L8 8M12 4L16 8"
								stroke="white"
								strokeWidth="2"
								strokeLinecap="round"
								strokeLinejoin="round"
							/>
							<path
								d="M4 14V18C4 19.1046 4.89543 20 6 20H18C19.1046 20 20 19.1046 20 18V14"
								stroke="white"
								strokeWidth="2"
								strokeLinecap="round"
								strokeLinejoin="round"
							/>
						</svg>
					</div>
					<p className="text-[14px] text-[#101828] leading-[22px]">
						點擊上傳或拖曳圖片至此
					</p>
					<p className="text-[12px] text-[#6a7282]">{hint}</p>
				</button>
			)}

			<input
				ref={inputRef}
				type="file"
				accept="image/png,image/jpeg"
				className="hidden"
				onChange={(e) => {
					const file = e.target.files?.[0];
					if (file) handleFileSelect(slot, file);
				}}
			/>
			<FieldErrorText message={error} />
		</div>
	);

	const errorOf = (...paths: string[]) => fieldMessage(fieldErrors, ...paths);

	return (
		<div className="flex items-start justify-center py-8 font-['Roboto','Noto_Sans_JP',sans-serif]">
			<div className="bg-white border border-[var(--border-color)] rounded-[15px] shadow-[0px_4px_4px_0px_rgba(0,0,0,0.1)] p-[24px] flex flex-col gap-[24px] w-[720px]">
				<div className="flex flex-col gap-[8px]">
					<h2 className="text-[18px] font-bold text-black leading-[1.23]">
						{isEdit ? "編輯Banner" : "上傳新 Banner"}
					</h2>
					<p className="text-[14px] text-[#717182] leading-[22px]">
						需同時上傳首頁顯示的小圖與點擊後彈出的大圖，支援 JPG、PNG。
					</p>
				</div>

				<div className="flex flex-col gap-[24px]">
					<div className="grid grid-cols-2 gap-[20px]">
						<ImageDropZone
							slot="preview"
							state={preview}
							inputRef={previewInputRef}
							label="Banner 預覽圖（首頁顯示）"
							hint="建議 1600x900px（16:9）, 5MB 內"
							error={errorOf("banner.preview_image_staged_token")}
						/>
						<ImageDropZone
							slot="popup"
							state={popup}
							inputRef={popupInputRef}
							label="彈窗大圖（點擊後顯示）"
							hint="建議 900x1200px, 5MB 內"
							error={errorOf("banner.popup_image_staged_token")}
						/>
					</div>

					<div className="flex flex-col gap-[16px]">
						<div className="flex items-center gap-[16px]">
							<label
								htmlFor="banner-title"
								className="w-[126px] text-[16px] font-bold text-[var(--brand-gray1-100)] text-right shrink-0"
							>
								標題
							</label>
							<input
								id="banner-title"
								type="text"
								value={title}
								onChange={(e) => setTitle(e.target.value)}
								placeholder={isEdit ? "" : "例如：夏季水果大促銷"}
								className="flex-1 h-[36px] bg-[#f3f3f5] rounded-[8px] px-[12px] text-[16px] text-black leading-[24px] outline-none placeholder:text-[var(--brand-gray1-50)]"
							/>
						</div>
						<div className="ml-[142px]">
							<FieldErrorText message={errorOf("banner.title")} />
						</div>

						<div className="flex items-start gap-[16px]">
							<label
								htmlFor="banner-action-type"
								className="w-[126px] text-[16px] font-bold text-[var(--brand-gray1-100)] text-right shrink-0 mt-[8px]"
							>
								點擊動作
							</label>
							<div className="flex-1 flex flex-col gap-[8px]">
								<select
									id="banner-action-type"
									value={actionType}
									onChange={(e) => {
										setActionType(e.target.value as BannerActionType);
										setActionUrl("");
										setActionId("");
									}}
									className="h-[36px] bg-[#f3f3f5] rounded-[8px] px-[12px] text-[16px] text-black leading-[24px] outline-none w-full"
								>
									<option value="popup">顯示彈窗大圖</option>
									<option value="url">開啟外部連結</option>
									<option value="product">跳轉至商品</option>
									<option value="post">跳轉至貼文</option>
								</select>
								{actionType === "url" && (
									<input
										type="text"
										value={actionUrl}
										onChange={(e) => setActionUrl(e.target.value)}
										placeholder="https://example.com/promotion"
										className="h-[36px] bg-[#f3f3f5] rounded-[8px] px-[12px] text-[16px] text-black leading-[24px] outline-none placeholder:text-[var(--brand-gray1-50)]"
									/>
								)}
								{(actionType === "product" || actionType === "post") && (
									<input
										type="text"
										value={actionId}
										onChange={(e) => setActionId(e.target.value)}
										placeholder={
											actionType === "product" ? "輸入商品 ID" : "輸入貼文 ID"
										}
										className="h-[36px] bg-[#f3f3f5] rounded-[8px] px-[12px] text-[16px] text-black leading-[24px] outline-none placeholder:text-[var(--brand-gray1-50)]"
									/>
								)}
								<FieldErrorText message={errorOf("banner.action")} />
							</div>
						</div>

						{actionType !== "popup" && (
							<div className="flex items-center gap-[16px]">
								<label
									htmlFor="banner-button-text"
									className="w-[126px] text-[16px] font-bold text-[var(--brand-gray1-100)] text-right shrink-0"
								>
									按鈕文字
								</label>
								<input
									id="banner-button-text"
									type="text"
									value={buttonText}
									onChange={(e) => setButtonText(e.target.value)}
									placeholder={
										actionType === "url"
											? "留空則顯示「查看詳情」"
											: actionType === "product"
												? "留空則顯示「立即購買」"
												: "留空則顯示「查看貼文」"
									}
									maxLength={12}
									className="flex-1 h-[36px] bg-[#f3f3f5] rounded-[8px] px-[12px] text-[16px] text-black leading-[24px] outline-none placeholder:text-[var(--brand-gray1-50)]"
								/>
							</div>
						)}
						{actionType !== "popup" && (
							<div className="ml-[142px]">
								<FieldErrorText message={errorOf("banner.button_text")} />
							</div>
						)}

						<div className="flex flex-col gap-[8px]">
							<div className="flex items-center gap-[16px]">
								<label
									htmlFor="banner-start-date"
									className="w-[126px] text-[16px] font-bold text-[var(--brand-gray1-100)] text-right shrink-0"
								>
									開始時間
								</label>
								<div className="flex items-center gap-[8px]">
									<input
										id="banner-start-date"
										type="date"
										value={startDate}
										onChange={(e) => setStartDate(e.target.value)}
										className="h-[36px] bg-[#f3f3f5] rounded-[8px] px-[12px] text-[16px] text-[var(--brand-gray1-100)] leading-[24px] outline-none w-[200px]"
									/>
									<label
										htmlFor="banner-end-date"
										className="text-[16px] font-bold text-[var(--brand-gray1-100)] shrink-0"
									>
										結束時間
									</label>
									<input
										id="banner-end-date"
										type="date"
										value={endDate}
										onChange={(e) => setEndDate(e.target.value)}
										disabled={noExpiry}
										className="h-[36px] bg-[#f3f3f5] rounded-[8px] px-[12px] text-[16px] text-[var(--brand-gray1-100)] leading-[24px] outline-none w-[200px] disabled:opacity-50"
									/>
								</div>
							</div>
							<label className="flex items-center gap-[8px] ml-[142px] w-fit cursor-pointer">
								<input
									type="checkbox"
									checked={noExpiry}
									onChange={(e) => {
										setNoExpiry(e.target.checked);
										if (e.target.checked) setEndDate("");
									}}
									className="w-[16px] h-[16px] rounded-[4px] border-[1.4px] border-[var(--admin-primary-strong)] accent-[var(--admin-primary-strong)]"
								/>
								<span className="text-[16px] text-[var(--brand-gray1-100)] leading-[24px]">
									無期限
								</span>
							</label>
							<div className="ml-[142px]">
								<FieldErrorText
									message={errorOf("banner.start_time", "banner.end_time")}
								/>
							</div>
						</div>
					</div>

					{latest ? (
						<VersionConflictNotice
							title="這則 Banner 已被其他人修改"
							busy={saving}
							onOverwrite={() => {
								const etagOfLatest = latest.etag;
								setLatest(null);
								void handleSave(etagOfLatest);
							}}
							onUseLatest={() => {
								fillForm(latest);
								setLatest(null);
							}}
						>
							<ConflictRow label="標題" value={latest.title} />
							<ConflictRow
								label="點擊動作"
								value={(() => {
									const a = actionFromProto(latest.action);
									return a.actionType === "url"
										? `${actionLabel.url}：${a.actionUrl}`
										: a.actionType === "popup"
											? actionLabel.popup
											: `${actionLabel[a.actionType]}：${a.actionId}`;
								})()}
							/>
							<ConflictRow label="按鈕文字" value={latest.buttonText || "-"} />
							<ConflictRow
								label="期間"
								value={`${startDateOf(latest.startTime) || "立即"} ～ ${endDateOf(latest.endTime) || "無期限"}`}
							/>
						</VersionConflictNotice>
					) : null}
					{formError && !latest ? (
						<p
							role="alert"
							className="text-[14px] text-[var(--hk-danger)] text-right"
						>
							{formError}
						</p>
					) : null}

					<div className="flex items-center justify-end gap-[20px] w-full">
						<button
							type="button"
							onClick={() => router.push("/admin/banners")}
							className="w-[84px] h-[36px] rounded-[8px] border border-[var(--brand-gray1-50)] text-[16px] text-[var(--brand-gray1-50)] text-center"
						>
							取消
						</button>
						<button
							type="button"
							onClick={() => void handleSave()}
							disabled={saving || !canSubmit || latest !== null}
							className="w-[84px] h-[36px] rounded-[8px] bg-[var(--color)] text-[16px] font-bold text-white text-center disabled:opacity-50 transition-opacity"
						>
							{uploading ? "上傳中..." : saving ? "儲存中..." : "確定發佈"}
						</button>
					</div>
				</div>
			</div>
		</div>
	);
};

export default BannerEditorClient;
