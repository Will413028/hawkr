"use client";

import "@/styles/features/cart-typography.css";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import Button from "@/components/ui/Button";
import {
	createStoreMapSession,
	openStoreMapPopup,
	type StoreMapResult,
	type StorePickup,
	type StorePickupInput,
} from "@/lib/checkout/store-pickups";
import { checkDisplayLength } from "@/lib/validation/display-length";
import { CartSheetPortal } from "./CartSheetPortal";

interface Props {
	open: boolean;
	pickups: StorePickup[];
	selectedId: string | null;
	loading?: boolean;
	error?: string | null;
	/** When set, the sheet opens straight into the form pre-filled for editing. */
	editTarget?: StorePickup | null;
	onSelect: (id: string) => void;
	onCreate: (body: StorePickupInput) => Promise<void>;
	/** Required only by callers that allow editing an existing pickup. */
	onUpdate?: (id: string, body: StorePickupInput) => Promise<void>;
	onClose: () => void;
}

const EMPTY_FORM: StorePickupInput = {
	receiverName: "",
	receiverPhone: "",
	storeId: "",
	storeName: "",
	storeAddress: "",
	isDefault: false,
};

/**
 * Picker for the buyer's saved 7-11 pickup records. The "新增門市" form
 * delegates store selection to the PayUni 電子地圖 popup (same backend
 * session API the mobile app uses); we never let the buyer hand-type
 * a storeCode here — the gateway's `/app/checkout/validate` will reject
 * any code that PayUni doesn't recognise, so manual entry was always a
 * dead-end.
 */
export function StorePickupPickerSheet({
	open,
	pickups,
	selectedId,
	loading,
	error,
	editTarget,
	onSelect,
	onCreate,
	onUpdate,
	onClose,
}: Props) {
	const [adding, setAdding] = React.useState(false);
	const [editingId, setEditingId] = React.useState<string | null>(null);
	const [form, setForm] = React.useState<StorePickupInput>(EMPTY_FORM);
	const [submitting, setSubmitting] = React.useState(false);
	const [submitError, setSubmitError] = React.useState<string | null>(null);
	const [pickedStore, setPickedStore] = React.useState<StoreMapResult | null>(
		null,
	);
	const [openingMap, setOpeningMap] = React.useState(false);

	React.useEffect(() => {
		if (!open) return;
		const onKey = (e: KeyboardEvent) => {
			if (e.key === "Escape") onClose();
		};
		window.addEventListener("keydown", onKey);
		return () => window.removeEventListener("keydown", onKey);
	}, [open, onClose]);

	React.useEffect(() => {
		if (!open) {
			setAdding(false);
			setEditingId(null);
			setForm(EMPTY_FORM);
			setPickedStore(null);
			setSubmitError(null);
			setOpeningMap(false);
			return;
		}
		if (editTarget) {
			setEditingId(editTarget.id);
			setAdding(true);
			setForm({
				receiverName: editTarget.receiverName,
				receiverPhone: editTarget.receiverPhone,
				storeId: editTarget.storeId,
				storeName: editTarget.storeName,
				storeAddress: editTarget.storeAddress ?? "",
				isDefault: editTarget.isDefault ?? false,
			});
			setPickedStore({
				storeId: editTarget.storeId,
				storeName: editTarget.storeName,
				storeAddress: editTarget.storeAddress ?? "",
			});
		}
	}, [open, editTarget]);

	if (!open) return null;

	const openStoreMap = async () => {
		setOpeningMap(true);
		setSubmitError(null);
		try {
			const session = await createStoreMapSession("web");
			if (!session.redirectUrl) {
				throw new Error("無法取得門市地圖連結");
			}
			const result = await openStoreMapPopup(session.redirectUrl);
			setPickedStore(result);
			setForm((f) => ({
				...f,
				storeId: result.storeId,
				storeName: result.storeName,
				storeAddress: result.storeAddress,
			}));
		} catch (e) {
			setSubmitError(e instanceof Error ? e.message : "開啟門市地圖失敗");
		} finally {
			setOpeningMap(false);
		}
	};

	const handleSave = async () => {
		setSubmitting(true);
		setSubmitError(null);
		try {
			if (editingId && onUpdate) {
				await onUpdate(editingId, form);
				onClose();
				return;
			}
			await onCreate(form);
			setAdding(false);
			setForm(EMPTY_FORM);
			setPickedStore(null);
		} catch (e) {
			setSubmitError(
				e instanceof Error
					? e.message
					: editingId
						? "更新取貨門市失敗"
						: "新增取貨門市失敗",
			);
		} finally {
			setSubmitting(false);
		}
	};

	// 超商取貨收件人姓名：顯示長度 [4, 10]（中文 1 字算 2、英文 1 字算 1），
	// 對齊後端 PayUni 物流欄位約束。只有使用者已輸入時才即時提示。
	const receiverNameError =
		form.receiverName.trim().length > 0
			? checkDisplayLength(form.receiverName, {
					label: "取貨人姓名",
					min: 4,
					max: 10,
				})
			: null;

	const formValid =
		form.receiverName.trim() !== "" &&
		receiverNameError === null &&
		form.receiverPhone.trim() !== "" &&
		form.storeId.trim() !== "" &&
		form.storeName.trim() !== "";

	return (
		<CartSheetPortal>
			<div
				className="cart-sheet cart-sheet--buyer-cart"
				role="dialog"
				aria-label="選擇取貨門市"
				aria-modal="true"
			>
				<button
					type="button"
					className="cart-sheet__scrim"
					aria-label="關閉"
					onClick={onClose}
				/>
				<div className="cart-sheet__panel">
					<header className="cart-sheet__header">
						<h2>
							{editingId
								? "編輯取貨門市"
								: adding
									? "新增取貨門市"
									: "選擇取貨門市"}
						</h2>
						<Button
							type="button"
							variant="light"
							color="default"
							size="md"
							radius="full"
							isIconOnly
							className="cart-sheet__close"
							onClick={onClose}
							aria-label="關閉"
						>
							<AppIcon name="close" size={24} />
						</Button>
					</header>

					{error ? (
						<p
							className="hkui-login__hint hkui-login__hint--error"
							role="alert"
							style={{ margin: "0 18px 8px" }}
						>
							{error}
						</p>
					) : null}

					<div className="cart-sheet__body">
						{adding ? null : loading && pickups.length === 0 ? (
							<p className="cart-sheet__hint">載入取貨門市中…</p>
						) : pickups.length === 0 ? (
							<div className="cart-sheet__empty">
								<p>還沒有取貨門市紀錄，請新增一筆。</p>
							</div>
						) : (
							<ul className="cart-sheet__list">
								{pickups.map((p) => {
									const active = selectedId === p.id;
									return (
										<li key={p.id}>
											<button
												type="button"
												className={`cart-sheet__row${active ? " cart-sheet__row--active" : ""}`}
												onClick={() => {
													onSelect(p.id);
													onClose();
												}}
											>
												<span
													className="cart-sheet__row-radio"
													aria-hidden="true"
												>
													<span />
												</span>
												<div className="cart-sheet__row-body">
													<strong>
														{p.storeName}
														{p.isDefault ? (
															<span className="cart-sheet__badge">預設</span>
														) : null}
													</strong>
													<span>7-11 · 店號 {p.storeId}</span>
													{p.storeAddress ? (
														<span>{p.storeAddress}</span>
													) : null}
													<span>
														{p.receiverName} · {p.receiverPhone}
													</span>
												</div>
											</button>
										</li>
									);
								})}
							</ul>
						)}

						{adding ? (
							<div className="cart-sheet__form cart-sheet__form--standalone">
								<label>
									<span>
										取貨人姓名
										<span className="cart-sheet__form-required">*</span>
									</span>
									<input
										type="text"
										value={form.receiverName}
										onChange={(e) =>
											setForm((f) => ({ ...f, receiverName: e.target.value }))
										}
										placeholder="例：王小明"
										maxLength={10}
										aria-invalid={Boolean(receiverNameError)}
										required
									/>
									{receiverNameError ? (
										<span
											className="hkui-login__hint hkui-login__hint--error"
											role="alert"
										>
											{receiverNameError}
										</span>
									) : null}
								</label>
								<label>
									<span>
										聯絡電話
										<span className="cart-sheet__form-required">*</span>
									</span>
									<input
										type="tel"
										inputMode="numeric"
										value={form.receiverPhone}
										onChange={(e) =>
											setForm((f) => ({ ...f, receiverPhone: e.target.value }))
										}
										placeholder="0912345678"
										maxLength={20}
										required
									/>
								</label>

								<div className="cart-sheet__store-pick">
									<span className="cart-sheet__store-pick-label">取貨門市</span>
									{pickedStore ? (
										<div className="cart-sheet__store-card">
											<strong>{pickedStore.storeName}</strong>
											<span>7-11 · 店號 {pickedStore.storeId}</span>
											{pickedStore.storeAddress ? (
												<span>{pickedStore.storeAddress}</span>
											) : null}
											<Button
												type="button"
												size="md"
												variant="bordered"
												className="cart-sheet__store-change"
												onClick={() => void openStoreMap()}
												disabled={openingMap}
												isLoading={openingMap}
												loadingLabel="開啟門市地圖中…"
											>
												重新選擇
											</Button>
										</div>
									) : (
										<Button
											type="button"
											className="cart-sheet__store-pick-btn"
											onClick={() => void openStoreMap()}
											disabled={openingMap}
											isLoading={openingMap}
											loadingLabel="開啟門市地圖中…"
										>
											選擇 7-11 門市
										</Button>
									)}
									<p className="cart-sheet__store-pick-hint">
										將在新視窗開啟 PayUni 電子地圖，選定後自動帶回門市資料。
									</p>
								</div>

								<label className="cart-sheet__form-check">
									<input
										type="checkbox"
										checked={!!form.isDefault}
										onChange={(e) =>
											setForm((f) => ({ ...f, isDefault: e.target.checked }))
										}
									/>
									<span>設為預設取貨門市</span>
								</label>
								{submitError ? (
									<p
										className="hkui-login__hint hkui-login__hint--error"
										role="alert"
									>
										{submitError}
									</p>
								) : null}
								<div className="cart-sheet__form-actions">
									<Button
										type="button"
										variant="bordered"
										color="default"
										size="lg"
										className="cart-sheet__secondary-action"
										onClick={() => {
											if (editingId) {
												onClose();
												return;
											}
											setAdding(false);
											setPickedStore(null);
											setSubmitError(null);
										}}
										disabled={submitting}
									>
										取消
									</Button>
									<Button
										type="button"
										size="lg"
										className="cart-sheet__primary-action"
										onClick={() => void handleSave()}
										disabled={!formValid || submitting}
										isLoading={submitting}
										loadingLabel={
											editingId ? "更新取貨門市中…" : "儲存取貨門市中…"
										}
									>
										{editingId ? "更新" : "儲存"}
									</Button>
								</div>
								{!formValid && !submitError ? (
									<p className="cart-sheet__form-hint" role="status">
										請填寫姓名、電話並選擇門市
									</p>
								) : null}
							</div>
						) : null}
					</div>
					{!adding ? (
						<footer className="cart-sheet__footer">
							<Button
								type="button"
								variant="bordered"
								size="md"
								fullWidth
								startContent={<AppIcon name="plus" size={20} />}
								className="cart-sheet__add"
								onClick={() => setAdding(true)}
							>
								新增取貨門市
							</Button>
						</footer>
					) : null}
				</div>
			</div>
		</CartSheetPortal>
	);
}
