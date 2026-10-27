"use client";

/**
 * 「販售加工食品」食品業者登錄 — 送審、撤回、更換與清除食藥署食品業者登錄字號。
 *
 * 登錄是獨立的審核資源（與商家認證分開）：賣家送出字號後，平台對照食藥署的登錄資料人工核對，
 * 通過的那一筆才是店面與商品頁公開的字號，`useCertGate` 的 `hasFoodBusinessReg` 也只看它。
 * 四個狀態對應 `Store.approved_food_registration`（已登錄）、`Store.pending_food_registration`
 * （審核中）與 `Store.rejected_food_registration`（未通過），各欄位有沒有值就是狀態：未登錄送出（SubmitFoodRegistration）、審核中撤回
 * （WithdrawFoodRegistration）、已登錄更換（再次送出，審核期間沿用目前的字號）或清除
 * （ClearFoodRegistration，立即生效）。
 *
 * 內嵌在「錢包與認證」頁的展開式 row；商家認證核准前也能送出，平台在認證通過後才核准。
 */

import type { DescMethod } from "@bufbuild/protobuf";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { fieldMessage, submitError } from "@hawkr/api/connect/submit";
import {
	AgreementKind,
	SellerAppService,
} from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import {
	sellerFoodRegistrationKycPendingNote,
	sellerFoodRegistrationStatusApproved,
	sellerFoodRegistrationStatusPending,
	sellerFoodRegistrationStatusRejected,
	sellerFoodRegistrationStatusRevoked,
} from "@hawkr/ui-text/keys";
import * as React from "react";
import Button from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { AgreementCheckbox } from "@/features/seller/components/AgreementCheckbox";
import { formatTaipeiDate } from "@/lib/date/taipei";
import { useDefaultErrorText, useUiText } from "@/lib/i18n/ui-text";
import {
	AGREEMENT_OUTDATED_NOTICE,
	acceptancesOf,
	agreementUrl,
	isAgreementOutdated,
} from "@/lib/seller/agreements";
import {
	clearFoodRegistration,
	submitFoodRegistration,
	withdrawFoodRegistration,
} from "@/lib/seller/repo";
import { foodRegistrationNumber, type MyStore } from "@/lib/seller/store";

export const FOOD_REG_NOTE =
	"上架加工食品前須完成食品業者登錄。送出後平台會核對食藥署的登錄資料，通過後才會顯示在店面與商品頁，並開放上架加工食品。審核期間沿用目前的設定。";

export const FOOD_REG_CLEAR_CONFIRM =
	"清除後，店面與商品頁不再顯示食品業者登錄字號，也無法上架加工食品類商品。";

const FOOD_REG_AGREEMENT_KINDS = [AgreementKind.FOOD_REGISTRATION_NOTICE];

export function FoodRegSupplement({
	seller,
	kycApproved = false,
	onSaved,
	bare = false,
}: {
	seller: MyStore | null;
	/** 商家認證已核准；還沒核准時審核中的狀態多一句說明。 */
	kycApproved?: boolean;
	/** 寫入成功後回呼 — 通常傳 useCertGate().refresh 讓 gate 立即翻新。 */
	onSaved?: () => void;
	/** 精簡模式：只渲染表單本體，不包 seller-card / 標題。用於嵌進展開式 row。 */
	bare?: boolean;
}) {
	const defaults = useDefaultErrorText();
	const ut = useUiText();
	const approvedNo = foodRegistrationNumber(seller?.store);
	// 狀態由欄位有沒有值表達（待審、被退回各一個欄位），不比較登錄的 state；帳號能不能操作由錢包頁的 STOREFRONT_WRITE 閘門擋。
	const pending = seller?.store?.pendingFoodRegistration ?? null;
	const rejected = seller?.store?.rejectedFoodRegistration ?? null;
	const hasApproved = approvedNo !== "";
	// 撤銷的說明由伺服器決定要不要帶（Store.revoked_food_registration），這裡原樣顯示，不自己推導。
	const revoked = seller?.store?.revokedFoodRegistration;

	const [regNo, setRegNo] = React.useState("");
	const [replacing, setReplacing] = React.useState(false);
	const [confirmingClear, setConfirmingClear] = React.useState(false);
	const [noticeRead, setNoticeRead] = React.useState(false);
	const [busy, setBusy] = React.useState(false);
	const [err, setErr] = React.useState<string | null>(null);

	const showForm = replacing || (!hasApproved && !pending);
	const normalizedRegNo = regNo.trim();

	async function run(
		method: DescMethod,
		action: () => Promise<unknown>,
		after?: () => void,
	) {
		setErr(null);
		setBusy(true);
		try {
			await action();
			after?.();
			onSaved?.();
		} catch (e) {
			if (isAgreementOutdated(e)) {
				setNoticeRead(false);
				setErr(AGREEMENT_OUTDATED_NOTICE);
				return;
			}
			// 這三支 RPC 都沒有 etag，伺服器不會回版本衝突；欄位錯誤優先顯示（與 seller-app 同序），
			// 其次伺服器的訊息，最後依錯誤碼的預設文字（submitError 已含）。
			const failure = submitError(e, defaults, method);
			if (failure.kind === "fields") {
				setErr(
					fieldMessage(failure.fields, "registration_number") ??
						failure.message ??
						Object.values(failure.fields)[0],
				);
			} else if (failure.kind === "message") {
				setErr(failure.message);
			} else {
				throw e;
			}
		} finally {
			setBusy(false);
		}
	}

	function submit() {
		void run(
			SellerAppService.method.submitFoodRegistration,
			() =>
				submitFoodRegistration(
					normalizedRegNo,
					acceptancesOf(FOOD_REG_AGREEMENT_KINDS),
				),
			() => {
				setRegNo("");
				setNoticeRead(false);
				setReplacing(false);
			},
		);
	}

	const body = (
		<div className="seller-cert__food-block seller-cert__food-supplement">
			<div className="seller-cert__food-supplement-intro">
				<strong>食品業者登錄字號</strong>
				<p>{FOOD_REG_NOTE}</p>
			</div>
			{hasApproved && (
				<div className="seller-cert__food-status" role="status">
					<p className="seller-cert__food-ok">
						{ut(sellerFoodRegistrationStatusApproved({ number: approvedNo }))}
					</p>
					{!confirmingClear && (
						<div className="seller-cert__food-actions">
							<Button
								variant="bordered"
								size="sm"
								disabled={busy}
								onClick={() => {
									setReplacing(true);
									setErr(null);
								}}
							>
								更換字號
							</Button>
							<Button
								variant="bordered"
								size="sm"
								disabled={busy}
								onClick={() => setConfirmingClear(true)}
							>
								清除
							</Button>
						</div>
					)}
				</div>
			)}
			{confirmingClear && (
				<div className="seller-cert__food-status" role="alert">
					<p>{FOOD_REG_CLEAR_CONFIRM}</p>
					<div className="seller-cert__food-actions">
						<Button
							disabled={busy}
							isLoading={busy}
							loadingLabel="清除中…"
							onClick={() =>
								void run(
									SellerAppService.method.clearFoodRegistration,
									clearFoodRegistration,
									() => setConfirmingClear(false),
								)
							}
						>
							確認清除
						</Button>
						<Button
							variant="bordered"
							disabled={busy}
							onClick={() => setConfirmingClear(false)}
						>
							取消
						</Button>
					</div>
				</div>
			)}
			{pending && (
				<div className="seller-cert__food-status" role="status">
					<p>
						{ut(
							sellerFoodRegistrationStatusPending({
								number: pending.registrationNumber,
								date: pending.submitTime
									? formatTaipeiDate(timestampDate(pending.submitTime))
									: "",
							}),
						)}
					</p>
					{!kycApproved && <p>{ut(sellerFoodRegistrationKycPendingNote())}</p>}
					<div className="seller-cert__food-actions">
						<Button
							variant="bordered"
							size="sm"
							disabled={busy}
							isLoading={busy}
							loadingLabel="撤回中…"
							onClick={() =>
								void run(
									SellerAppService.method.withdrawFoodRegistration,
									withdrawFoodRegistration,
								)
							}
						>
							撤回
						</Button>
					</div>
				</div>
			)}
			{revoked && (
				<p className="seller-cert__error" role="status">
					{ut(
						sellerFoodRegistrationStatusRevoked({
							number: revoked.registrationNumber,
							reason: revoked.endReason,
						}),
					)}
				</p>
			)}
			{rejected && (
				<p className="seller-cert__error" role="status">
					{ut(
						sellerFoodRegistrationStatusRejected({
							reason: rejected.rejectReason,
						}),
					)}
				</p>
			)}
			{showForm && (
				<>
					<Input
						label="食品業者登錄字號"
						description="食藥署食品業者登錄字號（例：A-123456789-00000-1）"
						value={regNo}
						onChange={(e) => {
							setRegNo(e.target.value.toUpperCase());
							setErr(null);
						}}
						placeholder="A-123456789-00000-1"
						isRequired
					/>
					<AgreementCheckbox
						checked={noticeRead}
						disabled={busy}
						onChange={(checked) => {
							setNoticeRead(checked);
							setErr(null);
						}}
						before="我已閱讀"
						linkText="食品業者登錄個人資料蒐集告知事項"
						href={agreementUrl(AgreementKind.FOOD_REGISTRATION_NOTICE)}
					/>
					<div className="seller-cert__food-actions">
						<Button
							onClick={submit}
							disabled={busy || !normalizedRegNo || !noticeRead}
							isLoading={busy}
							loadingLabel="送出中…"
						>
							{revoked ? "重新送出" : "送出審核"}
						</Button>
						{replacing && (
							<Button
								variant="bordered"
								disabled={busy}
								onClick={() => {
									setReplacing(false);
									setRegNo("");
									setNoticeRead(false);
									setErr(null);
								}}
							>
								取消
							</Button>
						)}
					</div>
				</>
			)}
			{err && (
				<p className="seller-cert__error" role="alert">
					{err}
				</p>
			)}
		</div>
	);

	if (bare) return body;

	return (
		<section className="seller-card">
			<header className="seller-card__head">
				<h2>販售加工食品</h2>
			</header>
			{body}
		</section>
	);
}
