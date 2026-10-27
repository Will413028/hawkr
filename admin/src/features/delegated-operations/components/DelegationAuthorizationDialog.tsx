"use client";

import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { type FormEvent, useState } from "react";
import { Button, Dialog } from "@/components/UI";
import {
	DELEGATION_AUTHORIZATION_SOURCES,
	DELEGATION_REASON_MAX_LENGTH,
	DELEGATION_SOURCE_REFERENCE_MAX_LENGTH,
	type DelegationAuthorization,
	type DelegationAuthorizationSource,
	type DelegationCapabilityCatalog,
	delegationCapabilityRequiredBy,
	deselectDelegationCapability,
	isDelegationOperationReason,
	isDelegationSourceReference,
	missingDelegationCapabilityDependencies,
	selectDelegationCapability,
	toDelegationEffectiveUntil,
	toDelegationLocalDateTime,
	unknownDelegationCapabilities,
} from "@/features/delegated-operations/authorization";
import {
	DELEGATED_AUDIT_RISK_LABELS,
	DELEGATION_AUTHORIZATION_SOURCE_HINTS,
	DELEGATION_AUTHORIZATION_SOURCE_LABELS,
	delegationCapabilityLabel,
} from "@/features/delegated-operations/labels";
import {
	createDelegatedOperationIdempotencyKey,
	delegationAuthorizationErrorText,
	grantDelegationAuthorization,
	updateDelegationAuthorization,
} from "@/services/delegated-operations";

/**
 * 開通／調整代行授權範圍。
 *
 * 骨架沿用 `OperationPreflightDialog`：loading／blocked／表單三個狀態互斥，**blocked
 * 不會退化成一個看起來可以送出的表單**。這裡 blocked 的成因只有一個——拿不到能力
 * 目錄。目錄是依賴閉包的唯一權威（`GET …/capabilities`），沒有它就無從決定「勾了
 * A 要不要連帶勾 B」，而猜錯的兩種結果（畫面擋掉合法組合／放行後被 9287 拒絕）都
 * 不會有任何東西轉紅。因此是 fail-closed：不開表單，不退回本地表。
 *
 * ⚠️ 送出走 `fetch`（同源、不跨 host），與 `OperationPreflightDialog` 的原生表單
 * 導覽不同——那一支是因為跨 host 的 303 只有瀏覽器跟得動。
 *
 * ⚠️ 冪等鍵：**內容改變就換一把**（那是另一個請求，不該被上一個請求的冪等結果
 * 接住），但**送出失敗不換**（開通可能已經生效而回應沒送到，換鍵會讓後端把重試當成
 * 第二次開通並回 9285）。
 */

interface CatalogLoadError {
	title: string;
	message: string;
}

interface DelegationAuthorizationDialogProps {
	/** grant ＝ 首建或復權（後端以同一支作答，實際分支由回應的 outcome 回報）。 */
	mode: "grant" | "update";
	storeId: string;
	storeName: string;
	/** 目前這一列（可能是已撤回的舊列）。grant 模式僅供對照，不預填範圍。 */
	current: DelegationAuthorization | null;
	catalog?: DelegationCapabilityCatalog;
	catalogLoading: boolean;
	catalogError?: CatalogLoadError;
	onRetryCatalog?: () => void;
	onClose: () => void;
	onSaved: (message: string) => void;
}

function safeIdempotencyKey(): string {
	try {
		return createDelegatedOperationIdempotencyKey();
	} catch {
		return "";
	}
}

export default function DelegationAuthorizationDialog({
	mode,
	storeId,
	storeName,
	current,
	catalog,
	catalogLoading,
	catalogError,
	onRetryCatalog,
	onClose,
	onSaved,
}: DelegationAuthorizationDialogProps) {
	const isUpdate = mode === "update";

	const [source, setSource] = useState<DelegationAuthorizationSource | "">(
		isUpdate && current ? current.source : "",
	);
	const [sourceReference, setSourceReference] = useState(
		isUpdate && current ? current.sourceReference : "",
	);
	/**
	 * 首次開通（含復權）一律從空集合開始 —— 設計書「首批一律最小必要子集」。預填上
	 * 一次的範圍會讓「復權」實際上變成「原樣還原」，而撤回的理由往往正是範圍太寬。
	 */
	const [selected, setSelected] = useState<string[]>(
		isUpdate && current ? current.grantedCapabilities : [],
	);
	const [unlimited, setUnlimited] = useState(
		isUpdate && current ? current.effectiveUntil === null : true,
	);
	const [effectiveUntilLocal, setEffectiveUntilLocal] = useState(
		isUpdate && current
			? toDelegationLocalDateTime(current.effectiveUntil)
			: "",
	);
	const [operationReason, setOperationReason] = useState("");
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);
	const [idempotencyKey, setIdempotencyKey] = useState(safeIdempotencyKey);
	const renewIdempotencyKey = () => setIdempotencyKey(safeIdempotencyKey());

	const waitingForCatalog = catalogLoading || (!catalog && !catalogError);
	const selectedSet = new Set(selected);
	const requiredBy = catalog
		? delegationCapabilityRequiredBy(catalog, selected)
		: new Map<string, string[]>();
	const missing = catalog
		? missingDelegationCapabilityDependencies(catalog, selected)
		: [];
	const unknown = catalog
		? unknownDelegationCapabilities(catalog, selected)
		: [];

	const touch = () => {
		renewIdempotencyKey();
		setError(null);
	};

	const toggleCapability = (value: string) => {
		if (!catalog) return;
		setSelected((previous) =>
			selectedSet.has(value)
				? deselectDelegationCapability(catalog, previous, value)
				: selectDelegationCapability(catalog, previous, value),
		);
		touch();
	};

	const clearCapabilities = () => {
		setSelected([]);
		touch();
	};

	const titleText = isUpdate ? "調整代行授權範圍" : "開通代行授權";
	const submitText = isUpdate ? "儲存授權範圍" : "開通授權";

	const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		if (!catalog) {
			setError("尚未取得代行能力目錄，已阻擋送出。");
			return;
		}
		if (!source) {
			setError("請選擇授權依據。");
			return;
		}
		if (!isDelegationSourceReference(sourceReference)) {
			setError("請填寫 4–200 字且至少含一個數字的依據編號。");
			return;
		}
		if (selected.length === 0) {
			setError("請至少勾選一項代行能力。");
			return;
		}
		if (unknown.length > 0) {
			// 版本落差：既有授權列帶著本版目錄沒有的能力。靜默送出會把它們一併撤掉，
			// 而畫面從未顯示過它們 —— 操作者不會知道自己縮小了範圍。
			setError(
				`目前的授權含有這個版本無法辨識的能力（${unknown.join("、")}），已停止送出，請聯絡技術人員確認 ADM 版本。`,
			);
			return;
		}
		if (missing.length > 0) {
			setError(
				`能力相依關係不完整，還缺少：${missing.map(delegationCapabilityLabel).join("、")}。`,
			);
			return;
		}
		if (!isDelegationOperationReason(operationReason)) {
			setError("請填寫 1–200 字的操作理由，它會寫入治理紀錄。");
			return;
		}
		const effectiveUntil = unlimited
			? null
			: toDelegationEffectiveUntil(effectiveUntilLocal);
		if (!unlimited && effectiveUntil === null) {
			setError("請填寫有效的到期時間，或改選「無期限」。");
			return;
		}
		if (!idempotencyKey) {
			setError("瀏覽器無法建立安全的操作識別碼，請更新瀏覽器後再試。");
			return;
		}

		setError(null);
		setSubmitting(true);
		const scope = {
			source,
			sourceReference: sourceReference.trim(),
			grantedCapabilities: selected,
			effectiveUntil,
			operationReason: operationReason.trim(),
		};
		try {
			if (isUpdate) {
				await updateDelegationAuthorization({
					storeId,
					scope,
					idempotencyKey,
				});
				onSaved("已更新代行授權範圍。");
				return;
			}
			const granted = await grantDelegationAuthorization({
				storeId,
				scope,
				idempotencyKey,
			});
			// created／reinstated 由伺服端依實際走到的 CAS 分支填入，不由這裡推定
			// ——畫面說「已重新開通」而後端其實是首建，兩者的稽核意義不同。
			onSaved(
				granted.outcome === "reinstated"
					? "已重新開通代行授權。"
					: "已開通代行授權。",
			);
		} catch (caught) {
			setError(
				delegationAuthorizationErrorText(
					caught,
					"授權設定失敗，請重試；重試會沿用同一次請求。",
				),
			);
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			variant="operation"
			size="preflight"
			title={titleText}
			titleIcon="shield_person"
			description="授權決定「這家店可不可以被代行、範圍多大」；每一次變更都會寫入授權變更歷程。"
			closeLabel={`關閉${titleText}視窗`}
			onClose={onClose}
			busy={submitting}
			ariaBusy={waitingForCatalog || submitting}
			form={catalog ? { onSubmit: handleSubmit } : undefined}
			footer={
				catalog ? (
					<>
						<Button
							variant="ghost"
							type="button"
							onClick={onClose}
							disabled={submitting}
						>
							取消
						</Button>
						<Button
							type="submit"
							loading={submitting}
							disabled={selected.length === 0}
						>
							{submitting ? "正在儲存" : submitText}
						</Button>
					</>
				) : (
					<>
						<Button variant="ghost" type="button" onClick={onClose}>
							關閉
						</Button>
						{!waitingForCatalog && onRetryCatalog && (
							<Button variant="outline" type="button" onClick={onRetryCatalog}>
								重新取得目錄
							</Button>
						)}
					</>
				)
			}
		>
			{catalog ? (
				<>
					<div className="adm-operation-identity-grid">
						<div className="adm-operation-identity-card adm-operation-identity-card--target">
							<span className="adm-operation-identity-label">
								<MaterialSymbol name="storefront" />
								授權對象
							</span>
							<strong>{storeName}</strong>
							<span>Store ID · {storeId}</span>
						</div>
						<div className="adm-operation-identity-card">
							<span className="adm-operation-identity-label">
								<MaterialSymbol name="history" />
								目前狀態
							</span>
							<strong>
								{current
									? current.revokedAt
										? "已撤回"
										: current.effective
											? "授權中"
											: "目前無效"
									: "尚未開通"}
							</strong>
							<span>
								{current
									? `${current.grantedCapabilities.length} 項能力 · ${current.grantedCapabilities.map(delegationCapabilityLabel).join("、")}`
									: "這家店還沒有任何代行授權紀錄。"}
							</span>
							{!isUpdate && current ? (
								<em>復權是一次全新授權，上方範圍僅供對照</em>
							) : null}
						</div>
					</div>

					<fieldset className="adm-operation-reasons">
						<legend>授權依據（必選）</legend>
						<p>「憑什麼授權這家店」必須指得出一份可查的東西。</p>
						<div className="adm-operation-reason-grid">
							{DELEGATION_AUTHORIZATION_SOURCES.map((option) => (
								<label
									className={`adm-operation-reason-card${source === option ? " is-selected" : ""}`}
									key={option}
								>
									<input
										type="radio"
										name="authorizationSource"
										value={option}
										checked={source === option}
										onChange={() => {
											setSource(option);
											touch();
										}}
										required
									/>
									<span>
										<strong>
											{DELEGATION_AUTHORIZATION_SOURCE_LABELS[option]}
										</strong>
										<small>
											{DELEGATION_AUTHORIZATION_SOURCE_HINTS[option]}
										</small>
									</span>
								</label>
							))}
						</div>
					</fieldset>

					<div className="adm-operation-field adm-operation-field--wide">
						<label htmlFor="adm-authorization-reference">
							依據編號（必填）
						</label>
						<input
							id="adm-authorization-reference"
							type="text"
							value={sourceReference}
							onChange={(event) => {
								setSourceReference(event.target.value);
								touch();
							}}
							placeholder="例如 CT-2026-0188"
							maxLength={DELEGATION_SOURCE_REFERENCE_MAX_LENGTH}
							required
							autoComplete="off"
						/>
						<span className="adm-operation-field-note">
							4–200
							字且至少含一個數字（合約號、工單號或政策文號）；不要填密碼或個資。
						</span>
					</div>

					<section
						className="adm-operation-section"
						aria-labelledby="adm-authorization-scope-title"
					>
						<div className="adm-operation-section-heading">
							<div>
								<h3 id="adm-authorization-scope-title">授權範圍（至少一項）</h3>
								<p>
									相依規則由伺服器下發：勾選編輯或發布會自動補上對應的檢視權限。首批請只給最小必要子集。
								</p>
							</div>
							<button
								type="button"
								className="adm-authorization-clear"
								onClick={clearCapabilities}
								disabled={selected.length === 0}
							>
								<MaterialSymbol name="ink_eraser" />
								全部取消
							</button>
						</div>
						<div className="adm-authorization-capability-list">
							{catalog.capabilities.map((descriptor) => {
								const dependents = requiredBy.get(descriptor.value) ?? [];
								const locked = dependents.length > 0;
								const checked = selectedSet.has(descriptor.value);
								const dependentNames = dependents
									.map(delegationCapabilityLabel)
									.join("」、「");
								return (
									<label
										className={`adm-authorization-capability${checked ? " is-selected" : ""}${locked ? " is-locked" : ""}`}
										key={descriptor.value}
									>
										<input
											type="checkbox"
											checked={checked}
											disabled={locked || submitting}
											onChange={() => toggleCapability(descriptor.value)}
										/>
										<span>
											<strong>
												{delegationCapabilityLabel(descriptor.value)}
											</strong>
											<small>
												{descriptor.value} ·{" "}
												{DELEGATED_AUDIT_RISK_LABELS[descriptor.risk]}
											</small>
											{locked ? (
												<em>
													由「{dependentNames}」連帶；要取消它，請先取消「
													{dependentNames}」
												</em>
											) : null}
										</span>
									</label>
								);
							})}
						</div>
						{selected.length === 0 ? (
							<p className="adm-authorization-scope-hint">
								未勾選任何能力等於沒有授權。若要停止代行，請直接使用「撤回授權」。
							</p>
						) : null}
					</section>

					<fieldset className="adm-operation-reasons">
						<legend>授權期限</legend>
						<p>到期後授權自動失效，不需要人記得回來撤回。</p>
						<div className="adm-operation-reason-grid">
							<label
								className={`adm-operation-reason-card${unlimited ? " is-selected" : ""}`}
							>
								<input
									type="radio"
									name="authorizationExpiry"
									checked={unlimited}
									onChange={() => {
										setUnlimited(true);
										touch();
									}}
								/>
								<span>
									<strong>無期限</strong>
									<small>需要人工撤回才會停止</small>
								</span>
							</label>
							<label
								className={`adm-operation-reason-card${unlimited ? "" : " is-selected"}`}
							>
								<input
									type="radio"
									name="authorizationExpiry"
									checked={!unlimited}
									onChange={() => {
										setUnlimited(false);
										touch();
									}}
								/>
								<span>
									<strong>指定到期時間</strong>
									<small>到期即自動失效</small>
								</span>
							</label>
						</div>
					</fieldset>

					{!unlimited && (
						<div className="adm-operation-field">
							<label htmlFor="adm-authorization-until">到期時間（必填）</label>
							<input
								id="adm-authorization-until"
								type="datetime-local"
								value={effectiveUntilLocal}
								onChange={(event) => {
									setEffectiveUntilLocal(event.target.value);
									touch();
								}}
								required
							/>
							<span className="adm-operation-field-note">
								以你所在時區輸入，送出時換算成 UTC 秒精度。
							</span>
						</div>
					)}

					<div className="adm-operation-field adm-operation-field--wide">
						<label htmlFor="adm-authorization-reason">操作理由（必填）</label>
						<textarea
							id="adm-authorization-reason"
							value={operationReason}
							onChange={(event) => {
								setOperationReason(event.target.value);
								touch();
							}}
							maxLength={DELEGATION_REASON_MAX_LENGTH}
							rows={3}
							placeholder="例如：依合約 CT-2026-0188 協助賣家上架初期商品"
							required
						/>
						<span className="adm-operation-field-note">
							理由會寫入授權變更歷程，最多 200 字。
						</span>
					</div>

					{error && (
						<div className="adm-operation-error" role="alert">
							<MaterialSymbol name="error" />
							<div>{error}</div>
						</div>
					)}
				</>
			) : waitingForCatalog ? (
				<div className="adm-operation-preflight-loading" role="status">
					<MaterialSymbol name="progress_activity" />
					<div>
						<strong>正在取得代行能力目錄</strong>
						<span>能力的相依規則由伺服器決定，瀏覽器不自行推定。</span>
					</div>
				</div>
			) : (
				<div className="adm-operation-preflight-blocked" role="alert">
					<MaterialSymbol name="gpp_bad" />
					<div>
						<strong>{catalogError?.title ?? "目前無法設定代行授權"}</strong>
						<p>
							{catalogError?.message ??
								"尚未取得代行能力目錄，已停止開啟設定表單。"}
						</p>
					</div>
				</div>
			)}
		</Dialog>
	);
}
