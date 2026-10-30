"use client";

import { errorText } from "@hawkr/api/connect/errors";
import { DeleteAccountResponse_DeletionState } from "@hawkr/api/gen/hawkr/identity/v1alpha1/identity_pb";
import * as React from "react";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { deleteAccount } from "@/lib/api/auth";
import { useAuth } from "@/lib/auth/context";
import { useDefaultErrorText } from "@/lib/i18n/ui-text";
import { commitDocumentNavigation } from "@/lib/navigation/document-location";

type Outcome =
	| { kind: "idle" }
	| { kind: "pending_orders" }
	| { kind: "error"; message: string };

const STORE_CLOSURE_NOTICE =
	"你的商店也會一起關閉，商店與商品不再公開，賣家身分會移除。有未結訂單時，帳號與商店會在訂單結清後才刪除，期間商店不能自行關閉或換店。";

/**
 * 設定頁的「刪除帳號」。沒有未完成的訂單時立即刪除並登出；有的話進入刪除流程，最後一筆訂單完成後
 * 自動刪除。
 */
export default function DeleteAccountSection() {
	const { signOut, hasSeller } = useAuth();
	const defaults = useDefaultErrorText();
	const [open, setOpen] = React.useState(false);
	const [busy, setBusy] = React.useState(false);
	const [outcome, setOutcome] = React.useState<Outcome>({ kind: "idle" });

	const close = () => {
		if (busy) return;
		setOpen(false);
	};

	const confirm = async () => {
		if (busy) return;
		setBusy(true);
		setOutcome({ kind: "idle" });
		try {
			const state = await deleteAccount();
			if (state === DeleteAccountResponse_DeletionState.PENDING_ORDERS) {
				setOutcome({ kind: "pending_orders" });
				setOpen(false);
				return;
			}
			if (state !== DeleteAccountResponse_DeletionState.DELETED) {
				setOutcome({ kind: "error", message: "刪除帳號失敗，請稍後再試" });
				return;
			}
			// 已刪除：session 不再有效，登出（清掉本機狀態）後整頁回首頁。
			try {
				await signOut();
			} finally {
				commitDocumentNavigation("/");
			}
		} catch (e) {
			setOutcome({ kind: "error", message: errorText(e, defaults) });
		} finally {
			setBusy(false);
		}
	};

	return (
		<section className="seller-card">
			<header className="seller-card__head">
				<h2>刪除帳號</h2>
			</header>
			<div className="flex flex-wrap items-center justify-between gap-3">
				<p className="m-0 text-sm text-foreground-soft">
					刪除後帳號資料會被移除，無法復原。
				</p>
				<Button
					color="danger"
					variant="bordered"
					onClick={() => {
						setOutcome({ kind: "idle" });
						setOpen(true);
					}}
				>
					刪除帳號
				</Button>
			</div>
			{outcome.kind === "pending_orders" ? (
				<p className="mt-3 mb-0 text-sm text-hk-heading" role="status">
					帳號刪除處理中，最後一筆未完成的訂單完成後會自動刪除；期間無法新增收藏與追蹤。
				</p>
			) : null}
			<Dialog
				open={open}
				onClose={close}
				size="sm"
				title="刪除帳號"
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
							loadingLabel="刪除帳號中…"
						>
							確認刪除
						</Button>
					</div>
				}
			>
				<p className="text-sm text-[var(--hkui-default-700)]">
					刪除後帳號資料會被移除，無法復原。還有未完成的訂單時，帳號會在最後一筆訂單完成後刪除，期間無法新增收藏與追蹤。
				</p>
				{hasSeller ? (
					<p className="mt-2 text-sm text-[var(--hkui-default-700)]">
						{STORE_CLOSURE_NOTICE}
					</p>
				) : null}
				{outcome.kind === "error" ? (
					<p
						className="mt-2 text-sm text-[var(--hkui-danger,#d4380d)]"
						role="alert"
					>
						{outcome.message}
					</p>
				) : null}
			</Dialog>
		</section>
	);
}
