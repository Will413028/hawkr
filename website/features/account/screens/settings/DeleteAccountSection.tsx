"use client";

import { useUser } from "@clerk/nextjs";
import { ConnectError } from "@connectrpc/connect";
import {
	classifyError,
	errorText,
	factsFromError,
} from "@hawkr/api/connect/errors";
import { ErrorReason } from "@hawkr/api/gen/hawkr/common/v1alpha1/error_reason_pb";
import { DeleteAccountResponse_DeletionState } from "@hawkr/api/gen/hawkr/identity/v1alpha1/identity_pb";
import * as React from "react";
import Link from "@/components/shared/Link";
import Button from "@/components/ui/Button";
import { Dialog } from "@/components/ui/Dialog";
import { deleteAccount } from "@/lib/api/auth";
import { useAuth } from "@/lib/auth/context";
import { useDefaultErrorText } from "@/lib/i18n/ui-text";
import { commitDocumentNavigation } from "@/lib/navigation/document-location";

type Outcome =
	| { kind: "idle" }
	| { kind: "has_store" }
	| { kind: "pending_orders" }
	| { kind: "error"; message: string };

const APPLE_REVOKE_GUIDANCE =
	"刪除後請到 Apple ID 設定的「使用 Apple 登入」移除 Hawkr 的授權。";

/**
 * 設定頁的「刪除帳號」。沒有未完成的訂單時立即刪除並登出；有的話進入刪除流程，最後一筆訂單完成後
 * 自動刪除。帳號還有沒關閉的店家時伺服器回 failed_precondition 與 ACCOUNT_HAS_OPEN_STORE，這裡引導先到商店設定關閉商店。
 */
export default function DeleteAccountSection() {
	const { signOut } = useAuth();
	const { user } = useUser();
	const defaults = useDefaultErrorText();
	const hasAppleAccount =
		user?.externalAccounts.some((account) => account.provider === "apple") ??
		false;
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
			// 已刪除：token 不再有效，結束 Clerk 的 session 後整頁回首頁。
			try {
				await signOut();
			} finally {
				commitDocumentNavigation("/");
			}
		} catch (e) {
			const decision = classifyError(factsFromError(ConnectError.from(e)));
			if (
				decision.action === "reason" &&
				decision.reason === ErrorReason.ACCOUNT_HAS_OPEN_STORE
			) {
				setOutcome({ kind: "has_store" });
			} else {
				setOutcome({ kind: "error", message: errorText(e, defaults) });
			}
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
				{hasAppleAccount ? (
					<p className="mt-2 text-sm text-[var(--hkui-default-700)]">
						{APPLE_REVOKE_GUIDANCE}
					</p>
				) : null}
				{outcome.kind === "has_store" ? (
					<p
						className="mt-2 text-sm text-[var(--hkui-danger,#d4380d)]"
						role="alert"
					>
						你的帳號還有商店，請先到
						<Link href="/seller-center/me" className="underline">
							商店設定
						</Link>
						關閉商店，再刪除帳號。
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
