"use client";

import { MaterialSymbol } from "@hawkr/icons/material-symbol";
import { type FormEvent, useState } from "react";
import { Button, Dialog } from "@/components/UI";
import {
	DELEGATED_OPERATION_CAPABILITY_LABELS,
	DELEGATED_OPERATION_PROHIBITED_LABELS,
} from "@/features/delegated-operations/labels";
import type { DelegatedOperationPreflight } from "@/features/delegated-operations/model";
import { createDelegatedOperationIdempotencyKey } from "@/services/delegated-operations";

/**
 * 工作區在另一個 host。送出走**原生表單導覽**而不是 `fetch`：跨 host 的 303
 * 只有瀏覽器跟得動（`fetch` 的 `redirect: "manual"` 拿到的是 opaqueredirect，
 * 讀不到 `Location`；`redirect: "follow"` 則會由 fetch 自己去打 www，一次性
 * 兌換碼當場燒掉、cookie 也落不到瀏覽器上）。
 *
 * ⚠️ 因此表單控制項**不得**在送出當下被 `disabled` —— React 會在預設行為之前
 * 沖掉 state 更新，被停用的欄位不會進到序列化結果裡。要擋重複送出只停用按鈕。
 *
 * 表單帶 `noValidate`：`required` 的原生氣泡會在 submit 事件**之前**攔下送出，於是
 * `handleSubmit` 裡兩則寫好的訊息（「請選擇代行原因。」與「請填寫可追溯的案件編號或
 * 工單編號。」）從來沒有機會出現 —— 操作者看到的是通用的「請填寫這個欄位。」，而那個
 * 氣泡還正好蓋住下方寫著格式要求的說明文字。關掉原生提示後由 `handleSubmit` 全權負責
 * （它已逐項檢查），錯誤走本元件自己的 `.adm-operation-error`；`route.ts` 的
 * `CASE_REFERENCE_REQUIRED` 仍是最後一道，關掉的只是瀏覽器那一層提示，不是驗證本身。
 * `required` 屬性保留：它同時是 a11y 語意（對 AT 曝光 `aria-required`）。
 */
const START_ACTION = "/api/admin/delegated-operations";

interface OperationPreflightLoadError {
	title: string;
	message: string;
	traceId?: string;
	/** preflight 的業務碼（9264／9269／9272／9286…）。文字說明給人看，這個給客服／工單追。 */
	deniedCode?: number;
}

/** Shorter windows the operator may pick; the server-issued value is always
 *  offered as well and is the longest option. */
const TTL_SHORTCUT_SECONDS = [5 * 60, 10 * 60, 15 * 60] as const;

function ttlOptionsFor(ttlSeconds: number): number[] {
	return [
		...TTL_SHORTCUT_SECONDS.filter((seconds) => seconds < ttlSeconds),
		ttlSeconds,
	];
}

/** Dev environments run TTL and idle windows down to a few seconds, so a
 *  minutes-only label would round them to a number the server never uses. */
function formatDurationLabel(seconds: number): string {
	return seconds % 60 === 0 ? `${seconds / 60} 分鐘` : `${seconds} 秒`;
}

/** 舊瀏覽器沒有 `crypto.randomUUID`；空字串在送出時被擋下並顯示原因。 */
function safeIdempotencyKey(): string {
	try {
		return createDelegatedOperationIdempotencyKey();
	} catch {
		return "";
	}
}

interface OperationPreflightDialogProps {
	preflight?: DelegatedOperationPreflight;
	requestedStoreId: string;
	loading?: boolean;
	loadError?: OperationPreflightLoadError;
	/** 上一次送出失敗時帶回來的 key，讓同一次申請的重試仍是同一個請求。 */
	resumedIdempotencyKey?: string;
	onRetry?: () => void;
	onClose: () => void;
}

export default function OperationPreflightDialog({
	preflight,
	requestedStoreId,
	loading = false,
	loadError,
	resumedIdempotencyKey,
	onRetry,
	onClose,
}: OperationPreflightDialogProps) {
	const [reasonCode, setReasonCode] = useState("");
	const [caseReference, setCaseReference] = useState("");
	const [chosenTtlSeconds, setChosenTtlSeconds] = useState<number | null>(null);
	const [validationError, setValidationError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);
	// 申請內容改變就換一把 key：那是另一個請求，不該被上一個請求的冪等結果接住。
	const [idempotencyKey, setIdempotencyKey] = useState(
		() => resumedIdempotencyKey ?? safeIdempotencyKey(),
	);
	const renewIdempotencyKey = () => setIdempotencyKey(safeIdempotencyKey());

	const ttlOptions = preflight ? ttlOptionsFor(preflight.ttlSeconds) : [];
	const ttlSeconds =
		chosenTtlSeconds !== null && ttlOptions.includes(chosenTtlSeconds)
			? chosenTtlSeconds
			: (preflight?.ttlSeconds ?? 0);
	const selectedReason = preflight?.reasonOptions.find(
		(option) => option.code === reasonCode,
	);
	// preflight 頂層的 `caseReferenceRequired` 是「任一選項需要」的 OR，不是
	// 「這一次要填」；本次是否必填只由被選中的那個原因決定，否則卡片上寫著
	// 「案件編號可選填」的原因照樣會在送出時被擋。
	const caseRequired = selectedReason?.caseReferenceRequired === true;
	const readCapabilities =
		preflight?.capabilities.filter((capability) =>
			capability.endsWith(".read"),
		) ?? [];
	const writeCapabilities =
		preflight?.capabilities.filter(
			(capability) => !capability.endsWith(".read"),
		) ?? [];
	const waitingForPreflight = loading || (!preflight && !loadError);

	const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
		const reject = (message: string) => {
			event.preventDefault();
			setValidationError(message);
		};
		setValidationError(null);
		if (!preflight || preflight.target.storeId !== requestedStoreId) {
			reject("尚未取得與目前店家相符的代行資格，已阻擋建立工作階段。");
			return;
		}
		if (!reasonCode) {
			reject("請選擇代行原因。");
			return;
		}
		if (caseRequired && !caseReference.trim()) {
			reject("請填寫可追溯的案件編號或工單編號。");
			return;
		}
		if (!idempotencyKey) {
			reject("瀏覽器無法建立安全的操作識別碼，請更新瀏覽器後再試。");
			return;
		}
		// 不呼叫 preventDefault：瀏覽器接手送出並跟著 303 走到工作區所在的 host。
		setSubmitting(true);
	};

	const displayError = validationError;

	return (
		<Dialog
			variant="operation"
			size="preflight"
			title="開始代行營運"
			titleIcon="shield_lock"
			description="你仍會以 ADM 身分操作；目標店家、原因、核發範圍與每次變更都會留下稽核紀錄。"
			closeLabel="關閉開始代行營運視窗"
			onClose={onClose}
			busy={submitting}
			ariaBusy={waitingForPreflight || submitting}
			form={
				preflight
					? { method: "post", action: START_ACTION, onSubmit: handleSubmit }
					: undefined
			}
			footerClassName={
				preflight ? "adm-operation-dialog-actions--preflight" : undefined
			}
			footer={
				preflight ? (
					<>
						<span>
							<MaterialSymbol name="policy" />
							建立後，工作列會持續顯示 ADM、店家與剩餘時間
						</span>
						<div>
							<Button
								variant="ghost"
								type="button"
								onClick={onClose}
								disabled={submitting}
							>
								取消
							</Button>
							<Button type="submit" loading={submitting}>
								{submitting ? "正在前往代行工作區" : "建立工作階段"}
							</Button>
						</div>
					</>
				) : (
					<>
						<Button variant="ghost" type="button" onClick={onClose}>
							關閉
						</Button>
						{!waitingForPreflight && onRetry && (
							<Button variant="outline" type="button" onClick={onRetry}>
								重新確認資格
							</Button>
						)}
					</>
				)
			}
		>
			{preflight ? (
				<>
					<input
						type="hidden"
						name="storeId"
						value={preflight.target.storeId}
					/>
					<input type="hidden" name="idempotencyKey" value={idempotencyKey} />
					<div className="adm-operation-identity-grid">
						<div className="adm-operation-identity-card adm-operation-identity-card--target">
							<span className="adm-operation-identity-label">
								<MaterialSymbol name="storefront" />
								代行目標
							</span>
							<strong>{preflight.target.storeName}</strong>
							<span>Store ID · {preflight.target.storeId}</span>
							<span>Account ID · {preflight.target.sellerAccountId}</span>
						</div>
						<div className="adm-operation-identity-card">
							<span className="adm-operation-identity-label">
								<MaterialSymbol name="admin_panel_settings" />
								實際操作者
							</span>
							<strong>{preflight.actor.displayName}</strong>
							<span>ADM Admin ID · {preflight.actor.adminId}</span>
							<em>不會切換成賣家登入身分</em>
						</div>
					</div>

					<fieldset className="adm-operation-reasons">
						{/* 必填標示與下方案件欄同一種寫法。少了它，這一組的必填性要等到
								    送出被擋、handleSubmit 吐「請選擇代行原因。」才第一次出現，而
								    同一張表單的案件欄一進場就講明了。 */}
						<legend>這次為什麼需要代行？（必選）</legend>
						<p>請選擇伺服器核准的原因；原因會和後續操作一起寫入稽核紀錄。</p>
						<div className="adm-operation-reason-grid">
							{preflight.reasonOptions.map((option) => (
								<label
									className={`adm-operation-reason-card${reasonCode === option.code ? " is-selected" : ""}`}
									key={option.code}
								>
									<input
										type="radio"
										name="reasonCode"
										value={option.code}
										checked={reasonCode === option.code}
										onChange={() => {
											setReasonCode(option.code);
											renewIdempotencyKey();
											setValidationError(null);
										}}
										required
									/>
									<span>
										<strong>{option.label}</strong>
										<small>
											{option.caseReferenceRequired
												? "需綁定案件或工單編號"
												: "案件編號可選填"}
										</small>
									</span>
								</label>
							))}
						</div>
					</fieldset>

					<div className="adm-operation-field adm-operation-field--wide">
						{/* 必填與否由被選中的原因決定（radio 小字說的就是這件事）。
								    還沒選原因時兩個答案都還不成立，所以不加後綴 —— 先寫
								    「（選填）」等於在資料出來之前先給一個宣稱。 */}
						<label htmlFor="adm-operation-case">
							案件或工單編號
							{selectedReason ? (caseRequired ? "（必填）" : "（選填）") : ""}
						</label>
						<input
							id="adm-operation-case"
							name="caseReference"
							type="text"
							value={caseReference}
							onChange={(event) => {
								setCaseReference(event.target.value);
								renewIdempotencyKey();
								setValidationError(null);
							}}
							placeholder="例如 CS-20260806-001"
							maxLength={200}
							required={caseRequired}
							autoComplete="off"
						/>
						<span className="adm-operation-field-note">
							請填可追溯的客服、招商或營運工單；不要填密碼、身分證或付款資料。
						</span>
					</div>

					<div className="adm-operation-field">
						<label htmlFor="adm-operation-ttl">代行工作階段時長</label>
						<select
							id="adm-operation-ttl"
							name="ttlSeconds"
							value={String(ttlSeconds)}
							onChange={(event) => {
								setChosenTtlSeconds(Number(event.target.value));
								renewIdempotencyKey();
								setValidationError(null);
							}}
						>
							{ttlOptions.map((seconds) => (
								<option key={seconds} value={String(seconds)}>
									{formatDurationLabel(seconds)}
								</option>
							))}
						</select>
						<span className="adm-operation-field-note">
							只能選擇不長於伺服器核發的窗口；閒置{" "}
							{formatDurationLabel(preflight.idleSeconds)}
							後自動結束。
						</span>
					</div>

					<section
						className="adm-operation-section"
						aria-labelledby="scope-title"
					>
						<div className="adm-operation-section-heading">
							<div>
								<h3 id="scope-title">伺服器核發的操作範圍</h3>
								<p>範圍只供確認，無法在瀏覽器新增權限。</p>
							</div>
							<span className="adm-operation-ttl">
								<MaterialSymbol name="schedule" />
								最長 {formatDurationLabel(ttlSeconds)}
							</span>
						</div>
						<div className="adm-operation-scope-groups">
							<div className="adm-operation-scope-group">
								<strong>
									<MaterialSymbol name="visibility" />
									可查看
								</strong>
								<div className="adm-operation-scope-list">
									{readCapabilities.map((capability) => (
										<span className="adm-operation-scope" key={capability}>
											{DELEGATED_OPERATION_CAPABILITY_LABELS[capability]}
										</span>
									))}
								</div>
							</div>
							<div className="adm-operation-scope-group adm-operation-scope-group--write">
								<strong>
									<MaterialSymbol name="edit" />
									可變更
								</strong>
								{writeCapabilities.length > 0 ? (
									<div className="adm-operation-scope-list">
										{writeCapabilities.map((capability) => (
											<span className="adm-operation-scope" key={capability}>
												{DELEGATED_OPERATION_CAPABILITY_LABELS[capability]}
											</span>
										))}
									</div>
								) : (
									<span className="adm-operation-scope-empty">
										此工作階段沒有內容寫入權限
									</span>
								)}
							</div>
						</div>
					</section>

					<div className="adm-operation-boundary">
						<div className="adm-operation-boundary-heading">
							<MaterialSymbol name="lock_person" />
							<div>
								<strong>賣家專屬，永遠不會核發</strong>
								<span>直接呼叫 API 也會被拒絕</span>
							</div>
						</div>
						<ul>
							{DELEGATED_OPERATION_PROHIBITED_LABELS.map((label) => (
								<li key={label}>{label}</li>
							))}
						</ul>
					</div>

					{displayError && (
						<div className="adm-operation-error" role="alert">
							<MaterialSymbol name="error" />
							<div>{displayError}</div>
						</div>
					)}
				</>
			) : waitingForPreflight ? (
				<div className="adm-operation-preflight-loading" role="status">
					<MaterialSymbol name="progress_activity" />
					<div>
						<strong>正在向伺服器確認代行資格</strong>
						<span>瀏覽器不會自行決定權限或工作階段時間。</span>
					</div>
				</div>
			) : (
				<div className="adm-operation-preflight-blocked" role="alert">
					<MaterialSymbol name="gpp_bad" />
					<div>
						<strong>{loadError?.title ?? "目前無法開始代行"}</strong>
						<p>
							{loadError?.message ??
								"伺服器尚未證明目前店家可被代行，已維持封鎖。"}
						</p>
						{loadError?.deniedCode ? (
							<span className="adm-operation-trace">
								原因代碼：{loadError.deniedCode}
							</span>
						) : null}
						{loadError?.traceId && (
							<span className="adm-operation-trace">
								追蹤編號：{loadError.traceId}
							</span>
						)}
					</div>
				</div>
			)}
		</Dialog>
	);
}
