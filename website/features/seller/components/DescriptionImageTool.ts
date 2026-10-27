import type {
	API,
	BlockAPI,
	BlockTool,
	BlockToolConstructorOptions,
	PasteEvent,
} from "@editorjs/editorjs";

export interface DescriptionImageData {
	file?: {
		url?: string;
		imageKey?: string;
		assetId?: string;
		stagedToken?: string;
	};
	caption?: string;
	[key: string]: unknown;
}

export interface DescriptionImageToolConfig {
	upload: (
		file: File,
		onProgress: (progress: number) => void,
	) => Promise<{ url: string; stagedToken?: string }>;
	replace: (
		id: string,
		data: DescriptionImageData,
		isCurrent: () => boolean,
	) => Promise<void>;
	remove: (id: string) => void;
	iconMarkup: string;
}

function imageUrl(data: DescriptionImageData): string | null {
	const url = data.file?.url;
	return typeof url === "string" &&
		url.trim() &&
		url !== "__pending__" &&
		url !== "__error__"
		? url
		: null;
}

function copyData(data: DescriptionImageData): DescriptionImageData {
	return { ...data, ...(data.file ? { file: { ...data.file } } : {}) };
}

/** Captions are retained for legacy data/alt only; never inserted into live DOM. */
function altText(caption: string | undefined): string {
	if (!caption) return "商品圖片";
	const template = document.createElement("template");
	template.innerHTML = caption;
	return template.content.textContent?.trim() || "商品圖片";
}

/**
 * Native Editor.js image block. Uploads are a draft until the owner commits a
 * same-id replacement; save() retains the original reference throughout a
 * pending/failed request. No private @editorjs/image APIs or caption editor.
 */
export default class DescriptionImageTool implements BlockTool {
	private data: DescriptionImageData;
	private readonly config: DescriptionImageToolConfig;
	private readonly api: API;
	private readonly block: BlockAPI;
	private readonly readOnly: boolean;
	private readonly events = new AbortController();
	private removalObserver: MutationObserver | null = null;
	private root: HTMLElement | null = null;
	private input: HTMLInputElement | null = null;
	private choose: HTMLButtonElement | null = null;
	private media: HTMLDivElement | null = null;
	private picture: HTMLImageElement | null = null;
	private actions: HTMLDivElement | null = null;
	private replaceButton: HTMLButtonElement | null = null;
	private status: HTMLDivElement | null = null;
	private progress: HTMLProgressElement | null = null;
	private fileName: HTMLParagraphElement | null = null;
	private alert: HTMLParagraphElement | null = null;
	private retry: HTMLButtonElement | null = null;
	private reload: HTMLButtonElement | null = null;
	private selectedFile: File | null = null;
	private previewUrl: string | null = null;
	private errorMessage = "";
	private imageLoadFailed = false;
	private uploading = false;
	private progressValue = 0;
	private destroyed = false;
	private requestToken = 0;

	constructor({
		data,
		config,
		api,
		block,
		readOnly,
	}: BlockToolConstructorOptions<
		DescriptionImageData,
		DescriptionImageToolConfig
	>) {
		if (!config)
			throw new Error(
				"DescriptionImageTool requires upload and history callbacks",
			);
		this.data = copyData(data);
		this.config = config;
		this.api = api;
		this.block = block;
		this.readOnly = readOnly;
	}

	static get toolbox() {
		// The production registration supplies the shared AppIcon image glyph.
		return { title: "圖片", icon: "" };
	}

	static get isReadOnlySupported() {
		return true;
	}

	static get conversionConfig() {
		return {
			import: (caption: string): DescriptionImageData => ({ caption }),
			export: (data: DescriptionImageData): string =>
				typeof data.caption === "string" ? data.caption : "",
		};
	}

	static get pasteConfig() {
		return { files: { mimeTypes: ["image/*"] } };
	}

	static get sanitize() {
		return { file: true, caption: true };
	}

	render(): HTMLElement {
		if (this.root) return this.root;
		const root = document.createElement("div");
		root.className = "hk-description-image image-tool";
		root.dataset.readonly = String(this.readOnly);
		this.root = root;
		this.media = document.createElement("div");
		this.media.className =
			"hk-description-image__preview hk-description-image__media image-tool__image";
		this.picture = document.createElement("img");
		this.picture.className =
			"hk-description-image__picture image-tool__image-picture";
		this.picture.alt = altText(this.data.caption);
		this.picture.draggable = false;
		this.picture.decoding = "async";
		this.picture.addEventListener(
			"load",
			() => {
				if (this.destroyed) return;
				this.imageLoadFailed = false;
				this.updateView();
			},
			{ signal: this.events.signal },
		);
		this.picture.addEventListener(
			"error",
			() => {
				if (this.destroyed || this.uploading) return;
				this.imageLoadFailed = true;
				this.updateView();
			},
			{ signal: this.events.signal },
		);
		this.media.append(this.picture);
		root.append(this.media);

		if (!this.readOnly) {
			this.input = document.createElement("input");
			this.input.type = "file";
			this.input.accept = "image/*";
			this.input.hidden = true;
			this.input.className = "hk-description-image__input";
			this.input.setAttribute("aria-label", "選擇商品圖片");
			this.input.addEventListener(
				"change",
				() => {
					const files = Array.from(this.input?.files ?? []);
					if (this.input) this.input.value = "";
					if (files.length) this.acceptFiles(files);
				},
				{ signal: this.events.signal },
			);
			root.append(this.input);

			this.choose = this.button("hk-description-image__choose", "", () =>
				this.input?.click(),
			);
			const icon = document.createElement("span");
			icon.className = "hk-description-image__icon";
			icon.setAttribute("aria-hidden", "true");
			// Source-owned SVG markup supplied by the editor, never uploaded data.
			icon.innerHTML = this.config.iconMarkup;
			this.choose.append(
				icon,
				this.text("span", "hk-description-image__title", "加入商品圖片"),
				this.text("span", "hk-description-image__hint", "將圖片拖曳到這裡"),
				this.text("span", "hk-description-image__cta", "選擇圖片"),
				this.text(
					"span",
					"hk-description-image__support",
					"支援圖片檔案・10 MB 以下",
				),
			);
			root.append(this.choose);

			this.actions = document.createElement("div");
			this.actions.className = "hk-description-image__actions";
			this.fileName = this.text("p", "hk-description-image__file-name", "");
			this.replaceButton = this.button(
				"hk-description-image__replace",
				"更換圖片",
				() => this.input?.click(),
			);
			const remove = this.button(
				"hk-description-image__remove",
				"移除圖片",
				() => {
					this.invalidateRequest();
					this.config.remove(this.block.id);
				},
			);
			this.actions.append(this.fileName, this.replaceButton, remove);
			root.append(this.actions);

			this.status = this.text(
				"div",
				"hk-description-image__status",
				"正在上傳圖片…",
			);
			this.status.setAttribute("role", "status");
			this.progress = document.createElement("progress");
			this.progress.className = "hk-description-image__progress";
			this.progress.max = 100;
			this.progress.value = 0;
			this.progress.setAttribute("aria-label", "圖片上傳進度");
			this.alert = this.text("p", "hk-description-image__error", "");
			this.alert.setAttribute("role", "alert");
			this.retry = this.button(
				"hk-description-image__retry",
				"重試上傳",
				() => {
					if (this.selectedFile) void this.uploadFile(this.selectedFile);
				},
			);
			this.reload = this.button(
				"hk-description-image__reload",
				"重新載入圖片",
				() => {
					this.imageLoadFailed = false;
					this.picture?.removeAttribute("src");
					this.updateView();
				},
			);
			root.append(
				this.status,
				this.progress,
				this.alert,
				this.retry,
				this.reload,
			);
			root.addEventListener(
				"dragover",
				(event) => {
					if (!event.dataTransfer?.types.includes("Files")) return;
					event.preventDefault();
					event.stopPropagation();
					event.dataTransfer.dropEffect = this.uploading ? "none" : "copy";
					root.dataset.dragover = String(!this.uploading);
				},
				{ signal: this.events.signal, capture: true },
			);
			root.addEventListener(
				"dragleave",
				(event) => {
					if (
						!(event.relatedTarget instanceof Node) ||
						!root.contains(event.relatedTarget)
					)
						delete root.dataset.dragover;
				},
				{ signal: this.events.signal },
			);
			root.addEventListener(
				"drop",
				(event) => {
					if (!event.dataTransfer?.types.includes("Files")) return;
					event.preventDefault();
					event.stopPropagation();
					delete root.dataset.dragover;
					this.acceptFiles(Array.from(event.dataTransfer.files));
				},
				{ signal: this.events.signal, capture: true },
			);
		}
		this.updateView();
		return root;
	}

	save(): DescriptionImageData {
		return copyData(this.data);
	}

	/** blocks.update replaces the holder without invoking the old destroy hook. */
	rendered(): void {
		this.removalObserver?.disconnect();
		const parent = this.root?.closest(".ce-block")?.parentElement;
		if (!parent || this.destroyed) return;
		this.removalObserver = new MutationObserver(() => {
			if (!this.ownsCurrentBlock()) this.destroy();
		});
		// Reordering may remove/reinsert a holder in one operation. Inspect the
		// final connection in the callback; observe only direct block children.
		this.removalObserver.observe(parent, { childList: true });
	}

	validate(data: DescriptionImageData): boolean {
		return imageUrl(data) !== null;
	}

	onPaste(event: PasteEvent): void {
		if (this.readOnly || this.destroyed || !("file" in event.detail)) return;
		this.acceptFiles([event.detail.file]);
	}

	/** Removal also invalidates in-flight uploads before Editor.js destroys us. */
	removed(): void {
		this.destroy();
	}

	destroy(): void {
		if (this.destroyed) return;
		this.destroyed = true;
		this.invalidateRequest();
		this.events.abort();
		this.removalObserver?.disconnect();
		this.removalObserver = null;
		this.selectedFile = null;
	}

	private text<K extends keyof HTMLElementTagNameMap>(
		tag: K,
		className: string,
		content: string,
	): HTMLElementTagNameMap[K] {
		const element = document.createElement(tag);
		element.className = className;
		element.textContent = content;
		return element;
	}

	private button(
		className: string,
		label: string,
		action: () => void,
	): HTMLButtonElement {
		const button = this.text("button", className, label);
		button.type = "button";
		button.addEventListener(
			"click",
			(event) => {
				event.preventDefault();
				event.stopPropagation();
				if (!this.destroyed && !this.readOnly) action();
			},
			{ signal: this.events.signal },
		);
		return button;
	}

	private acceptFiles(files: File[]): void {
		if (this.destroyed || this.readOnly || this.uploading || files.length === 0)
			return;
		if (files.length !== 1 || !files[0].type.startsWith("image/")) {
			this.errorMessage =
				files.length > 1
					? "一次只能加入一張圖片，請重新選擇。"
					: "請選擇圖片檔案。";
			this.selectedFile = null;
			this.updateView();
			return;
		}
		// Matches upload.ts DEFAULT_MAX_BYTES. Description images use
		// compress:false, so the original file must fit the 10 MiB boundary.
		if (files[0].size > 10 * 1024 * 1024) {
			this.errorMessage = "圖片不可超過 10 MB，請選擇較小的圖片。";
			this.selectedFile = null;
			this.updateView();
			return;
		}
		void this.uploadFile(files[0]);
	}

	private releasePreview(): void {
		if (this.previewUrl) URL.revokeObjectURL(this.previewUrl);
		this.previewUrl = null;
	}

	private invalidateRequest(): void {
		this.requestToken++;
		this.uploading = false;
		this.releasePreview();
	}

	private ownsCurrentBlock(): boolean {
		if (this.destroyed || !this.root?.isConnected) return false;
		return (
			this.api.blocks.getById(this.block.id)?.holder.contains(this.root) ??
			false
		);
	}

	private async uploadFile(file: File): Promise<void> {
		if (this.destroyed || this.readOnly || this.uploading) return;
		const token = ++this.requestToken;
		const isCurrent = () =>
			this.requestToken === token && this.ownsCurrentBlock();
		this.selectedFile = file;
		this.uploading = true;
		this.progressValue = 0;
		this.errorMessage = "";
		this.imageLoadFailed = false;
		this.releasePreview();
		try {
			this.previewUrl = URL.createObjectURL(file);
		} catch {
			/* Preview is optional; retain the committed image if unavailable. */
		}
		this.updateView();
		try {
			const uploaded = await this.config.upload(file, (progress) => {
				if (!isCurrent() || !Number.isFinite(progress)) return;
				this.progressValue = Math.round(
					Math.min(1, Math.max(0, progress)) * 100,
				);
				this.updateView();
			});
			if (!isCurrent()) return;
			if (!imageUrl({ file: { url: uploaded.url } }))
				throw new Error("Upload returned no image URL");
			// New upload identity replaces the whole old file object. Never carry
			// an old imageKey/assetId alongside the new stagedToken.
			const next: DescriptionImageData = {
				file: {
					url: uploaded.url,
					...(uploaded.stagedToken
						? { stagedToken: uploaded.stagedToken }
						: {}),
				},
				...(typeof this.data.caption === "string"
					? { caption: this.data.caption }
					: {}),
			};
			await this.config.replace(this.block.id, next, isCurrent);
			if (!isCurrent()) return;
			// Keeping this fallback supports an owner that updates the same tool
			// in place; replaced holders are rejected by the current-block guard.
			this.data = next;
			this.uploading = false;
			this.releasePreview();
			this.updateView();
		} catch {
			if (!isCurrent()) return;
			this.uploading = false;
			this.releasePreview();
			this.errorMessage = "圖片上傳失敗，請重試或選擇其他圖片。";
			this.updateView();
		} finally {
			if (!isCurrent()) this.destroy();
		}
	}

	private updateView(): void {
		if (this.destroyed || !this.root || !this.picture || !this.media) return;
		const committedUrl = imageUrl(this.data);
		const shownUrl =
			this.uploading && this.previewUrl ? this.previewUrl : committedUrl;
		const message =
			this.errorMessage ||
			(this.imageLoadFailed ? "圖片暫時無法顯示，請重新載入或更換圖片。" : "");
		this.root.dataset.state = this.uploading
			? "uploading"
			: message
				? "error"
				: committedUrl
					? "filled"
					: "empty";
		this.root.setAttribute("aria-busy", String(this.uploading));
		this.root.hidden = this.readOnly && !committedUrl;
		this.media.hidden = !shownUrl;
		this.picture.hidden =
			this.imageLoadFailed && !this.readOnly && !this.uploading;
		this.picture.alt = altText(this.data.caption);
		if (shownUrl && this.picture.getAttribute("src") !== shownUrl)
			this.picture.src = shownUrl;
		else if (!shownUrl) this.picture.removeAttribute("src");
		if (this.choose) {
			this.choose.hidden = !!committedUrl || this.uploading;
			this.choose.disabled = this.uploading;
		}
		if (this.actions)
			this.actions.hidden = !committedUrl && !this.uploading && !message;
		if (this.replaceButton) this.replaceButton.disabled = this.uploading;
		if (this.status) this.status.hidden = !this.uploading;
		if (this.progress) {
			this.progress.hidden = !this.uploading;
			this.progress.value = this.progressValue;
		}
		if (this.fileName) {
			this.fileName.textContent = this.selectedFile?.name ?? "";
			this.fileName.hidden = !this.selectedFile;
		}
		if (this.alert) {
			this.alert.textContent = this.uploading ? "" : message;
			this.alert.hidden = this.uploading || !message;
		}
		if (this.retry)
			this.retry.hidden =
				this.uploading || !this.errorMessage || !this.selectedFile;
		if (this.reload)
			this.reload.hidden = this.uploading || !this.imageLoadFailed;
	}
}
