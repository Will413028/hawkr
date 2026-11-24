"use client";

import { createConnectQueryKey, useMutation } from "@connectrpc/connect-query";
import { NotificationAdminService } from "@hawkr/api/gen/hawkr/admin/notification/v1alpha1/notification_pb";
import { useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useRef, useState } from "react";
import { Button, Dialog, useToast } from "@/components/UI";
import {
	type AnnouncementAttempt,
	type AnnouncementForm,
	AUDIENCE_OPTIONS,
	announcementAttempt,
	announcementFormError,
	audienceText,
	createAnnouncementRequest,
	emptyAnnouncementForm,
	toggleAudience,
} from "@/lib/announcement";
import { runAction } from "@/lib/connect-action";

const CREATE_FAILED_TEXT = "發送失敗，請稍後再試。";

/**
 * 發送系統公告。公告一律只寫英文，所有使用者看到同一份。
 * 內容不變的失敗重送沿用同一個 `request_id`，伺服器以它去重，不會建出兩則；內容改過就換新的
 * （同一個 request_id 配不同內容會被伺服器拒絕）；成功後對話框關閉，再開是新的。
 */
export default function CreateAnnouncementDialog({
	onClose,
}: {
	onClose: () => void;
}) {
	const { showToast } = useToast();
	const queryClient = useQueryClient();
	const create = useMutation(
		NotificationAdminService.method.createAnnouncement,
	);
	const attemptRef = useRef<AnnouncementAttempt | null>(null);
	const [form, setForm] = useState<AnnouncementForm>(emptyAnnouncementForm);
	const [error, setError] = useState<string | null>(null);
	const [submitting, setSubmitting] = useState(false);

	const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
		event.preventDefault();
		if (submitting) return;
		const invalid = announcementFormError(form);
		if (invalid) {
			setError(invalid);
			return;
		}
		setError(null);
		const attempt = announcementAttempt(form, attemptRef.current, () =>
			crypto.randomUUID(),
		);
		attemptRef.current = attempt;
		setSubmitting(true);
		try {
			const result = await runAction(
				() =>
					create.mutateAsync(
						createAnnouncementRequest(form, attempt.requestId),
					),
				{ fallback: CREATE_FAILED_TEXT, conflictText: CREATE_FAILED_TEXT },
			);
			if (!result.ok) {
				setError(result.text);
				return;
			}
			void queryClient.invalidateQueries({
				queryKey: createConnectQueryKey({
					schema: NotificationAdminService.method.listAnnouncements,
					cardinality: undefined,
				}),
			});
			showToast("公告已建立，正在派送給使用者", "success");
			onClose();
		} finally {
			setSubmitting(false);
		}
	};

	return (
		<Dialog
			title="發送系統公告"
			closeLabel="關閉發送系統公告視窗"
			maxWidth={500}
			onClose={onClose}
			busy={submitting}
			form={{ onSubmit: handleSubmit }}
			footer={
				<>
					<Button
						variant="outline"
						type="button"
						onClick={onClose}
						disabled={submitting}
					>
						取消
					</Button>
					<Button variant="primary" type="submit" loading={submitting}>
						{submitting ? "發送中…" : "確認發送"}
					</Button>
				</>
			}
		>
			<div className="form-row">
				<label htmlFor="announcement-title">公告標題（英文）*</label>
				<input
					id="announcement-title"
					type="text"
					className="form-input"
					value={form.title}
					onChange={(e) => {
						setForm((f) => ({ ...f, title: e.target.value }));
						setError(null);
					}}
					placeholder="e.g. Scheduled maintenance"
					disabled={submitting}
				/>
			</div>
			<div className="form-row">
				<label htmlFor="announcement-body">公告內容（英文）*</label>
				<textarea
					id="announcement-body"
					className="form-input"
					rows={4}
					value={form.body}
					onChange={(e) => {
						setForm((f) => ({ ...f, body: e.target.value }));
						setError(null);
					}}
					placeholder="e.g. Hawkr will be unavailable on Sunday from 2am to 3am."
					style={{ fontFamily: "inherit" }}
					disabled={submitting}
				/>
			</div>
			<fieldset className="form-row" disabled={submitting}>
				<legend>發送對象</legend>
				{AUDIENCE_OPTIONS.map((audience) => (
					<label key={audience} style={{ display: "block" }}>
						<input
							type="checkbox"
							checked={form.audiences.includes(audience)}
							onChange={() => {
								setForm((f) => ({
									...f,
									audiences: toggleAudience(f.audiences, audience),
								}));
								setError(null);
							}}
						/>{" "}
						{audienceText(audience)}
					</label>
				))}
			</fieldset>
			<p className="cell-sub">
				公告只寫英文，所有使用者看到同一份；沒有裝置 token
				的使用者也會收到收件匣通知。
			</p>
			{error ? (
				<p role="alert" style={{ color: "var(--admin-danger)" }}>
					{error}
				</p>
			) : null}
		</Dialog>
	);
}
