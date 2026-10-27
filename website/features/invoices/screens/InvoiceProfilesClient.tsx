"use client";

import * as React from "react";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import { Chip, Dialog, Select } from "@/components/ui";
import Button from "@/components/ui/Button";
import type { InvoiceProfileFormValues } from "@/features/invoices/components/InvoiceProfileForm";
import { InvoiceProfileForm } from "@/features/invoices/components/InvoiceProfileForm";
import { MaskedValue } from "@/features/invoices/components/MaskedValue";
import {
	INVOICE_PROFILE_ERROR_TEXT,
	INVOICE_PROFILE_TEXT,
} from "@/features/invoices/i18n";
import { toast } from "@/lib/feedback/toast";
import { useInvoiceProfiles } from "@/lib/hooks/useInvoiceProfiles";
import {
	INVOICE_BIZ_CODE,
	invoiceBizCode,
	invoiceErrorMessage,
	newIdempotencyKey,
} from "@/lib/invoices/buyer-repo";
import { invoiceCarrierLabel } from "@/lib/invoices/carrier";
import type { InvoiceProfile } from "@/lib/invoices/types";

const T = INVOICE_PROFILE_TEXT;
const E = INVOICE_PROFILE_ERROR_TEXT;

/** 成功 banner 的存活時間。8 秒之後淡出，或被下一次操作取代。 */
const BANNER_TTL_MS = 8000;

type Banner = { tone: "status" | "alert"; text: string };

/** 這一筆在 banner 與 aria-label 裡怎麼稱呼——有備註就用備註，否則用類型名稱。 */
function profileName(profile: InvoiceProfile): string {
	return profile.label || invoiceCarrierLabel(profile.carrierType);
}

/**
 * B08｜常用發票資料（58A §B08）。
 *
 * 三件事值得先讀：
 *
 *   1. **冪等鍵一次意圖一把**（§0.4 ②）。`keysRef` 以「意圖字串」為索引：同一次
 *      操作重試沿用同一把，成功之後才丟掉，只有拿到 9350 才換新的。每次送出重生
 *      一把等於告訴後端「這是第二次操作」。
 *   2. **刪除預設是「先設新預設、成功才刪」**（§B-1.0.9 B11t）。顛倒之後若第一步
 *      失敗就留下「零個預設」，下一次結帳靜默換成另一種載具，而發票開錯載具只能
 *      作廢重開。9304 是後端把這件事擋下來，不是幫你改指。
 *   3. **成功用就地 banner 而不是 toast**（需求書 B08，刻意偏離既有地址頁）。這一頁
 *      的操作結果改變的是清單本身；刪除的 toast 消失之後，使用者只能靠「那一列不見
 *      了」確認刪對了。失敗則 banner ＋ toast 兩者皆有——使用者可能已經捲到別處。
 */
export default function InvoiceProfilesClient() {
	const {
		profiles,
		loading,
		error: listError,
		refresh,
		create,
		update,
		remove,
		setDefault,
	} = useInvoiceProfiles();

	const [adding, setAdding] = React.useState(false);
	const [editingId, setEditingId] = React.useState<string | null>(null);
	const [submitting, setSubmitting] = React.useState(false);
	const [formError, setFormError] = React.useState<string | null>(null);
	const [fieldError, setFieldError] = React.useState<string | null>(null);
	const [banner, setBanner] = React.useState<Banner | null>(null);
	const [busyProfileId, setBusyProfileId] = React.useState<string | null>(null);

	const [deleteTarget, setDeleteTarget] = React.useState<InvoiceProfile | null>(
		null,
	);
	const [newDefaultId, setNewDefaultId] = React.useState("");
	const [deleteError, setDeleteError] = React.useState<string | null>(null);
	const [deleteBusy, setDeleteBusy] = React.useState(false);

	const bannerRef = React.useRef<HTMLDivElement | null>(null);
	const keysRef = React.useRef<Map<string, string>>(new Map());

	const keyFor = (intent: string) => {
		const existing = keysRef.current.get(intent);
		if (existing) return existing;
		const key = newIdempotencyKey();
		keysRef.current.set(intent, key);
		return key;
	};
	const dropKey = (intent: string) => {
		keysRef.current.delete(intent);
	};

	// 成功 banner 8 秒後自動收掉；失敗 banner 留著，並把焦點移過去（§B08 A11y）。
	React.useEffect(() => {
		if (!banner) return;
		if (banner.tone === "alert") {
			bannerRef.current?.focus();
			return;
		}
		const timer = window.setTimeout(() => setBanner(null), BANNER_TTL_MS);
		return () => window.clearTimeout(timer);
	}, [banner]);

	const editingProfile =
		profiles.find((profile) => profile.profileId === editingId) ?? null;

	const closeForm = () => {
		setAdding(false);
		setEditingId(null);
		setFormError(null);
		setFieldError(null);
	};

	const startAdding = () => {
		closeForm();
		dropKey("create");
		setAdding(true);
	};

	const startEditing = (profile: InvoiceProfile) => {
		closeForm();
		dropKey(`update:${profile.profileId}`);
		setEditingId(profile.profileId);
	};

	/**
	 * 把後端業務碼折成畫面上的位置：9305 進欄位下方、9303／9306 進表單橫幅。
	 * 9350 是唯一會換冪等鍵的碼。
	 */
	const applyFormError = (err: unknown, intent: string) => {
		const code = invoiceBizCode(err);
		setFieldError(null);
		setFormError(null);
		switch (code) {
			case INVOICE_BIZ_CODE.CARRIER_INVALID:
				setFieldError(invoiceErrorMessage(err) || E.generic);
				return;
			case INVOICE_BIZ_CODE.PROFILE_LIMIT_EXCEEDED:
				setFormError(E.limitExceeded);
				return;
			case INVOICE_BIZ_CODE.TARGET_CONFLICT:
				setFormError(invoiceErrorMessage(err) || E.targetConflict);
				return;
			case INVOICE_BIZ_CODE.PROFILE_NOT_FOUND:
				setFormError(E.notFound);
				closeForm();
				void refresh();
				return;
			case INVOICE_BIZ_CODE.IDEMPOTENCY_PAYLOAD_MISMATCH:
				console.error("invoice profile idempotency payload mismatch", err);
				dropKey(intent);
				setFormError(E.idempotencyMismatch);
				return;
			default:
				setFormError(invoiceErrorMessage(err) || E.generic);
		}
	};

	const showFailure = (text: string) => {
		setBanner({ tone: "alert", text });
		toast.error(text);
	};

	const submitForm = async (values: InvoiceProfileFormValues) => {
		const intent = editingProfile
			? `update:${editingProfile.profileId}`
			: "create";
		setSubmitting(true);
		setFormError(null);
		setFieldError(null);
		try {
			if (editingProfile) {
				const saved = await update(
					editingProfile.profileId,
					{
						value: values.value,
						buyerTaxId: values.buyerTaxId,
						buyerName: values.buyerName,
						label: values.label,
					},
					keyFor(intent),
				);
				dropKey(intent);
				closeForm();
				setBanner({ tone: "status", text: `已更新「${profileName(saved)}」` });
			} else {
				const created = await create(values, keyFor(intent));
				dropKey(intent);
				closeForm();
				setBanner({
					tone: "status",
					text: `已新增「${profileName(created)}」${
						created.isDefault ? "（並設為預設）" : ""
					}`,
				});
			}
		} catch (err) {
			applyFormError(err, intent);
		} finally {
			setSubmitting(false);
		}
	};

	const handleSetDefault = async (profile: InvoiceProfile) => {
		const intent = `default:${profile.profileId}`;
		setBusyProfileId(profile.profileId);
		try {
			await setDefault(profile.profileId, keyFor(intent));
			dropKey(intent);
			setBanner({
				tone: "status",
				text: `已將「${profileName(profile)}」設為預設`,
			});
		} catch (err) {
			const code = invoiceBizCode(err);
			if (code === INVOICE_BIZ_CODE.PROFILE_NOT_FOUND) {
				await refresh();
				showFailure(E.notFound);
			} else if (code === INVOICE_BIZ_CODE.IDEMPOTENCY_PAYLOAD_MISMATCH) {
				console.error("invoice profile idempotency payload mismatch", err);
				dropKey(intent);
				showFailure(E.idempotencyMismatch);
			} else {
				showFailure(invoiceErrorMessage(err) || E.generic);
			}
		} finally {
			setBusyProfileId(null);
		}
	};

	const otherProfiles = deleteTarget
		? profiles.filter((profile) => profile.profileId !== deleteTarget.profileId)
		: [];
	// 「預設且還有別筆」才必須先指定接手的那一筆；最後一筆預設直接刪，不強迫指定。
	const needsNewDefault =
		Boolean(deleteTarget?.isDefault) && otherProfiles.length > 0;

	const openDelete = (profile: InvoiceProfile) => {
		dropKey(`delete:${profile.profileId}`);
		setDeleteTarget(profile);
		setNewDefaultId("");
		setDeleteError(null);
	};

	const closeDelete = () => {
		setDeleteTarget(null);
		setNewDefaultId("");
		setDeleteError(null);
	};

	const confirmDelete = async () => {
		if (!deleteTarget) return;
		if (needsNewDefault && !newDefaultId) {
			setDeleteError(E.defaultRequired);
			return;
		}
		const target = deleteTarget;
		const deleteIntent = `delete:${target.profileId}`;
		setDeleteBusy(true);
		setDeleteError(null);
		try {
			// 順序是硬性的：先設新預設，成功才刪。第一步失敗就不執行刪除。
			if (needsNewDefault) {
				const promoteIntent = `default:${newDefaultId}`;
				await setDefault(newDefaultId, keyFor(promoteIntent));
				dropKey(promoteIntent);
			}
			await remove(target.profileId, keyFor(deleteIntent));
			dropKey(deleteIntent);
			closeDelete();
			setBanner({
				tone: "status",
				text: `已刪除「${profileName(target)}」`,
			});
		} catch (err) {
			const code = invoiceBizCode(err);
			if (code === INVOICE_BIZ_CODE.PROFILE_NOT_FOUND) {
				closeDelete();
				await refresh();
				showFailure(E.notFound);
			} else if (code === INVOICE_BIZ_CODE.PROFILE_DEFAULT_REQUIRED) {
				setDeleteError(E.defaultRequired);
			} else if (code === INVOICE_BIZ_CODE.IDEMPOTENCY_PAYLOAD_MISMATCH) {
				console.error("invoice profile idempotency payload mismatch", err);
				dropKey(deleteIntent);
				setDeleteError(E.idempotencyMismatch);
			} else {
				setDeleteError(invoiceErrorMessage(err) || E.generic);
			}
		} finally {
			setDeleteBusy(false);
		}
	};

	const formOpen = adding || editingProfile != null;

	return (
		<SellerPageShell
			title={T.pageTitle}
			subtitle={T.pageSubtitle}
			actions={
				<Button size="lg" onClick={startAdding} disabled={formOpen}>
					{T.addCta}
				</Button>
			}
		>
			{banner ? (
				<div
					ref={bannerRef}
					// biome-ignore lint/a11y/noNoninteractiveTabindex: 失敗時要把焦點移到這裡，成功時不搶焦點。
					tabIndex={-1}
					role={banner.tone === "alert" ? "alert" : "status"}
					className={
						banner.tone === "alert"
							? "rounded-hkui-lg border border-danger-200 bg-danger-50 p-4 text-sm text-danger-700"
							: "rounded-hkui-lg border border-success-200 bg-success-50 p-4 text-sm text-success-700"
					}
				>
					{banner.text}
				</div>
			) : null}

			{formOpen ? (
				<section className="seller-card">
					<InvoiceProfileForm
						key={editingProfile?.profileId ?? "create"}
						mode={editingProfile ? "edit" : "create"}
						profile={editingProfile}
						submitting={submitting}
						formError={formError}
						fieldError={fieldError}
						onCancel={closeForm}
						onSubmit={(values) => void submitForm(values)}
					/>
				</section>
			) : null}

			{listError ? (
				<p
					className="my-4 px-4 py-3 rounded-hkui-md bg-danger-50 text-danger-700 text-sm"
					role="alert"
				>
					{T.listError}{" "}
					<Button variant="light" size="sm" onClick={() => void refresh()}>
						{T.retry}
					</Button>
				</p>
			) : loading ? (
				<p className="text-foreground-soft text-sm">{T.loading}</p>
			) : profiles.length === 0 ? (
				<p className="text-foreground-soft text-sm">{T.emptyHint}</p>
			) : (
				<ul className="flex flex-col gap-3 list-none m-0 p-0">
					{profiles.map((profile) => {
						const typeLabel = invoiceCarrierLabel(profile.carrierType);
						const busy = busyProfileId === profile.profileId;
						return (
							<li
								key={profile.profileId}
								className="bg-content1 border border-divider rounded-hkui-lg p-4"
							>
								<div className="flex flex-wrap items-center justify-between gap-2">
									<div className="flex flex-col gap-1 min-w-0">
										<div className="flex items-center gap-2 min-w-0">
											<strong className="text-sm text-[var(--hkui-foreground)]">
												{typeLabel}
											</strong>
											<MaskedValue
												value={profile.displayValue}
												label={typeLabel}
												className="truncate min-w-0 text-sm text-foreground-soft"
											/>
											{profile.isDefault ? (
												<Chip variant="flat" color="primary" size="sm">
													{T.defaultChip}
												</Chip>
											) : null}
										</div>
										{profile.label ? (
											<span className="text-xs text-foreground-soft">
												{profile.label}
											</span>
										) : null}
									</div>
									<div className="flex items-center gap-2">
										{profile.isDefault ? null : (
											<Button
												variant="light"
												size="sm"
												disabled={busy || formOpen}
												isLoading={busy}
												loadingLabel={T.settingDefault}
												aria-label={`${T.setDefaultCta} ${typeLabel} ${profile.displayValue}`}
												onClick={() => void handleSetDefault(profile)}
											>
												{T.setDefaultCta}
											</Button>
										)}
										<Button
											variant="bordered"
											color="default"
											size="sm"
											disabled={formOpen}
											aria-label={`${T.editCta} ${typeLabel} ${profile.displayValue}`}
											onClick={() => startEditing(profile)}
										>
											{T.editCta}
										</Button>
										<Button
											variant="bordered"
											color="danger"
											size="sm"
											disabled={formOpen}
											aria-label={`${T.deleteCta} ${typeLabel} ${profile.displayValue}`}
											onClick={() => openDelete(profile)}
										>
											{T.deleteCta}
										</Button>
									</div>
								</div>
							</li>
						);
					})}
				</ul>
			)}

			{deleteTarget ? (
				<Dialog
					open
					onClose={closeDelete}
					size="sm"
					title={T.deleteTitle}
					isDismissable={!deleteBusy}
					showCloseButton={!deleteBusy}
					footer={
						<>
							<Button
								variant="bordered"
								color="default"
								onClick={closeDelete}
								disabled={deleteBusy}
							>
								{T.cancel}
							</Button>
							<Button
								color="danger"
								onClick={() => void confirmDelete()}
								disabled={deleteBusy}
								isLoading={deleteBusy}
								loadingLabel={T.deleting}
							>
								{T.deleteConfirm}
							</Button>
						</>
					}
				>
					<div className="flex flex-col gap-4">
						<p className="m-0 text-sm text-foreground-soft">
							{needsNewDefault
								? T.deleteBodyDefault
								: deleteTarget.isDefault
									? T.deleteBodyLastDefault
									: T.deleteBodyPlain}
						</p>

						{needsNewDefault ? (
							<div className="flex flex-col gap-1.5">
								<Select
									id="invoice-delete-new-default"
									variant="bordered"
									size="lg"
									label={T.deleteNewDefaultLabel}
									isRequired
									// Select 的 description／error 沒有 id、也沒接 aria-describedby
									// （components/ui/Input.tsx 的既有 divergence）⇒ 說明自己渲染並手動接上。
									aria-describedby="invoice-delete-new-default-hint"
									value={newDefaultId}
									disabled={deleteBusy}
									placeholder={T.deleteNewDefaultLabel}
									onChange={(event) => setNewDefaultId(event.target.value)}
								>
									{otherProfiles.map((profile) => (
										<option key={profile.profileId} value={profile.profileId}>
											{`${invoiceCarrierLabel(profile.carrierType)}　${profile.displayValue}`}
										</option>
									))}
								</Select>
								<span
									id="invoice-delete-new-default-hint"
									className="text-xs text-foreground-soft"
								>
									{T.deleteNewDefaultHint}
								</span>
							</div>
						) : null}

						<p className="m-0 text-xs text-foreground-soft">
							{T.retryKeepsRequest}
						</p>

						{deleteError ? (
							<p
								className="m-0 text-xs font-semibold text-danger-700"
								role="alert"
							>
								{deleteError}
							</p>
						) : null}
					</div>
				</Dialog>
			) : null}
		</SellerPageShell>
	);
}
