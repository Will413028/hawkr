"use client";

import { type FormHTMLAttributes, type ReactNode, useId, useRef } from "react";
import { createPortal } from "react-dom";
import {
	ModalParentContext,
	useEscapeKey,
	useFocusTrap,
	useModalStack,
} from "@/lib/a11y";
import "./Dialog.css";
import {
	MaterialSymbol,
	type MaterialSymbolName,
} from "@hawkr/icons/material-symbol";
import { ToastRegions } from "./ToastRegions";

/**
 * 後台唯一的對話框元件：overlay、面板、標題列、內容、動作列，加上 WAI-ARIA APG modal dialog
 * 要求的全部行為（role／aria-modal／aria-labelledby、focus trap、Escape、背景 inert、關閉後還焦）。
 * 各呼叫端不得自己放 overlay 或 `role="dialog"`（`tests/dialog-skeleton-guard.test.ts` 會擋）。
 *
 * ## 為什麼是 React 元件加 `@/lib/a11y` 的 hooks，不是原生 `<dialog>.showModal()`
 *
 * 原生 `<dialog>` 的 top layer、`::backdrop` 與預設樣式和現有兩套 CSS（`.modal-*`、
 * `.adm-operation-dialog-*`）的層級與外觀不同，換過去就是視覺改動；JSDOM 也沒有 `showModal()`，
 * 行為測試無從跑。所以沿用已被測試的 hooks，結構與 class 與遷移前逐字相同。
 *
 * ## Portal 是唯一行為
 *
 * overlay 一律掛到 `document.body`（頂層），祖先的 `overflow`、`transform`、`z-index`／stacking context
 * （例如表格的 sticky 儲存格）都裁不到、壓不住它。沒有 `portal` 開關：業界的 modal 都是這樣，就地渲染
 * 只在對話框依賴祖先限定的 CSS 時才有差，而本目錄沒有這種對話框（逐一核對過，見 Dialog 測試的樣式不依賴祖先檢查）。
 * 新對話框的樣式不得寫成 `.某頁 .modal-*`。
 *
 * ## 字型與文字色：Dialog 自己定，不繼承
 *
 * portal 之後 overlay 不在頁面根底下，頁面根（`.post-mgmt-page`、`.hcal-page`、`.certification-detail-page`）
 * 設的 `font-family`／`color` 不會傳進來。overlay 根（`.modal-overlay`、`.adm-operation-dialog-overlay`）
 * 明確設與 body 相同的字型、字級、行高與文字色（皆取自 `AdminVisualRefresh.css` 的 token），
 * 對話框的外觀就與它從哪一頁開啟無關。不要在頁面補償。
 *
 * ## 堆疊：只有最上層的對話框處理 Escape 與 focus trap
 *
 * 每個掛載中的 Dialog 登記進 `@/lib/a11y` 的堆疊（`useModalStack`）；Escape 與 Tab 循環只在自己是最上層時才處理，
 * 所以對話框上再開對話框時，Escape 只關最上層。堆疊只有一份；同一個 commit 內巢狀掛載時（子層 effect 先跑），靠 `ModalParentContext` 的外層 id 把外層排在後代之下。背景 inert 也由同一份堆疊推導（只對最上層套用，上層會把下層的 overlay 也設為 inert），任意順序關閉或一起卸載都會正確還原。
 *
 * ## 用法：掛載即開啟
 *
 * 呼叫端用條件渲染決定開關（`{open && <Dialog …/>}`），元件掛載時開始 trap／inert、卸載時還原。
 *
 * ## 變體（外觀並存是既有的視覺，不是元件的選擇）
 *
 * - `standard`：`.modal-*`，商家治理與一般管理頁。標題 `h3`，內容 `.modal-body`，動作列 `.modal-footer`。
 * - `operation`：`.adm-operation-dialog-*`，代行、發票、認證退回。標題 `h2`（可帶圖示）與說明 `p`，
 *   動作列 `.adm-operation-dialog-actions`。
 * - `side`：`standard` 的側欄版（右側滑出的整高面板），樣式在 `AdminVisualRefresh.css` 的 `.modal-*--side`。
 *
 * ## 結構（standard、operation）
 *
 * 面板限高並排成直欄，標題列與動作列固定，只有內容區捲動（CSS：`AdminVisualRefresh.css`、
 * `Dialog.css`）。有 `form` 時，內容與動作列一起包在 `<form noValidate>` 裡。
 */

interface DialogBaseProps {
	/** 關閉。`busy` 為真時 Escape 與標題列的關閉鍵都不會呼叫它。 */
	onClose: () => void;
	/** 送出中：擋下 Escape 與標題列的關閉鍵，關閉鍵 disabled。預設 false。 */
	busy?: boolean;
	/** `aria-busy` 的值，預設跟 `busy`。載入中但仍可關閉時單獨傳。 */
	ariaBusy?: boolean;
	/** 面板額外的 class。 */
	className?: string;
	children: ReactNode;
}

interface DialogChromeProps extends DialogBaseProps {
	title: ReactNode;
	/** 標題列關閉鍵的 `aria-label`，預設「關閉」。 */
	closeLabel?: string;
	/** 標題色調：`warning` 用 `--admin-warning`（不可逆或會立即生效的確認）。預設沿用標題的一般色。 */
	tone?: "default" | "warning";
	/** 不出標題列的關閉鍵（結果畫面只留動作列的按鈕）。Escape 仍照 `busy` 關閉。 */
	hideClose?: boolean;
	/**
	 * 內容與動作列外面包 `<form noValidate>`。`onSubmit` 之外的屬性（method、action）也在這裡傳。
	 * `noValidate` 固定開啟：必填由各對話框自己驗，原生氣泡會搶在 submit 之前出現。
	 */
	form?: Omit<FormHTMLAttributes<HTMLFormElement>, "children" | "noValidate">;
	/** 動作列內容；省略就不出動作列。 */
	footer?: ReactNode;
}

export interface StandardDialogProps extends DialogChromeProps {
	/**
	 * `standard` 置中的對話框；`side` 同樣的標題列／內容／動作列，改成從右側滑出的整高側欄
	 * （清單旁的唯讀檢視、長表單）。兩者的 a11y 與結構完全相同，只差 overlay 與面板的 modifier class。
	 */
	variant?: "standard" | "side";
	/** 面板的 `max-width`（px 或 CSS 長度），預設用 `.modal-content` 的 480px。`side` 的寬度由 CSS 決定，不傳。 */
	maxWidth?: number | string;
	/** 內容裡說明段落的 id，成為面板的 `aria-describedby`（standard 沒有標題下的說明列）。 */
	describedBy?: string;
}

export interface OperationDialogProps extends DialogChromeProps {
	variant: "operation";
	/** 標題前的圖示（裝飾用）。 */
	titleIcon?: MaterialSymbolName;
	/** 標題下方的說明，並成為面板的 `aria-describedby`。 */
	description?: ReactNode;
	/** 面板寬度：預設 680px；`revoke`、`preflight`（780px）、`audit` 是 `Dialog.css` 的修飾 class。 */
	size?: "revoke" | "preflight" | "audit";
	/** 動作列額外的 class（例如 `adm-operation-dialog-actions--preflight`）。 */
	footerClassName?: string;
}

export type DialogProps = StandardDialogProps | OperationDialogProps;

function DialogPortal({
	children,
	modalId,
}: {
	children: ReactNode;
	modalId: object;
}) {
	// 沒有 document（SSR、renderToStaticMarkup）時就地渲染而不是回 null，否則 markup 測試只會驗到空殼。
	// 對話框都是使用者互動後才掛載，真正的 SSR／hydration 渲染不到它們。
	// 裡面再開的 Dialog 靠這個 context 知道外層是誰（見 `ModalParentContext`）。
	const scoped = (
		<ModalParentContext.Provider value={modalId}>
			{children}
		</ModalParentContext.Provider>
	);
	if (typeof document === "undefined") return scoped;
	return createPortal(scoped, document.body);
}

function classNames(...parts: (string | false | undefined)[]) {
	return parts.filter(Boolean).join(" ");
}

export default function Dialog(props: DialogProps) {
	const { onClose, busy = false, children } = props;
	const ariaBusy = props.ariaBusy ?? busy;
	const overlayRef = useRef<HTMLDivElement>(null);
	const dialogRef = useRef<HTMLDivElement>(null);
	const titleId = useId();
	const descriptionId = useId();

	// 先登記進堆疊（同時負責背景隔離），Escape 與 focus trap 才問得到自己是不是最上層。
	const { isTopmost, id: modalId } = useModalStack(overlayRef, true);
	useEscapeKey(
		() => {
			if (!busy) onClose();
		},
		true,
		isTopmost,
	);
	useFocusTrap(dialogRef, true, isTopmost);

	const requestClose = () => {
		if (!busy) onClose();
	};
	const closeLabel = props.closeLabel ?? "關閉";

	if (props.variant === "operation") {
		const hasDescription = props.description !== undefined;
		const body = <div className="adm-operation-dialog-body">{children}</div>;
		const footer =
			props.footer === undefined ? null : (
				<div
					className={classNames(
						"adm-operation-dialog-actions",
						props.footerClassName,
					)}
				>
					{props.footer}
				</div>
			);
		return (
			<DialogPortal modalId={modalId}>
				<div ref={overlayRef} className="adm-operation-dialog-overlay">
					<div
						ref={dialogRef}
						className={classNames(
							"adm-operation-dialog",
							props.size && `adm-operation-dialog--${props.size}`,
							props.className,
						)}
						aria-labelledby={titleId}
						aria-describedby={hasDescription ? descriptionId : undefined}
						role="dialog"
						aria-modal="true"
						aria-busy={ariaBusy}
						tabIndex={-1}
					>
						<div
							className={classNames(
								"adm-operation-dialog-header",
								props.tone === "warning" &&
									"adm-operation-dialog-header--warning",
							)}
						>
							<div>
								<h2 id={titleId}>
									{props.titleIcon && <MaterialSymbol name={props.titleIcon} />}
									{props.title}
								</h2>
								{hasDescription && (
									<p id={descriptionId}>{props.description}</p>
								)}
							</div>
							{!props.hideClose && (
								<button
									type="button"
									className="adm-operation-dialog-close"
									onClick={requestClose}
									disabled={busy}
									aria-label={closeLabel}
								>
									<MaterialSymbol name="close" />
								</button>
							)}
						</div>
						{props.form ? (
							<form {...props.form} noValidate>
								{body}
								{footer}
							</form>
						) : (
							<>
								{body}
								{footer}
							</>
						)}
						<ToastRegions />
					</div>
				</div>
			</DialogPortal>
		);
	}

	const body = <div className="modal-body">{children}</div>;
	const footer =
		props.footer === undefined ? null : (
			<div className="modal-footer">{props.footer}</div>
		);
	return (
		<DialogPortal modalId={modalId}>
			<div
				ref={overlayRef}
				className={classNames(
					"modal-overlay",
					props.variant === "side" && "modal-overlay--side",
				)}
			>
				<div
					ref={dialogRef}
					className={classNames(
						"modal-content",
						props.variant === "side" && "modal-content--side",
						props.className,
					)}
					style={
						props.maxWidth === undefined
							? undefined
							: { maxWidth: props.maxWidth }
					}
					aria-labelledby={titleId}
					aria-describedby={props.describedBy}
					role="dialog"
					aria-modal="true"
					aria-busy={ariaBusy}
					tabIndex={-1}
				>
					<div
						className={classNames(
							"modal-header",
							props.tone === "warning" && "modal-header--warning",
						)}
					>
						<h3 id={titleId}>{props.title}</h3>
						{!props.hideClose && (
							<button
								type="button"
								className="modal-close"
								onClick={requestClose}
								disabled={busy}
								aria-label={closeLabel}
							>
								<MaterialSymbol name="close" />
							</button>
						)}
					</div>
					{props.form ? (
						<form {...props.form} noValidate>
							{body}
							{footer}
						</form>
					) : (
						<>
							{body}
							{footer}
						</>
					)}
					<ToastRegions />
				</div>
			</div>
		</DialogPortal>
	);
}
