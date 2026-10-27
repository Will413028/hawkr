"use client";

/**
 * Three shipment dialogs ported from
 *   hawkr_seller/lib/features/order/presentation/{tcat_reservation,call_tcat,seven_eleven_ship}_dialog.dart
 *
 * Pattern matches LoginRequiredModal: portal-mounted, ESC closes, backdrop
 * click closes, body scroll locked while open. They are state-driven (open /
 * onClose / onSubmit) rather than imperative-promise-style — easier to reason
 * about in React, and the parent already needs local state for "submitting"
 * anyway.
 *
 * Photo upload uses the project's standard staged-token flow
 * (`uploadImageFile` → 3-step signed-URL upload). The dialogs handle the
 * upload internally so the parent's `onSubmit` only sees `stagedToken`.
 */

import * as React from "react";
import { createPortal } from "react-dom";
import AppIcon from "@/components/shared/AppIcon";
import { TruckSpinner } from "@/components/shared/TruckSpinner";
import UploadLoadingIndicator from "@/components/shared/UploadLoadingIndicator";
import Button from "@/components/ui/Button";
import { ImageUploader } from "@/components/ui/ImageUploader";
import { acquireBodyScrollLock } from "@/lib/browser/body-scroll-lock";
import { formatTaipeiShortDate } from "@/lib/date/taipei";
import { uploadImageFile } from "@/lib/media/upload";
import {
	type BusinessDay,
	fetchShippingBusinessDays,
} from "@/lib/seller/shipment";

export const SHIPMENT_PHOTO_TILE_STYLE = {
	height: 220,
	minHeight: 220,
	maxHeight: 220,
} satisfies React.CSSProperties;

const CloseIcon = () => <AppIcon name="close" />;

/* ============================================================
 *        Shared shipping-brand marks + field icons
 *        (exported so the bulk dialog in SellerOrdersPage can
 *         reuse the exact same visual language — 風格統一)
 * ============================================================ */

type ShippingBrand = "tcat" | "711";

const BRAND_LOGO: Record<ShippingBrand, { text: string; alt: string }> = {
	tcat: { text: "黑貓", alt: "黑貓宅急便" },
	711: { text: "7-11", alt: "7-11 店到店" },
};

/** White rounded label chip for the carrier name. */
export function ShippingBrandLogo({
	brand,
	compact = false,
}: {
	brand: ShippingBrand;
	compact?: boolean;
}) {
	const { text, alt } = BRAND_LOGO[brand];
	return (
		<span
			className={`shipment-dialog__logo${compact ? " shipment-dialog__logo--compact" : ""}`}
		>
			<span role="img" aria-label={alt} className="shipment-dialog__logo-text">
				{text}
			</span>
		</span>
	);
}

/** 電話聯絡 */
export const PhoneIcon = ({ className }: { className?: string }) => (
	<AppIcon name="call" className={className} />
);

/** 需要推車 */
export const CartIcon = ({ className }: { className?: string }) => (
	<AppIcon name="buy" className={className} />
);

/** 備註 */
export const NoteIcon = ({ className }: { className?: string }) => (
	<AppIcon name="edit" className={className} />
);

interface DialogShellProps {
	open: boolean;
	onClose: () => void;
	closeDisabled?: boolean;
	titleId: string;
	className?: string;
	children: React.ReactNode;
}

function DialogShell({
	open,
	onClose,
	closeDisabled = false,
	titleId,
	className,
	children,
}: DialogShellProps) {
	const [mounted, setMounted] = React.useState(false);
	React.useEffect(() => {
		setMounted(true);
	}, []);

	React.useEffect(() => {
		if (!open) return;
		const onKey = (e: KeyboardEvent) => {
			if (e.key === "Escape" && !closeDisabled) onClose();
		};
		const releaseScrollLock = acquireBodyScrollLock();
		document.addEventListener("keydown", onKey);
		return () => {
			document.removeEventListener("keydown", onKey);
			releaseScrollLock();
		};
	}, [closeDisabled, onClose, open]);

	if (!open || !mounted) return null;

	return createPortal(
		<div
			className={`shipment-dialog ${className ?? ""}`}
			role="dialog"
			aria-modal="true"
			aria-labelledby={titleId}
		>
			<button
				type="button"
				className="shipment-dialog__backdrop"
				onClick={onClose}
				aria-label="關閉"
				disabled={closeDisabled}
			/>
			<section className="shipment-dialog__panel">
				<button
					type="button"
					className="shipment-dialog__close"
					onClick={onClose}
					aria-label="關閉"
					disabled={closeDisabled}
				>
					<CloseIcon />
				</button>
				{children}
			</section>
		</div>,
		document.body,
	);
}

/* ============================================================
 *                  Photo picker (shared)
 * ============================================================ */

interface PhotoPickerProps {
	preview: string | null;
	uploading: boolean;
	onPick: (file: File) => void;
	disabled?: boolean;
}

function PhotoPicker({
	preview,
	uploading,
	onPick,
	disabled,
}: PhotoPickerProps) {
	return (
		<ImageUploader
			variant="card"
			size="md"
			accept="image/*"
			crop="photo"
			tileStyle={SHIPMENT_PHOTO_TILE_STYLE}
			previewFit="contain"
			actionsVariant="button"
			fileHint="JPG / PNG / WebP · 10 MB 以下"
			value={preview ?? ""}
			uploading={uploading}
			disabled={disabled}
			onPick={onPick}
			hint="上傳產品或包裝照片"
			fullWidth
		/>
	);
}

/* ============================================================
 *               TcatReservationDialog (黑貓預約)
 * ============================================================ */

export interface TcatReservationResult {
	/** yyyyMMdd e.g. "20260512" */
	shipDate: string;
	/** "13時前" / "14-18時" / "不指定" */
	timeSlot: string;
	stagedToken?: string;
	previewUrl?: string;
}

export interface TcatReservationSubmitResult {
	ok: boolean;
	error?: string;
}

interface TcatReservationProps {
	open: boolean;
	onClose: () => void;
	onSubmit: (
		result: TcatReservationResult,
	) => Promise<TcatReservationSubmitResult>;
}

const TIME_SLOTS = ["13時前", "14-18時", "不指定"];

export function TcatReservationDialog({
	open,
	onClose,
	onSubmit,
}: TcatReservationProps) {
	const [days, setDays] = React.useState<BusinessDay[] | null>(null);
	const [loadError, setLoadError] = React.useState<string | null>(null);
	const [selectedDate, setSelectedDate] = React.useState(0);
	const [selectedTime, setSelectedTime] = React.useState(0);
	const [photoPreview, setPhotoPreview] = React.useState<string | null>(null);
	const [stagedToken, setStagedToken] = React.useState<string | undefined>();
	const [previewUrl, setPreviewUrl] = React.useState<string | undefined>();
	const [uploading, setUploading] = React.useState(false);
	const [uploadError, setUploadError] = React.useState<string | null>(null);
	const [submitting, setSubmitting] = React.useState(false);
	const [submitError, setSubmitError] = React.useState<string | null>(null);

	React.useEffect(() => {
		if (!open) return;
		setLoadError(null);
		fetchShippingBusinessDays()
			.then((d) => {
				setDays(d);
				if (d.length === 0) setLoadError("無可選出貨日期");
			})
			.catch(() => setLoadError("載入日期失敗"));
	}, [open]);

	React.useEffect(() => {
		if (!open) {
			// reset state when closed
			setPhotoPreview(null);
			setStagedToken(undefined);
			setPreviewUrl(undefined);
			setUploading(false);
			setUploadError(null);
			setSubmitError(null);
			setSubmitting(false);
			setSelectedDate(0);
			setSelectedTime(0);
		}
	}, [open]);

	const handlePick = async (file: File) => {
		setUploading(true);
		setUploadError(null);
		try {
			const local = URL.createObjectURL(file);
			setPhotoPreview(local);
			const asset = await uploadImageFile(file);
			setStagedToken(asset.stagedToken);
			setPreviewUrl(asset.previewUrl);
		} catch (e) {
			setUploadError(e instanceof Error ? e.message : "上傳失敗");
			setPhotoPreview(null);
			setStagedToken(undefined);
			setPreviewUrl(undefined);
		} finally {
			setUploading(false);
		}
	};

	const canSubmit =
		days !== null && days.length > 0 && !uploading && !submitting;

	const handleSubmit = async () => {
		if (!canSubmit || !days) return;
		const day = days[selectedDate];
		if (!day) return;
		setSubmitting(true);
		setSubmitError(null);
		try {
			const result = await onSubmit({
				shipDate: day.date.replaceAll("-", ""),
				timeSlot: TIME_SLOTS[selectedTime] ?? "不指定",
				stagedToken,
				previewUrl,
			});
			if (!result.ok) {
				setSubmitError(result.error || "預約出貨失敗，請稍後再試");
			}
		} catch (error) {
			setSubmitError(
				error instanceof Error ? error.message : "預約出貨失敗，請稍後再試",
			);
		} finally {
			setSubmitting(false);
		}
	};

	return (
		<DialogShell
			open={open}
			onClose={onClose}
			closeDisabled={submitting}
			titleId="tcat-reservation-title"
			className="shipment-dialog--tcat"
		>
			<header className="shipment-dialog__head">
				<ShippingBrandLogo brand="tcat" />
				<div className="shipment-dialog__head-text">
					<h2 id="tcat-reservation-title">預約黑貓上門取件</h2>
					<small>請選擇期望的取件時間並上傳產品照片</small>
				</div>
			</header>

			<fieldset className="shipment-dialog__field">
				<legend>
					<AppIcon
						name="calendar"
						size={16}
						className="inline-block mr-1 align-[-0.15em]"
					/>
					期望取件日期
				</legend>
				{days === null ? (
					<p className="shipment-dialog__placeholder">載入中…</p>
				) : loadError ? (
					<p className="shipment-dialog__placeholder">{loadError}</p>
				) : (
					<div className="shipment-dialog__chip-row">
						{days.map((d, i) => {
							const dateLabel = formatTaipeiShortDate(d.date);
							const active = selectedDate === i;
							return (
								<button
									key={d.date}
									type="button"
									aria-pressed={active}
									className={`shipment-dialog__chip ${active ? "is-active" : ""}`}
									disabled={submitting}
									onClick={() => setSelectedDate(i)}
								>
									<small>週{d.dayOfWeek}</small>
									<strong>{dateLabel}</strong>
								</button>
							);
						})}
					</div>
				)}
			</fieldset>

			<fieldset className="shipment-dialog__field">
				<legend>
					<AppIcon
						name="clock"
						size={16}
						className="inline-block mr-1 align-[-0.15em]"
					/>
					期望取件時段
				</legend>
				<div className="shipment-dialog__chip-row">
					{TIME_SLOTS.map((slot, i) => {
						const active = selectedTime === i;
						return (
							<button
								key={slot}
								type="button"
								aria-pressed={active}
								className={`shipment-dialog__chip ${active ? "is-active" : ""}`}
								disabled={submitting}
								onClick={() => setSelectedTime(i)}
							>
								<strong>{slot}</strong>
							</button>
						);
					})}
				</div>
			</fieldset>

			<section className="shipment-dialog__field">
				<PhotoPicker
					preview={photoPreview}
					uploading={uploading}
					disabled={submitting}
					onPick={handlePick}
				/>
				{uploadError && (
					<p className="shipment-dialog__error" role="alert">
						{uploadError}
					</p>
				)}
				{submitError && (
					<p className="shipment-dialog__error" role="alert">
						{submitError}
					</p>
				)}
			</section>

			<footer className="shipment-dialog__foot">
				<Button
					className="flex-1"
					onClick={() => void handleSubmit()}
					disabled={!canSubmit}
					isLoading={submitting}
					loadingIndicator={<TruckSpinner size={18} label="預約出貨中" />}
					loadingLabel="預約出貨中…"
				>
					預約出貨
				</Button>
			</footer>
		</DialogShell>
	);
}

/* ============================================================
 *                  CallTcatDialog (黑貓叫件)
 * ============================================================ */

export interface CallTcatParams {
	isContact: "Y" | "N";
	isTrolley: "Y" | "N";
	memo: string;
}

interface CallTcatProps {
	open: boolean;
	onClose: () => void;
	onSubmit: (params: CallTcatParams) => void;
}

export function CallTcatDialog({ open, onClose, onSubmit }: CallTcatProps) {
	const [isContact, setIsContact] = React.useState(false);
	const [isTrolley, setIsTrolley] = React.useState(false);
	const [memo, setMemo] = React.useState("");
	const [submitting, setSubmitting] = React.useState(false);

	React.useEffect(() => {
		if (!open) {
			setIsContact(false);
			setIsTrolley(false);
			setMemo("");
			setSubmitting(false);
		}
	}, [open]);

	const handleSubmit = () => {
		setSubmitting(true);
		onSubmit({
			isContact: isContact ? "Y" : "N",
			isTrolley: isTrolley ? "Y" : "N",
			memo: memo.trim(),
		});
	};

	return (
		<DialogShell
			open={open}
			onClose={onClose}
			titleId="call-tcat-title"
			className="shipment-dialog--call-tcat"
		>
			<header className="shipment-dialog__head">
				<ShippingBrandLogo brand="tcat" />
				<div className="shipment-dialog__head-text">
					<h2 id="call-tcat-title">呼叫黑貓取件</h2>
					<small>司機將於兩日內到府收件</small>
				</div>
			</header>

			<div className="shipment-dialog__toggle-group">
				<label className="shipment-dialog__toggle">
					<PhoneIcon className="shipment-dialog__toggle-icon" />
					<span className="shipment-dialog__toggle-label">電話聯絡</span>
					<input
						type="checkbox"
						checked={isContact}
						onChange={(e) => setIsContact(e.target.checked)}
					/>
					<span className="shipment-dialog__switch" aria-hidden="true" />
				</label>
				<label className="shipment-dialog__toggle">
					<CartIcon className="shipment-dialog__toggle-icon" />
					<span className="shipment-dialog__toggle-label">需要推車</span>
					<input
						type="checkbox"
						checked={isTrolley}
						onChange={(e) => setIsTrolley(e.target.checked)}
					/>
					<span className="shipment-dialog__switch" aria-hidden="true" />
				</label>
			</div>

			<section className="shipment-dialog__field">
				<label htmlFor="call-tcat-memo">
					<NoteIcon className="shipment-dialog__label-icon" />
					給宅配人員的備註
				</label>
				<textarea
					id="call-tcat-memo"
					value={memo}
					onChange={(e) => setMemo(e.target.value)}
					placeholder="例如：請小心輕放"
					rows={3}
					maxLength={200}
				/>
			</section>

			<footer className="shipment-dialog__foot">
				<Button
					className="flex-1"
					onClick={handleSubmit}
					disabled={submitting}
					isLoading={submitting}
					loadingIndicator={<TruckSpinner size={18} label="呼叫黑貓取件中" />}
					loadingLabel="呼叫黑貓取件中…"
				>
					呼叫黑貓取件
				</Button>
			</footer>
		</DialogShell>
	);
}

/* ============================================================
 *               SevenElevenShipDialog (7-11 出貨確認)
 * ============================================================ */

export interface SevenElevenShipResult {
	stagedToken?: string;
	previewUrl?: string;
}

interface SevenElevenProps {
	open: boolean;
	onClose: () => void;
	onSubmit: (result: SevenElevenShipResult) => void;
	odno: string;
	validationNo: string;
}

export function SevenElevenShipDialog({
	open,
	onClose,
	onSubmit,
	odno,
	validationNo,
}: SevenElevenProps) {
	const [photoPreview, setPhotoPreview] = React.useState<string | null>(null);
	const [stagedToken, setStagedToken] = React.useState<string | undefined>();
	const [previewUrl, setPreviewUrl] = React.useState<string | undefined>();
	const [uploading, setUploading] = React.useState(false);
	const [uploadError, setUploadError] = React.useState<string | null>(null);
	const [submitMode, setSubmitMode] = React.useState<
		"with-photo" | "without-photo" | null
	>(null);
	const submitting = submitMode !== null;

	React.useEffect(() => {
		if (!open) {
			setPhotoPreview(null);
			setStagedToken(undefined);
			setPreviewUrl(undefined);
			setUploading(false);
			setUploadError(null);
			setSubmitMode(null);
		}
	}, [open]);

	const handlePick = async (file: File) => {
		setUploading(true);
		setUploadError(null);
		try {
			const local = URL.createObjectURL(file);
			setPhotoPreview(local);
			const asset = await uploadImageFile(file);
			setStagedToken(asset.stagedToken);
			setPreviewUrl(asset.previewUrl);
		} catch (e) {
			setUploadError(e instanceof Error ? e.message : "上傳失敗");
			setPhotoPreview(null);
			setStagedToken(undefined);
			setPreviewUrl(undefined);
		} finally {
			setUploading(false);
		}
	};

	const submit = (withPhoto: boolean) => {
		setSubmitMode(withPhoto ? "with-photo" : "without-photo");
		if (withPhoto && stagedToken) {
			onSubmit({ stagedToken, previewUrl });
		} else {
			onSubmit({});
		}
	};

	const displayCode = `${odno}${validationNo}`;

	return (
		<DialogShell
			open={open}
			onClose={onClose}
			titleId="seven-eleven-title"
			className="shipment-dialog--711"
		>
			<header className="shipment-dialog__head">
				<ShippingBrandLogo brand="711" />
				<div className="shipment-dialog__head-text">
					<h2 id="seven-eleven-title">7-11 出貨確認</h2>
					<small>交貨便代碼：{displayCode}</small>
				</div>
			</header>

			<section className="shipment-dialog__field">
				<p className="shipment-dialog__field-title">上傳產品的照片並出貨</p>
				<PhotoPicker
					preview={photoPreview}
					uploading={uploading}
					onPick={handlePick}
				/>
				{uploadError && (
					<p className="shipment-dialog__error" role="alert">
						{uploadError}
					</p>
				)}
			</section>

			<footer className="shipment-dialog__foot shipment-dialog__foot--split">
				<Button
					variant="bordered"
					color="default"
					className="flex-1"
					onClick={() => submit(true)}
					disabled={!stagedToken || uploading || submitting}
					isLoading={submitMode === "with-photo"}
					loadingIndicator={<UploadLoadingIndicator size={18} />}
					loadingLabel="送出照片中…"
				>
					送出照片
				</Button>
				<Button
					className="flex-1"
					onClick={() => submit(false)}
					disabled={uploading || submitting}
					isLoading={submitMode === "without-photo"}
					loadingIndicator={<TruckSpinner size={18} label="確認出貨中" />}
					loadingLabel="確認出貨中…"
				>
					出貨且略過
				</Button>
			</footer>
		</DialogShell>
	);
}
