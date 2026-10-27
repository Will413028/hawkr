"use client";

import type { Gender } from "@hawkr/api/gen/hawkr/buyer/v1alpha1/buyer_pb";
import * as React from "react";
import Avatar from "@/components/shared/Avatar";
import { SellerPageShell } from "@/components/shared/SellerPageShell";
import UploadLoadingIndicator from "@/components/shared/UploadLoadingIndicator";
import { DatePicker, Dropdown, Input } from "@/components/ui";
import Button from "@/components/ui/Button";
import { ImageCropDialog } from "@/components/ui/ImageCropDialog";
import { useProfile } from "@/lib/hooks/useAccount";
import { useUiText } from "@/lib/i18n/ui-text";
import { uploadImageFile } from "@/lib/media/upload";
import { updateBuyerAvatar } from "@/lib/profile/repo";
import { genderFromValue, genderOptions } from "./genderOptions";

export default function ProfileClient() {
	const { profile, loading, saving, error, refresh, save } = useProfile();
	const [nickname, setNickname] = React.useState("");
	const ut = useUiText();
	const options = React.useMemo(() => genderOptions(ut), [ut]);
	const [gender, setGender] = React.useState<Gender | undefined>(undefined);
	const [birthday, setBirthday] = React.useState("");
	const [hint, setHint] = React.useState<string | null>(null);
	const [avatarBusy, setAvatarBusy] = React.useState(false);
	const [avatarError, setAvatarError] = React.useState<string | null>(null);
	const [pendingAvatar, setPendingAvatar] = React.useState<File | null>(null);
	const avatarInputRef = React.useRef<HTMLInputElement | null>(null);

	React.useEffect(() => {
		if (!profile) return;
		setNickname(profile.nickname ?? "");
		setGender(profile.gender);
		setBirthday(profile.birthday ?? "");
	}, [profile]);

	const handlePickAvatar = () => {
		avatarInputRef.current?.click();
	};

	const handleAvatarChange = (e: React.ChangeEvent<HTMLInputElement>) => {
		const file = e.target.files?.[0];
		e.target.value = "";
		if (!file || avatarBusy) return;
		// Crop first (circular framing), upload on confirm.
		setPendingAvatar(file);
	};

	const uploadAvatar = async (file: File) => {
		setAvatarError(null);
		setAvatarBusy(true);
		try {
			// Finalize via the dedicated endpoint so we store a permanent
			// objectKey, not the temporary upload preview URL.
			const asset = await uploadImageFile(file);
			await updateBuyerAvatar(asset.stagedToken, profile ?? {});
			await refresh();
			setHint("頭像已更新");
		} catch (err) {
			setAvatarError(err instanceof Error ? err.message : "頭像更新失敗");
		} finally {
			setAvatarBusy(false);
		}
	};

	const handleSubmit = async (e: React.FormEvent) => {
		e.preventDefault();
		setHint(null);
		try {
			await save({
				nickname: nickname.trim(),
				gender,
				birthDate: birthday,
			});
			setHint("已儲存");
		} catch {
			setHint(null);
		}
	};

	if (loading && !profile) {
		return (
			<main className="profile-page">
				<SellerPageShell title="個人資料">
					<p className="text-foreground-soft text-sm">載入中…</p>
				</SellerPageShell>
			</main>
		);
	}

	return (
		<main className="profile-page">
			<SellerPageShell
				title="個人資料"
				subtitle="管理你的暱稱、生日與聯絡資訊。"
			>
				<form className="flex flex-col gap-5" onSubmit={handleSubmit}>
					<section className="seller-card">
						<div className="flex items-center gap-4">
							<Button
								type="button"
								onClick={handlePickAvatar}
								disabled={avatarBusy}
								isLoading={avatarBusy}
								loadingIndicator={<UploadLoadingIndicator size={20} />}
								loadingLabel="上傳頭像中…"
								isIconOnly
								radius="full"
								aria-label={avatarBusy ? "上傳頭像中…" : "更換頭像"}
								className="group relative size-18 shrink-0 rounded-full disabled:cursor-default"
							>
								<Avatar
									src={profile?.avatarKey}
									alt={profile?.nickname || "頭像"}
									entity="buyer"
									className="size-18 rounded-full bg-content2 object-cover"
									fallback="empty"
								/>
								<span
									className="pointer-events-none absolute inset-0 flex items-center justify-center rounded-full bg-black/45 text-white opacity-0 transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100 data-[busy=true]:opacity-100"
									data-busy={avatarBusy || undefined}
								>
									{avatarBusy ? (
										<span className="text-xs">上傳中</span>
									) : (
										<svg
											width="20"
											height="20"
											viewBox="0 0 24 24"
											fill="none"
											stroke="currentColor"
											strokeWidth="1.8"
											strokeLinecap="round"
											strokeLinejoin="round"
											aria-hidden="true"
										>
											<path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z" />
											<circle cx="12" cy="13" r="4" />
										</svg>
									)}
								</span>
							</Button>
							<input
								ref={avatarInputRef}
								type="file"
								accept="image/*"
								className="hidden"
								onChange={handleAvatarChange}
							/>
							<ImageCropDialog
								files={pendingAvatar ? [pendingAvatar] : []}
								crop="avatar"
								onCropped={(file) => void uploadAvatar(file)}
								onClose={() => setPendingAvatar(null)}
							/>
							<div className="flex min-w-0 flex-col gap-1">
								<strong className="text-lg font-bold text-foreground">
									{profile?.nickname || "—"}
								</strong>
								{profile?.phone && (
									<small className="text-sm text-foreground-soft truncate">
										{profile.phone}
									</small>
								)}
								{profile?.email && (
									<small className="text-sm text-foreground-soft truncate">
										{profile.email}
									</small>
								)}
							</div>
						</div>
					</section>

					<section className="seller-card">
						<header className="seller-card__head">
							<h2>基本資料</h2>
						</header>
						<div className="grid grid-cols-2 gap-x-5 gap-y-4 max-[720px]:grid-cols-1 [&>*:first-child]:col-span-full">
							<Input
								floatingLabel
								variant="bordered"
								size="lg"
								label="暱稱"
								value={nickname}
								onChange={(e) => setNickname(e.target.value)}
								maxLength={20}
							/>
							<Dropdown
								floatingLabel
								variant="bordered"
								size="lg"
								label="性別"
								placeholder={options[0].label}
								value={gender === undefined ? "" : String(gender)}
								onChange={(v) => setGender(genderFromValue(v))}
								items={options}
							/>
							<DatePicker
								floatingLabel
								variant="bordered"
								size="lg"
								label="生日"
								value={birthday}
								onChange={setBirthday}
								maxDate={new Date().toISOString().slice(0, 10)}
							/>
						</div>
					</section>

					{(error || avatarError) && (
						<p className="text-danger-700 text-sm font-semibold" role="alert">
							{error || avatarError}
						</p>
					)}
					{hint && !error && (
						<p className="text-success-700 text-sm font-semibold" role="status">
							{hint}
						</p>
					)}

					<div className="flex justify-end">
						<Button
							type="submit"
							disabled={saving}
							isLoading={saving}
							loadingLabel="儲存資料中"
						>
							儲存個人資料
						</Button>
					</div>
				</form>
			</SellerPageShell>
		</main>
	);
}
