"use client";

import { errorText } from "@hawkr/api/connect/errors";
import { SellerCapability } from "@hawkr/api/gen/hawkr/seller/v1alpha1/seller_pb";
import * as React from "react";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { useSellerSelf } from "@/lib/hooks/useSellerSelf";
import { useDefaultErrorText } from "@/lib/i18n/ui-text";
import { commitDocumentNavigation } from "@/lib/navigation/document-location";
import { closeMyStore } from "@/lib/seller/repo";
import { hasCapability } from "@/lib/seller/store";

/**
 * 商店設定頁底部的「關閉商店」。能不能關由 `capabilities` 的 ACCOUNT_CLOSE 決定（等待刪除時
 * 沒有，由刪除帳號的流程關閉）。關閉之後賣家角色移除、帳號與買家身分保留，所以成功後整頁重載，
 * 讓登入狀態與快取跟著重新建立。
 */
export default function CloseStoreSection() {
	const { data: my } = useSellerSelf();
	const defaults = useDefaultErrorText();
	const [open, setOpen] = React.useState(false);
	const [busy, setBusy] = React.useState(false);
	const [error, setError] = React.useState<string | null>(null);

	if (!hasCapability(my, SellerCapability.ACCOUNT_CLOSE)) return null;

	const close = () => {
		if (busy) return;
		setOpen(false);
		setError(null);
	};

	const confirm = async () => {
		if (busy) return;
		setBusy(true);
		setError(null);
		try {
			await closeMyStore();
			commitDocumentNavigation("/", true);
		} catch (e) {
			setError(errorText(e, defaults));
			setBusy(false);
		}
	};

	return (
		<section className="seller-card seller-profile__main">
			<div className="seller-card__head">
				<h2>關閉商店</h2>
			</div>
			<div className="flex flex-wrap items-center justify-between gap-3">
				<p className="m-0 text-sm text-[var(--hkui-default-700)]">
					關閉後商店與商品不再公開，賣家身分會移除；帳號與買家身分會保留。
				</p>
				<Button color="danger" variant="bordered" onClick={() => setOpen(true)}>
					關閉商店
				</Button>
			</div>
			<Dialog
				open={open}
				onClose={close}
				size="sm"
				title="關閉商店"
				footer={
					<div className="flex w-full justify-end gap-2">
						<Button
							variant="light"
							color="default"
							onClick={close}
							disabled={busy}
						>
							取消
						</Button>
						<Button
							color="danger"
							onClick={() => void confirm()}
							isLoading={busy}
							loadingLabel="關閉商店中…"
						>
							確認關閉
						</Button>
					</div>
				}
			>
				<p className="text-sm text-[var(--hkui-default-700)]">
					關閉後商店與商品不再公開，logo
					與封面會清除，賣家身分會移除；帳號與買家身分會保留。這個動作無法復原，之後要再販售需重新申請開店。有未結訂單時無法關閉。
				</p>
				{error ? (
					<p
						className="mt-2 text-sm text-[var(--hkui-danger,#d4380d)]"
						role="alert"
					>
						{error}
					</p>
				) : null}
			</Dialog>
		</section>
	);
}
