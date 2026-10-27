"use client";

import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import {
	useInvalidateNotificationPreferences,
	useNotificationPreferences,
} from "@/lib/hooks/useNotificationPreferences";
import { usePathname } from "@/lib/intl/navigation";
import { updateNotificationPreference } from "@/lib/notifications/repo";
import type { NotificationPreferenceItem } from "@/lib/notifications/types";
import { notificationWorkspaceRole } from "@/lib/notifications/workspace";

/**
 * Buyer/seller notification preference centre.
 *
 * Backend (`/app/{buyers|sellers}/me/notification-preferences`) stores
 * one `enabled` flag per category, no per-channel split — the UI here
 * mirrors that 1:1. Toggling a row PUTs the single category immediately
 * (optimistic + rollback on error); the "儲存" button is gone because
 * there's nothing left to batch.
 */

// Fallback descriptors used until the backend's first response lands.
// Server response titles/descriptions take precedence so categories can
// be added/renamed server-side without a client redeploy.
const CATEGORY_FALLBACK: Record<
	string,
	{ title: string; description: string }
> = {
	order_update: { title: "新訂單更新", description: "訂單狀態改變時通知" },
	refund_update: { title: "退貨訂單更新", description: "退貨申請與包裹狀態" },
	system: { title: "系統公告", description: "平台規範與重要通知" },
	chat: { title: "聊天通知", description: "聊天訊息提醒" },
	coupon: { title: "優惠卷通知", description: "優惠卷發送與到期通知" },
	post: { title: "賣家貼文通知", description: "追蹤賣家發文與貼文提醒" },
	message: { title: "賣家留言通知", description: "留言提醒" },
	restock: { title: "補貨通知", description: "追蹤的售完商品補貨時通知" },
	follower: { title: "追蹤/收藏/瀏覽通知", description: "潛在商機提醒" },
	shipment_anomaly: {
		title: "配送異常通知",
		description: "出貨/配送異常即時提醒（強制接收，無法關閉）",
	},
};

/**
 * Categories the backend forces on per role (notification design doc §3.1).
 *
 * Backend now returns `forced=true` on the matching pref row (proto
 * `NotificationPreferenceItem.forced`, gateway DTO `forced` 欄位、§3.1）—
 * `pref.forced` is the primary source. This map remains as a defensive
 * fallback in case a stale notification-service build omits the field.
 *
 * Sellers cannot disable `shipment_anomaly` because a missed alert directly
 * causes a buyer-side issue. Buyer has no forced category (議題 3：
 * `shipment_anomaly` is seller-only — not in BuyerCategories).
 */
const FORCED_BY_ROLE: Record<"buyer" | "seller", ReadonlyArray<string>> = {
	buyer: [],
	seller: ["shipment_anomaly"],
};

const FALLBACK_PREFS: NotificationPreferenceItem[] = Object.entries(
	CATEGORY_FALLBACK,
).map(([category, meta]) => ({
	category,
	title: meta.title,
	description: meta.description,
	enabled: true,
}));

export default function PreferencesClient() {
	const pathname = usePathname();
	if (!pathname) return null;
	return <PreferencesContent role={notificationWorkspaceRole(pathname)} />;
}

function PreferencesContent({
	role,
}: {
	role: ReturnType<typeof notificationWorkspaceRole>;
}) {
	const [prefs, setPrefs] = React.useState<NotificationPreferenceItem[]>([]);
	const [pendingCategory, setPendingCategory] = React.useState<string | null>(
		null,
	);
	const [error, setError] = React.useState<string | null>(null);
	const [savedAt, setSavedAt] = React.useState<number | null>(null);

	const {
		prefs: remotePrefs,
		loading,
		error: loadErrorRaw,
	} = useNotificationPreferences(role);
	const invalidatePreferences = useInvalidateNotificationPreferences();
	const loadError = loadErrorRaw
		? `${loadErrorRaw}（顯示預設設定，切換後會嘗試儲存）`
		: null;
	const displayError = error ?? loadError;

	// Seed the editable list from the shared read ONCE per role (per mount). We
	// own the optimistic toggle locally, so we must NOT re-seed when the shared
	// query refetches — the post-toggle invalidate (or a cross-toggle refetch)
	// would otherwise clobber the just-flipped value or, on a stale GET-after-PUT,
	// revert it. The cache stays the source of truth for the NEXT mount; this
	// mount keeps its optimistic state. Empty/failed read → fallback rows (first
	// toggle creates the row server-side).
	const seededRoleRef = React.useRef<string | null>(null);
	React.useEffect(() => {
		if (loading || seededRoleRef.current === role) return;
		seededRoleRef.current = role;
		setPrefs(
			remotePrefs && remotePrefs.length > 0 ? remotePrefs : FALLBACK_PREFS,
		);
	}, [remotePrefs, loading, role]);

	// `shipment_anomaly` (and any future forced category) is locked-on for
	// the matching role — we treat the server flag as the truth, then fall
	// back to the role-keyed list so the lock still shows on backends that
	// haven't shipped the `forced` field yet.
	const forcedSet = React.useMemo(() => {
		return new Set<string>(FORCED_BY_ROLE[role]);
	}, [role]);

	const isForced = React.useCallback(
		(p: NotificationPreferenceItem) =>
			Boolean(p.forced) || forcedSet.has(p.category),
		[forcedSet],
	);

	// Optimistic toggle: flip the UI immediately, PUT to backend, roll back
	// on failure. One-flag-per-row matches the openapi PUT body shape
	// `{ category, enabled }` — no bulk save needed.
	const handleToggle = async (category: string, nextEnabled: boolean) => {
		// Defensive — the input is disabled for forced rows, but a
		// click that races a re-render could still call this; bail out
		// before hitting the API.
		const target = prefs.find((p) => p.category === category);
		if (target && isForced(target)) return;

		setPrefs((prev) =>
			prev.map((p) =>
				p.category === category ? { ...p, enabled: nextEnabled } : p,
			),
		);
		setPendingCategory(category);
		setError(null);
		setSavedAt(null);
		try {
			await updateNotificationPreference(category, nextEnabled, { role });
			setSavedAt(Date.now());
			// Sync the shared cache so a remount (or the notifications list) sees
			// the new value instead of the 30s-stale pre-toggle list.
			void invalidatePreferences(role);
		} catch (e) {
			setPrefs((prev) =>
				prev.map((p) =>
					p.category === category ? { ...p, enabled: !nextEnabled } : p,
				),
			);
			setError(e instanceof Error ? e.message : "儲存失敗");
		} finally {
			setPendingCategory(null);
		}
	};

	const backHref =
		role === "seller"
			? "/seller-center/notifications"
			: "/account/notifications";

	const labelFor = (p: NotificationPreferenceItem) => {
		const fb = CATEGORY_FALLBACK[p.category];
		return {
			title: p.title || fb?.title || p.category,
			description: p.description || fb?.description || "",
		};
	};

	// Backend 已實作 forced 欄位 (proto / domain.IsForcedCategory)，正常情況下
	// shipment_anomaly 會以 forced=true 出現於賣家 GET 回應中。此處的 synthesise
	// 邏輯保留為防禦性 fallback：若 notification-service 為舊版本（缺 forced 欄位
	// 或忘了把 shipment_anomaly 加進 SellerCategories），UI 仍會顯示鎖定 row。
	// Buyer 不 synthesise（議題 3：shipment_anomaly 為賣家專屬，從 BuyerCategories 移除）。
	const displayedPrefs = React.useMemo<NotificationPreferenceItem[]>(() => {
		const seen = new Set(prefs.map((p) => p.category));
		const extras: NotificationPreferenceItem[] = [];
		for (const category of forcedSet) {
			if (seen.has(category)) continue;
			const fb = CATEGORY_FALLBACK[category];
			extras.push({
				category,
				title: fb?.title,
				description: fb?.description,
				enabled: true,
				forced: true,
			});
		}
		return [...prefs, ...extras];
	}, [prefs, forcedSet]);

	return (
		<main className="bg-transparent">
			<SellerPageShell
				title="通知設定"
				subtitle={`選擇要接收哪些通知。設定僅套用於目前的「${role === "seller" ? "賣家" : "買家"}」身份。`}
				actions={
					<Button href={backHref} variant="bordered" color="default" size="sm">
						<AppIcon name="arrow-left" size={16} />
						回通知
					</Button>
				}
			>
				{loading ? (
					<p className="text-[length:var(--hkui-text-small)] text-[var(--hkui-foreground-soft)]">
						載入中…
					</p>
				) : (
					<>
						<ul className="list-none m-0 p-0 flex flex-col gap-[var(--hkui-space-2)]">
							{displayedPrefs.map((p) => {
								const meta = labelFor(p);
								const isPending = pendingCategory === p.category;
								const forced = isForced(p);
								// Forced rows: backend keeps enabled=true (or returns it
								// missing), but the UI always shows "已開啟" and the toggle
								// is disabled so the seller can't even attempt to flip it.
								const effectiveEnabled = forced ? true : p.enabled;
								return (
									<li
										key={p.category}
										className="flex items-center justify-between gap-[var(--hkui-space-4)] p-[var(--hkui-space-4)_var(--hkui-space-5)] bg-[var(--hkui-content1)] border border-[var(--hkui-divider)] rounded-[var(--hkui-radius-lg)]"
									>
										<div className="flex flex-col gap-[2px]">
											<span className="text-[length:var(--hkui-text-medium)] font-semibold text-[var(--hkui-foreground)]">
												{meta.title}
											</span>
											{meta.description ? (
												<span className="text-[length:var(--hkui-text-small)] text-[var(--hkui-foreground-soft)]">
													{meta.description}
												</span>
											) : null}
											{forced ? (
												<span className="text-[length:var(--hkui-text-tiny)] text-[var(--hkui-foreground-muted,var(--hkui-foreground-soft))]">
													強制接收，無法關閉
												</span>
											) : null}
										</div>
										<label
											className={`inline-flex min-h-11 items-center gap-[var(--hkui-space-3)] select-none shrink-0 transition-opacity duration-200 ${
												isPending
													? "opacity-60 pointer-events-none cursor-pointer"
													: forced
														? "cursor-not-allowed"
														: "cursor-pointer"
											}`}
										>
											<input
												type="checkbox"
												className="absolute opacity-0 pointer-events-none w-px h-px"
												checked={effectiveEnabled}
												disabled={isPending || forced}
												onChange={(e) =>
													void handleToggle(p.category, e.target.checked)
												}
											/>
											<div
												className={`relative w-11 h-[26px] rounded-full transition-colors duration-[var(--hkui-dur-base)] ease-[var(--hkui-ease)] ${effectiveEnabled ? "bg-[var(--hkui-primary)]" : "bg-[var(--hkui-default-200,rgb(var(--color-ink-950-rgb)/0.18))]"}`}
												aria-hidden="true"
											>
												<span
													className={`absolute top-[3px] left-[3px] w-5 h-5 rounded-full bg-white shadow-[0_1px_3px_rgba(0,0,0,0.18)] transition-transform duration-[var(--hkui-dur-base)] ease-[var(--hkui-ease)] ${effectiveEnabled ? "translate-x-[18px]" : ""}`}
												/>
											</div>
											<span
												className={`text-[length:var(--hkui-text-tiny)] min-w-[3.5em] transition-colors duration-[var(--hkui-dur-base)] ease-[var(--hkui-ease)] ${effectiveEnabled ? "text-[var(--hkui-primary)] font-medium" : "text-[var(--hkui-foreground-soft)]"}`}
											>
												{forced
													? "強制開啟"
													: effectiveEnabled
														? "已開啟"
														: "已關閉"}
											</span>
										</label>
									</li>
								);
							})}
						</ul>

						{displayError && (
							<p
								className="my-[var(--hkui-space-4)] p-[var(--hkui-space-3)_var(--hkui-space-4)] rounded-[var(--hkui-radius-md)] bg-[var(--hkui-danger-50)] text-[var(--hkui-danger-700)] text-[length:var(--hkui-text-small)]"
								role="alert"
							>
								{displayError}
							</p>
						)}

						{savedAt && !displayError && (
							<p
								className="mt-[var(--hkui-space-3)] text-[length:var(--hkui-text-tiny)] text-[var(--hkui-primary)] text-right"
								role="status"
							>
								已儲存
							</p>
						)}
					</>
				)}
			</SellerPageShell>
		</main>
	);
}
