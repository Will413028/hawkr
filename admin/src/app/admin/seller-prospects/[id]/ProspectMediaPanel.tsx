"use client";

import { useMutation } from "@connectrpc/connect-query";
import {
	type Prospect,
	SellerProspectAdminService,
} from "@hawkr/api/gen/hawkr/admin/seller/v1alpha1/prospect_pb";
import { type FormEvent, useRef, useState } from "react";
import { Button, useToast } from "@/components/UI";
import { runAction } from "@/lib/connect-action";
import {
	normalizeSourceUrl,
	type ProspectMediaKind,
	prospectImageUpdate,
	SOURCE_URL_MAX_LENGTH,
} from "@/lib/seller-prospect-media";
import { stageProspectImage } from "@/services/seller-prospect-media";
import ProspectBrandImage from "../ProspectBrandImage";
import { prepareProspectImage } from "./prepareProspectImage";
import "@/styles/pages/SellerProspectMedia.css";

interface Selection {
	file: File;
	preview: string;
}

const UPLOAD_FAILED_TEXT = "圖片上傳失敗，請重新選擇圖片後再試。";
const SAVE_FAILED_TEXT = "圖片上傳失敗，尚未儲存。請稍後重試。";
const CONFLICT_TEXT =
	"資料已由其他操作更新（文字或圖片）。畫面已重新讀取，請確認最新資料後再重新選擇圖片。";

function previewDataUrl(file: File): Promise<string> {
	return new Promise((resolve, reject) => {
		const reader = new FileReader();
		reader.onload = () =>
			typeof reader.result === "string"
				? resolve(reader.result)
				: reject(new Error("無法讀取圖片預覽。"));
		reader.onerror = () => reject(new Error("無法讀取圖片預覽。"));
		reader.readAsDataURL(file);
	});
}

/**
 * 品牌圖片：選圖、預覽、上傳到 media 後以 `UpdateProspect` 換圖（mask 只列 `logo_upload` 或 `cover_upload`）。
 * 與文字表單共用頁面持有的 etag。
 */
export default function ProspectMediaPanel({
	prospect,
	onUpdated,
	reload,
	readOnly = false,
}: {
	prospect: Prospect;
	onUpdated: (prospect: Prospect) => void;
	reload: () => Promise<unknown>;
	/** 非進行中的業者：目前的圖片與來源照常顯示，選圖、來源網址與儲存停用。 */
	readOnly?: boolean;
}) {
	const { showToast } = useToast();
	const update = useMutation(SellerProspectAdminService.method.updateProspect);
	const [kind, setKind] = useState<ProspectMediaKind>("logo");
	const [selection, setSelection] = useState<Selection | null>(null);
	const [sourceUrl, setSourceUrl] = useState("");
	const [busy, setBusy] = useState(false);
	const [message, setMessage] = useState("");
	const [error, setError] = useState("");
	const [sourceInvalid, setSourceInvalid] = useState(false);
	const fileInput = useRef<HTMLInputElement>(null);
	const sourceInput = useRef<HTMLInputElement>(null);
	const disabled = busy || readOnly;
	const label = kind === "logo" ? "Logo／大頭貼" : "背景封面";

	const chooseFile = async (file?: File) => {
		if (!file || disabled) return;
		setBusy(true);
		setError("");
		setMessage("正在準備圖片預覽…");
		setSelection(null);
		try {
			const prepared = await prepareProspectImage(file, kind);
			setSelection({
				file: prepared,
				preview: await previewDataUrl(prepared),
			});
			setMessage("圖片已備妥，尚未儲存。請確認預覽與來源頁面。");
		} catch (caught) {
			setError(
				caught instanceof Error ? caught.message : "圖片處理失敗，請重新選擇。",
			);
			setMessage("");
		} finally {
			setBusy(false);
		}
	};

	const save = async (event: FormEvent) => {
		event.preventDefault();
		if (disabled) return;
		setError("");
		setMessage("");
		if (!selection) {
			setError("請先選擇要儲存的圖片。");
			fileInput.current?.focus();
			return;
		}
		const source = sourceUrl.trim() === "" ? "" : normalizeSourceUrl(sourceUrl);
		if (source === null) {
			setSourceInvalid(true);
			setError("來源頁面網址必須是 https 開頭的公開網址。");
			sourceInput.current?.focus();
			return;
		}
		setSourceInvalid(false);
		setBusy(true);
		try {
			setMessage("正在上傳圖片…");
			let stagedToken: string;
			try {
				stagedToken = await stageProspectImage(selection.file);
			} catch {
				setMessage("");
				setError(UPLOAD_FAILED_TEXT);
				return;
			}
			setMessage("正在儲存品牌圖片…");
			const result = await runAction(
				() =>
					update.mutateAsync(
						prospectImageUpdate({
							id: prospect.id,
							etag: prospect.etag,
							kind,
							stagedToken,
							sourceUrl: source,
						}),
					),
				{
					reload,
					fallback: SAVE_FAILED_TEXT,
					conflictText: CONFLICT_TEXT,
				},
			);
			setMessage("");
			if (!result.ok) {
				if (result.kind === "conflict") {
					setSelection(null);
					if (fileInput.current) fileInput.current.value = "";
				}
				setError(result.text);
				return;
			}
			if (result.value.prospect) onUpdated(result.value.prospect);
			setSelection(null);
			setSourceUrl("");
			if (fileInput.current) fileInput.current.value = "";
			showToast(kind === "logo" ? "已更新 logo" : "已更新封面", "success");
		} finally {
			setBusy(false);
		}
	};

	return (
		<section
			className="card prospect-media"
			aria-labelledby="prospect-media-title"
		>
			<div className="card-header">
				<h2 id="prospect-media-title">品牌圖片</h2>
			</div>
			<div className="card-body prospect-media__layout">
				<div>
					<p className="prospect-media__caption">
						{selection ? "待儲存預覽" : "目前建檔圖片"} · 後台資料
					</p>
					<div className="prospect-media__preview">
						<ProspectBrandImage
							className="prospect-media__cover"
							image={prospect.coverImage}
							preset="banner"
							preview={kind === "cover" ? selection?.preview : undefined}
							alt={`${prospect.displayName}背景封面`}
							empty="尚未補齊背景封面"
						/>
						<div className="prospect-media__identity">
							<ProspectBrandImage
								className="prospect-media__logo"
								image={prospect.logoImage}
								preset="avatar"
								preview={kind === "logo" ? selection?.preview : undefined}
								alt={`${prospect.displayName} Logo`}
								empty="尚無 Logo"
							/>
							<div>
								<strong>{prospect.displayName}</strong>
								<p>
									{[prospect.region, prospect.category]
										.filter(Boolean)
										.join(" · ") || "尚未補齊地區與品類"}
								</p>
							</div>
						</div>
					</div>
					<p className="prospect-media__caption">
						此處儲存業者圖片；買家公開顯示仍需另行啟用。調研備註不放入名片預覽。
					</p>
					<dl className="prospect-media__sources">
						{(["logo", "cover"] as const).map((slot) => {
							const url = normalizeSourceUrl(prospect[`${slot}SourceUrl`]);
							return (
								<div key={slot}>
									<dt>{slot === "logo" ? "Logo 來源" : "封面來源"}</dt>
									<dd>
										{url ? (
											<a href={url} target="_blank" rel="noreferrer">
												查看來源頁面
												<span className="sr-only">（另開分頁）</span>
											</a>
										) : (
											"尚未記錄"
										)}
									</dd>
								</div>
							);
						})}
					</dl>
				</div>
				<form
					onSubmit={save}
					noValidate
					className="prospect-media__form"
					aria-busy={busy}
				>
					<label htmlFor="prospect-media-kind">圖片類型</label>
					<select
						id="prospect-media-kind"
						value={kind}
						disabled={disabled}
						onChange={(event) => {
							setKind(event.target.value as ProspectMediaKind);
							setSelection(null);
							setSourceUrl("");
							setMessage("");
							setError("");
							setSourceInvalid(false);
							if (fileInput.current) fileInput.current.value = "";
						}}
					>
						<option value="logo">Logo／大頭貼</option>
						<option value="cover">背景封面</option>
					</select>
					<label htmlFor="prospect-media-file">選擇{label}</label>
					<input
						ref={fileInput}
						id="prospect-media-file"
						type="file"
						accept="image/png,image/jpeg,image/webp"
						disabled={disabled}
						aria-describedby="prospect-media-file-hint"
						onChange={(event) => {
							const file = event.target.files?.[0];
							event.target.value = "";
							void chooseFile(file);
						}}
					/>
					<p id="prospect-media-file-hint" className="prospect-media__hint">
						JPG、PNG 或 WebP，最大 5 MB。
						{kind === "cover"
							? "封面會置中裁切成 4:1，請確認左側預覽。"
							: "Logo 保留完整圖像比例。"}
					</p>
					<label htmlFor="prospect-media-source">
						圖片來源頁面 <span>（選填）</span>
					</label>
					<input
						ref={sourceInput}
						id="prospect-media-source"
						type="url"
						value={sourceUrl}
						maxLength={SOURCE_URL_MAX_LENGTH}
						disabled={disabled}
						aria-invalid={sourceInvalid || undefined}
						aria-describedby={
							sourceInvalid
								? "prospect-media-source-hint prospect-media-error"
								: "prospect-media-source-hint"
						}
						onChange={(event) => {
							setSourceUrl(event.target.value);
							setSourceInvalid(false);
						}}
						placeholder="https://品牌官網/來源頁面"
					/>
					<p id="prospect-media-source-hint" className="prospect-media__hint">
						請上傳品牌提供或允許使用的素材；有來源頁面就一併記錄。網址只作來源紀錄，不會自動擷取圖片。
					</p>
					<p role="status" className="prospect-media__status">
						{message}
					</p>
					{error && (
						<p
							role="alert"
							id="prospect-media-error"
							className="prospect-media__error"
						>
							{error}
						</p>
					)}
					<div className="prospect-media__actions">
						<Button type="submit" disabled={disabled} loading={busy}>
							{busy ? "處理中…" : `儲存${label}`}
						</Button>
					</div>
				</form>
			</div>
		</section>
	);
}
