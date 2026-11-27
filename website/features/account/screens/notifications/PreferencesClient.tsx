"use client";

import { errorText } from "@hawkr/api/connect/errors";
import type {
	NotificationCategory,
	NotificationChannel,
} from "@hawkr/api/gen/hawkr/notification/v1alpha1/notification_pb";
import {
	commonUnknown,
	notificationNotificationCategory,
	notificationNotificationChannel,
} from "@hawkr/ui-text/keys";
import * as React from "react";
import AppIcon from "@/components/shared/AppIcon";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import Button from "@/components/ui/Button";
import {
	useInvalidateNotificationPreferences,
	useNotificationPreferences,
} from "@/lib/hooks/useNotificationPreferences";
import {
	optionalText,
	type UiText,
	useDefaultErrorText,
	useUiText,
} from "@/lib/i18n/ui-text";
import { usePathname } from "@/lib/intl/navigation";
import {
	audienceForRole,
	type PreferenceEntry,
	updateNotificationPreference,
} from "@/lib/notifications/inbox";
import { notificationWorkspaceRole } from "@/lib/notifications/workspace";

/**
 * Buyer/seller notification preference centre.
 *
 * The server returns one entry per (category, channel) it actually delivers
 * for the audience, and whether the user may turn it off (`canDisable`). The
 * page shows one row per category with a switch for each channel returned for
 * that category; entries that cannot be disabled are locked on. Toggling a
 * switch calls `UpdateNotificationPreference` immediately (optimistic +
 * rollback on error). Category and channel names come from the shared ui-text
 * catalog.
 */

const entryKey = (
	category: NotificationCategory,
	channel: NotificationChannel,
) => `${category}:${channel}`;

interface CategoryGroup {
	category: NotificationCategory;
	entries: PreferenceEntry[];
}

function groupByCategory(entries: PreferenceEntry[]): CategoryGroup[] {
	const groups: CategoryGroup[] = [];
	for (const entry of entries) {
		const group = groups.find((g) => g.category === entry.category);
		if (group) group.entries.push(entry);
		else groups.push({ category: entry.category, entries: [entry] });
	}
	return groups;
}

function nounText(ut: UiText, m: Parameters<typeof optionalText>[1]): string {
	return optionalText(ut, m) ?? ut(commonUnknown());
}

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
	const ut = useUiText();
	const defaults = useDefaultErrorText();
	const [entries, setEntries] = React.useState<PreferenceEntry[]>([]);
	const [pendingKeys, setPendingKeys] = React.useState<ReadonlySet<string>>(
		new Set(),
	);
	const [error, setError] = React.useState<string | null>(null);
	const [savedAt, setSavedAt] = React.useState<number | null>(null);

	const {
		entries: remoteEntries,
		loading,
		error: loadError,
	} = useNotificationPreferences(role);
	const invalidatePreferences = useInvalidateNotificationPreferences();
	const displayError = error ?? loadError;

	const seededRoleRef = React.useRef<string | null>(null);
	React.useEffect(() => {
		if (loading || remoteEntries == null || seededRoleRef.current === role)
			return;
		seededRoleRef.current = role;
		setEntries(remoteEntries);
	}, [remoteEntries, loading, role]);

	const setEnabled = (target: PreferenceEntry, enabled: boolean) =>
		setEntries((prev) =>
			prev.map((e) =>
				e.category === target.category && e.channel === target.channel
					? { ...e, enabled }
					: e,
			),
		);

	const handleToggle = async (
		target: PreferenceEntry,
		nextEnabled: boolean,
	) => {
		if (!target.canDisable) return;
		setEnabled(target, nextEnabled);
		const key = entryKey(target.category, target.channel);
		setPendingKeys((prev) => new Set(prev).add(key));
		setError(null);
		setSavedAt(null);
		try {
			await updateNotificationPreference({
				audience: audienceForRole(role),
				category: target.category,
				channel: target.channel,
				enabled: nextEnabled,
			});
			setSavedAt(Date.now());
			void invalidatePreferences(role);
		} catch (e) {
			setEnabled(target, !nextEnabled);
			setError(errorText(e, defaults));
		} finally {
			setPendingKeys((prev) => {
				const next = new Set(prev);
				next.delete(key);
				return next;
			});
		}
	};

	const backHref =
		role === "seller"
			? "/seller-center/notifications"
			: "/account/notifications";

	const groups = React.useMemo(() => groupByCategory(entries), [entries]);

	return (
		<div className="bg-transparent">
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
							{groups.map((group) => {
								const locked = group.entries.every((e) => !e.canDisable);
								return (
									<li
										key={group.category}
										className="flex items-center justify-between gap-[var(--hkui-space-4)] p-[var(--hkui-space-4)_var(--hkui-space-5)] bg-[var(--hkui-content1)] border border-[var(--hkui-divider)] rounded-[var(--hkui-radius-lg)]"
									>
										<div className="flex flex-col gap-[2px]">
											<span className="text-[length:var(--hkui-text-medium)] font-semibold text-[var(--hkui-foreground)]">
												{nounText(
													ut,
													notificationNotificationCategory(group.category),
												)}
											</span>
											{locked ? (
												<span className="text-[length:var(--hkui-text-tiny)] text-[var(--hkui-foreground-muted,var(--hkui-foreground-soft))]">
													強制接收，無法關閉
												</span>
											) : null}
										</div>
										<div className="flex flex-col items-end gap-[var(--hkui-space-1)] shrink-0">
											{group.entries.map((entry) => (
												<ChannelSwitch
													key={entryKey(entry.category, entry.channel)}
													entry={entry}
													label={nounText(
														ut,
														notificationNotificationChannel(entry.channel),
													)}
													pending={pendingKeys.has(
														entryKey(entry.category, entry.channel),
													)}
													onToggle={(next) => void handleToggle(entry, next)}
												/>
											))}
										</div>
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
		</div>
	);
}

function ChannelSwitch({
	entry,
	label,
	pending,
	onToggle,
}: {
	entry: PreferenceEntry;
	label: string;
	pending: boolean;
	onToggle: (next: boolean) => void;
}) {
	const locked = !entry.canDisable;
	const effectiveEnabled = locked ? true : entry.enabled;
	return (
		<label
			className={`inline-flex min-h-11 items-center gap-[var(--hkui-space-3)] select-none transition-opacity duration-200 ${
				pending
					? "opacity-60 pointer-events-none cursor-pointer"
					: locked
						? "cursor-not-allowed"
						: "cursor-pointer"
			}`}
		>
			<input
				type="checkbox"
				className="absolute opacity-0 pointer-events-none w-px h-px"
				checked={effectiveEnabled}
				disabled={pending || locked}
				onChange={(e) => onToggle(e.target.checked)}
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
				{label}
				{locked ? <span className="block">強制開啟</span> : null}
			</span>
		</label>
	);
}
