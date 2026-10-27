"use client";

import { countyCodeOfTownCode } from "@hawkr/twaddr/lookup";
import { zodResolver } from "@hookform/resolvers/zod";
import * as React from "react";
import { Controller, useForm } from "react-hook-form";
import AppIcon, { type AppIconName } from "@/components/shared/AppIcon";
import {
	ScEmpty,
	ScEmptyDesc,
	ScEmptyTitle,
} from "@/components/shared/ScPrimitives";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { Input } from "@/components/ui";
import Button from "@/components/ui/Button";
import { AccountEmptyStateIcon } from "@/features/account/components/AccountEmptyIcon";
import { CartSheetPortal } from "@/features/cart/components/CartSheetPortal";
import { StorePickupPickerSheet } from "@/features/cart/components/StorePickupPickerSheet";
import {
	type Address,
	type AddressInput,
	addressText,
} from "@/lib/addresses/repo";
import {
	type AddressFormValues,
	addressFormSchema,
} from "@/lib/addresses/schema";
import type { StorePickup } from "@/lib/checkout/store-pickups";
import { toastActionResult } from "@/lib/feedback/action-toast";
import {
	formActionFailure,
	formActionSuccess,
} from "@/lib/forms/action-result";
import { useAddresses } from "@/lib/hooks/useAddresses";
import { useStorePickups } from "@/lib/hooks/useStorePickups";
import { TownFields } from "./TownFields";

function normalizePhone(value: string): string {
	return value.replace(/[^\d]/g, "");
}

export default function AddressesClient() {
	const {
		addresses,
		defaultAddress,
		loading,
		error,
		create,
		update,
		remove,
		setDefault,
	} = useAddresses();
	const {
		pickups,
		defaultPickup,
		loading: pickupsLoading,
		error: pickupsError,
		remove: removePickup,
		create: createPickup,
		update: updatePickup,
	} = useStorePickups();
	const [busyId, setBusyId] = React.useState<string | null>(null);
	const [busyPickupId, setBusyPickupId] = React.useState<string | null>(null);
	const [addressSheetOpen, setAddressSheetOpen] = React.useState(false);
	const [addressEditTarget, setAddressEditTarget] =
		React.useState<Address | null>(null);
	const [pickupSheetOpen, setPickupSheetOpen] = React.useState(false);
	const [pickupEditTarget, setPickupEditTarget] =
		React.useState<StorePickup | null>(null);

	const openAddressSheet = (target: Address | null) => {
		setAddressEditTarget(target);
		setAddressSheetOpen(true);
	};

	const openPickupSheet = (target: StorePickup | null) => {
		setPickupEditTarget(target);
		setPickupSheetOpen(true);
	};

	const setPickupDefault = (id: string) => {
		const pickup = pickups.find((p) => p.id === id);
		if (!pickup) return;
		void wrapPickup(id, () =>
			updatePickup(id, {
				receiverName: pickup.receiverName,
				receiverPhone: pickup.receiverPhone,
				storeId: pickup.storeId,
				storeName: pickup.storeName,
				storeAddress: pickup.storeAddress,
				isDefault: true,
			}),
		);
	};

	const wrap = async (id: string, op: () => Promise<void>) => {
		setBusyId(id);
		try {
			await op();
		} finally {
			setBusyId(null);
		}
	};

	const wrapPickup = async (id: string, op: () => Promise<void>) => {
		setBusyPickupId(id);
		try {
			await op();
		} finally {
			setBusyPickupId(null);
		}
	};

	return (
		<main className="addresses-page">
			<SellerPageShell
				title="收件地址"
				subtitle="這裡顯示你目前的預設宅配地址與 7-11 取貨門市，結帳時會自動帶入。點「管理」可切換預設或新增。"
			>
				{error && (
					<p className="text-danger-700 text-sm font-semibold" role="alert">
						{error}
					</p>
				)}
				{pickupsError && (
					<p className="text-danger-700 text-sm font-semibold" role="alert">
						{pickupsError}
					</p>
				)}

				<section className="flex flex-col gap-3">
					<AddressSectionHeading
						icon="truck"
						title="宅配地址"
						desc="結帳選擇宅配到家時，會自動帶入這個預設地址。"
						action={
							<Button
								type="button"
								size="sm"
								radius="full"
								onClick={() => openAddressSheet(null)}
							>
								管理宅配地址
							</Button>
						}
					/>
					{loading && !defaultAddress ? (
						<p>載入中…</p>
					) : !defaultAddress ? (
						<ScEmpty>
							<AccountEmptyStateIcon variant="address" />
							<ScEmptyTitle>還沒有宅配地址</ScEmptyTitle>
							<ScEmptyDesc>
								先新增至少一筆地址，結帳時就能一鍵帶入。
							</ScEmptyDesc>
							<Button
								type="button"
								size="lg"
								radius="full"
								onClick={() => openAddressSheet(null)}
							>
								新增第一筆地址
							</Button>
						</ScEmpty>
					) : (
						<div className="flex justify-between gap-4 bg-content1 border border-divider rounded-hkui-lg p-4 flex-wrap">
							<div className="flex flex-col gap-[2px] flex-1 min-w-[220px]">
								<strong className="flex items-center gap-2 text-base font-semibold">
									{defaultAddress.receiverName}
									<span className="text-[0.6875rem] font-bold bg-hk-primary-strong text-[#fff9e8] px-1.5 py-0.5 rounded-full">
										預設
									</span>
								</strong>
								<span className="text-sm text-foreground-muted">
									{defaultAddress.receiverPhone}
								</span>
								<span className="text-sm text-foreground-muted">
									{addressText(defaultAddress)}
								</span>
							</div>
							<div className="flex gap-2 items-center flex-wrap">
								<Button
									type="button"
									variant="bordered"
									color="default"
									size="sm"
									onClick={() => openAddressSheet(defaultAddress)}
								>
									編輯
								</Button>
								<Button
									variant="bordered"
									color="danger"
									size="sm"
									disabled={busyId === defaultAddress.id}
									isLoading={busyId === defaultAddress.id}
									loadingLabel="刪除地址中…"
									onClick={() => {
										if (window.confirm("確定要刪除這個地址？")) {
											void wrap(defaultAddress.id, () =>
												remove(defaultAddress.id),
											);
										}
									}}
								>
									刪除
								</Button>
							</div>
						</div>
					)}
				</section>

				<section className="flex flex-col gap-3 pt-3">
					<AddressSectionHeading
						icon="bag"
						title="7-11 取貨門市"
						desc="結帳選擇 7-11 取貨時，會自動帶入這個預設門市。"
						action={
							<Button
								type="button"
								size="sm"
								radius="full"
								onClick={() => openPickupSheet(null)}
							>
								管理取貨門市
							</Button>
						}
					/>

					{pickupsLoading && !defaultPickup ? (
						<p>載入取貨門市中…</p>
					) : !defaultPickup ? (
						<ScEmpty>
							<AccountEmptyStateIcon variant="sellers" />
							<ScEmptyTitle>還沒有取貨門市</ScEmptyTitle>
							<ScEmptyDesc>
								先設定常用的 7-11 門市，結帳時就能一鍵帶入。
							</ScEmptyDesc>
							<Button
								type="button"
								size="lg"
								radius="full"
								onClick={() => openPickupSheet(null)}
							>
								新增取貨門市
							</Button>
						</ScEmpty>
					) : (
						<div className="flex justify-between gap-4 bg-content1 border border-divider rounded-hkui-lg p-4 flex-wrap">
							<div className="flex flex-col gap-[2px] flex-1 min-w-[220px]">
								<strong className="flex items-center gap-2 text-base font-semibold">
									{defaultPickup.storeName || "7-11 取貨門市"}
									{defaultPickup.isDefault && (
										<span className="text-[0.6875rem] font-bold bg-hk-primary-strong text-[#fff9e8] px-1.5 py-0.5 rounded-full">
											預設
										</span>
									)}
								</strong>
								<span className="text-sm text-foreground-muted">
									{defaultPickup.receiverName} · {defaultPickup.receiverPhone}
								</span>
								<span className="text-sm text-foreground-muted">
									7-11 店號 {defaultPickup.storeId || "—"}
								</span>
								{defaultPickup.storeAddress && (
									<span className="text-sm text-foreground-muted">
										{defaultPickup.storeAddress}
									</span>
								)}
							</div>
							<div className="flex gap-2 items-center flex-wrap">
								<Button
									type="button"
									variant="bordered"
									color="default"
									size="sm"
									onClick={() => openPickupSheet(defaultPickup)}
								>
									編輯
								</Button>
								<Button
									variant="bordered"
									color="danger"
									size="sm"
									disabled={busyPickupId === defaultPickup.id}
									isLoading={busyPickupId === defaultPickup.id}
									loadingLabel="刪除門市中…"
									onClick={() => {
										if (window.confirm("確定要刪除這個取貨門市？")) {
											void wrapPickup(defaultPickup.id, () =>
												removePickup(defaultPickup.id),
											);
										}
									}}
								>
									刪除
								</Button>
							</div>
						</div>
					)}
				</section>
			</SellerPageShell>
			<AddressPickerSheet
				open={addressSheetOpen}
				addresses={addresses}
				loading={loading}
				editTarget={addressEditTarget}
				onSelectDefault={async (id) => {
					await setDefault(id);
				}}
				onCreate={async (body) => {
					await create(body);
				}}
				onUpdate={async (id, etag, body) => {
					await update(id, etag, body);
				}}
				onClose={() => {
					setAddressSheetOpen(false);
					setAddressEditTarget(null);
				}}
			/>
			<StorePickupPickerSheet
				open={pickupSheetOpen}
				pickups={pickups}
				selectedId={defaultPickup?.id ?? null}
				loading={pickupsLoading}
				error={pickupsError}
				editTarget={pickupEditTarget}
				onSelect={(id) => {
					setPickupDefault(id);
					setPickupSheetOpen(false);
				}}
				onCreate={async (body) => {
					await createPickup(body);
				}}
				onUpdate={async (id, body) => {
					await updatePickup(id, body);
				}}
				onClose={() => {
					setPickupSheetOpen(false);
					setPickupEditTarget(null);
				}}
			/>
		</main>
	);
}

function AddressPickerSheet({
	open,
	addresses,
	loading,
	editTarget,
	onSelectDefault,
	onCreate,
	onUpdate,
	onClose,
}: {
	open: boolean;
	addresses: Address[];
	loading: boolean;
	/** When set, the sheet opens straight into the form pre-filled for editing. */
	editTarget?: Address | null;
	onSelectDefault: (id: string) => Promise<void>;
	onCreate: (body: AddressInput) => Promise<void>;
	onUpdate?: (id: string, etag: string, body: AddressInput) => Promise<void>;
	onClose: () => void;
}) {
	const [adding, setAdding] = React.useState(false);
	const [editingId, setEditingId] = React.useState<string | null>(null);
	const [selectingId, setSelectingId] = React.useState<string | null>(null);
	const form = useForm<AddressFormValues>({
		resolver: zodResolver(addressFormSchema),
		mode: "onChange",
		defaultValues: {
			receiverName: "",
			receiverPhone: "",
			countyCode: "",
			townCode: "",
			detail: "",
			isDefault: false,
		},
	});
	const {
		control,
		formState: { errors, isSubmitting, submitCount, touchedFields },
		handleSubmit,
		register,
		reset,
		setValue,
		watch,
	} = form;
	const [countyCode, townCode] = watch(["countyCode", "townCode"]);
	const [formError, setFormError] = React.useState<string | null>(null);

	const resetForm = React.useCallback(() => {
		reset();
		setFormError(null);
	}, [reset]);

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
			setSelectingId(null);
			resetForm();
			return;
		}
		if (editTarget) {
			setEditingId(editTarget.id);
			setAdding(true);
			reset({
				receiverName: editTarget.receiverName,
				receiverPhone: editTarget.receiverPhone,
				countyCode: countyCodeOfTownCode(editTarget.townCode),
				townCode: editTarget.townCode,
				detail: editTarget.addressDetail,
				isDefault: editTarget.isDefault,
			});
			setFormError(null);
		}
	}, [open, editTarget, reset, resetForm]);

	if (!open) return null;

	const errorFor = (field: keyof AddressFormValues) =>
		touchedFields[field] || submitCount > 0
			? errors[field]?.message
			: undefined;

	const submitAddress = handleSubmit(async (values) => {
		setFormError(null);
		const body: AddressInput = {
			receiverName: values.receiverName.trim(),
			receiverPhone: normalizePhone(values.receiverPhone),
			townCode: values.townCode,
			addressDetail: values.detail.trim(),
			isDefault: values.isDefault,
		};
		try {
			if (editingId && editTarget && onUpdate) {
				await onUpdate(editTarget.id, editTarget.etag, body);
				toastActionResult(formActionSuccess(undefined, "地址已更新"));
				onClose();
				return;
			}
			await onCreate(body);
			toastActionResult(formActionSuccess(undefined, "地址已儲存"));
			resetForm();
			setAdding(false);
		} catch (e) {
			const message = e instanceof Error ? e.message : "儲存失敗";
			setFormError(message);
			toastActionResult(formActionFailure(message));
		}
	});

	const handleSelect = async (address: Address) => {
		if (selectingId !== null) return;
		if (address.isDefault) {
			onClose();
			return;
		}
		const addressId = String(address.id);
		setSelectingId(addressId);
		try {
			await onSelectDefault(addressId);
			onClose();
		} catch {
			setSelectingId(null);
		}
	};

	return (
		<CartSheetPortal>
			<div
				className="cart-sheet"
				role="dialog"
				aria-label="選擇宅配地址"
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
								? "編輯宅配地址"
								: adding
									? "新增宅配地址"
									: "選擇宅配地址"}
						</h2>
						<button
							type="button"
							className="cart-sheet__close"
							onClick={onClose}
							aria-label="關閉"
						>
							×
						</button>
					</header>

					<div className="cart-sheet__body">
						{adding ? null : loading && addresses.length === 0 ? (
							<p className="cart-sheet__hint">載入地址中…</p>
						) : addresses.length === 0 ? (
							<div className="cart-sheet__empty">
								<p>還沒有宅配地址，請新增一筆。</p>
							</div>
						) : (
							<ul className="cart-sheet__list">
								{addresses.map((a) => {
									const active = a.isDefault;
									return (
										<li key={a.id}>
											<Button
												type="button"
												variant="light"
												color="default"
												className={`cart-sheet__row !block h-auto whitespace-normal p-[12px_14px]${active ? " cart-sheet__row--active" : ""}`}
												disabled={selectingId !== null}
												isLoading={selectingId === String(a.id)}
												loadingLabel="設定預設地址中…"
												onClick={() => void handleSelect(a)}
											>
												<span className="grid w-full grid-cols-[20px_minmax(0,1fr)] gap-3 text-left">
													<span
														className="cart-sheet__row-radio"
														aria-hidden="true"
													>
														<span />
													</span>
													<span className="cart-sheet__row-body">
														<strong>
															{a.receiverName}
															{a.isDefault ? (
																<span className="cart-sheet__badge">預設</span>
															) : null}
														</strong>
														<span>{a.receiverPhone}</span>
														<span>{addressText(a)}</span>
													</span>
												</span>
											</Button>
										</li>
									);
								})}
							</ul>
						)}

						{adding ? (
							<form
								className="flex flex-col gap-2.5"
								onSubmit={submitAddress}
								autoComplete="on"
							>
								<Input
									floatingLabel
									variant="bordered"
									size="lg"
									label="收件人姓名"
									isRequired
									maxLength={20}
									autoComplete="shipping name"
									errorMessage={errorFor("receiverName")}
									isInvalid={Boolean(errorFor("receiverName"))}
									{...register("receiverName")}
								/>
								<Input
									floatingLabel
									variant="bordered"
									size="lg"
									label="聯絡電話"
									type="tel"
									isRequired
									placeholder="09xx-xxx-xxx"
									autoComplete="shipping tel-national"
									errorMessage={errorFor("receiverPhone")}
									isInvalid={Boolean(errorFor("receiverPhone"))}
									{...register("receiverPhone")}
								/>
								<div className="grid grid-cols-2 gap-3 max-[600px]:grid-cols-1">
									<Controller
										control={control}
										name="countyCode"
										render={() => (
											<TownFields
												countyCode={countyCode}
												townCode={townCode}
												onChange={(nextCounty, nextTown) => {
													setValue("countyCode", nextCounty, {
														shouldDirty: true,
														shouldTouch: true,
														shouldValidate: true,
													});
													setValue("townCode", nextTown, {
														shouldDirty: true,
														shouldTouch: Boolean(nextTown),
														shouldValidate: true,
													});
												}}
												countyError={errorFor("countyCode")}
												townError={errorFor("townCode")}
											/>
										)}
									/>
								</div>
								<div>
									<Input
										floatingLabel
										variant="bordered"
										size="lg"
										label="詳細地址"
										isRequired
										placeholder="信義路一段 1 號 5 樓"
										maxLength={255}
										autoComplete="shipping street-address"
										errorMessage={errorFor("detail")}
										isInvalid={Boolean(errorFor("detail"))}
										{...register("detail")}
									/>
								</div>
								<label className="inline-flex items-center gap-2 text-sm text-foreground-muted cursor-pointer select-none">
									<input
										type="checkbox"
										className="w-[18px] h-[18px] accent-hkui-primary cursor-pointer"
										{...register("isDefault")}
									/>
									<span>設為預設地址</span>
								</label>
								{formError ? (
									<p
										className="hkui-login__hint hkui-login__hint--error"
										role="alert"
									>
										{formError}
									</p>
								) : null}
								<div className="flex gap-2 justify-end">
									<button
										type="button"
										className="inline-flex items-center justify-center min-h-[48px] px-[22px] rounded-[var(--radius-pill)] border border-hk-border-soft bg-hk-surface-glass text-hk-heading font-bold no-underline transition-all duration-200 ease-in-out hover:-translate-y-0.5 hover:shadow-hk-card disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:translate-y-0 disabled:hover:shadow-none"
										onClick={() => {
											if (editingId) {
												onClose();
												return;
											}
											setAdding(false);
											resetForm();
										}}
										disabled={isSubmitting}
									>
										取消
									</button>
									<Button
										type="submit"
										className="hk-cta-primary inline-flex items-center justify-center min-h-[48px] px-[22px] rounded-[var(--radius-pill)] border border-hk-primary bg-hk-primary text-hk-inverse font-bold no-underline transition-all duration-200 ease-in-out hover:-translate-y-0.5 hover:shadow-hk-card disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:translate-y-0 disabled:hover:shadow-none"
										disabled={isSubmitting || loading}
										isLoading={isSubmitting}
										loadingLabel={editingId ? "更新地址中…" : "儲存地址中…"}
									>
										{editingId ? "更新地址" : "儲存地址"}
									</Button>
								</div>
							</form>
						) : null}
					</div>

					{!adding ? (
						<footer className="cart-sheet__footer">
							<button
								type="button"
								className="cart-sheet__add"
								onClick={() => setAdding(true)}
							>
								＋ 新增宅配地址
							</button>
						</footer>
					) : null}
				</div>
			</div>
		</CartSheetPortal>
	);
}

function AddressSectionHeading({
	icon,
	title,
	desc,
	action,
}: {
	icon: AppIconName;
	title: string;
	desc: string;
	action?: React.ReactNode;
}) {
	return (
		<div className="flex flex-wrap items-center justify-between gap-3">
			<div className="flex min-w-0 items-center gap-3">
				<span
					className="relative grid size-11 shrink-0 place-items-center rounded-[10px] border border-divider bg-white shadow-sm"
					aria-hidden="true"
				>
					<AppIcon name={icon} size={24} />
				</span>
				<div className="min-w-0">
					<h2 className="text-xl font-semibold leading-tight text-foreground">
						{title}
					</h2>
					<p className="mt-0 text-sm leading-snug text-foreground-muted">
						{desc}
					</p>
				</div>
			</div>
			{action ? <div className="shrink-0">{action}</div> : null}
		</div>
	);
}
